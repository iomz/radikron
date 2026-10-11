package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iomz/radikron"
)

func waitForStatus(t *testing.T, app *App, ok func(Status) bool) Status {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		status := app.GetStatus()
		if ok(status) {
			return status
		}
		if time.Now().After(deadline) {
			t.Fatalf("status did not reach the expected state: %+v", status)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func lastStatusEvent(t *testing.T, events *recordingEventSink) Status {
	t.Helper()
	events.mu.Lock()
	defer events.mu.Unlock()
	for i := len(events.events) - 1; i >= 0; i-- {
		if events.events[i].name == statusChangedEvent {
			return events.events[i].data.(Status)
		}
	}
	t.Fatal("no status-changed event")
	return Status{}
}

func TestStartupFailureIsReportedAndRetried(t *testing.T) {
	events := &recordingEventSink{}
	app := NewApp()
	app.events = events
	app.monitorLoop = func(ctx context.Context) { <-ctx.Done() }
	catalog := radikron.Stations{"TBS": {Areas: []string{radikron.DefaultArea}, Name: "TBS Radio"}}
	catalogErr := errors.New("radiko is unreachable")
	app.fetchStationCatalog = func(context.Context, *http.Client) (radikron.Stations, error) {
		return nil, catalogErr
	}

	startupErr := errors.New("failed to load config file at config.yml: yaml: line 3: bad indentation")
	app.initFunc = func() error { return startupErr }
	if err := app.startup(); !errors.Is(err, startupErr) {
		t.Fatalf("startup() error = %v, want %v", err, startupErr)
	}
	status := app.GetStatus()
	if status.Starting || status.Monitoring || status.ConfigError != startupErr.Error() {
		t.Fatalf("status after failed startup = %+v", status)
	}
	if got := lastStatusEvent(t, events); got != status {
		t.Errorf("last status-changed event = %+v, want %+v", got, status)
	}

	// The user fixes the file and retries.
	app.initFunc = func() error {
		app.mu.Lock()
		defer app.mu.Unlock()
		app.asset = &radikron.Asset{}
		app.config = testConfig(t.TempDir())
		return nil
	}
	if err := app.Retry(); err != nil {
		t.Fatalf("Retry() after startup failure error: %v", err)
	}
	t.Cleanup(func() { _ = app.StopMonitoring() })
	status = app.GetStatus()
	if !status.Monitoring || status.ConfigError != "" {
		t.Fatalf("status after successful retry = %+v", status)
	}

	// Startup refreshes stations in the background; that failure is reported
	// separately and does not block anything else.
	waitForStatus(t, app, func(s Status) bool { return s.StationsError != "" })

	app.fetchStationCatalog = func(context.Context, *http.Client) (radikron.Stations, error) {
		return catalog, nil
	}
	if err := app.Retry(); err != nil {
		t.Fatalf("Retry() after station failure error: %v", err)
	}
	if status := app.GetStatus(); status.StationsError != "" {
		t.Fatalf("station error not cleared: %+v", status)
	}
	if stations, _ := app.GetAvailableStations(); len(stations) != 1 || stations[0] != "TBS" {
		t.Errorf("available stations = %v, want [TBS]", stations)
	}
}

func TestConfigReloadErrorIsReportedAndCleared(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte("rules: [unclosed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.events = &recordingEventSink{}
	app.asset = &radikron.Asset{}
	app.configFile = configPath

	if err := app.reloadConfigIfNeeded(); err == nil {
		t.Fatal("reloadConfigIfNeeded() accepted a broken config")
	}
	if status := app.GetStatus(); status.ConfigError == "" {
		t.Fatalf("config error not reported: %+v", status)
	}

	if err := testConfig(dir).SaveConfig(configPath); err != nil {
		t.Fatal(err)
	}
	if err := app.Retry(); err != nil {
		t.Fatalf("Retry() error: %v", err)
	}
	if status := app.GetStatus(); status.ConfigError != "" {
		t.Fatalf("config error not cleared: %+v", status)
	}
	select {
	case <-app.configChanged:
	default:
		t.Error("Retry() did not wake automatic checks after the config loaded")
	}
}

func TestStatusReportsStartupAndNextCheck(t *testing.T) {
	app := NewApp()
	app.events = &recordingEventSink{}
	app.startupReady = make(chan struct{})
	if status := app.GetStatus(); !status.Starting {
		t.Fatalf("status during startup = %+v", status)
	}
	close(app.startupReady)

	next := time.Now().Add(time.Hour)
	app.asset = &radikron.Asset{NextFetchTime: &next}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		app.sleepUntilNextFetch(ctx)
		close(done)
	}()
	status := waitForStatus(t, app, func(s Status) bool { return s.NextCheck != "" })
	cancel()
	<-done

	got, err := time.Parse(time.RFC3339, status.NextCheck)
	if err != nil {
		t.Fatalf("NextCheck %q is not RFC 3339: %v", status.NextCheck, err)
	}
	if diff := got.Sub(next); diff < -time.Second || diff > time.Second {
		t.Errorf("NextCheck = %s, want about %s", got, next)
	}
	if status.Starting || status.HasRules {
		t.Errorf("status = %+v, want started and without rules", status)
	}
}
