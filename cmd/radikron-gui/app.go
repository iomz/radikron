package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goRuntime "runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/iomz/radikron"
	"github.com/iomz/radikron/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
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
	stationCatalogTimeout    = 30 * time.Second
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

// wailsEventSink delivers events through the Wails application; its zero value
// drops events until the application is attached.
type wailsEventSink struct {
	app *application.App
}

func (s wailsEventSink) Emit(_ context.Context, eventName string, data any) {
	if s.app != nil {
		s.app.Event.Emit(eventName, data)
	}
}

// App struct represents the Wails application
type App struct {
	ctx                    context.Context
	wails                  *application.App
	logger                 *slog.Logger
	asset                  *radikron.Asset
	client                 *http.Client
	config                 *config.Config
	configFile             string
	manualInjectionsFile   string
	monitoring             bool
	monitorStopping        bool
	monitorDone            chan struct{}
	monitorStopped         chan struct{}
	monitorWg              *sync.WaitGroup
	monitorCancel          context.CancelFunc
	programSnapshots       map[string]radikron.Progs   // stationID -> programs snapshot
	snapshotLoad           *snapshotLoad               // the weekly programs being fetched, if any
	manualInjections       map[string]*manualInjection // progKey -> injection data
	pendingManualDownloads map[string]string           // progKey -> program ID (for tracking downloads in progress)
	events                 frontendEventSink
	monitorLoop            func(context.Context)
	startupReady           chan struct{}
	startupErr             error
	configChanged          chan struct{}
	updatesEnabled         bool   // set by main when this build can update itself
	availableUpdate        string // newest release version found by a background check
	startupMu              sync.Mutex
	checking               bool                       // a check is fetching programs and matching rules
	lastCheck              time.Time                  // when the last check finished
	nextCheck              time.Time                  // when the next check is due
	configErr              string                     // last failure to reload the configuration
	stationsErr            string                     // last failure to fetch the station catalog
	downloads              map[string]*downloadRecord // last download event per program, by downloadKey
	active                 map[string]*activeDownload // downloads in progress, by downloadKey
	skipped                map[string]*skippedProgram // programs the user skipped, by downloadKey
	downloadsFile          string                     // where downloads and skipped are saved
	downloadsMu            sync.Mutex                 // guards downloads, active, skipped, downloadsFile; never held with mu
	downloadsSaveMu        sync.Mutex                 // serializes writes of downloadsFile
	downloadsSaves         sync.WaitGroup             // background saves of downloadsFile
	mu                     sync.RWMutex

	// Test seams; nil uses the real implementation.
	initFunc            func() error
	fetchStationCatalog func(context.Context, *http.Client) (radikron.Stations, error)
}

// NewApp creates a new App application struct
func NewApp() *App {
	startupReady := make(chan struct{})
	close(startupReady)
	return &App{
		monitorWg:              &sync.WaitGroup{},
		programSnapshots:       make(map[string]radikron.Progs),
		manualInjections:       make(map[string]*manualInjection),
		pendingManualDownloads: make(map[string]string),
		events:                 wailsEventSink{},
		logger:                 slog.Default(),
		startupReady:           startupReady,
		configChanged:          make(chan struct{}, 1),
	}
}

// attach connects the App to the Wails application before it runs.
func (a *App) attach(wailsApp *application.App) {
	a.wails = wailsApp
	a.events = wailsEventSink{app: wailsApp}
	a.logger = wailsApp.Logger
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
		AreaID:                    radikron.DefaultArea,
		ExtraStations:             []string{},
		IgnoreStations:            []string{},
		FileFormat:                radikron.AudioFormatAAC,
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

// ServiceStartup is called by Wails when the app starts. Initialization
// failures are reported to the frontend instead of aborting the app, so the
// user can still fix the configuration from the GUI.
func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.ctx = ctx
	// Updates do not depend on the configuration, so check even when it fails to load.
	a.startBackgroundUpdateChecks(ctx)
	a.startupMu.Lock()
	defer a.startupMu.Unlock()
	// A failed startup is reported through GetStatus, and Retry runs it again.
	_ = a.startup()
	return nil
}

func (a *App) signalConfigChanged() {
	select {
	case a.configChanged <- struct{}{}:
	default:
	}
}

func (a *App) waitForStartup() error {
	a.mu.RLock()
	ready := a.startupReady
	a.mu.RUnlock()
	if ready != nil {
		<-ready
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.startupErr
}

func (a *App) refreshStationCatalogInBackground() {
	if _, err := a.RefreshStations(); err != nil {
		if a.ctx != nil {
			a.logger.Warn(fmt.Sprintf("Failed to refresh station catalog: %v", err))
			a.emit("log-message", map[string]any{
				"type":    "error",
				"message": fmt.Sprintf("Failed to refresh stations: %v. Use Refresh Stations to retry.", err),
			})
		}
		return
	}
	a.emit("stations-loaded", nil)
}

func (a *App) loadManualInjectionsInBackground() {
	if err := a.loadManualInjections(); err != nil {
		if a.ctx != nil {
			a.logger.Warn(fmt.Sprintf("Failed to load manual injections: %v", err))
			a.emit("log-message", map[string]any{
				"type":    "error",
				"message": fmt.Sprintf("Failed to load manual injections: %v", err),
			})
		}
	}
}

func (a *App) initialize() error {
	// Build everything first and publish it under the lock at the end: Retry
	// can run this again while the GUI is reading the previous state.
	configFile := "config.yml"
	manualInjectionsFile := "manual-injections.json"
	downloadsFilePath := "downloads.json"
	if appConfigDir, err := getAppConfigDir(); err != nil {
		a.logger.Error(fmt.Sprintf("Failed to get app config directory: %v", err))
		// Fall back to the current directory.
	} else {
		configFile = filepath.Join(appConfigDir, "config.yml")
		manualInjectionsFile = filepath.Join(appConfigDir, "manual-injections.json")
		downloadsFilePath = filepath.Join(appConfigDir, "downloads.json")
	}
	a.mu.Lock()
	a.configFile = configFile
	a.manualInjectionsFile = manualInjectionsFile
	a.mu.Unlock()
	a.downloadsMu.Lock()
	a.downloadsFile = downloadsFilePath
	a.downloadsMu.Unlock()
	if err := a.loadDownloadsFile(downloadsFilePath, time.Now()); err != nil {
		a.logger.Error(fmt.Sprintf("Failed to load download history: %v", err))
	}

	// Initialize the HTTP client used for Radiko requests.
	client, err := radikron.NewRadikoHTTPClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Initialize embedded data without blocking startup on the remote station catalog.
	asset, err := radikron.NewAssetWithoutStationCatalog(client)
	if err != nil {
		return fmt.Errorf("failed to create asset: %w", err)
	}

	// Check if config file exists before attempting to load
	_, err = os.Stat(configFile)
	configExists := err == nil
	configNotFound := err != nil && os.IsNotExist(err)

	var cfg *config.Config
	if configNotFound {
		// Config file doesn't exist, create default config
		a.logger.Info(fmt.Sprintf("Config file not found at %s, creating default config", configFile))
		cfg, err = createDefaultConfig(configFile)
		if err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
		a.logger.Info(fmt.Sprintf("Created default config at %s", configFile))
	} else if configExists {
		// Config file exists, try to load it
		cfg, err = loadConfigFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config file at %s: %w", configFile, err)
		}
	} else {
		return fmt.Errorf("failed to check config file at %s: %w", configFile, err)
	}

	// Apply config to asset
	if err := cfg.ApplyToAsset(asset); err != nil {
		return fmt.Errorf("failed to apply config to asset: %w", err)
	}

	a.mu.Lock()
	a.client = client
	a.asset = asset
	a.config = cfg
	a.configErr = ""
	a.mu.Unlock()
	a.logger.Info(fmt.Sprintf("Config loaded successfully from %s", configFile))

	return nil
}

// fetchProgramDetailsForStations fetches weekly programs for given stations and returns full program details by progKey
func (a *App) fetchProgramDetailsForStations(stationIDs []string) map[string]*radikron.Prog {
	programDetailsMap := make(map[string]*radikron.Prog)
	fetcher := &radikronProgramFetcher{}

	for _, stationID := range stationIDs {
		weeklyPrograms, err := fetcher.FetchWeeklyPrograms(stationID)
		if err != nil {
			a.logger.Error(fmt.Sprintf(
				"Failed to fetch weekly programs for station %s to update manual injection details: %v",
				stationID, err))
			continue
		}

		for _, p := range weeklyPrograms {
			programDetailsMap[progKey(p)] = p
		}
	}

	return programDetailsMap
}

// updateProgWithDetails updates a program with full details if available
func updateProgWithDetails(prog *radikron.Prog, programDetailsMap map[string]*radikron.Prog) {
	if fullProg, exists := programDetailsMap[progKey(prog)]; exists {
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

	eventEmitter, cleanupLogger := SetupLogger(a.ctx, a.emitDownloadEvent)
	key := progKey(prog)
	eventEmitter.SetDownloadCompletedCallback(func(stationID, title, _ string) {
		a.handleManualDownloadCompleted(key, stationID, title)
	})
	ctx = context.WithValue(ctx, radikron.ContextKey("eventEmitter"), eventEmitter)

	go func() {
		time.Sleep(loggerCleanupDelay)
		cleanupLogger()
	}()

	wg := &sync.WaitGroup{}
	err := a.startDownload(ctx, wg, prog, radikron.Download)
	if err != nil {
		errMsg := fmt.Sprintf("%s [%s]%s: %v", errorMsgPrefix, prog.StationID, prog.Title, err)
		a.logger.Error(errMsg)
		a.events.Emit(a.ctx, "log-message", map[string]any{
			"type":    "error",
			"message": errMsg,
		})
		a.emitDownloadEvent(a.ctx, "download-failed", downloadFailedEvent(prog, err))

		// Track failure in manual injection
		a.mu.Lock()
		if inj, exists := a.manualInjections[key]; exists {
			now := time.Now()
			inj.LastFailureTime = &now
			inj.RetryCount++
			inj.LastError = err.Error()
		}
		a.mu.Unlock()

		// Save failure state (outside lock to avoid deadlock)
		go func() {
			if saveErr := a.saveManualInjections(); saveErr != nil {
				a.logger.Error(fmt.Sprintf("Failed to save manual injection failure state: %v", saveErr))
			}
		}()

		return err
	}

	// Clear failure state on success
	a.mu.Lock()
	if inj, exists := a.manualInjections[key]; exists {
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

// findProgramInSchedules finds a program in schedules by progKey.
func (a *App) findProgramInSchedules(key string) *radikron.Prog {
	for _, prog := range a.asset.Schedules {
		if progKey(prog) == key {
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
		inj := injectionMap[progKey(prog)]
		if inj == nil {
			continue
		}

		if inj.LastFailureTime == nil {
			a.logger.Info(fmt.Sprintf(
				"Downloading past program [%s]%s",
				prog.StationID, prog.Title))
		} else {
			a.logger.Info(fmt.Sprintf(
				"Retrying download for [%s]%s (attempt %d/%d)",
				prog.StationID, prog.Title, inj.RetryCount+1, maxDownloadRetries))
		}

		// Download the program
		err := a.setupDownloadForPastProgram(prog)
		if err != nil {
			a.logger.Info(fmt.Sprintf(
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
			a.logger.Info(fmt.Sprintf(
				"Cleaned up stale failed manual injection [%s]%s (ID: %s) - %s",
				inj.StationID, inj.Title, id, reason))
		}
	}

	if removed {
		// Save outside the lock to avoid deadlock
		go func() {
			if err := a.saveManualInjections(); err != nil {
				a.logger.Error(fmt.Sprintf("Failed to save manual injections after cleanup: %v", err))
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
		a.logger.Error(fmt.Sprintf("Failed to parse start time for loaded program %s: %v", prog.ID, err))
		if a.findProgramInSchedules(progKey(prog)) == nil {
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
			delete(a.manualInjections, inj.key())
			a.logger.Info(fmt.Sprintf("Skipping loaded program [%s]%s - file already exists", prog.StationID, prog.Title))
			return
		}

		// Add to schedules for visibility
		if a.findProgramInSchedules(progKey(prog)) == nil {
			a.asset.Schedules = append(a.asset.Schedules, prog)
		}
		// Mark for pending download (actual download happens in retry loop)
		a.pendingManualDownloads[progKey(prog)] = prog.ID
	} else {
		// Future program
		if a.findProgramInSchedules(progKey(prog)) == nil {
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
			a.logger.Warn(fmt.Sprintf(
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
		a.manualInjections[inj.key()] = inj

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

	a.logger.Info(fmt.Sprintf("Loaded %d manually injected programs", len(injections)))
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

// IsManualInjection reports whether the program on stationID starting at
// start was added from Search.
func (a *App) IsManualInjection(stationID, start string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	_, exists := a.manualInjections[downloadKey(stationID, start)]
	return exists
}

// DeleteManualInjection removes a manually injected program from schedules and persistence.
// It removes the program from schedules, pending downloads, and the manual injections map,
// then saves the updated state to disk.
func (a *App) DeleteManualInjection(stationID, start string) error {
	key := downloadKey(stationID, start)
	a.mu.Lock()

	// Check if it's a manual injection
	if _, exists := a.manualInjections[key]; !exists {
		a.mu.Unlock()
		return fmt.Errorf("program on %s at %s is not a manual injection", stationID, start)
	}

	a.removeScheduledProgramLocked(key)

	// Remove from pending manual downloads
	delete(a.pendingManualDownloads, key)

	// Remove from manual injections map
	delete(a.manualInjections, key)

	// Release lock before saving to avoid deadlock (saveManualInjections acquires its own lock)
	a.mu.Unlock()

	// Save to disk
	if err := a.saveManualInjections(); err != nil {
		return fmt.Errorf("failed to save manual injections: %w", err)
	}

	a.logger.Info(fmt.Sprintf("Deleted manual injection: %s", key))
	a.emit(downloadsChangedEvent, nil)
	return nil
}

// removeScheduledProgramLocked removes a program from schedules by progKey.
// Caller must hold a.mu.
func (a *App) removeScheduledProgramLocked(key string) {
	if a.asset == nil {
		return
	}
	schedules := a.asset.Schedules[:0]
	for _, prog := range a.asset.Schedules {
		if progKey(prog) != key {
			schedules = append(schedules, prog)
		}
	}
	a.asset.Schedules = schedules
}

// ServiceShutdown is called by Wails when the app closes
func (a *App) ServiceShutdown() error { //nolint:unparam // Wails service interface
	if err := a.StopMonitoring(); err != nil {
		a.logger.Error(fmt.Sprintf("Failed to stop monitoring on shutdown: %v", err))
	}
	return nil
}

// GetConfig returns the current configuration
func (a *App) GetConfig() (*config.Config, error) {
	if err := a.waitForStartup(); err != nil {
		return nil, fmt.Errorf("startup failed: %w", err)
	}
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
			prog.IsManualInjection = a.manualInjections[progKey(prog)] != nil
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
	if a.findProgramInSchedules(progKey(prog)) == nil {
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
	a.removeScheduledProgramLocked(progKey(prog))

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
	if err := a.injectProgram(prog, ruleName); err != nil {
		return err
	}
	a.emit(downloadsChangedEvent, nil)
	return nil
}

func (a *App) injectProgram(prog *radikron.Prog, ruleName string) error {
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
	a.applyInjectionRule(prog, matchedRule)

	// Check for duplicates
	if err := a.handleDuplicateCheckForInjection(prog, startTime); err != nil {
		a.mu.Unlock()
		return err
	}

	// Handle past vs future programs
	if !startTime.After(radikron.CurrentTime) {
		return a.injectPastProgramLocked(prog)
	}
	return a.injectFutureProgramLocked(prog)
}

// applyInjectionRule copies matching rule metadata to the injected program.
func (a *App) applyInjectionRule(prog *radikron.Prog, matchedRule *radikron.Rule) {
	if matchedRule == nil {
		prog.RuleName = ""
		prog.RuleFolder = ""
		return
	}
	prog.RuleName = matchedRule.Name
	prog.RuleFolder = matchedRule.Folder
}

// injectPastProgramLocked registers and downloads a past program. Caller holds a.mu.
func (a *App) injectPastProgramLocked(prog *radikron.Prog) error {
	progCopy := *prog
	a.manualInjections[progKey(&progCopy)] = &manualInjection{
		ProgramID: progCopy.ID, StationID: progCopy.StationID, Ft: progCopy.Ft,
		To: progCopy.To, Title: progCopy.Title, RuleName: progCopy.RuleName,
		RuleFolder: progCopy.RuleFolder,
	}
	a.mu.Unlock()
	if err := a.saveManualInjections(); err != nil {
		a.logger.Error(fmt.Sprintf("Failed to save manual injection: %v", err))
	}

	downloadErr := a.downloadPastInjectedProgram(&progCopy)
	a.mu.Lock()
	if a.manualInjections[progKey(&progCopy)] != nil && a.findProgramInSchedules(progKey(&progCopy)) == nil {
		a.asset.Schedules = append(a.asset.Schedules, &progCopy)
	}
	a.mu.Unlock()
	if downloadErr != nil {
		a.logger.Info(fmt.Sprintf(
			"Past program [%s]%s download failed, will be retried automatically",
			progCopy.StationID, progCopy.Title))
	}
	return nil
}

// injectFutureProgramLocked schedules a future program. Caller holds a.mu.
func (a *App) injectFutureProgramLocked(prog *radikron.Prog) error {
	a.asset.Schedules = append(a.asset.Schedules, prog)
	endTime, err := time.ParseInLocation(radikron.DatetimeLayout, prog.To, radikron.Location)
	if err == nil {
		next := endTime.Add(radikron.BufferMinutes * time.Minute)
		if a.asset.NextFetchTime == nil || a.asset.NextFetchTime.After(next) {
			a.asset.NextFetchTime = &next
		}
	}
	a.manualInjections[progKey(prog)] = &manualInjection{
		ProgramID: prog.ID, StationID: prog.StationID, Ft: prog.Ft,
		To: prog.To, Title: prog.Title, RuleName: prog.RuleName,
		RuleFolder: prog.RuleFolder,
	}
	a.mu.Unlock()
	if err := a.saveManualInjections(); err != nil {
		a.logger.Error(fmt.Sprintf("Failed to save manual injection: %v", err))
	}
	a.events.Emit(a.ctx, "program-injected", map[string]any{
		"program": prog.Title,
		"station": prog.StationID,
	})
	return nil
}

// findProgramIDFromPendingDownloads finds the progKey of a pending manual
// download that is still scheduled with the same title.
func (a *App) findProgramIDFromPendingDownloads(stationID, title, startTime string) string {
	key := downloadKey(stationID, startTime)
	if _, pending := a.pendingManualDownloads[key]; !pending {
		return ""
	}
	if s := a.findProgramInSchedules(key); s != nil && s.Title == title {
		return key
	}
	return ""
}

// findProgramIDFromManualInjections finds the progKey of a manual injection by matching metadata.
// It searches through manual injections to match by station ID, title, and start time.
func (a *App) findProgramIDFromManualInjections(stationID, title, startTime string) string {
	for id, inj := range a.manualInjections {
		if inj.StationID == stationID && inj.Title == title && inj.Ft == startTime {
			return id
		}
	}
	return ""
}

// removeManualInjectionAndSave removes a program, by progKey, from manual
// injections, pending downloads, and schedules, then saves to disk.
func (a *App) removeManualInjectionAndSave(key, stationID, title string) {
	if key == "" {
		return
	}

	a.mu.Lock()
	delete(a.manualInjections, key)
	delete(a.pendingManualDownloads, key)
	a.removeScheduledProgramLocked(key)
	a.mu.Unlock()

	// Save outside the lock to avoid deadlock (saveManualInjections acquires its own lock)
	if err := a.saveManualInjections(); err != nil {
		a.logger.Error(fmt.Sprintf("Failed to save manual injections after download: %v", err))
	} else if a.ctx != nil {
		a.logger.Info(fmt.Sprintf("Removed manually injected program [%s]%s after download completion", stationID, title))
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

// handleManualDownloadCompleted is called when a download started for a
// known program, by progKey, completes.
func (a *App) handleManualDownloadCompleted(key, stationID, title string) {
	a.mu.Lock()
	_, exists := a.manualInjections[key]
	a.mu.Unlock()

	if exists {
		a.removeManualInjectionAndSave(key, stationID, title)
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

	// Build a set of available programs from snapshots
	availablePrograms := make(map[string]bool)
	for _, programs := range a.programSnapshots {
		for _, prog := range programs {
			availablePrograms[progKey(prog)] = true
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
		if !availablePrograms[id] {
			// Program is no longer available - remove it
			delete(a.manualInjections, id)
			a.removeScheduledProgramLocked(id)

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
			a.logger.Info(fmt.Sprintf(
				"Removed manually injected program [%s]%s - no longer available in weekly programs",
				entry.stationID, entry.title))
		}

		if err := a.saveManualInjections(); err != nil {
			a.logger.Error(fmt.Sprintf("Failed to save manual injections after cleanup: %v", err))
		}
	}
}

// LoadConfig loads configuration from a file
func (a *App) LoadConfig(filename string) error {
	if err := a.loadConfig(filename); err != nil {
		return err
	}
	a.publishStatus()
	return nil
}

func (a *App) loadConfig(filename string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	cfg, err := loadConfigFile(filename)
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
	a.configErr = ""
	a.signalConfigChanged()

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
	a.signalConfigChanged()

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
	if a.wails == nil {
		return fmt.Errorf("save dialog unavailable before the app starts")
	}
	savePath, err := a.wails.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Filename: defaultFilename,
		Title:    "Save Configuration File",
		Filters: []application.FileFilter{
			{
				DisplayName: "YAML Files (*.yml, *.yaml)",
				Pattern:     "*.yml;*.yaml",
			},
			{
				DisplayName: "All Files (*.*)",
				Pattern:     "*.*",
			},
		},
	}).PromptForSingleSelection()
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
	if a.wails == nil {
		return "", fmt.Errorf("directory dialog unavailable before the app starts")
	}
	selectedPath, err := a.wails.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                "Select Download Directory",
		CanChooseDirectories: true,
		CanChooseFiles:       false,
	}).PromptForSingleSelection()
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
	if err := a.waitForStartup(); err != nil {
		return nil, fmt.Errorf("startup failed: %w", err)
	}
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.asset == nil {
		return nil, fmt.Errorf("asset not initialized")
	}

	return append([]string{}, a.asset.AvailableStations...), nil
}

// GetStationNames returns display names keyed by station ID.
func (a *App) GetStationNames() (map[string]string, error) {
	if err := a.waitForStartup(); err != nil {
		return nil, fmt.Errorf("startup failed: %w", err)
	}
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.asset == nil {
		return nil, fmt.Errorf("asset not initialized")
	}

	names := make(map[string]string, len(a.asset.Stations))
	for stationID, station := range a.asset.Stations {
		if station != nil && station.Name != "" {
			names[stationID] = station.Name
		}
	}
	return names, nil
}

// RefreshStations fetches the station catalog and applies the active station filters.
func (a *App) RefreshStations() ([]string, error) { //nolint:unparam // The frontend reads the station list.
	if err := a.waitForStartup(); err != nil {
		return nil, fmt.Errorf("startup failed: %w", err)
	}
	a.mu.RLock()
	if a.asset == nil {
		a.mu.RUnlock()
		return nil, fmt.Errorf("asset not initialized")
	}
	client := a.asset.DefaultClient
	a.mu.RUnlock()

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, stationCatalogTimeout)
	defer cancel()
	fetch := radikron.FetchStationCatalog
	if a.fetchStationCatalog != nil {
		fetch = a.fetchStationCatalog
	}
	available, err := a.applyFetchedStations(fetch(ctx, client))
	a.updateStatus(func() { a.stationsErr = errorText(err) })
	return available, err
}

// applyFetchedStations applies a fetched station catalog to the asset.
func (a *App) applyFetchedStations(stations radikron.Stations, fetchErr error) ([]string, error) {
	if fetchErr != nil {
		return nil, fmt.Errorf("failed to fetch station catalog: %w", fetchErr)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.asset == nil {
		return nil, fmt.Errorf("asset not initialized")
	}
	available, err := applyStationCatalog(a.asset, a.config, stations)
	if err != nil {
		return nil, fmt.Errorf("failed to apply station configuration: %w", err)
	}
	return available, nil
}

func applyStationCatalog(asset *radikron.Asset, cfg *config.Config, stations radikron.Stations) ([]string, error) {
	updatedAsset := &radikron.Asset{Stations: stations}
	if cfg != nil {
		if err := cfg.ApplyToAsset(updatedAsset); err != nil {
			return nil, err
		}
	} else {
		updatedAsset.LoadAvailableStations(radikron.DefaultArea)
	}

	asset.Stations = updatedAsset.Stations
	asset.AvailableStations = updatedAsset.AvailableStations
	return append([]string{}, asset.AvailableStations...), nil
}

// searchMatcher returns the program filter for SearchWeeklyPrograms. Unlike
// rules, search ignores case. The keyword is split on spaces (including
// full-width ones) and every word must appear in the title, performer, info,
// description, or tags; title and pfm, when set, must appear in those fields.
// With nothing to match it accepts every program, so the GUI can list a
// station or day. Stations are filtered by the caller.
func searchMatcher(title, pfm, keyword string) func(string, *radikron.Prog) bool {
	lower := strings.ToLower
	title, pfm = lower(title), lower(pfm)
	terms := strings.Fields(lower(keyword))
	return func(_ string, prog *radikron.Prog) bool {
		if title != "" && !strings.Contains(lower(prog.Title), title) {
			return false
		}
		if pfm != "" && !strings.Contains(lower(prog.Pfm), pfm) {
			return false
		}
		text := lower(strings.Join(append([]string{prog.Title, prog.Pfm, prog.Info, prog.Desc}, prog.Tags...), "\n"))
		for _, term := range terms {
			if !strings.Contains(text, term) {
				return false
			}
		}
		return true
	}
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

// FetchProgramSnapshots loads the weekly programs of stations that have none
// yet, for program search. Automatic checks keep them up to date.
func (a *App) FetchProgramSnapshots() error {
	a.mu.RLock()
	ready := a.asset != nil
	a.mu.RUnlock()
	if !ready {
		return fmt.Errorf("asset not initialized")
	}
	a.loadProgramSnapshots(radikron.FetchWeeklyPrograms, false)
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

	matches := searchMatcher(ruleTitle, rulePfm, ruleKeyword)

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
			a.logger.Error(fmt.Sprintf("No program snapshot available for station %s", stationID))
			continue
		}

		// Filter programs using the rule
		for _, prog := range programs {
			if matches(stationID, prog) {
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
	if err := a.waitForStartup(); err != nil {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.monitoring
}

// StartMonitoring starts the monitoring loop.
// It creates a context, sets up the monitoring goroutine, and begins continuous program monitoring.
func (a *App) StartMonitoring() error {
	if err := a.startMonitoring(); err != nil {
		return err
	}
	a.publishStatus()
	return nil
}

func (a *App) startMonitoring() error {
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
	log.Printf("automatic downloads enabled")
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
	if a.monitorStopping {
		stopped := a.monitorStopped
		a.mu.Unlock()
		// Wait for the first caller to finish resetting the lifecycle state, so
		// GetMonitoringStatus and StartMonitoring observe a completed stop.
		if stopped != nil {
			<-stopped
		}
		return nil
	}
	cancel := a.monitorCancel
	done := a.monitorDone
	stopped := make(chan struct{})
	a.monitorStopped = stopped
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
	a.monitorStopped = nil
	a.checking = false
	a.nextCheck = time.Time{}
	a.mu.Unlock()

	// Log and emit event to frontend
	log.Printf("automatic downloads stopped")
	a.emit("monitoring-stopped", nil)
	a.publishStatus()

	// Release concurrent callers only after the stop is fully observable, so they
	// cannot start a new run that emits monitoring-started before this event.
	close(stopped)

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

// reloadConfigIfNeeded reloads the configuration file and records the result
// in the status.
func (a *App) reloadConfigIfNeeded() error {
	err := a.reloadConfig()
	a.updateStatus(func() { a.configErr = errorText(err) })
	return err
}

// reloadConfig reloads and applies configuration if available.
// It loads the config file and applies it to the asset without blocking on I/O.
func (a *App) reloadConfig() error {
	// RLock to capture current configFile snapshot
	a.mu.RLock()
	configFile := a.configFile
	a.mu.RUnlock()

	// If no config file, don't reload
	if configFile == "" {
		return nil
	}

	// Load config using the snapshot (outside lock since it can take time)
	cfg, err := loadConfigFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", configFile, err)
	}

	active := a.activeKeys()
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
	// The rules may have changed in the file.
	a.pruneSchedulesLocked(active)

	return nil
}

// programWithStation wraps a program with its station ID
type programWithStation struct {
	prog      *radikron.Prog
	stationID string
}

// collectProgramsFromStations refreshes the weekly programs of every station
// and returns the ones on stations the rules watch, without duplicates.
func (a *App) collectProgramsFromStations(
	asset *radikron.Asset,
	fetch func(stationID string) (radikron.Progs, error),
) []*programWithStation {
	a.loadProgramSnapshots(fetch, true)

	allPrograms := make(map[string]*programWithStation) // by downloadKey
	a.mu.RLock()
	for _, stationID := range asset.AvailableStations {
		if !asset.Rules.HasRuleWithoutStationID() && !asset.Rules.HasRuleForStationID(stationID) {
			continue
		}
		// Keep only the first occurrence of each program. Not by ID: radiko
		// gives the parts of a split program the same ID.
		for _, p := range a.programSnapshots[stationID] {
			key := downloadKey(stationID, p.Ft)
			if _, exists := allPrograms[key]; !exists {
				allPrograms[key] = &programWithStation{prog: p, stationID: stationID}
			}
		}
	}
	a.mu.RUnlock()

	programList := make([]*programWithStation, 0, len(allPrograms))
	for _, pws := range allPrograms {
		programList = append(programList, pws)
	}
	return programList
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
	if inj, exists := a.manualInjections[progKey(p)]; exists {
		p.RuleName = inj.RuleName
		p.RuleFolder = inj.RuleFolder
		return true
	}
	return false
}

// applyRuleToProgram applies rule information to program if not already set by manual injection.
// It sets the rule name and folder on the program from the matched rule.
func (a *App) applyRuleToProgram(p *radikron.Prog, matchedRule *radikron.Rule, isManualInjection bool) {
	if matchedRule != nil && !isManualInjection {
		p.RuleName = matchedRule.Name
		p.RuleFolder = matchedRule.Folder
	}
}

// processProgram handles a single program: matches it against the rules and
// downloads it if it is wanted and not on disk yet. Programs that are not
// available yet stay scheduled and are tried again on later checks.
// Returns (matched, duplicate): duplicate means the program needed no work.
func (a *App) processProgram(
	pws *programWithStation,
	asset *radikron.Asset,
	downloadCtx context.Context,
	processedInThisIteration map[string]bool,
	download downloadFunc,
) (matched, duplicate bool) {
	p := pws.prog
	stationID := pws.stationID

	// Ensure StationID is set correctly (it should be set from XML, but ensure it matches)
	if p.StationID == "" {
		p.StationID = stationID
	}
	key := progKey(p)

	a.mu.Lock()
	if processedInThisIteration[key] {
		a.mu.Unlock()
		return false, true
	}
	processedInThisIteration[key] = true
	isManualInjection := a.setupManualInjectionForProcess(p)
	a.mu.Unlock()

	matchedRule := a.findMatchingRuleForProcessProgram(p, stationID, asset, isManualInjection)
	if matchedRule == nil && !isManualInjection {
		return false, false
	}
	a.applyRuleToProgram(p, matchedRule, isManualInjection)

	if a.leaveAlone(p, asset) {
		return true, true
	}

	// Download() logs "rule matched" and "start downloading" only if it starts;
	// it returns nil without starting for programs not available yet.
	if err := a.startDownload(downloadCtx, a.monitorWg, p, download); err != nil {
		a.events.Emit(a.ctx, "log-message", map[string]any{
			"type":    "error",
			"message": fmt.Sprintf("download failed for [%s]%s: %s", p.StationID, p.Title, err),
		})
		a.emitDownloadEvent(a.ctx, "download-failed", downloadFailedEvent(p, err))
		return true, false
	}
	a.mu.Lock()
	a.rememberScheduledLocked(p)
	a.mu.Unlock()
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
		a.events.Emit(a.ctx, "log-message", map[string]any{
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
	// This check works out when the next program becomes available.
	a.mu.Lock()
	asset.NextFetchTime = nil
	a.mu.Unlock()

	// Collect all programs from all stations
	programList := a.collectProgramsFromStations(asset, fetcher.FetchWeeklyPrograms)
	log.Printf("collected %d programs from stations", len(programList))

	// Track programs processed in this iteration to prevent duplicates
	processedInThisIteration := make(map[string]bool)

	// Process each program
	matchedCount := 0
	duplicateCount := 0
	processedCount := 0
	for _, pws := range programList {
		matched, duplicate := a.processProgram(pws, asset, downloadCtx, processedInThisIteration, downloader.Download)
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

	// Wake early when config changes, or sleep for the next poll interval.
	sleepDuration := time.Hour
	if nextFetchTime != nil {
		sleepDuration = time.Until(*nextFetchTime)
	}
	if sleepDuration <= 0 {
		return
	}
	a.updateStatus(func() { a.nextCheck = time.Now().Add(sleepDuration) })

	timer := time.NewTimer(sleepDuration)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()
	select {
	case <-ctx.Done():
	case <-a.configChanged:
	case <-timer.C:
	}
}

// runMonitoringLoop runs the main monitoring loop (similar to CLI's run function).
// It continuously fetches programs, matches them against rules, and downloads matching programs.
// The loop respects the next fetch time and can be canceled via context.
func (a *App) runMonitoringLoop(ctx context.Context) {
	log.Printf("automatic program checks started")
	a.events.Emit(a.ctx, "log-message", map[string]any{
		"type":    "info",
		"message": "Automatic program checks started",
	})

	// Capture radikron log messages and record download states as events arrive.
	eventEmitter, cleanupLogger := SetupLogger(a.ctx, a.emitDownloadEvent)
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
			log.Printf("automatic program checks stopped (app is closing)")
			a.events.Emit(a.ctx, "log-message", map[string]any{
				"type":    "info",
				"message": "Automatic program checks stopped",
			})
			return
		default:
		}

		// Update current time
		radikron.CurrentTime = time.Now().In(radikron.Location)

		// Reload config
		if err := a.reloadConfigIfNeeded(); err != nil {
			log.Printf("warning: failed to reload config: %v", err)
			a.events.Emit(a.ctx, "log-message", map[string]any{
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
			a.events.Emit(a.ctx, "log-message", map[string]any{
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

		// Skip processing when no rule has criteria and nothing was added from
		// Program Search; programs added there are downloaded without rules.
		if !hasMonitoringCriteria(asset) && !a.hasManualInjections() {
			log.Printf("skipping program collection: no rules with criteria configured")
			a.events.Emit(a.ctx, "log-message", map[string]any{
				"type":    "warning",
				"message": "No rules with criteria configured - skipping program collection",
			})
			// Sleep until next fetch time
			a.logAndSleepUntilNextFetch(asset, ctx)
			continue
		}

		// Collect and process programs
		a.updateStatus(func() {
			a.checking = true
			a.nextCheck = time.Time{}
		})
		a.processAllPrograms(asset, fetcher, downloadCtx, downloader)

		// Check and cleanup manually injected programs that are no longer available
		a.checkAndCleanupManualInjections()

		// Retry failed downloads for manual injections
		a.retryFailedDownloads()

		// Cleanup stale failed downloads
		a.cleanupStaleFailedDownloads()
		a.updateStatus(func() {
			a.checking = false
			a.lastCheck = time.Now()
		})
		a.emit(downloadsChangedEvent, nil)

		// Sleep until next fetch time
		a.logAndSleepUntilNextFetch(asset, ctx)
	}
}

// hasManualInjections reports whether any program added from Program Search
// is waiting to be downloaded.
func (a *App) hasManualInjections() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.manualInjections) > 0
}

func hasMonitoringCriteria(asset *radikron.Asset) bool {
	return asset != nil && len(asset.Rules) > 0 && asset.Rules.HasRuleWithCriteria()
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
