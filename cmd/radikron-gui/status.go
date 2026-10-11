package main

import (
	"errors"
	"fmt"
	"time"
)

// statusChangedEvent carries a Status whenever any of its fields change.
const statusChangedEvent = "status-changed"

// Status summarizes what the backend is doing, so the GUI can show the state
// of automatic downloads and any failure that needs the user's attention.
// Times are RFC 3339 strings in local time, or "" when unknown.
type Status struct {
	// Starting is true while the configuration is being loaded.
	Starting bool `json:"starting"`
	// Monitoring is true while automatic checks run. They run only while the
	// app is open.
	Monitoring bool `json:"monitoring"`
	// Checking is true while a check fetches programs and matches rules.
	Checking bool `json:"checking"`
	// HasRules is false when no rule has search criteria, so checks find
	// nothing to download.
	HasRules  bool   `json:"hasRules"`
	LastCheck string `json:"lastCheck"`
	NextCheck string `json:"nextCheck"`
	// ConfigError is the last failure to load the configuration, at startup
	// or when a check reloads it; "" once it loads.
	ConfigError string `json:"configError"`
	// StationsError is the last failure to fetch the station catalog; "" once
	// a refresh succeeds.
	StationsError string `json:"stationsError"`
}

// GetStatus returns the current Status. Unlike most methods it does not wait
// for startup, so the GUI can show that startup is still running.
func (a *App) GetStatus() Status {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.statusLocked()
}

func (a *App) statusLocked() Status {
	starting := false
	if a.startupReady != nil {
		select {
		case <-a.startupReady:
		default:
			starting = true
		}
	}
	configErr := a.configErr
	if a.startupErr != nil {
		configErr = a.startupErr.Error()
	}
	return Status{
		Starting:      starting,
		Monitoring:    a.monitoring,
		Checking:      a.checking,
		HasRules:      hasMonitoringCriteria(a.asset),
		LastCheck:     formatStatusTime(a.lastCheck),
		NextCheck:     formatStatusTime(a.nextCheck),
		ConfigError:   configErr,
		StationsError: a.stationsErr,
	}
}

func formatStatusTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format(time.RFC3339)
}

// publishStatus emits the current Status. Callers must not hold a.mu.
func (a *App) publishStatus() {
	a.emit(statusChangedEvent, a.GetStatus())
}

// updateStatus applies change to the status fields under a.mu and publishes
// the result.
func (a *App) updateStatus(change func()) {
	a.mu.Lock()
	change()
	a.mu.Unlock()
	a.publishStatus()
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// startup loads the configuration and starts automatic checks. ServiceStartup
// runs it once; Retry runs it again after it failed.
func (a *App) startup() error {
	a.mu.Lock()
	a.startupReady = make(chan struct{})
	a.startupErr = nil
	a.mu.Unlock()
	a.publishStatus()

	initialize := a.initialize
	if a.initFunc != nil {
		initialize = a.initFunc
	}
	err := initialize()
	a.mu.Lock()
	a.startupErr = err
	close(a.startupReady)
	a.mu.Unlock()
	a.publishStatus()
	if err != nil {
		a.logger.Error(fmt.Sprintf("GUI startup initialization failed: %v", err))
		return err
	}

	if err := a.StartMonitoring(); err != nil {
		a.logger.Error(fmt.Sprintf("Failed to start automatic downloads: %v", err))
		a.emit("log-message", map[string]any{
			"type":    "error",
			"message": fmt.Sprintf("Failed to start automatic downloads: %v", err),
		})
	}
	go a.refreshStationCatalogInBackground()
	go a.loadManualInjectionsInBackground()
	return nil
}

// Retry retries what failed: startup when the configuration could not be
// loaded, otherwise the last configuration reload and station catalog fetch.
// It returns the first failure that remains.
func (a *App) Retry() error {
	a.mu.RLock()
	startupFailed := a.startupErr != nil
	configFailed := a.configErr != ""
	stationsFailed := a.stationsErr != ""
	a.mu.RUnlock()

	if startupFailed {
		if !a.startupMu.TryLock() {
			return errors.New("startup is already running")
		}
		defer a.startupMu.Unlock()
		return a.startup()
	}

	var errs []error
	if configFailed {
		if err := a.reloadConfigIfNeeded(); err != nil {
			errs = append(errs, err)
		} else {
			// Run a check with the repaired configuration now.
			a.signalConfigChanged()
		}
	}
	if stationsFailed {
		if _, err := a.RefreshStations(); err != nil {
			errs = append(errs, err)
		} else {
			a.emit("stations-loaded", nil)
		}
	}
	return errors.Join(errs...)
}
