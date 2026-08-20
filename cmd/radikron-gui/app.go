package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	goRuntime "runtime"
	"sort"
	"sync"
	"time"

	"github.com/iomz/radikron"
	"github.com/iomz/radikron/internal/config"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yyoshiki41/go-radiko"
	"github.com/yyoshiki41/radigo"
)

const (
	// assetRetryDelay is the delay before retrying when asset is nil
	assetRetryDelay = 10 * time.Second
	// defaultDirPerm is the default permission mode for directories (0755)
	defaultDirPerm = 0755
	// loggerCleanupDelay is the delay before cleaning up the logger after download starts
	loggerCleanupDelay = 10 * time.Minute
	// manualInjectionsFilePerm is the file permission mode for manual injections file (0600 = read/write for owner only)
	manualInjectionsFilePerm = 0600
	// maxDownloadRetries is the maximum number of retry attempts for failed downloads
	maxDownloadRetries = 5
	// failedDownloadCleanupThreshold is the time after which failed downloads are cleaned up
	failedDownloadCleanupThreshold = 7 * 24 * time.Hour // 7 days
	// initialRetryDelay is the initial delay before first retry
	initialRetryDelay = 1 * time.Hour
	// maxRetryDelay is the maximum delay between retries
	maxRetryDelay = 24 * time.Hour
	// maxSafeBitShift is the maximum safe value for bit shift operations (uint64 max is 63)
	maxSafeBitShift = 63
)

// manualInjection represents a manually injected program
type manualInjection struct {
	ProgramID       string     `json:"program_id"`
	StationID       string     `json:"station_id"`
	Ft              string     `json:"ft"`
	To              string     `json:"to"`
	Title           string     `json:"title"`
	RuleName        string     `json:"rule_name,omitempty"`
	RuleFolder      string     `json:"rule_folder,omitempty"`
	LastFailureTime *time.Time `json:"last_failure_time,omitempty"` // When the last download attempt failed
	RetryCount      int        `json:"retry_count,omitempty"`       // Number of retry attempts
	LastError       string     `json:"last_error,omitempty"`        // Last error message
}

// frontendEventSink isolates backend state transitions from Wails runtime delivery.
type frontendEventSink interface {
	Emit(context.Context, string, any)
}

type wailsEventSink struct{}

func (wailsEventSink) Emit(ctx context.Context, eventName string, data any) {
	runtime.EventsEmit(ctx, eventName, data)
}

// App struct represents the Wails application
type App struct {
	ctx                    context.Context
	asset                  *radikron.Asset
	client                 *radiko.Client
	config                 *config.Config
	configFile             string
	manualInjectionsFile   string
	monitoring             bool
	monitorStopping        bool
	monitorDone            chan struct{}
	monitorWg              *sync.WaitGroup
	monitorCancel          context.CancelFunc
	programSnapshots       map[string]radikron.Progs   // stationID -> programs snapshot
	manualInjections       map[string]*manualInjection // program ID -> injection data
	pendingManualDownloads map[string]string           // program ID -> program ID (for tracking downloads in progress)
	events                 frontendEventSink
	monitorLoop            func(context.Context)
	mu                     sync.RWMutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		monitorWg:              &sync.WaitGroup{},
		programSnapshots:       make(map[string]radikron.Progs),
		manualInjections:       make(map[string]*manualInjection),
		pendingManualDownloads: make(map[string]string),
		events:                 wailsEventSink{},
	}
}

func (a *App) emit(eventName string, data any) {
	if a.events != nil {
		a.events.Emit(a.ctx, eventName, data)
	}
}

// getAppConfigDir returns the application config directory
func getAppConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config dir: %w", err)
	}
	appConfigDir := filepath.Join(configDir, "Radikron")

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(appConfigDir, defaultDirPerm); err != nil {
		return "", fmt.Errorf("failed to create app config dir: %w", err)
	}

	return appConfigDir, nil
}

// createDefaultConfig creates a default configuration file
func createDefaultConfig(configPath string) (*config.Config, error) {
	// Get current area ID
	currentAreaID, err := radiko.AreaID()
	if err != nil {
		currentAreaID = radikron.DefaultArea
	}

	// Get user's Downloads directory (cross-platform)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to "radiko" if we can't get home directory
		homeDir = ""
	}
	var downloadDir string
	if homeDir != "" {
		downloadDir = filepath.Join(homeDir, "Downloads", "radiko")
	} else {
		downloadDir = "radiko"
	}

	// Create the download directory if it doesn't exist
	if err := os.MkdirAll(downloadDir, radikron.DirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create download directory: %w", err)
	}

	// Create default config
	cfg := &config.Config{
		AreaID:                    currentAreaID,
		ExtraStations:             []string{},
		IgnoreStations:            []string{},
		FileFormat:                radigo.AudioFormatAAC,
		MinimumOutputSize:         radikron.DefaultMinimumOutputSize * radikron.Kilobytes * radikron.Kilobytes,
		DownloadDir:               downloadDir,
		Rules:                     radikron.Rules{},
		MaxDownloadingConcurrency: radikron.MaxDownloadingConcurrency,
		MaxEncodingConcurrency:    radikron.MaxEncodingConcurrency,
	}

	// Save the default config
	if err := cfg.SaveConfig(configPath); err != nil {
		return nil, fmt.Errorf("failed to save default config: %w", err)
	}

	return cfg, nil
}

// OnStartup is called when the app starts
func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx

	// Get app config directory
	appConfigDir, err := getAppConfigDir()
	if err != nil {
		runtime.LogError(ctx, fmt.Sprintf("Failed to get app config directory: %v", err))
		// Fallback to current directory
		a.configFile = "config.yml"
		a.manualInjectionsFile = "manual-injections.json"
	} else {
		// Use config.yml in app config directory
		a.configFile = filepath.Join(appConfigDir, "config.yml")
		a.manualInjectionsFile = filepath.Join(appConfigDir, "manual-injections.json")
	}

	// Initialize radiko client
	client, err := radiko.New("")
	if err != nil {
		runtime.LogError(ctx, fmt.Sprintf("Failed to create radiko client: %v", err))
		return
	}
	a.client = client

	// Create initial asset
	asset, err := radikron.NewAsset(client)
	if err != nil {
		runtime.LogError(ctx, fmt.Sprintf("Failed to create asset: %v", err))
		return
	}
	a.asset = asset

	// Check if config file exists before attempting to load
	_, err = os.Stat(a.configFile)
	configExists := err == nil
	configNotFound := err != nil && os.IsNotExist(err)

	var cfg *config.Config
	if configNotFound {
		// Config file doesn't exist, create default config
		runtime.LogInfo(ctx, fmt.Sprintf("Config file not found at %s, creating default config", a.configFile))
		cfg, err = createDefaultConfig(a.configFile)
		if err != nil {
			runtime.LogError(ctx, fmt.Sprintf("Failed to create default config: %v", err))
			// Continue with default values from asset
			return
		}
		runtime.LogInfo(ctx, fmt.Sprintf("Created default config at %s", a.configFile))
	} else if configExists {
		// Config file exists, try to load it
		cfg, err = config.LoadConfig(a.configFile)
		if err != nil {
			// File exists but failed to load (e.g., malformed YAML, permission issues)
			runtime.LogError(ctx, fmt.Sprintf("Failed to load config file at %s: %v. Continuing with default values.", a.configFile, err))
			// Continue with default values from asset, don't overwrite the existing file
			return
		}
	} else {
		// Error checking file existence (not IsNotExist)
		runtime.LogError(ctx, fmt.Sprintf("Failed to check config file at %s: %v. Continuing with default values.", a.configFile, err))
		// Continue with default values from asset
		return
	}

	// Apply config to asset
	if err := cfg.ApplyToAsset(a.asset); err != nil {
		runtime.LogError(ctx, fmt.Sprintf("Failed to apply config to asset: %v", err))
		// Continue with default values
		return
	}

	a.config = cfg
	runtime.LogInfo(ctx, fmt.Sprintf("Config loaded successfully from %s", a.configFile))

	// Load manually injected programs
	if err := a.loadManualInjections(); err != nil {
		runtime.LogError(ctx, fmt.Sprintf("Failed to load manual injections: %v", err))
		// Continue without manual injections
	}
}

// fetchProgramDetailsForStations fetches weekly programs for given stations and returns a map of program IDs to full program details
func (a *App) fetchProgramDetailsForStations(stationIDs []string) map[string]*radikron.Prog {
	programDetailsMap := make(map[string]*radikron.Prog)
	fetcher := &radikronProgramFetcher{}

	for _, stationID := range stationIDs {
		weeklyPrograms, err := fetcher.FetchWeeklyPrograms(stationID)
		if err != nil {
			runtime.LogError(a.ctx, fmt.Sprintf(
				"Failed to fetch weekly programs for station %s to update manual injection details: %v",
				stationID, err))
			continue
		}

		for _, p := range weeklyPrograms {
			programDetailsMap[p.ID] = p
		}
	}

	return programDetailsMap
}

// updateProgWithDetails updates a program with full details if available
func updateProgWithDetails(prog *radikron.Prog, programDetailsMap map[string]*radikron.Prog) {
	if fullProg, exists := programDetailsMap[prog.ID]; exists {
		prog.Desc = fullProg.Desc
		prog.Info = fullProg.Info
		prog.Pfm = fullProg.Pfm
		prog.Tags = fullProg.Tags
		prog.Genre = fullProg.Genre
		prog.M3U8 = fullProg.M3U8
	}
}

// calculateRetryDelay calculates the delay before the next retry attempt using exponential backoff
func calculateRetryDelay(retryCount int) time.Duration {
	// Clamp retryCount to prevent overflow
	// maxDownloadRetries is 5, so max retryCount is 4, making max multiplier 16 (safe)
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount > maxSafeBitShift {
		retryCount = maxSafeBitShift
	}
	// Use multiplication instead of bit shift to avoid int->uint conversion warning
	// This calculates 2^retryCount: 1, 2, 4, 8, 16, 32, ...
	multiplier := 1
	for i := 0; i < retryCount; i++ {
		multiplier *= 2
	}
	delay := initialRetryDelay * time.Duration(multiplier) // Exponential backoff: 1h, 2h, 4h, 8h, 16h
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	return delay
}

// downloadProgramImmediately sets up and starts download for a program immediately
// It handles context setup, logger configuration, and error reporting
func (a *App) downloadProgramImmediately(prog *radikron.Prog, errorMsgPrefix string) error {
	ctx := context.WithValue(a.ctx, radikron.ContextKey("asset"), a.asset)

	emitEvent := func(ctx context.Context, eventName string, data any) {
		runtime.EventsEmit(ctx, eventName, data)
	}
	eventEmitter, cleanupLogger := SetupLogger(a.ctx, emitEvent)
	programID := prog.ID
	eventEmitter.SetDownloadCompletedCallback(func(stationID, title, startTime string) {
		a.HandleDownloadCompletedByID(programID, stationID, title, startTime)
	})
	ctx = context.WithValue(ctx, radikron.ContextKey("eventEmitter"), eventEmitter)

	go func() {
		time.Sleep(loggerCleanupDelay)
		cleanupLogger()
	}()

	wg := &sync.WaitGroup{}
	err := radikron.Download(ctx, wg, prog)
	if err != nil {
		errMsg := fmt.Sprintf("%s [%s]%s: %v", errorMsgPrefix, prog.StationID, prog.Title, err)
		runtime.LogError(a.ctx, errMsg)
		runtime.EventsEmit(a.ctx, "log-message", map[string]any{
			"type":    "error",
			"message": errMsg,
		})

		// Track failure in manual injection
		a.mu.Lock()
		if inj, exists := a.manualInjections[programID]; exists {
			now := time.Now()
			inj.LastFailureTime = &now
			inj.RetryCount++
			inj.LastError = err.Error()
		}
		a.mu.Unlock()

		// Save failure state (outside lock to avoid deadlock)
		go func() {
			if saveErr := a.saveManualInjections(); saveErr != nil {
				runtime.LogError(a.ctx, fmt.Sprintf("Failed to save manual injection failure state: %v", saveErr))
			}
		}()

		return err
	}

	// Clear failure state on success
	a.mu.Lock()
	if inj, exists := a.manualInjections[programID]; exists {
		inj.LastFailureTime = nil
		inj.RetryCount = 0
		inj.LastError = ""
	}
	a.mu.Unlock()

	return nil
}

// setupDownloadForPastProgram sets up and starts download for a past program

// shouldRetryDownload checks if a download should be retried based on retry count and time since last failure.
// It returns true if the retry count is below the maximum and enough time has passed since the last failure.
func (a *App) shouldRetryDownload(inj *manualInjection) bool {
	if inj.RetryCount >= maxDownloadRetries {
		return false
	}
	if inj.LastFailureTime == nil {
		return true
	}
	timeSinceFailure := time.Since(*inj.LastFailureTime)
	nextRetryDelay := calculateRetryDelay(inj.RetryCount - 1)
	return timeSinceFailure >= nextRetryDelay
}

// findProgramInSchedules finds a program in schedules by ID.
func (a *App) findProgramInSchedules(programID string) *radikron.Prog {
	for _, prog := range a.asset.Schedules {
		if prog.ID == programID {
			return prog
		}
	}
	return nil
}

// isPastProgram checks if a program is in the past based on its start time.
func isPastProgram(prog *radikron.Prog) bool {
	startTime, err := time.ParseInLocation(radikron.DatetimeLayout, prog.Ft, radikron.Location)
	if err != nil {
		return false
	}
	return !startTime.After(radikron.CurrentTime)
}

// collectNewInjectionForDownload collects a new injection (no failure time) if it's a past program.
func (a *App) collectNewInjectionForDownload(
	id string,
	inj *manualInjection,
	retryList *[]*radikron.Prog,
	injectionMap map[string]*manualInjection,
) {
	foundProg := a.findProgramInSchedules(id)
	if foundProg == nil {
		return
	}

	if !isPastProgram(foundProg) {
		return
	}

	// Past program - add to download list
	progCopy := *foundProg // copy to decouple from shared slice
	*retryList = append(*retryList, &progCopy)
	injectionMap[id] = inj
}

// collectFailedInjectionForRetry collects a failed injection if it's ready for retry.
func (a *App) collectFailedInjectionForRetry(
	id string,
	inj *manualInjection,
	retryList *[]*radikron.Prog,
	injectionMap map[string]*manualInjection,
) {
	if !a.shouldRetryDownload(inj) {
		return
	}

	foundProg := a.findProgramInSchedules(id)
	if foundProg == nil {
		return
	}

	progCopy := *foundProg // copy to decouple from shared slice
	*retryList = append(*retryList, &progCopy)
	injectionMap[id] = inj
}

// retryFailedDownloads retries downloads for manual injections that have failed and are ready for retry.
// It also handles new injections (those that haven't been tried yet) for past programs.
// It checks each manual injection and attempts to download programs that meet the retry criteria.
func (a *App) retryFailedDownloads() {
	a.mu.RLock()
	// Collect programs that need download
	retryList := make([]*radikron.Prog, 0)
	injectionMap := make(map[string]*manualInjection)

	for id, inj := range a.manualInjections {
		if inj.LastFailureTime == nil {
			a.collectNewInjectionForDownload(id, inj, &retryList, injectionMap)
		} else {
			a.collectFailedInjectionForRetry(id, inj, &retryList, injectionMap)
		}
	}
	a.mu.RUnlock()

	// Download outside the lock
	for _, prog := range retryList {
		inj := injectionMap[prog.ID]
		if inj == nil {
			continue
		}

		if inj.LastFailureTime == nil {
			runtime.LogInfo(a.ctx, fmt.Sprintf(
				"Downloading past program [%s]%s",
				prog.StationID, prog.Title))
		} else {
			runtime.LogInfo(a.ctx, fmt.Sprintf(
				"Retrying download for [%s]%s (attempt %d/%d)",
				prog.StationID, prog.Title, inj.RetryCount+1, maxDownloadRetries))
		}

		// Download the program
		err := a.setupDownloadForPastProgram(prog)
		if err != nil {
			runtime.LogInfo(a.ctx, fmt.Sprintf(
				"Download failed for [%s]%s: %v (will retry again later)",
				prog.StationID, prog.Title, err))
		}
	}
}

// cleanupStaleFailedDownloads removes manual injections that have exceeded max retries or time threshold.
// It removes entries that have failed more than maxDownloadRetries times or have been failing for longer
// than failedDownloadCleanupThreshold.
func (a *App) cleanupStaleFailedDownloads() {
	a.mu.Lock()
	defer a.mu.Unlock()

	removed := false

	for id, inj := range a.manualInjections {
		// Only clean up entries that have failed at least once
		if inj.LastFailureTime == nil {
			continue
		}

		shouldRemove := false
		reason := ""

		// Check if max retries exceeded
		if inj.RetryCount >= maxDownloadRetries {
			shouldRemove = true
			reason = fmt.Sprintf("exceeded max retries (%d)", maxDownloadRetries)
		} else if time.Since(*inj.LastFailureTime) > failedDownloadCleanupThreshold {
			// Check if time threshold exceeded
			shouldRemove = true
			reason = fmt.Sprintf("exceeded cleanup threshold (%v)", failedDownloadCleanupThreshold)
		}

		if shouldRemove {
			delete(a.manualInjections, id)
			delete(a.pendingManualDownloads, id)
			removed = true
			runtime.LogInfo(a.ctx, fmt.Sprintf(
				"Cleaned up stale failed manual injection [%s]%s (ID: %s) - %s",
				inj.StationID, inj.Title, id, reason))
		}
	}

	if removed {
		// Save outside the lock to avoid deadlock
		go func() {
			if err := a.saveManualInjections(); err != nil {
				runtime.LogError(a.ctx, fmt.Sprintf("Failed to save manual injections after cleanup: %v", err))
			}
		}()
	}
}

// setupDownloadForPastProgram sets up and starts download for a past program.
// It delegates to downloadProgramImmediately with an appropriate error message prefix.
func (a *App) setupDownloadForPastProgram(prog *radikron.Prog) error {
	return a.downloadProgramImmediately(prog, "Failed to download past program")
}

// processLoadedInjection processes a single loaded injection (past or future).
// For past programs, it checks if the file exists and marks them for download (actual download
// is handled by retryFailedDownloads to avoid deadlock).
// For future programs, it adds them to schedules and updates the next fetch time.
func (a *App) processLoadedInjection(inj *manualInjection, prog *radikron.Prog, programDetailsMap map[string]*radikron.Prog) {
	updateProgWithDetails(prog, programDetailsMap)

	startTime, err := time.ParseInLocation(radikron.DatetimeLayout, prog.Ft, radikron.Location)
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to parse start time for loaded program %s: %v", prog.ID, err))
		if !a.asset.Schedules.HasDuplicate(prog) {
			a.asset.Schedules = append(a.asset.Schedules, prog)
		}
		return
	}

	if !startTime.After(radikron.CurrentTime) {
		// Past program - check if file exists
		fileBaseName := fmt.Sprintf(
			"%s_%s_%s",
			startTime.In(radikron.Location).Format(radikron.OutputDatetimeLayout),
			prog.StationID,
			prog.Title,
		)

		output, err := radikron.NewOutputConfig(
			fileBaseName,
			a.asset.OutputFormat,
			a.asset.DownloadDir,
			prog.RuleFolder,
		)
		if err == nil && output.IsExist() {
			delete(a.manualInjections, inj.ProgramID)
			runtime.LogInfo(a.ctx, fmt.Sprintf("Skipping loaded program [%s]%s - file already exists", prog.StationID, prog.Title))
			return
		}

		// Add to schedules for visibility
		if !a.asset.Schedules.HasDuplicate(prog) {
			a.asset.Schedules = append(a.asset.Schedules, prog)
		}
		// Mark for pending download (actual download happens in retry loop)
		a.pendingManualDownloads[prog.ID] = prog.ID
	} else {
		// Future program
		if !a.asset.Schedules.HasDuplicate(prog) {
			a.asset.Schedules = append(a.asset.Schedules, prog)
		}
		endTime, err := time.ParseInLocation(radikron.DatetimeLayout, prog.To, radikron.Location)
		if err == nil {
			next := endTime.Add(radikron.BufferMinutes * time.Minute)
			if a.asset.NextFetchTime == nil || a.asset.NextFetchTime.After(next) {
				a.asset.NextFetchTime = &next
			}
		}
	}
}

// loadManualInjections loads manually injected programs from disk.
// It reads the manual injections file, fetches program details, and processes each injection.
// The lock is released before network I/O to avoid blocking other operations.
func (a *App) loadManualInjections() error {
	a.mu.Lock()

	if a.manualInjectionsFile == "" {
		a.mu.Unlock()
		return nil
	}

	// Check if file exists
	_, err := os.Stat(a.manualInjectionsFile)
	if os.IsNotExist(err) {
		// File doesn't exist yet, that's okay
		a.mu.Unlock()
		return nil
	}
	if err != nil {
		a.mu.Unlock()
		return fmt.Errorf("failed to stat manual injections file: %w", err)
	}

	// Read file
	data, err := os.ReadFile(a.manualInjectionsFile)
	if err != nil {
		a.mu.Unlock()
		return fmt.Errorf("failed to read manual injections file: %w", err)
	}

	// Parse JSON
	var injections []*manualInjection
	if len(data) > 0 {
		if err := json.Unmarshal(data, &injections); err != nil {
			a.mu.Unlock()
			return fmt.Errorf("failed to parse manual injections JSON: %w", err)
		}
	}

	// Group injections by station ID (no lock needed)
	stationInjectionMap := make(map[string][]*manualInjection)
	stationIDs := make([]string, 0, len(injections))
	supportedInjections := make([]*manualInjection, 0, len(injections))
	for _, inj := range injections {
		if !radikron.SupportsArchive(inj.StationID) {
			runtime.LogWarning(a.ctx, fmt.Sprintf(
				"Ignoring saved manual injection for unsupported archive station %s",
				inj.StationID))
			continue
		}
		supportedInjections = append(supportedInjections, inj)
		if _, exists := stationInjectionMap[inj.StationID]; !exists {
			stationIDs = append(stationIDs, inj.StationID)
		}
		stationInjectionMap[inj.StationID] = append(stationInjectionMap[inj.StationID], inj)
	}
	injections = supportedInjections
	a.mu.Unlock()

	// Fetch weekly programs for each station WITHOUT holding the lock
	programDetailsMap := a.fetchProgramDetailsForStations(stationIDs)

	// Re-acquire lock to update state
	a.mu.Lock()
	defer a.mu.Unlock()

	// Update current time
	radikron.CurrentTime = time.Now().In(radikron.Location)

	// Store in map and add to schedules
	for _, inj := range injections {
		a.manualInjections[inj.ProgramID] = inj

		prog := &radikron.Prog{
			ID:         inj.ProgramID,
			StationID:  inj.StationID,
			Ft:         inj.Ft,
			To:         inj.To,
			Title:      inj.Title,
			RuleName:   inj.RuleName,
			RuleFolder: inj.RuleFolder,
		}

		a.processLoadedInjection(inj, prog, programDetailsMap)
	}

	runtime.LogInfo(a.ctx, fmt.Sprintf("Loaded %d manually injected programs", len(injections)))
	return nil
}

// saveManualInjections saves manually injected programs to disk.
// It writes the manual injections map to a JSON file atomically using a temporary file.
func (a *App) saveManualInjections() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.manualInjectionsFile == "" {
		return fmt.Errorf("manual injections file path not set")
	}

	// Convert map to slice
	injections := make([]*manualInjection, 0, len(a.manualInjections))
	for _, inj := range a.manualInjections {
		injections = append(injections, inj)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(injections, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manual injections: %w", err)
	}

	// Write to file atomically
	tmpFile := a.manualInjectionsFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, manualInjectionsFilePerm); err != nil {
		return fmt.Errorf("failed to write manual injections file: %w", err)
	}

	if err := os.Rename(tmpFile, a.manualInjectionsFile); err != nil {
		return fmt.Errorf("failed to rename manual injections file: %w", err)
	}

	return nil
}

// IsManualInjection checks if a program is manually injected.
// It returns true if the program ID exists in the manual injections map.
func (a *App) IsManualInjection(programID string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	_, exists := a.manualInjections[programID]
	return exists
}

// DeleteManualInjection removes a manually injected program from schedules and persistence.
// It removes the program from schedules, pending downloads, and the manual injections map,
// then saves the updated state to disk.
func (a *App) DeleteManualInjection(programID string) error {
	a.mu.Lock()

	// Check if it's a manual injection
	if _, exists := a.manualInjections[programID]; !exists {
		a.mu.Unlock()
		return fmt.Errorf("program %s is not a manual injection", programID)
	}

	// Remove from schedules
	if a.asset != nil {
		newSchedules := make(radikron.Schedules, 0, len(a.asset.Schedules))
		for _, prog := range a.asset.Schedules {
			if prog.ID != programID {
				newSchedules = append(newSchedules, prog)
			}
		}
		a.asset.Schedules = newSchedules
	}

	// Remove from pending manual downloads
	delete(a.pendingManualDownloads, programID)

	// Remove from manual injections map
	delete(a.manualInjections, programID)

	// Release lock before saving to avoid deadlock (saveManualInjections acquires its own lock)
	a.mu.Unlock()

	// Save to disk
	if err := a.saveManualInjections(); err != nil {
		return fmt.Errorf("failed to save manual injections: %w", err)
	}

	runtime.LogInfo(a.ctx, fmt.Sprintf("Deleted manual injection: %s", programID))
	return nil
}

// OnShutdown is called when the app closes
func (a *App) OnShutdown(_ context.Context) {
	if err := a.StopMonitoring(); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to stop monitoring on shutdown: %v", err))
	}
}

// GetConfig returns the current configuration
func (a *App) GetConfig() (*config.Config, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config == nil {
		return nil, fmt.Errorf("config not loaded")
	}
	return a.config, nil
}

// GetConfigFile returns the current config file path
func (a *App) GetConfigFile() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.configFile == "" {
		return "", fmt.Errorf("config file not set")
	}

	return a.configFile, nil
}

// GetSchedules returns the current scheduled programs that haven't been downloaded yet.
// It filters out programs whose files already exist and marks manual injections.
func (a *App) GetSchedules() (radikron.Progs, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.asset == nil {
		return nil, fmt.Errorf("asset not initialized")
	}

	// Filter out programs whose files already exist
	pendingSchedules := make(radikron.Progs, 0, len(a.asset.Schedules))
	for _, prog := range a.asset.Schedules {
		if !radikron.SupportsArchive(prog.StationID) {
			continue
		}
		// Check if the file already exists
		startTime, err := time.ParseInLocation(radikron.DatetimeLayout, prog.Ft, radikron.Location)
		if err != nil {
			// Skip programs with invalid start time
			continue
		}

		fileBaseName := fmt.Sprintf(
			"%s_%s_%s",
			startTime.In(radikron.Location).Format(radikron.OutputDatetimeLayout),
			prog.StationID,
			prog.Title,
		)

		// Create output config using the exported function
		output, err := radikron.NewOutputConfig(
			fileBaseName,
			a.asset.OutputFormat,
			a.asset.DownloadDir,
			prog.RuleFolder,
		)
		if err != nil {
			// Skip programs where we can't create output config
			continue
		}

		// Only include programs whose files don't exist yet
		if !output.IsExist() {
			// Set IsManualInjection flag based on whether program is in manualInjections map
			prog.IsManualInjection = a.manualInjections[prog.ID] != nil
			pendingSchedules = append(pendingSchedules, prog)
		}
	}

	return pendingSchedules, nil
}

// GetDesignatedFolder returns the full path where a program will be saved.
// It constructs the output path based on the program metadata, rule folder, and download directory.
func (a *App) GetDesignatedFolder(prog *radikron.Prog, ruleFolder, downloadDir string) (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.asset == nil {
		return "", fmt.Errorf("asset not initialized")
	}

	// Parse start time
	startTime, err := time.ParseInLocation(radikron.DatetimeLayout, prog.Ft, radikron.Location)
	if err != nil {
		return "", fmt.Errorf("failed to parse start time: %w", err)
	}

	// Create file base name
	fileBaseName := fmt.Sprintf(
		"%s_%s_%s",
		startTime.In(radikron.Location).Format(radikron.OutputDatetimeLayout),
		prog.StationID,
		prog.Title,
	)

	// Use provided ruleFolder or empty string
	folder := ruleFolder

	// Create output config to get the full path
	output, err := radikron.NewOutputConfig(
		fileBaseName,
		a.asset.OutputFormat,
		downloadDir,
		folder,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create output config: %w", err)
	}

	// Return the directory path (not the file path)
	return output.DirFullPath, nil
}

// findMatchingRuleForInjection finds a matching rule for program injection.
// If ruleName is provided, it searches for a rule with that name.
// Otherwise, it uses silent rule matching to find the first matching rule.
func (a *App) findMatchingRuleForInjection(prog *radikron.Prog, ruleName string) *radikron.Rule {
	if ruleName != "" {
		for _, rule := range a.asset.Rules {
			if rule.Name == ruleName {
				return rule
			}
		}
	}
	return a.asset.Rules.FindMatchSilent(prog.StationID, prog)
}

// handleDuplicateCheckForInjection checks if program is duplicate and handles it.
// If the program is a duplicate and the file exists, it returns an error.
// If the program is a duplicate but the file doesn't exist, it removes the duplicate from schedules.
func (a *App) handleDuplicateCheckForInjection(prog *radikron.Prog, startTime time.Time) error {
	if !a.asset.Schedules.HasDuplicate(prog) {
		return nil
	}

	fileBaseName := fmt.Sprintf(
		"%s_%s_%s",
		startTime.In(radikron.Location).Format(radikron.OutputDatetimeLayout),
		prog.StationID,
		prog.Title,
	)

	output, err := radikron.NewOutputConfig(
		fileBaseName,
		a.asset.OutputFormat,
		a.asset.DownloadDir,
		prog.RuleFolder,
	)
	if err == nil && output.IsExist() {
		return fmt.Errorf("program is already scheduled and file exists")
	}

	// File doesn't exist - remove from schedules to allow re-injection
	for i, s := range a.asset.Schedules {
		if s.ID == prog.ID {
			a.asset.Schedules = append(a.asset.Schedules[:i], a.asset.Schedules[i+1:]...)
			break
		}
	}

	return nil
}

// downloadPastInjectedProgram sets up and downloads a past injected program.
// It delegates to downloadProgramImmediately with an appropriate error message prefix.
func (a *App) downloadPastInjectedProgram(prog *radikron.Prog) error {
	return a.downloadProgramImmediately(prog, "Failed to download past program")
}

// InjectProgram adds a program to scheduled downloads.
// For past programs, it attempts to download immediately.
// For future programs, it adds them to schedules and updates the next fetch time.
// The program is saved as a manual injection for persistence.
func (a *App) InjectProgram(prog *radikron.Prog, ruleName string) error {
	if prog == nil {
		return fmt.Errorf("program cannot be nil")
	}
	if !radikron.SupportsArchive(prog.StationID) {
		return fmt.Errorf("station %s does not support Radiko archive/timeshift downloads", prog.StationID)
	}

	a.mu.Lock()
	// Note: We release the lock before calling saveManualInjections() to avoid deadlock

	if a.asset == nil {
		a.mu.Unlock()
		return fmt.Errorf("asset not initialized")
	}

	// Update current time
	radikron.CurrentTime = time.Now().In(radikron.Location)

	// Parse start time to check if program is in the past
	startTime, err := time.ParseInLocation(radikron.DatetimeLayout, prog.Ft, radikron.Location)
	if err != nil {
		a.mu.Unlock()
		return fmt.Errorf("failed to parse start time: %w", err)
	}

	// Find matching rule
	matchedRule := a.findMatchingRuleForInjection(prog, ruleName)
	if matchedRule != nil {
		prog.RuleName = matchedRule.Name
		prog.RuleFolder = matchedRule.Folder
	} else {
		prog.RuleName = ""
		prog.RuleFolder = ""
	}

	// Check for duplicates
	if err := a.handleDuplicateCheckForInjection(prog, startTime); err != nil {
		a.mu.Unlock()
		return err
	}

	// Handle past vs future programs
	if !startTime.After(radikron.CurrentTime) {
		// Past program - prepare for download outside the lock
		// Create a copy of prog to avoid issues after releasing the lock
		progCopy := *prog
		a.mu.Unlock() // Release lock before download to avoid deadlock

		// Download outside the lock (downloadProgramImmediately will acquire its own lock)
		downloadErr := a.downloadPastInjectedProgram(&progCopy)

		// Re-acquire lock to update schedules
		a.mu.Lock()
		// Even if download fails, we add to schedules so user can see what failed
		// The failure is tracked in the manual injection for retry logic
		if !a.asset.Schedules.HasDuplicate(&progCopy) {
			a.asset.Schedules = append(a.asset.Schedules, &progCopy)
		}

		// Save as manual injection
		inj := &manualInjection{
			ProgramID:  progCopy.ID,
			StationID:  progCopy.StationID,
			Ft:         progCopy.Ft,
			To:         progCopy.To,
			Title:      progCopy.Title,
			RuleName:   progCopy.RuleName,
			RuleFolder: progCopy.RuleFolder,
		}
		a.manualInjections[progCopy.ID] = inj
		a.mu.Unlock()

		// Save outside the lock to avoid deadlock (saveManualInjections acquires its own lock)
		if err := a.saveManualInjections(); err != nil {
			runtime.LogError(a.ctx, fmt.Sprintf("Failed to save manual injection: %v", err))
			// Continue anyway - the program is in schedules
		}

		// If download failed, the failure is already tracked in downloadPastInjectedProgram
		if downloadErr != nil {
			runtime.LogInfo(a.ctx, fmt.Sprintf(
				"Past program [%s]%s download failed, will be retried automatically",
				progCopy.StationID, progCopy.Title))
		}
	} else {
		// Future program - add to schedules and update next fetch time
		a.asset.Schedules = append(a.asset.Schedules, prog)
		endTime, err := time.ParseInLocation(radikron.DatetimeLayout, prog.To, radikron.Location)
		if err == nil {
			next := endTime.Add(radikron.BufferMinutes * time.Minute)
			if a.asset.NextFetchTime == nil || a.asset.NextFetchTime.After(next) {
				a.asset.NextFetchTime = &next
			}
		}

		// Save as manual injection
		inj := &manualInjection{
			ProgramID:  prog.ID,
			StationID:  prog.StationID,
			Ft:         prog.Ft,
			To:         prog.To,
			Title:      prog.Title,
			RuleName:   prog.RuleName,
			RuleFolder: prog.RuleFolder,
		}
		a.manualInjections[prog.ID] = inj
		a.mu.Unlock() // Release lock before saving to avoid deadlock

		// Save outside the lock to avoid deadlock (saveManualInjections acquires its own lock)
		if err := a.saveManualInjections(); err != nil {
			runtime.LogError(a.ctx, fmt.Sprintf("Failed to save manual injection: %v", err))
			// Continue anyway - the program is in schedules
		}

		// Reload monitoring to recalculate next fetch timer
		// This is done by triggering a reload of the monitoring loop
		// Since we can't directly trigger it, we'll emit an event
		// The monitoring loop will pick up the change on its next iteration
		runtime.EventsEmit(a.ctx, "program-injected", map[string]any{
			"program": prog.Title,
			"station": prog.StationID,
		})
	}

	return nil
}

// findProgramIDFromPendingDownloads finds program ID from pending manual downloads.
// It searches through pending downloads and schedules to match by station, title, and start time.
func (a *App) findProgramIDFromPendingDownloads(stationID, title, startTime string) string {
	for id := range a.pendingManualDownloads {
		for _, s := range a.asset.Schedules {
			if s.ID == id && s.StationID == stationID && s.Title == title && s.Ft == startTime {
				return id
			}
		}
	}
	return ""
}

// findProgramIDFromManualInjections finds program ID from manual injections by matching metadata.
// It searches through manual injections to match by station ID, title, and start time.
func (a *App) findProgramIDFromManualInjections(stationID, title, startTime string) string {
	for id, inj := range a.manualInjections {
		if inj.StationID == stationID && inj.Title == title && inj.Ft == startTime {
			return id
		}
	}
	return ""
}

// removeManualInjectionAndSave removes a program from manual injections and saves to disk.
// It removes the program from both manualInjections and pendingManualDownloads maps,
// then saves the updated state to disk.
func (a *App) removeManualInjectionAndSave(programID, stationID, title string) {
	if programID == "" {
		return
	}

	a.mu.Lock()
	delete(a.manualInjections, programID)
	delete(a.pendingManualDownloads, programID)
	a.mu.Unlock()

	// Save outside the lock to avoid deadlock (saveManualInjections acquires its own lock)
	if err := a.saveManualInjections(); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Failed to save manual injections after download: %v", err))
	} else {
		runtime.LogInfo(a.ctx, fmt.Sprintf("Removed manually injected program [%s]%s after download completion", stationID, title))
	}
}

// HandleDownloadCompleted is called when a download completes.
// It checks if the program was manually injected and removes it if so.
// This version matches by station, title, and start time (for backward compatibility).
func (a *App) HandleDownloadCompleted(stationID, title, startTime string) {
	a.mu.Lock()

	// First check if this is a pending manual download by checking schedules
	programID := a.findProgramIDFromPendingDownloads(stationID, title, startTime)

	// If not found in pending, try matching by station, title, and start time
	if programID == "" {
		programID = a.findProgramIDFromManualInjections(stationID, title, startTime)
	}

	a.mu.Unlock()

	// Remove and save outside the lock
	a.removeManualInjectionAndSave(programID, stationID, title)
}

// HandleDownloadCompletedByID is called when a download completes with a known program ID.
// This is more reliable than matching by station/title/time as it uses the program ID directly.
func (a *App) HandleDownloadCompletedByID(programID, stationID, title, _ string) {
	a.mu.Lock()

	// Check if this program ID is in manual injections
	_, exists := a.manualInjections[programID]
	if exists {
		// Remove from manual injections and pending downloads
		delete(a.manualInjections, programID)
		delete(a.pendingManualDownloads, programID)
	}
	a.mu.Unlock()

	// Save outside the lock to avoid deadlock (saveManualInjections acquires its own lock)
	if exists {
		if err := a.saveManualInjections(); err != nil {
			runtime.LogError(a.ctx, fmt.Sprintf("Failed to save manual injections after download: %v", err))
		} else {
			runtime.LogInfo(a.ctx, fmt.Sprintf(
				"Removed manually injected program [%s]%s (ID: %s) after download completion",
				stationID, title, programID))
		}
	}
}

// checkAndCleanupManualInjections checks if manually injected programs are still available
// in the weekly program list and removes them if they're no longer available.
// It compares manual injections against program snapshots and removes entries that no longer exist.
func (a *App) checkAndCleanupManualInjections() {
	a.mu.Lock()

	if len(a.manualInjections) == 0 {
		a.mu.Unlock()
		return
	}

	// Build a set of available program IDs from snapshots
	availableProgramIDs := make(map[string]bool)
	for _, programs := range a.programSnapshots {
		for _, prog := range programs {
			availableProgramIDs[prog.ID] = true
		}
	}

	// Check each manual injection and collect removed entries for logging
	removed := false
	removedEntries := make([]struct {
		stationID string
		title     string
	}, 0)

	for id, inj := range a.manualInjections {
		// Check if program is still available
		if !availableProgramIDs[id] {
			// Program is no longer available - remove it
			delete(a.manualInjections, id)

			// Also remove from schedules
			for i, s := range a.asset.Schedules {
				if s.ID == id {
					a.asset.Schedules = append(a.asset.Schedules[:i], a.asset.Schedules[i+1:]...)
					break
				}
			}

			removed = true
			// Capture logging data while holding the lock
			removedEntries = append(removedEntries, struct {
				stationID string
				title     string
			}{inj.StationID, inj.Title})
		}
	}

	a.mu.Unlock()

	// Log and save outside the lock
	if removed {
		for _, entry := range removedEntries {
			runtime.LogInfo(a.ctx, fmt.Sprintf(
				"Removed manually injected program [%s]%s - no longer available in weekly programs",
				entry.stationID, entry.title))
		}

		if err := a.saveManualInjections(); err != nil {
			runtime.LogError(a.ctx, fmt.Sprintf("Failed to save manual injections after cleanup: %v", err))
		}
	}
}

// LoadConfig loads configuration from a file
func (a *App) LoadConfig(filename string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	cfg, err := config.LoadConfig(filename)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if asset is initialized before applying config
	if a.asset == nil {
		return fmt.Errorf("asset not initialized")
	}

	// Apply config to asset
	if err := cfg.ApplyToAsset(a.asset); err != nil {
		return fmt.Errorf("failed to apply config: %w", err)
	}

	a.config = cfg
	a.configFile = filename

	// Emit event to frontend
	a.emit("config-loaded", map[string]any{
		"success": true,
	})

	return nil
}

// UpdateConfig updates the current configuration with new values
func (a *App) UpdateConfig(newConfig *config.Config) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if newConfig == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Check if asset is initialized before applying config
	if a.asset == nil {
		return fmt.Errorf("asset not initialized")
	}

	// Apply config to asset
	if err := newConfig.ApplyToAsset(a.asset); err != nil {
		return fmt.Errorf("failed to apply config: %w", err)
	}

	a.config = newConfig

	// Emit event to frontend
	a.emit("config-updated", map[string]any{
		"success": true,
	})

	return nil
}

// SaveConfig saves the current configuration to a file
// If filename is empty, uses the current config file path
func (a *App) SaveConfig(filename string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config == nil {
		return fmt.Errorf("config not loaded")
	}

	// Use current config file if filename is not provided
	if filename == "" {
		if a.configFile == "" {
			return fmt.Errorf("config file path not set")
		}
		filename = a.configFile
	}

	// Save config to file using the config package
	if err := a.config.SaveConfig(filename); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Update config file path on success
	a.configFile = filename

	// Emit event to frontend
	a.emit("config-saved", map[string]any{
		"success": true,
		"file":    filename,
	})

	return nil
}

// ReadConfigFile reads the config file content and returns it as a string
func (a *App) ReadConfigFile() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.configFile == "" {
		return "", fmt.Errorf("config file path not set")
	}

	content, err := os.ReadFile(a.configFile)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	return string(content), nil
}

// ExportConfigFile opens a save file dialog and saves the config file to the selected location
func (a *App) ExportConfigFile() error {
	// Acquire read lock only long enough to copy configFile
	a.mu.RLock()
	configFile := a.configFile
	a.mu.RUnlock()

	if configFile == "" {
		return fmt.Errorf("config file path not set")
	}

	// Read the config file content (no lock needed for file I/O)
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Get the default filename from the config file path
	defaultFilename := filepath.Base(configFile)
	if defaultFilename == "" {
		defaultFilename = "config.yml"
	}

	// Open save file dialog (no lock needed for dialog operations)
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultFilename,
		Title:           "Save Configuration File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "YAML Files (*.yml, *.yaml)",
				Pattern:     "*.yml;*.yaml",
			},
			{
				DisplayName: "All Files (*.*)",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("save dialog canceled or failed: %w", err)
	}

	if savePath == "" {
		return fmt.Errorf("no file path selected")
	}

	// Write the content to the selected file (no lock needed for file I/O)
	if err := os.WriteFile(savePath, content, manualInjectionsFilePerm); err != nil { //nolint:gosec // path comes from native save dialog
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// SelectDirectory opens a directory picker dialog and returns the selected directory path
func (a *App) SelectDirectory() (string, error) {
	selectedPath, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Download Directory",
	})
	if err != nil {
		return "", fmt.Errorf("directory dialog canceled or failed: %w", err)
	}

	if selectedPath == "" {
		return "", fmt.Errorf("no directory selected")
	}

	return selectedPath, nil
}

// GetAvailableStations returns the list of available stations
func (a *App) GetAvailableStations() ([]string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.asset == nil {
		return nil, fmt.Errorf("asset not initialized")
	}

	return a.asset.AvailableStations, nil
}

// GetAllStations returns all stations from asset.Stations (all possible stations, not just available ones)
func (a *App) GetAllStations() ([]string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.asset == nil {
		return nil, fmt.Errorf("asset not initialized")
	}

	allStations := make([]string, 0, len(a.asset.Stations))
	for stationID := range a.asset.Stations {
		if !radikron.SupportsArchive(stationID) {
			continue
		}
		allStations = append(allStations, stationID)
	}

	// Sort for consistent ordering
	sort.Strings(allStations)

	return allStations, nil
}

// FetchProgramSnapshots fetches program snapshots from all available stations.
// This function performs network I/O without holding the lock to avoid blocking.
// The snapshots are used for program search and manual injection cleanup.
func (a *App) FetchProgramSnapshots() error {
	// Get list of stations to fetch (read lock only to get the list)
	a.mu.RLock()
	if a.asset == nil {
		a.mu.RUnlock()
		return fmt.Errorf("asset not initialized")
	}
	stations := make([]string, len(a.asset.AvailableStations))
	copy(stations, a.asset.AvailableStations)
	a.mu.RUnlock()

	// Fetch programs from all stations without holding the lock
	snapshots := make(map[string]radikron.Progs)
	for _, stationID := range stations {
		programs, err := radikron.FetchWeeklyPrograms(stationID)
		if err != nil {
			// Log error but continue with other stations
			runtime.LogError(a.ctx, fmt.Sprintf("Failed to fetch programs for station %s: %v", stationID, err))
			continue
		}
		snapshots[stationID] = programs
	}

	// Update snapshots with write lock (brief, no I/O)
	a.mu.Lock()
	a.programSnapshots = snapshots
	a.mu.Unlock()

	return nil
}

// SearchWeeklyPrograms searches weekly programs using rule criteria.
// It searches on the cached program snapshots instead of making network calls.
// Returns programs that match the specified title, personality, keyword, and station ID criteria.
func (a *App) SearchWeeklyPrograms(ruleTitle, rulePfm, ruleKeyword, ruleStationID string) (radikron.Progs, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.asset == nil {
		return nil, fmt.Errorf("asset not initialized")
	}

	// Create a temporary rule for matching
	tempRule := &radikron.Rule{
		Name:      "search",
		Title:     ruleTitle,
		Pfm:       rulePfm,
		Keyword:   ruleKeyword,
		StationID: ruleStationID,
	}

	// Get stations to search (all if no station specified, otherwise just the specified one)
	stationsToSearch := a.asset.AvailableStations
	if ruleStationID != "" {
		// Check if the station exists
		found := false
		for _, station := range a.asset.AvailableStations {
			if station == ruleStationID {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("station %s not found", ruleStationID)
		}
		stationsToSearch = []string{ruleStationID}
	}

	// Search on cached snapshots instead of making network calls
	var matchingPrograms radikron.Progs
	for _, stationID := range stationsToSearch {
		programs, exists := a.programSnapshots[stationID]
		if !exists {
			// If snapshot doesn't exist for this station, skip it
			runtime.LogError(a.ctx, fmt.Sprintf("No program snapshot available for station %s", stationID))
			continue
		}

		// Filter programs using the rule
		for _, prog := range programs {
			if tempRule.MatchSilent(stationID, prog) {
				matchingPrograms = append(matchingPrograms, prog)
			}
		}
	}

	return matchingPrograms, nil
}

// OpenDirectory opens the specified directory in the system's file browser.
// It uses platform-specific commands (open on macOS, explorer on Windows, xdg-open on Linux).
func (a *App) OpenDirectory(dirPath string) error {
	if dirPath == "" {
		return fmt.Errorf("directory path is empty")
	}

	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dirPath)
	}

	var cmd *exec.Cmd
	switch goRuntime.GOOS {
	case "darwin": // macOS
		cmd = exec.CommandContext(a.ctx, "open", dirPath)
	case "windows":
		// Use explorer with the directory path
		cmd = exec.CommandContext(a.ctx, "explorer", dirPath)
	case "linux":
		// Use xdg-open for Linux
		cmd = exec.CommandContext(a.ctx, "xdg-open", dirPath)
	default:
		return fmt.Errorf("unsupported platform: %s", goRuntime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open directory: %w", err)
	}

	return nil
}

// GetMonitoringStatus returns whether monitoring is currently active.
func (a *App) GetMonitoringStatus() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.monitoring
}

// StartMonitoring starts the monitoring loop.
// It creates a context, sets up the monitoring goroutine, and begins continuous program monitoring.
func (a *App) StartMonitoring() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.monitoring {
		return fmt.Errorf("monitoring is already running")
	}

	if a.asset == nil {
		return fmt.Errorf("asset not initialized")
	}

	// Create context for monitoring
	parentCtx := a.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	ctx = context.WithValue(ctx, radikron.ContextKey("asset"), a.asset)
	a.monitorCancel = cancel
	a.monitorDone = make(chan struct{})

	a.monitoring = true

	// Start monitoring in goroutine
	a.monitorWg.Add(1)
	go a.executeMonitoringLoop(ctx)

	// Log and emit event to frontend
	log.Printf("monitoring started")
	a.emit("monitoring-started", nil)

	return nil
}

// StopMonitoring stops the monitoring loop.
// It cancels the monitoring context and waits for the monitoring goroutine to finish.
func (a *App) StopMonitoring() error { //nolint:unparam // Keep Wails binding's established Promise<void> contract.
	a.mu.Lock()
	if !a.monitoring {
		a.mu.Unlock()
		return nil
	}
	cancel := a.monitorCancel
	done := a.monitorDone
	if a.monitorStopping {
		a.mu.Unlock()
		if done != nil {
			<-done
		}
		a.monitorWg.Wait()
		return nil
	}
	a.monitorStopping = true
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	a.monitorWg.Wait()

	a.mu.Lock()
	a.monitoring = false
	a.monitorStopping = false
	a.monitorCancel = nil
	a.monitorDone = nil
	a.mu.Unlock()

	// Log and emit event to frontend
	log.Printf("monitoring stopped")
	a.emit("monitoring-stopped", nil)

	return nil
}

func (a *App) executeMonitoringLoop(ctx context.Context) {
	defer a.monitorWg.Done()
	defer close(a.monitorDone)

	if a.monitorLoop != nil {
		a.monitorLoop(ctx)
		return
	}
	a.runMonitoringLoop(ctx)
}

// reloadConfigIfNeeded reloads and applies configuration if available.
// It loads the config file and applies it to the asset without blocking on I/O.
func (a *App) reloadConfigIfNeeded() error {
	// RLock to capture current configFile snapshot
	a.mu.RLock()
	configFile := a.configFile
	a.mu.RUnlock()

	// If no config file, don't reload
	if configFile == "" {
		return nil
	}

	// Load config using the snapshot (outside lock since it can take time)
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", configFile, err)
	}

	// Lock while mutating a.asset and a.config
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.asset == nil {
		return fmt.Errorf("asset not initialized")
	}

	// Apply config to asset while holding the lock
	if err := cfg.ApplyToAsset(a.asset); err != nil {
		return fmt.Errorf("failed to apply config to asset: %w", err)
	}

	// Update config while holding the lock
	a.config = cfg

	return nil
}

// programWithStation wraps a program with its station ID
type programWithStation struct {
	prog      *radikron.Prog
	stationID string
}

// collectProgramsFromStations collects and deduplicates programs from all stations
func (a *App) collectProgramsFromStations(asset *radikron.Asset, fetcher *radikronProgramFetcher) []*programWithStation {
	allPrograms := make(map[string]*programWithStation) // key: program ID
	snapshots := make(map[string]radikron.Progs)        // stationID -> programs for snapshot update

	// Process all stations and collect programs
	for _, stationID := range asset.AvailableStations {
		// Fetch weekly programs for all stations (needed for manual injection cleanup)
		weeklyPrograms, err := fetcher.FetchWeeklyPrograms(stationID)
		if err != nil {
			log.Printf("failed to fetch the %s program: %v", stationID, err)
			continue
		}

		// Update snapshots for cleanup check
		snapshots[stationID] = weeklyPrograms

		// Skip if no rules match this station (for processing, but we still have snapshots)
		if !asset.Rules.HasRuleWithoutStationID() && !asset.Rules.HasRuleForStationID(stationID) {
			continue
		}

		// Collect programs, keeping only the first occurrence of each program ID
		for _, p := range weeklyPrograms {
			if _, exists := allPrograms[p.ID]; !exists {
				allPrograms[p.ID] = &programWithStation{
					prog:      p,
					stationID: stationID,
				}
			}
		}
	}

	// Update program snapshots for manual injection cleanup
	a.mu.Lock()
	for stationID, programs := range snapshots {
		a.programSnapshots[stationID] = programs
	}
	a.mu.Unlock()

	// Convert map to slice for deterministic iteration order
	programList := make([]*programWithStation, 0, len(allPrograms))
	for _, pws := range allPrograms {
		programList = append(programList, pws)
	}
	return programList
}

// checkManualInjectionFileExists checks if the file for a manually injected program already exists.
// It constructs the expected file path and checks if the file exists on disk.
func (a *App) checkManualInjectionFileExists(p *radikron.Prog, asset *radikron.Asset) bool {
	startTime, err := time.ParseInLocation(radikron.DatetimeLayout, p.Ft, radikron.Location)
	if err != nil {
		return false
	}

	fileBaseName := fmt.Sprintf(
		"%s_%s_%s",
		startTime.In(radikron.Location).Format(radikron.OutputDatetimeLayout),
		p.StationID,
		p.Title,
	)
	output, err := radikron.NewOutputConfig(
		fileBaseName,
		asset.OutputFormat,
		asset.DownloadDir,
		p.RuleFolder,
	)
	return err == nil && output.IsExist()
}

// findMatchingRuleForProcessProgram finds a matching rule for a program during processing.
// For manual injections, it searches by rule name if provided.
// For regular programs, it uses silent rule matching.
func (a *App) findMatchingRuleForProcessProgram(
	p *radikron.Prog,
	stationID string,
	asset *radikron.Asset,
	isManualInjection bool,
) *radikron.Rule {
	if isManualInjection && p.RuleName != "" {
		// Find the rule by name for manual injections
		for _, r := range asset.Rules {
			if r.Name == p.RuleName {
				return r
			}
		}
		// If rule not found by name, still proceed (manual injection might not need a rule)
		return nil
	}
	// Normal rule matching
	return asset.Rules.FindMatchSilent(stationID, p)
}

// setupManualInjectionForProcess sets up rule info for manually injected programs.
// It copies rule name and folder from the manual injection to the program.
// Returns true if the program is a manual injection, false otherwise.
func (a *App) setupManualInjectionForProcess(p *radikron.Prog) bool {
	if inj, exists := a.manualInjections[p.ID]; exists {
		p.RuleName = inj.RuleName
		p.RuleFolder = inj.RuleFolder
		return true
	}
	return false
}

// checkDuplicateForProcessProgram checks if program is duplicate and handles manual injection cases.
// Note: Uses a.asset.Schedules (live state) for duplicate checking to catch duplicates within the same
// iteration, but uses asset parameter (snapshot) for file existence checks to ensure consistency.
// For manual injections, it also checks if the file already exists.
func (a *App) checkDuplicateForProcessProgram(p *radikron.Prog, asset *radikron.Asset, isManualInjection bool) bool {
	// Check against live state to catch duplicates from previous iterations and within current iteration
	if !a.asset.Schedules.HasDuplicate(p) {
		return false
	}
	if !isManualInjection {
		return true
	}
	// For manual injections, check if file exists using the asset snapshot parameter
	return a.checkManualInjectionFileExists(p, asset)
}

// applyRuleToProgram applies rule information to program if not already set by manual injection.
// It sets the rule name and folder on the program from the matched rule.
func (a *App) applyRuleToProgram(p *radikron.Prog, matchedRule *radikron.Rule, isManualInjection bool) {
	if matchedRule != nil && !isManualInjection {
		p.RuleName = matchedRule.Name
		p.RuleFolder = matchedRule.Folder
	}
}

// checkAlreadyLogged checks if program was already logged in this iteration.
// It tracks logged programs to avoid duplicate log messages.
func (a *App) checkAlreadyLogged(processedInThisIteration map[string]bool, programID string) bool {
	alreadyLogged := processedInThisIteration[programID+"_logged"]
	if !alreadyLogged {
		processedInThisIteration[programID+"_logged"] = true
	}
	return alreadyLogged
}

// processProgram handles a single program: checks duplicates, matches rules, and downloads if needed.
// Returns (matched, duplicate) to indicate if the program matched a rule or was a duplicate.
func (a *App) processProgram(
	pws *programWithStation,
	asset *radikron.Asset,
	downloadCtx context.Context,
	processedInThisIteration map[string]bool,
	downloader *radikronDownloader,
) (matched, duplicate bool) {
	p := pws.prog
	stationID := pws.stationID

	// Lock to prevent race conditions when checking duplicates
	a.mu.Lock()
	if processedInThisIteration[p.ID] {
		a.mu.Unlock()
		return false, true
	}

	// Check if this is a manually injected program
	isManualInjection := a.setupManualInjectionForProcess(p)

	// Check for duplicates
	if a.checkDuplicateForProcessProgram(p, asset, isManualInjection) {
		a.mu.Unlock()
		return false, true
	}

	processedInThisIteration[p.ID] = true
	a.mu.Unlock()

	// Check if rule matches using the asset snapshot
	matchedRule := a.findMatchingRuleForProcessProgram(p, stationID, asset, isManualInjection)
	if matchedRule == nil && !isManualInjection {
		a.mu.Lock()
		delete(processedInThisIteration, p.ID)
		a.mu.Unlock()
		return false, false
	}

	// Apply rule information if needed
	a.applyRuleToProgram(p, matchedRule, isManualInjection)

	// Check if already logged
	a.mu.Lock()
	if a.checkAlreadyLogged(processedInThisIteration, p.ID) {
		a.mu.Unlock()
		return true, false
	}
	a.mu.Unlock()

	// Ensure StationID is set correctly (it should be set from XML, but ensure it matches)
	if p.StationID == "" {
		p.StationID = stationID
	}

	// Call Download() - it will log "rule matched" and "start downloading" if it actually starts
	// Note: Download() may return nil if program is in future, duplicate, or file exists
	err := downloader.Download(downloadCtx, a.monitorWg, p)
	if err != nil {
		// Download failed - remove from processed tracking
		a.mu.Lock()
		delete(processedInThisIteration, p.ID)
		a.mu.Unlock()
		runtime.EventsEmit(a.ctx, "log-message", map[string]any{
			"type":    "error",
			"message": fmt.Sprintf("download failed for [%s]%s: %s", p.StationID, p.Title, err),
		})
		runtime.EventsEmit(a.ctx, "download-failed", map[string]any{
			"station": p.StationID,
			"title":   p.Title,
			"error":   err.Error(),
		})
		return true, false
	}
	// Download() returned nil - it either started the download or skipped it
	// Add to schedules to prevent processing it again in future iterations
	// Note: Download() checks for duplicates internally, so if it skipped due to duplicate,
	// we still add it here to ensure it's tracked (though it won't be downloaded)
	a.mu.Lock()
	if !a.asset.Schedules.HasDuplicate(p) {
		a.asset.Schedules = append(a.asset.Schedules, p)
	}
	a.mu.Unlock()
	// The download-started event will be emitted by Download() if it actually starts

	return true, false
}

// checkAndLogRulesCount checks and logs the number of configured rules.
// It warns if no rules are configured, as programs won't be downloaded without rules.
func (a *App) checkAndLogRulesCount(asset *radikron.Asset) {
	a.mu.RLock()
	rulesCount := len(asset.Rules)
	a.mu.RUnlock()
	if rulesCount == 0 {
		log.Printf("warning: no rules configured, programs won't be downloaded")
		runtime.EventsEmit(a.ctx, "log-message", map[string]any{
			"type":    "error",
			"message": "No rules configured - please configure rules to download programs",
		})
	} else {
		log.Printf("configured with %d rules", rulesCount)
	}
}

// processAllPrograms collects programs from stations and processes them.
// It collects programs from all stations, matches them against rules, and downloads matching programs.
func (a *App) processAllPrograms(
	asset *radikron.Asset,
	fetcher *radikronProgramFetcher,
	downloadCtx context.Context,
	downloader *radikronDownloader,
) {
	// Collect all programs from all stations
	programList := a.collectProgramsFromStations(asset, fetcher)
	log.Printf("collected %d programs from stations", len(programList))

	// Track programs processed in this iteration to prevent duplicates
	processedInThisIteration := make(map[string]bool)

	// Process each program
	matchedCount := 0
	duplicateCount := 0
	processedCount := 0
	for _, pws := range programList {
		matched, duplicate := a.processProgram(pws, asset, downloadCtx, processedInThisIteration, downloader)
		if matched {
			matchedCount++
		}
		if duplicate {
			duplicateCount++
		}
		processedCount++
	}
	log.Printf("processed %d programs: %d matched rules, %d duplicates", processedCount, matchedCount, duplicateCount)
}

// logAndSleepUntilNextFetch logs the next fetch time and sleeps until then.
// It reads the next fetch time from the asset and sleeps until that time or context cancellation.
func (a *App) logAndSleepUntilNextFetch(asset *radikron.Asset, ctx context.Context) {
	a.mu.RLock()
	nextFetchTime := asset.NextFetchTime
	a.mu.RUnlock()
	if nextFetchTime != nil {
		log.Printf("sleeping until next fetch time: %s", nextFetchTime.Format(time.RFC3339))
	} else {
		log.Printf("sleeping until next fetch time: (not scheduled, defaulting to 1 hour)")
	}
	a.sleepUntilNextFetch(ctx)
}

// sleepUntilNextFetch sleeps until the next fetch time or until context is canceled.
// It uses a timer that can be canceled if the context is done.
func (a *App) sleepUntilNextFetch(ctx context.Context) {
	// Acquire read lock to safely read NextFetchTime
	a.mu.RLock()
	var nextFetchTime *time.Time
	if a.asset != nil && a.asset.NextFetchTime != nil {
		// Copy the time value to avoid holding lock during sleep
		nextFetchTimeCopy := *a.asset.NextFetchTime
		nextFetchTime = &nextFetchTimeCopy
	}
	a.mu.RUnlock()

	// Use the local copy to decide whether to create a timer or sleep
	if nextFetchTime != nil {
		sleepDuration := time.Until(*nextFetchTime)
		if sleepDuration > 0 {
			timer := time.NewTimer(sleepDuration)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
		}
	} else {
		// Default sleep if no next fetch time - use timer with context cancellation
		timer := time.NewTimer(1 * time.Hour)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

// runMonitoringLoop runs the main monitoring loop (similar to CLI's run function).
// It continuously fetches programs, matches them against rules, and downloads matching programs.
// The loop respects the next fetch time and can be canceled via context.
func (a *App) runMonitoringLoop(ctx context.Context) {
	log.Printf("monitoring loop started")
	runtime.EventsEmit(a.ctx, "log-message", map[string]any{
		"type":    "info",
		"message": "Monitoring loop started",
	})

	// Setup logger to capture radikron log messages and emit events
	emitEvent := func(ctx context.Context, eventName string, data any) {
		runtime.EventsEmit(ctx, eventName, data)
		// Also check for manual injection removal on download-completed events
		// This is a fallback in case the callback doesn't work
		if eventName == "download-completed" {
			if dataMap, ok := data.(map[string]any); ok {
				stationID, _ := dataMap["station"].(string)
				title, _ := dataMap["title"].(string)
				startTime, _ := dataMap["start"].(string)
				// Try to match by pending downloads first, then by station/title/time
				a.HandleDownloadCompleted(stationID, title, startTime)
			}
		}
	}
	eventEmitter, cleanupLogger := SetupLogger(a.ctx, emitEvent)
	defer cleanupLogger()

	// Set callback to handle download completion for manual injections
	// Use the matching by station/title/time for monitoring loop (programs from rules)
	eventEmitter.SetDownloadCompletedCallback(func(stationID, title, startTime string) {
		a.HandleDownloadCompleted(stationID, title, startTime)
	})

	fetcher := &radikronProgramFetcher{}
	downloader := &radikronDownloader{}

	for {
		select {
		case <-ctx.Done():
			log.Printf("monitoring loop stopped (context canceled)")
			runtime.EventsEmit(a.ctx, "log-message", map[string]any{
				"type":    "info",
				"message": "Monitoring loop stopped",
			})
			return
		default:
		}

		// Update current time
		radikron.CurrentTime = time.Now().In(radikron.Location)

		// Reload config
		if err := a.reloadConfigIfNeeded(); err != nil {
			log.Printf("warning: failed to reload config: %v", err)
			runtime.EventsEmit(a.ctx, "log-message", map[string]any{
				"type":    "error",
				"message": fmt.Sprintf("Failed to reload config: %v", err),
			})
		}

		// Get asset snapshot while holding lock
		a.mu.RLock()
		asset := a.asset
		a.mu.RUnlock()

		if asset == nil {
			log.Printf("warning: asset is nil, skipping iteration")
			runtime.EventsEmit(a.ctx, "log-message", map[string]any{
				"type":    "error",
				"message": "Asset is not initialized, skipping iteration",
			})
			time.Sleep(assetRetryDelay) // Wait a bit before retrying
			continue
		}

		// Create context with asset and event emitter for downloads
		downloadCtx := context.WithValue(ctx, radikron.ContextKey("asset"), asset)
		downloadCtx = context.WithValue(downloadCtx, radikron.ContextKey("eventEmitter"), eventEmitter)

		// Check if rules are configured
		a.checkAndLogRulesCount(asset)

		// Skip processing if no rules or all rules have no criteria
		if len(asset.Rules) == 0 || !asset.Rules.HasRuleWithCriteria() {
			log.Printf("skipping program collection: no rules with criteria configured")
			runtime.EventsEmit(a.ctx, "log-message", map[string]any{
				"type":    "warning",
				"message": "No rules with criteria configured - skipping program collection",
			})
			// Sleep until next fetch time
			a.logAndSleepUntilNextFetch(asset, ctx)
			continue
		}

		// Collect and process programs
		a.processAllPrograms(asset, fetcher, downloadCtx, downloader)

		// Check and cleanup manually injected programs that are no longer available
		a.checkAndCleanupManualInjections()

		// Retry failed downloads for manual injections
		a.retryFailedDownloads()

		// Cleanup stale failed downloads
		a.cleanupStaleFailedDownloads()

		// Sleep until next fetch time
		a.logAndSleepUntilNextFetch(asset, ctx)
	}
}

// radikronProgramFetcher implements ProgramFetcher interface for fetching weekly programs.
type radikronProgramFetcher struct{}

// FetchWeeklyPrograms fetches weekly programs for the specified station ID.
func (f *radikronProgramFetcher) FetchWeeklyPrograms(stationID string) (radikron.Progs, error) {
	return radikron.FetchWeeklyPrograms(stationID)
}

// radikronDownloader implements Downloader interface for downloading programs.
type radikronDownloader struct{}

// Download downloads a program using the radikron download functionality.
func (d *radikronDownloader) Download(ctx context.Context, wg *sync.WaitGroup, prog *radikron.Prog) error {
	return radikron.Download(ctx, wg, prog)
}
