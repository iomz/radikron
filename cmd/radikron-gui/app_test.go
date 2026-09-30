package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/iomz/radikron"
	"github.com/iomz/radikron/internal/config"
)

type recordedEvent struct {
	name string
	data any
}

type recordingEventSink struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (s *recordingEventSink) Emit(_ context.Context, name string, data any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, recordedEvent{name: name, data: data})
}

func (s *recordingEventSink) names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.events))
	for _, event := range s.events {
		names = append(names, event.name)
	}
	return names
}

func testConfig(downloadDir string) *config.Config {
	return &config.Config{
		AreaID:                    radikron.DefaultArea,
		ExtraStations:             []string{"TBS"},
		FileFormat:                radikron.AudioFormatAAC,
		MinimumOutputSize:         radikron.Kilobytes * radikron.Kilobytes,
		DownloadDir:               downloadDir,
		Rules:                     radikron.Rules{},
		MaxDownloadingConcurrency: radikron.MaxDownloadingConcurrency,
		MaxEncodingConcurrency:    radikron.MaxEncodingConcurrency,
	}
}

func TestConfigLifecycleEmitsEventsAndAppliesValues(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "source.yml")
	if err := testConfig(dir).SaveConfig(configPath); err != nil {
		t.Fatalf("SaveConfig() fixture error: %v", err)
	}

	events := &recordingEventSink{}
	app := NewApp()
	app.asset = &radikron.Asset{}
	app.events = events

	if err := app.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	loaded, err := app.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error: %v", err)
	}
	if loaded.DownloadDir != dir || !slices.Contains(app.asset.AvailableStations, "TBS") {
		t.Fatalf("loaded config not applied: config=%+v stations=%v", loaded, app.asset.AvailableStations)
	}

	updatedDir := filepath.Join(dir, "updated")
	if err := app.UpdateConfig(testConfig(updatedDir)); err != nil {
		t.Fatalf("UpdateConfig() error: %v", err)
	}
	savedPath := filepath.Join(dir, "saved.yml")
	if err := app.SaveConfig(savedPath); err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}
	select {
	case <-app.configChanged:
	default:
		t.Fatal("SaveConfig() did not wake automatic monitoring")
	}
	if _, err := os.Stat(savedPath); err != nil {
		t.Fatalf("saved config missing: %v", err)
	}
	if app.asset.DownloadDir != updatedDir {
		t.Errorf("asset download dir = %q, want %q", app.asset.DownloadDir, updatedDir)
	}
	if got := events.names(); !slices.Equal(got, []string{"config-loaded", "config-updated", "config-saved"}) {
		t.Errorf("events = %v", got)
	}
}

func TestConfigLifecycleErrorsDoNotEmitEvents(t *testing.T) {
	events := &recordingEventSink{}
	app := NewApp()
	app.events = events

	checks := []struct {
		name string
		call func() error
		want string
	}{
		{name: "get config before load", call: func() error { _, err := app.GetConfig(); return err }, want: "config not loaded"},
		{name: "get config file before set", call: func() error { _, err := app.GetConfigFile(); return err }, want: "config file not set"},
		{
			name: "load missing file",
			call: func() error { return app.LoadConfig(filepath.Join(t.TempDir(), "missing.yml")) },
			want: "failed to load config",
		},
		{name: "update nil", call: func() error { return app.UpdateConfig(nil) }, want: "config cannot be nil"},
		{name: "update before asset", call: func() error { return app.UpdateConfig(testConfig(t.TempDir())) }, want: "asset not initialized"},
		{name: "save before load", call: func() error { return app.SaveConfig("") }, want: "config not loaded"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			err := check.call()
			if err == nil || !strings.Contains(err.Error(), check.want) {
				t.Fatalf("error = %v, want substring %q", err, check.want)
			}
		})
	}
	if got := events.names(); len(got) != 0 {
		t.Errorf("error paths emitted events: %v", got)
	}
}

func TestGetConfigWaitsForStartupAndReturnsStartupError(t *testing.T) {
	app := NewApp()
	app.startupReady = make(chan struct{})
	wantErr := errors.New("asset initialization failed")
	result := make(chan error, 1)
	go func() {
		_, err := app.GetConfig()
		result <- err
	}()

	select {
	case err := <-result:
		t.Fatalf("GetConfig() returned before startup finished: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	app.mu.Lock()
	app.startupErr = wantErr
	close(app.startupReady)
	app.mu.Unlock()

	select {
	case err := <-result:
		if !errors.Is(err, wantErr) {
			t.Fatalf("GetConfig() error = %v, want startup error %v", err, wantErr)
		}
	case <-time.After(time.Second):
		t.Fatal("GetConfig() remained blocked after startup completed")
	}
}

func TestUpdateConfigRejectsUnsupportedStationWithoutMutation(t *testing.T) {
	events := &recordingEventSink{}
	app := NewApp()
	app.asset = &radikron.Asset{DownloadDir: "original"}
	app.events = events
	invalid := testConfig(t.TempDir())
	invalid.ExtraStations = []string{"JOAK"}

	err := app.UpdateConfig(invalid)
	if err == nil || !strings.Contains(err.Error(), "do not support Radiko archive/timeshift downloads") {
		t.Fatalf("UpdateConfig() error = %v", err)
	}
	if app.config != nil || app.asset.DownloadDir != "original" {
		t.Fatalf("invalid config mutated app: config=%+v asset=%+v", app.config, app.asset)
	}
	if len(events.names()) != 0 {
		t.Errorf("invalid config emitted event: %v", events.names())
	}
}

func TestGetSchedulesFiltersDownloadedInvalidAndUnsupportedPrograms(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "2026-08-20-1200_TBS_Existing.aac")
	if err := os.WriteFile(existingPath, []byte("audio"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	app := NewApp()
	app.asset = &radikron.Asset{
		OutputFormat: radikron.AudioFormatAAC,
		DownloadDir:  dir,
		Schedules: radikron.Schedules{
			{ID: "pending", StationID: "TBS", Title: "Pending", Ft: "20260820130000"},
			{ID: "existing", StationID: "TBS", Title: "Existing", Ft: "20260820120000"},
			{ID: "invalid", StationID: "TBS", Title: "Invalid", Ft: "not-a-time"},
			{ID: "unsupported", StationID: "JOAK", Title: "NHK", Ft: "20260820140000"},
		},
	}
	app.manualInjections["pending"] = &manualInjection{ProgramID: "pending"}

	schedules, err := app.GetSchedules()
	if err != nil {
		t.Fatalf("GetSchedules() error: %v", err)
	}
	if len(schedules) != 1 || schedules[0].ID != "pending" || !schedules[0].IsManualInjection {
		t.Fatalf("GetSchedules() = %+v, want pending manual injection", schedules)
	}
}

func TestGetSchedulesRequiresAsset(t *testing.T) {
	_, err := NewApp().GetSchedules()
	if err == nil || err.Error() != "asset not initialized" {
		t.Fatalf("GetSchedules() error = %v", err)
	}
}

func TestGetAvailableStationsReturnsEmptyArrayInsteadOfNil(t *testing.T) {
	app := NewApp()
	app.asset = &radikron.Asset{}
	stations, err := app.GetAvailableStations()
	if err != nil {
		t.Fatalf("GetAvailableStations() error = %v", err)
	}
	if stations == nil {
		t.Fatal("GetAvailableStations() returned nil; want an empty array")
	}
}

func TestGetStationNamesReturnsCatalogDisplayNames(t *testing.T) {
	app := NewApp()
	app.asset = &radikron.Asset{Stations: radikron.Stations{
		"FMJ": {Name: "J-WAVE"},
		"TBS": {Name: "Tokyo Broadcasting System"},
		"XYZ": nil,
	}}

	names, err := app.GetStationNames()
	if err != nil {
		t.Fatalf("GetStationNames() error: %v", err)
	}
	if names["FMJ"] != "J-WAVE" || names["TBS"] != "Tokyo Broadcasting System" {
		t.Fatalf("GetStationNames() = %#v", names)
	}
	if _, ok := names["XYZ"]; ok {
		t.Errorf("GetStationNames() included station without a display name")
	}
}

func TestHasMonitoringCriteria(t *testing.T) {
	tests := []struct {
		name  string
		asset *radikron.Asset
		want  bool
	}{
		{name: "nil asset"},
		{name: "no rules", asset: &radikron.Asset{}},
		{name: "rule without criteria", asset: &radikron.Asset{Rules: radikron.Rules{{}}}},
		{
			name: "rule with keyword",
			asset: &radikron.Asset{Rules: radikron.Rules{{
				Criteria: radikron.Criteria{Keyword: "news"},
			}}},
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := hasMonitoringCriteria(test.asset); got != test.want {
				t.Errorf("hasMonitoringCriteria() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestSleepUntilNextFetchWakesWhenConfigChanges(t *testing.T) {
	app := NewApp()
	done := make(chan struct{})
	go func() {
		app.sleepUntilNextFetch(context.Background())
		close(done)
	}()

	app.signalConfigChanged()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sleepUntilNextFetch() did not wake after config change")
	}
}

func TestSleepUntilNextFetchStopsOnContextCancellation(t *testing.T) {
	app := NewApp()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		app.sleepUntilNextFetch(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sleepUntilNextFetch() did not stop after cancellation")
	}
}

func TestHandleDownloadCompletedByIDRemovesInjectionAndSchedule(t *testing.T) {
	manualInjectionsFile := filepath.Join(t.TempDir(), "manual-injections.json")
	app := NewApp()
	app.manualInjectionsFile = manualInjectionsFile
	app.asset = &radikron.Asset{
		Schedules: radikron.Schedules{{ID: "program-1", StationID: "TBS", Title: "Test"}},
	}
	app.manualInjections["program-1"] = &manualInjection{
		ProgramID: "program-1",
		StationID: "TBS",
		Title:     "Test",
	}
	app.pendingManualDownloads["program-1"] = "program-1"

	app.HandleDownloadCompletedByID("program-1", "TBS", "Test", "20260820120000")

	if len(app.asset.Schedules) != 0 {
		t.Errorf("completed program remains scheduled: %+v", app.asset.Schedules)
	}
	if app.IsManualInjection("program-1") {
		t.Error("completed program remains a manual injection")
	}
	if _, exists := app.pendingManualDownloads["program-1"]; exists {
		t.Error("completed program remains pending")
	}
	data, err := os.ReadFile(manualInjectionsFile)
	if err != nil {
		t.Fatalf("manual injection state not saved: %v", err)
	}
	if string(data) != "[]" {
		t.Errorf("saved manual injections = %s, want []", data)
	}
}

func TestMonitoringLifecycleEmitsStateTransitions(t *testing.T) {
	events := &recordingEventSink{}
	started := make(chan struct{})
	app := NewApp()
	app.asset = &radikron.Asset{}
	app.events = events
	app.monitorLoop = func(ctx context.Context) {
		close(started)
		<-ctx.Done()
	}

	if err := app.StartMonitoring(); err != nil {
		t.Fatalf("StartMonitoring() error: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("monitor loop did not start")
	}
	if !app.GetMonitoringStatus() {
		t.Fatal("monitoring status false after start")
	}
	if err := app.StartMonitoring(); err == nil || err.Error() != "monitoring is already running" {
		t.Fatalf("second StartMonitoring() error = %v", err)
	}
	if err := app.StopMonitoring(); err != nil {
		t.Fatalf("StopMonitoring() error: %v", err)
	}
	if app.GetMonitoringStatus() {
		t.Fatal("monitoring status true after stop")
	}
	if err := app.StopMonitoring(); err != nil {
		t.Fatalf("second StopMonitoring() error: %v", err)
	}
	if got := events.names(); !slices.Equal(got, []string{"monitoring-started", "monitoring-stopped"}) {
		t.Errorf("events = %v", got)
	}
}

func TestConcurrentStopMonitoringObservesCompletedStop(t *testing.T) {
	started := make(chan struct{})
	var startOnce sync.Once
	app := NewApp()
	app.asset = &radikron.Asset{}
	app.events = &recordingEventSink{}
	app.monitorLoop = func(ctx context.Context) {
		startOnce.Do(func() { close(started) })
		<-ctx.Done()
	}

	if err := app.StartMonitoring(); err != nil {
		t.Fatalf("StartMonitoring() error: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("monitor loop did not start")
	}

	// Every concurrent caller must observe the fully reset lifecycle state once
	// StopMonitoring returns, not just the monitoring goroutine's exit.
	const callers = 8
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := app.StopMonitoring(); err != nil {
				errs <- fmt.Errorf("StopMonitoring() error: %w", err)
				return
			}
			if app.GetMonitoringStatus() {
				errs <- errors.New("monitoring status true after StopMonitoring returned")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	if err := app.StartMonitoring(); err != nil {
		t.Fatalf("StartMonitoring() after stop error: %v", err)
	}
	if err := app.StopMonitoring(); err != nil {
		t.Fatalf("final StopMonitoring() error: %v", err)
	}
}

func TestStartMonitoringRequiresAsset(t *testing.T) {
	app := NewApp()
	app.events = &recordingEventSink{}
	err := app.StartMonitoring()
	if err == nil || err.Error() != "asset not initialized" {
		t.Fatalf("StartMonitoring() error = %v", err)
	}
}
