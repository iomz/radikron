package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

func TestRemovingARuleDropsItsScheduledPrograms(t *testing.T) {
	app, _ := ruleTestApp(t, radikron.Rules{{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}}})
	processOnce(app, &fakeDownloads{app: app})
	manual := &radikron.Prog{ID: "m1", StationID: "TBS", Title: "Show", Ft: "29990102100000", To: "29990102110000"}
	app.asset.Schedules = append(app.asset.Schedules, manual)
	app.manualInjections[progKey(manual)] = &manualInjection{ProgramID: manual.ID}

	if err := app.RemoveRule("Show"); err != nil {
		t.Fatal(err)
	}
	if len(app.asset.Schedules) != 1 || app.asset.Schedules[0].ID != "m1" {
		t.Errorf("schedules after removing the rule = %+v, want only the program added from Search", app.asset.Schedules)
	}
}

func TestSkipCancelsAndOutlivesARestart(t *testing.T) {
	app, _ := ruleTestApp(t, radikron.Rules{{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}}})
	app.downloadsFile = filepath.Join(t.TempDir(), "downloads.json")
	f := &fakeDownloads{app: app, start: true}
	processOnce(app, f)

	if err := app.SkipDownload("TBS", showProg.Ft); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(f.ctx.Err(), context.Canceled) {
		t.Fatal("skipping a download in progress did not cancel it")
	}
	// The core reports the cancel as a failure.
	app.emitDownloadEvent(context.Background(), "download-failed", map[string]any{
		"station": "TBS", "title": "Show", "start": showProg.Ft, "error": "context canceled",
	})
	if err := app.saveDownloadsFile(); err != nil {
		t.Fatal(err)
	}

	restarted, _ := ruleTestApp(t, radikron.Rules{{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}}})
	if err := restarted.loadDownloadsFile(app.downloadsFile, time.Now()); err != nil {
		t.Fatal(err)
	}
	again := &fakeDownloads{app: restarted}
	processOnce(restarted, again)
	if again.calls != 0 {
		t.Fatal("a rule downloaded a skipped program after a restart")
	}
	downloads, err := restarted.GetDownloads()
	if err != nil {
		t.Fatal(err)
	}
	if len(downloads.Skipped) != 1 || len(downloads.Recent) != 0 || len(downloads.Scheduled) != 0 {
		t.Errorf("downloads = %+v, want only the skipped program", downloads)
	}

	if err := restarted.UndoSkip("TBS", showProg.Ft); err != nil {
		t.Fatal(err)
	}
	processOnce(restarted, again)
	if again.calls != 1 {
		t.Error("undoing a skip did not let the rule download the program")
	}
}

func TestFinishedDownloadsOutliveARestart(t *testing.T) {
	app := NewApp()
	app.downloadsFile = filepath.Join(t.TempDir(), "downloads.json")
	now := time.Now()
	event := func(name, title, start string, at time.Time, extra ...string) {
		data := map[string]any{"station": "TBS", "title": title, "start": start}
		if len(extra) > 0 {
			data["error"] = extra[0]
		}
		app.recordDownloadEvent(name, data, at)
	}
	event("download-completed", "Show", "20261011200000", now)
	event("download-failed", "Quit", "20261011210000", now, "context canceled")
	event("download-completed", "Old", "20260101200000", now.Add(-100*24*time.Hour))
	if err := app.saveDownloadsFile(); err != nil {
		t.Fatal(err)
	}

	restarted := NewApp()
	if err := restarted.loadDownloadsFile(app.downloadsFile, now); err != nil {
		t.Fatal(err)
	}
	if len(restarted.downloads) != 1 || restarted.downloads[downloadKey("TBS", "20261011200000")] == nil {
		// Interrupted downloads are not failures, and old ones expire.
		t.Errorf("restored %+v, want only the recent completed download", restarted.downloads)
	}
	if err := restarted.loadDownloadsFile(filepath.Join(t.TempDir(), "missing.json"), now); err != nil {
		t.Errorf("a missing file is an empty history, got %v", err)
	}
	if _, err := os.Stat(app.downloadsFile + ".tmp"); !os.IsNotExist(err) {
		t.Error("left a temporary file behind")
	}
}

func TestPartsOfASplitProgramAreDownloadedSeparately(t *testing.T) {
	app, _ := ruleTestApp(t, radikron.Rules{{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}}})
	// radiko gives both parts of a split program the same ID.
	parts := radikron.Progs{
		{ID: "p1", StationID: "TBS", Title: "Show (1)", Ft: "29990101100000", To: "29990101110000"},
		{ID: "p1", StationID: "TBS", Title: "Show (2)", Ft: "29990101110000", To: "29990101120000"},
	}
	app.asset.AvailableStations = []string{"TBS"}
	fetch := func(string) (radikron.Progs, error) { return parts, nil }

	f := &fakeDownloads{app: app}
	processed := map[string]bool{}
	for _, pws := range app.collectProgramsFromStations(app.asset, fetch) {
		app.processProgram(pws, app.asset, context.Background(), processed, f.download)
	}
	if f.calls != 2 {
		t.Errorf("download tried %d times, want once for each part", f.calls)
	}
	if len(app.asset.Schedules) != 2 {
		t.Errorf("schedules = %+v, want both parts", app.asset.Schedules)
	}
}

func TestBothPartsOfASplitProgramCanBeAddedFromSearch(t *testing.T) {
	app, _ := ruleTestApp(t, nil)
	app.manualInjectionsFile = filepath.Join(t.TempDir(), "manual-injections.json")
	for _, part := range []*radikron.Prog{
		{ID: "p1", StationID: "TBS", Title: "Show (1)", Ft: "29990101100000", To: "29990101110000"},
		{ID: "p1", StationID: "TBS", Title: "Show (2)", Ft: "29990101110000", To: "29990101120000"},
	} {
		if err := app.InjectProgram(part, ""); err != nil {
			t.Fatal(err)
		}
	}
	if len(app.manualInjections) != 2 || len(app.asset.Schedules) != 2 {
		t.Errorf("injections = %d, schedules = %d, want both parts", len(app.manualInjections), len(app.asset.Schedules))
	}
	if !app.IsManualInjection("TBS", "29990101110000") {
		t.Error("the second part is not a manual injection")
	}
}

func TestClearHistoryForgetsFinishedAndSkippedDownloads(t *testing.T) {
	app, _ := ruleTestApp(t, radikron.Rules{{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}}})
	app.downloadsFile = filepath.Join(t.TempDir(), "downloads.json")
	now := time.Now()
	event := func(name, start string, extra ...string) {
		data := map[string]any{"station": "TBS", "title": "Show", "start": start}
		if len(extra) > 0 {
			data["error"] = extra[0]
		}
		app.recordDownloadEvent(name, data, now)
	}
	event("download-completed", "20261011200000")
	event("download-failed", "20261011210000", "HTTP 403")
	event("download-started", "20261011220000")
	processOnce(app, &fakeDownloads{app: app})
	if err := app.SkipDownload("TBS", showProg.Ft); err != nil {
		t.Fatal(err)
	}

	if err := app.ClearHistory(); err != nil {
		t.Fatal(err)
	}
	if len(app.skipped) != 0 {
		t.Errorf("skipped = %+v, want none", app.skipped)
	}
	if len(app.downloads) != 1 || app.downloads[downloadKey("TBS", "20261011220000")] == nil {
		t.Errorf("downloads = %+v, want only the one in progress", app.downloads)
	}
	restarted := NewApp()
	if err := restarted.loadDownloadsFile(app.downloadsFile, now); err != nil {
		t.Fatal(err)
	}
	if len(restarted.downloads) != 0 || len(restarted.skipped) != 0 {
		t.Errorf("the saved history was not cleared: downloads=%d skipped=%d", len(restarted.downloads), len(restarted.skipped))
	}
}

func TestAProgramWhoseFileExistsShowsAsDownloaded(t *testing.T) {
	app := NewApp()
	skip := func(reason string) (changed, finished bool) {
		return app.recordDownloadEvent("download-skipped", map[string]any{
			"reason": reason, "station": "TBS", "title": "Show", "start": "20261011200000",
		}, time.Now())
	}
	if changed, _ := skip("not yet available"); changed {
		t.Error("a program skipped for another reason was recorded")
	}
	if changed, finished := skip("already exists"); !changed || !finished {
		t.Error("a program whose file exists was not recorded")
	}
	if rec := app.downloads[downloadKey("TBS", "20261011200000")]; rec == nil || rec.item.State != DownloadCompleted {
		t.Fatalf("record = %+v, want completed", rec)
	}
	if changed, _ := skip("already exists"); changed {
		t.Error("each later check updated the record again")
	}
}

func TestUndoSkipKeepsTheSkipWhenAddingFails(t *testing.T) {
	app, _ := ruleTestApp(t, nil)
	key := downloadKey("TBS", "not-a-time")
	app.skipped = map[string]*skippedProgram{}
	app.skipped[key] = &skippedProgram{StationID: "TBS", Start: "not-a-time", Title: "Show", Manual: true}
	if err := app.UndoSkip("TBS", "not-a-time"); err == nil {
		t.Fatal("UndoSkip succeeded for a program that cannot be added")
	}
	if app.skipped[key] == nil {
		t.Error("a failed undo dropped the skip")
	}
}
