package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	goRuntime "runtime"
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
)

// App struct represents the Wails application
type App struct {
	ctx           context.Context
	asset         *radikron.Asset
	client        *radiko.Client
	config        *config.Config
	configFile    string
	monitoring    bool
	monitorDone   chan struct{}
	monitorWg     *sync.WaitGroup
	monitorCancel context.CancelFunc
	mu            sync.RWMutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		monitorWg: &sync.WaitGroup{},
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
	} else {
		// Use config.yml in app config directory
		a.configFile = filepath.Join(appConfigDir, "config.yml")
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
	runtime.EventsEmit(a.ctx, "config-loaded", map[string]any{
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
	runtime.EventsEmit(a.ctx, "config-updated", map[string]any{
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
	runtime.EventsEmit(a.ctx, "config-saved", map[string]any{
		"success": true,
		"file":    filename,
	})

	return nil
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

// SearchWeeklyPrograms searches weekly programs using rule criteria
// It fetches programs from all available stations and filters them using the provided rule
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

	// Fetch programs from all stations and filter
	var matchingPrograms radikron.Progs
	for _, stationID := range stationsToSearch {
		programs, err := radikron.FetchWeeklyPrograms(stationID)
		if err != nil {
			// Log error but continue with other stations
			runtime.LogError(a.ctx, fmt.Sprintf("Failed to fetch programs for station %s: %v", stationID, err))
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

// OpenDirectory opens the specified directory in the system's file browser
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

// GetMonitoringStatus returns whether monitoring is active
func (a *App) GetMonitoringStatus() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.monitoring
}

// StartMonitoring starts the monitoring loop
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
	go a.runMonitoringLoop(ctx)

	// Log and emit event to frontend
	log.Printf("monitoring started")
	runtime.EventsEmit(a.ctx, "monitoring-started", nil)

	return nil
}

// StopMonitoring stops the monitoring loop
func (a *App) StopMonitoring() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.monitoring {
		return nil
	}

	if a.monitorCancel != nil {
		a.monitorCancel()
	}

	if a.monitorDone != nil {
		<-a.monitorDone
	}

	a.monitorWg.Wait()
	a.monitoring = false

	// Log and emit event to frontend
	log.Printf("monitoring stopped")
	runtime.EventsEmit(a.ctx, "monitoring-stopped", nil)

	return nil
}

// reloadConfigIfNeeded reloads and applies configuration if available
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

	// Process all stations and collect programs
	for _, stationID := range asset.AvailableStations {
		// Skip if no rules match this station
		if !asset.Rules.HasRuleWithoutStationID() && !asset.Rules.HasRuleForStationID(stationID) {
			continue
		}

		// Fetch weekly programs
		weeklyPrograms, err := fetcher.FetchWeeklyPrograms(stationID)
		if err != nil {
			log.Printf("failed to fetch the %s program: %v", stationID, err)
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

	// Convert map to slice for deterministic iteration order
	programList := make([]*programWithStation, 0, len(allPrograms))
	for _, pws := range allPrograms {
		programList = append(programList, pws)
	}
	return programList
}

// processProgram handles a single program: checks duplicates, matches rules, and downloads if needed
// Returns (matched, duplicate) to indicate if the program matched a rule or was a duplicate
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
	if a.asset.Schedules.HasDuplicate(p) {
		a.mu.Unlock()
		return false, true
	}
	processedInThisIteration[p.ID] = true
	// Don't add to schedules yet - Download() will check for duplicates
	// We'll add to schedules after Download() confirms it will proceed
	a.mu.Unlock()

	// Check if rule matches using the asset snapshot
	matchedRule := asset.Rules.FindMatchSilent(stationID, p)
	if matchedRule == nil {
		// Rule didn't match - remove from processed tracking
		a.mu.Lock()
		delete(processedInThisIteration, p.ID)
		a.mu.Unlock()
		return false, false
	}

	// Check if we've already logged this program in this iteration
	// (to prevent duplicate log messages for the same program)
	a.mu.Lock()
	alreadyLogged := processedInThisIteration[p.ID+"_logged"]
	if !alreadyLogged {
		processedInThisIteration[p.ID+"_logged"] = true
		a.mu.Unlock()
		// First time processing this program - continue to download attempt
	} else {
		a.mu.Unlock()
		// Already processed this program in this iteration - skip
		return true, false
	}

	p.RuleName = matchedRule.Name
	p.RuleFolder = matchedRule.Folder
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

// checkAndLogRulesCount checks and logs the number of configured rules
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

// processAllPrograms collects programs from stations and processes them
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

// logAndSleepUntilNextFetch logs the next fetch time and sleeps until then
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

// sleepUntilNextFetch sleeps until the next fetch time or until context is canceled
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
		// Default sleep if no next fetch time
		time.Sleep(1 * time.Hour)
	}
}

// runMonitoringLoop runs the main monitoring loop (similar to CLI's run function)
func (a *App) runMonitoringLoop(ctx context.Context) {
	defer a.monitorWg.Done()
	defer close(a.monitorDone)

	log.Printf("monitoring loop started")
	runtime.EventsEmit(a.ctx, "log-message", map[string]any{
		"type":    "info",
		"message": "Monitoring loop started",
	})

	// Setup logger to capture radikron log messages and emit events
	emitEvent := func(ctx context.Context, eventName string, data any) {
		runtime.EventsEmit(ctx, eventName, data)
	}
	eventEmitter, cleanupLogger := SetupLogger(a.ctx, emitEvent)
	defer cleanupLogger()

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

		// Collect and process programs
		a.processAllPrograms(asset, fetcher, downloadCtx, downloader)

		// Sleep until next fetch time
		a.logAndSleepUntilNextFetch(asset, ctx)
	}
}

// radikronProgramFetcher implements ProgramFetcher
type radikronProgramFetcher struct{}

func (f *radikronProgramFetcher) FetchWeeklyPrograms(stationID string) (radikron.Progs, error) {
	return radikron.FetchWeeklyPrograms(stationID)
}

// radikronDownloader implements Downloader
type radikronDownloader struct{}

func (d *radikronDownloader) Download(ctx context.Context, wg *sync.WaitGroup, prog *radikron.Prog) error {
	return radikron.Download(ctx, wg, prog)
}
