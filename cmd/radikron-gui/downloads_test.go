package main

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/iomz/radikron"
)

func jst(t *testing.T, value string) time.Time {
	t.Helper()
	tm, err := time.ParseInLocation(radikron.DatetimeLayout, value, radikron.Location)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

func TestScheduledState(t *testing.T) {
	now := jst(t, "20261012120000")
	tests := []struct {
		name   string
		ft, to string
		want   DownloadState
	}{
		{"not aired yet", "20261012200000", "20261012210000", DownloadWaiting},
		{"airing", "20261012113000", "20261012123000", DownloadWaiting},
		{"within the buffer after the end", "20261012110000", "20261012115800", DownloadWaiting},
		{"available", "20261011200000", "20261011210000", DownloadAvailable},
		{"older than the archive", "20261005110000", "20261005120000", DownloadExpired},
		{"invalid start", "not-a-time", "20261012210000", DownloadWaiting},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scheduledState(&radikron.Prog{Ft: tt.ft, To: tt.to}, now)
			if got != tt.want {
				t.Errorf("scheduledState(%s-%s) = %s, want %s", tt.ft, tt.to, got, tt.want)
			}
		})
	}
}

func TestRecordDownloadEventTracksLifecycle(t *testing.T) {
	app := NewApp()
	now := time.Now()
	event := func(name string, extra map[string]any) bool {
		data := map[string]any{"station": "TBS", "title": "Show", "start": "20261011200000"}
		for k, v := range extra {
			data[k] = v
		}
		return app.recordDownloadEvent(name, data, now)
	}
	record := func() DownloadItem {
		return app.downloads[downloadKey("TBS", "20261011200000")].item
	}

	if !event("download-started", nil) || record().State != DownloadDownloading {
		t.Fatalf("after start: %+v", record())
	}
	event("download-progress", map[string]any{"stage": "fetching playlist", "completed": 3, "total": 10})
	if got := record(); got.State != DownloadDownloading || got.Stage != "fetching playlist" || got.Completed != 3 || got.Total != 10 {
		t.Fatalf("after progress: %+v", got)
	}
	event("download-failed", map[string]any{"stage": "encoding", "error": "ffmpeg not found"})
	if got := record(); got.State != DownloadFailed || got.Error != "ffmpeg not found" || got.Stage != "encoding" {
		t.Fatalf("after failure: %+v", got)
	}
	event("download-completed", nil)
	if got := record(); got.State != DownloadCompleted || got.Error != "" {
		t.Fatalf("after completion: %+v", got)
	}

	if app.recordDownloadEvent("log-message", map[string]any{"station": "TBS", "start": "1"}, now) {
		t.Error("recorded a non-download event")
	}
	if app.recordDownloadEvent("download-failed", map[string]any{"station": "TBS", "title": "No start"}, now) {
		t.Error("recorded an event without a start time")
	}
}

func TestEmitDownloadEventForwardsAndAnnouncesChanges(t *testing.T) {
	events := &recordingEventSink{}
	app := NewApp()
	app.events = events
	app.emitDownloadEvent(app.ctx, "download-started", map[string]any{"station": "TBS", "title": "Show", "start": "20261011200000"})
	app.emitDownloadEvent(app.ctx, "log-message", map[string]any{"type": "info", "message": "hello"})
	want := []string{"download-started", downloadsChangedEvent, "log-message"}
	if got := events.names(); !slices.Equal(got, want) {
		t.Errorf("events = %v, want %v", got, want)
	}
}

func TestGetDownloadsCombinesSchedulesAndHistory(t *testing.T) {
	app := NewApp()
	app.asset = &radikron.Asset{
		OutputFormat: radikron.AudioFormatAAC,
		DownloadDir:  t.TempDir(),
		Schedules: radikron.Schedules{
			{ID: "later", StationID: "TBS", Title: "Later", Ft: "29990101100000", To: "29990101110000", RuleName: "news"},
			{ID: "failing", StationID: "QRR", Title: "Failing", Ft: "29981231100000", To: "29981231110000"},
		},
	}
	app.manualInjections["failing"] = &manualInjection{ProgramID: "failing"}

	base := time.Now()
	app.recordDownloadEvent("download-failed", map[string]any{
		"station": "QRR", "title": "Failing", "start": "29981231100000", "error": "HTTP 403",
	}, base)
	app.recordDownloadEvent("download-completed", map[string]any{
		"station": "LFR", "title": "Done", "start": "20261011200000",
	}, base.Add(time.Minute))

	downloads, err := app.GetDownloads()
	if err != nil {
		t.Fatal(err)
	}
	// summary keeps the fields this test controls; UpdatedAt is checked separately.
	type summary struct {
		ID, Title, End, RuleName, Error string
		State                           DownloadState
		Manual, Updated                 bool
	}
	summarize := func(items []DownloadItem) []summary {
		out := make([]summary, 0, len(items))
		for i := range items {
			it := &items[i]
			out = append(out, summary{it.ID, it.Title, it.End, it.RuleName, it.Error, it.State, it.Manual, it.UpdatedAt != ""})
		}
		return out
	}
	wantScheduled := []summary{
		{ID: "failing", Title: "Failing", End: "29981231110000", Error: "HTTP 403", State: DownloadFailed, Manual: true, Updated: true},
		{ID: "later", Title: "Later", End: "29990101110000", RuleName: "news", State: DownloadWaiting},
	}
	if got := summarize(downloads.Scheduled); !slices.Equal(got, wantScheduled) {
		t.Errorf("scheduled = %+v\nwant %+v", got, wantScheduled)
	}
	wantRecent := []summary{
		{Title: "Done", State: DownloadCompleted, Updated: true},
		{Title: "Failing", Error: "HTTP 403", State: DownloadFailed, Updated: true},
	}
	if got := summarize(downloads.Recent); !slices.Equal(got, wantRecent) {
		t.Errorf("recent = %+v\nwant %+v", got, wantRecent)
	}
}

func TestGetDownloadsRequiresAsset(t *testing.T) {
	if _, err := NewApp().GetDownloads(); err == nil {
		t.Fatal("GetDownloads() without an asset returned no error")
	}
}

func TestRecentDownloadsAreBounded(t *testing.T) {
	app := NewApp()
	base := time.Now()
	app.recordDownloadEvent("download-started", map[string]any{"station": "TBS", "start": "active"}, base)
	for i := range maxRecentDownloads + 5 {
		app.recordDownloadEvent("download-completed", map[string]any{
			"station": "TBS", "start": fmt.Sprintf("done-%02d", i),
		}, base.Add(time.Duration(i)*time.Second))
	}
	if got := len(app.downloads); got != maxRecentDownloads+1 {
		t.Fatalf("kept %d records, want %d finished plus the active one", got, maxRecentDownloads)
	}
	if app.downloads[downloadKey("TBS", "active")] == nil {
		t.Error("pruned an active download")
	}
	if app.downloads[downloadKey("TBS", "done-00")] != nil {
		t.Error("kept the oldest finished download")
	}
}

func TestDownloadFailedEventUsesProgramStart(t *testing.T) {
	data := downloadFailedEvent(&radikron.Prog{StationID: "TBS", Title: "Show", Ft: "20261011200000"}, errors.New("boom"))
	if data["start"] != "20261011200000" || data["error"] != "boom" {
		t.Errorf("event data = %v", data)
	}
}
