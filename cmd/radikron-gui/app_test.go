package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/iomz/radikron"
	"github.com/iomz/radikron/internal/config"
	"github.com/yyoshiki41/radigo"
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
		FileFormat:                radigo.AudioFormatAAC,
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
		OutputFormat: radigo.AudioFormatAAC,
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

func TestStartMonitoringRequiresAsset(t *testing.T) {
	app := NewApp()
	app.events = &recordingEventSink{}
	err := app.StartMonitoring()
	if err == nil || err.Error() != "asset not initialized" {
		t.Fatalf("StartMonitoring() error = %v", err)
	}
}
