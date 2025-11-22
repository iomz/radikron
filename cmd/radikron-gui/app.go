package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/iomz/radikron"
	"github.com/iomz/radikron/internal/config"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yyoshiki41/go-radiko"
)

const (
	// assetRetryDelay is the delay before retrying when asset is nil
	assetRetryDelay = 10 * time.Second
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

// OnStartup is called when the app starts
func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	a.configFile = "config.yml" // Default config file

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

	// Load config if it exists
	if cfg, err := config.LoadConfig(a.configFile); err == nil {
		if err := cfg.ApplyToAsset(a.asset); err == nil {
			a.config = cfg
		}
	}
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

// LoadConfig loads configuration from a file
func (a *App) LoadConfig(filename string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	cfg, err := config.LoadConfig(filename)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
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

// SaveConfig saves the current configuration to a file
func (a *App) SaveConfig(filename string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config == nil {
		return fmt.Errorf("config not loaded")
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
	ctx, cancel := context.WithCancel(context.Background())
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

// programWithStation wraps a program with its station ID
type programWithStation struct {
	prog      *radikron.Prog
	stationID string
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

	// Apply config to asset while holding the lock
	if err := cfg.ApplyToAsset(a.asset); err != nil {
		return fmt.Errorf("failed to apply config to asset: %w", err)
	}

	// Update config while holding the lock
	a.config = cfg

	return nil
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

	// Lock to prevent race conditions when checking/adding to schedules
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
	a.asset.Schedules = append(a.asset.Schedules, p)
	a.mu.Unlock()

	// Check if rule matches using the asset snapshot
	matchedRule := asset.Rules.FindMatchSilent(stationID, p)
	if matchedRule == nil {
		// Rule didn't match, remove from schedules
		a.mu.Lock()
		for i, s := range a.asset.Schedules {
			if s.ID == p.ID {
				a.asset.Schedules = append(a.asset.Schedules[:i], a.asset.Schedules[i+1:]...)
				break
			}
		}
		a.mu.Unlock()
		delete(processedInThisIteration, p.ID)
		return false, false
	}

	// Double-check that program is still in schedules
	a.mu.Lock()
	stillInSchedules := a.asset.Schedules.HasDuplicate(p)
	alreadyLogged := processedInThisIteration[p.ID+"_logged"]
	if !alreadyLogged && stillInSchedules {
		processedInThisIteration[p.ID+"_logged"] = true
	}
	a.mu.Unlock()

	if !stillInSchedules || alreadyLogged {
		return true, false
	}

	p.RuleName = matchedRule.Name
	p.RuleFolder = matchedRule.Folder

	log.Printf("rule[%s] matched [%s]%s - attempting download (start time: %s)", matchedRule.Name, stationID, p.Title, p.Ft)
	runtime.EventsEmit(a.ctx, "log-message", map[string]any{
		"type":    "info",
		"message": fmt.Sprintf("Rule '%s' matched [%s]%s (start: %s) - attempting download", matchedRule.Name, stationID, p.Title, p.Ft),
	})

	// Call Download() - it will log "start downloading" if it actually starts
	// Note: Download() may return nil if program is in future, duplicate, or file exists
	err := downloader.Download(downloadCtx, a.monitorWg, p)
	if err != nil {
		log.Printf("download failed for [%s]%s: %s", p.StationID, p.Title, err)
		runtime.EventsEmit(a.ctx, "download-failed", map[string]any{
			"station": p.StationID,
			"title":   p.Title,
			"error":   err.Error(),
		})
		return true, false
	}
	// If err == nil, Download() may have skipped silently (future, duplicate, or exists)
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
	if a.asset.NextFetchTime != nil {
		sleepDuration := time.Until(*a.asset.NextFetchTime)
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
