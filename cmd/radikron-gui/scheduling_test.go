package main

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/iomz/radikron"
)

// fakeDownloads stands in for radikron.Download. With start set, it reports
// download-started like the core does when a download begins.
type fakeDownloads struct {
	app   *App
	start bool
	calls int
	ctx   context.Context
}

func (f *fakeDownloads) download(ctx context.Context, _ *sync.WaitGroup, prog *radikron.Prog) error {
	f.calls++
	f.ctx = ctx
	if f.start {
		f.app.emitDownloadEvent(ctx, "download-started", map[string]any{"station": prog.StationID, "title": prog.Title, "start": prog.Ft})
	}
	return nil
}

var showProg = &radikron.Prog{ID: "p1", StationID: "TBS", Title: "Show", Ft: "29990101100000", To: "29990101110000"}

func processOnce(app *App, f *fakeDownloads) {
	prog := *showProg
	app.processProgram(&programWithStation{prog: &prog, stationID: "TBS"}, app.asset, context.Background(), map[string]bool{}, f.download)
}

func TestScheduledProgramIsTriedAgainOnLaterChecks(t *testing.T) {
	app, _ := ruleTestApp(t, radikron.Rules{{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}}})
	f := &fakeDownloads{app: app}
	processOnce(app, f)
	processOnce(app, f)
	if f.calls != 2 {
		t.Fatalf("download tried %d times, want 2: a scheduled program must be tried again once it airs", f.calls)
	}
	if len(app.asset.Schedules) != 1 || app.asset.Schedules[0].RuleName != "Show" {
		t.Errorf("schedules = %+v", app.asset.Schedules)
	}
}

func TestDownloadInProgressIsNotStartedTwice(t *testing.T) {
	app, _ := ruleTestApp(t, radikron.Rules{{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}}})
	f := &fakeDownloads{app: app, start: true}
	processOnce(app, f)
	processOnce(app, f)
	if f.calls != 1 {
		t.Fatalf("download started %d times, want 1", f.calls)
	}
	app.emitDownloadEvent(context.Background(), "download-completed", map[string]any{"station": "TBS", "title": "Show", "start": showProg.Ft})
	if len(app.activeKeys()) != 0 {
		t.Error("a finished download is still active")
	}
	if !errors.Is(f.ctx.Err(), context.Canceled) {
		t.Error("a finished download's context was not released")
	}
}
