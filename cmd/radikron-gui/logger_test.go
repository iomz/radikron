package main

import (
	"context"
	"slices"
	"testing"
)

func TestWailsEventEmitterEmitsStructuredEvents(t *testing.T) {
	var events []recordedEvent
	emitter := NewWailsEventEmitter(context.Background())
	emitter.emitEvent = func(_ context.Context, name string, data any) {
		events = append(events, recordedEvent{name: name, data: data})
	}
	var completed []string
	emitter.SetDownloadCompletedCallback(func(stationID, title, startTime string) {
		completed = []string{stationID, title, startTime}
	})

	emitter.EmitDownloadStarted("TBS", "Show", "20260820120000")
	emitter.EmitDownloadCompleted("", "", "20260820120000", "/tmp/2026-08-20-1200_TBS_Show_Title.aac")
	emitter.EmitFileSaved("", "", "/tmp/2026-08-20-1200_FMT_Saved.mp3")
	emitter.EmitDownloadSkipped("already exists", "TBS", "Show", "20260820120000")
	emitter.EmitEncodingStarted("/tmp/show.aac")
	emitter.EmitEncodingCompleted("/tmp/show.mp3")
	emitter.EmitLogMessage("error", "failed")

	wantNames := []string{
		"download-started", "download-completed", "file-saved", "download-skipped",
		"encoding-started", "encoding-completed", "log-message",
	}
	gotNames := make([]string, 0, len(events))
	for _, event := range events {
		gotNames = append(gotNames, event.name)
	}
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("events = %v, want %v", gotNames, wantNames)
	}
	if !slices.Equal(completed, []string{"TBS", "Show_Title", "20260820120000"}) {
		t.Errorf("completion callback = %v", completed)
	}
	completedPayload := events[1].data.(map[string]any)
	if completedPayload["station"] != "TBS" || completedPayload["title"] != "Show_Title" {
		t.Errorf("completion payload = %v", completedPayload)
	}
}

func TestExtractProgramInfoFromPath(t *testing.T) {
	checks := []struct {
		path        string
		wantStation string
		wantTitle   string
	}{
		{path: "/tmp/2026-08-20-1200_TBS_Show_Title.aac", wantStation: "TBS", wantTitle: "Show_Title"},
		{path: "2026-08-20_1200_FMT_Legacy_Show.mp3", wantStation: "FMT", wantTitle: "Legacy_Show"},
		{path: "/tmp/incomplete.aac"},
	}
	for _, check := range checks {
		station, title := extractProgramInfoFromPath(check.path)
		if station != check.wantStation || title != check.wantTitle {
			t.Errorf("extractProgramInfoFromPath(%q) = (%q, %q), want (%q, %q)",
				check.path, station, title, check.wantStation, check.wantTitle)
		}
	}
}

func TestEventLoggerClassifiesCompleteLines(t *testing.T) {
	var events []recordedEvent
	logger := NewEventLogger(context.Background(), func(_ context.Context, name string, data any) {
		events = append(events, recordedEvent{name: name, data: data})
	})

	if n, err := logger.Write([]byte("download started\n")); err != nil || n == 0 {
		t.Fatalf("Write() = (%d, %v)", n, err)
	}
	if _, err := logger.Write([]byte("failed to save\n")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := logger.Write(nil); err != nil {
		t.Fatalf("Write(nil) error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %v, want two complete lines", events)
	}
	if events[0].data.(map[string]any)["type"] != logTypeInfo {
		t.Errorf("first event = %v", events[0])
	}
	if events[1].data.(map[string]any)["type"] != logTypeError {
		t.Errorf("second event = %v", events[1])
	}
}
