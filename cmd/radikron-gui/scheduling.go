package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/iomz/radikron"
)

// activeDownload is a download the GUI asked the core to start. Each has its
// own context so it can be canceled alone.
type activeDownload struct {
	cancel context.CancelFunc
	// started is set when the core reports download-started; until then the
	// core may still decide to skip the program.
	started bool
}

// downloadFunc starts a download; radikron.Download in production.
type downloadFunc func(context.Context, *sync.WaitGroup, *radikron.Prog) error

// startDownload asks the core to download prog under a context of its own. It
// does nothing when the program is already downloading, so checks and retries
// can call it for every program they want on disk.
func (a *App) startDownload(ctx context.Context, wg *sync.WaitGroup, prog *radikron.Prog, download downloadFunc) error {
	key := downloadKey(prog.StationID, prog.Ft)
	a.downloadsMu.Lock()
	if a.active == nil {
		a.active = make(map[string]*activeDownload)
	}
	if _, running := a.active[key]; running {
		a.downloadsMu.Unlock()
		return nil
	}
	downloadCtx, cancel := context.WithCancel(ctx)
	a.active[key] = &activeDownload{cancel: cancel}
	a.downloadsMu.Unlock()

	err := download(downloadCtx, wg, prog)

	// The core returns without starting a download for programs that are not
	// available yet or already on disk.
	a.downloadsMu.Lock()
	if d := a.active[key]; d != nil && !d.started {
		delete(a.active, key)
		cancel()
	}
	a.downloadsMu.Unlock()
	return err
}

// trackDownloadEventLocked follows a download through the core's events.
// Callers hold a.downloadsMu.
func (a *App) trackDownloadEventLocked(key string, state DownloadState) {
	d := a.active[key]
	if d == nil {
		return
	}
	switch state {
	case DownloadDownloading:
		d.started = true
	case DownloadCompleted, DownloadFailed:
		delete(a.active, key)
		d.cancel()
	}
}

// activeKeys returns the downloads in progress, by downloadKey.
func (a *App) activeKeys() map[string]bool {
	a.downloadsMu.Lock()
	defer a.downloadsMu.Unlock()
	keys := make(map[string]bool, len(a.active))
	for key := range a.active {
		keys[key] = true
	}
	return keys
}

// fileOnDisk reports whether prog's file exists where it would be saved.
func fileOnDisk(prog *radikron.Prog, asset *radikron.Asset) bool {
	startTime, err := time.ParseInLocation(radikron.DatetimeLayout, prog.Ft, radikron.Location)
	if err != nil {
		return false
	}
	fileBaseName := fmt.Sprintf(
		"%s_%s_%s",
		startTime.In(radikron.Location).Format(radikron.OutputDatetimeLayout),
		prog.StationID,
		prog.Title,
	)
	output, err := radikron.NewOutputConfig(fileBaseName, asset.OutputFormat, asset.DownloadDir, prog.RuleFolder)
	return err == nil && output.IsExist()
}

// leaveAlone reports whether a check should not try to download prog: it is
// downloading, the user skipped it, or its file exists.
func (a *App) leaveAlone(prog *radikron.Prog, asset *radikron.Asset) bool {
	key := downloadKey(prog.StationID, prog.Ft)
	a.downloadsMu.Lock()
	_, running := a.active[key]
	_, skipped := a.skipped[key]
	a.downloadsMu.Unlock()
	return running || skipped || fileOnDisk(prog, asset)
}

// rememberScheduledLocked adds prog to the schedule, replacing an older copy
// of the same program. Callers hold a.mu.
func (a *App) rememberScheduledLocked(prog *radikron.Prog) {
	for i, s := range a.asset.Schedules {
		if s.ID == prog.ID {
			a.asset.Schedules[i] = prog
			return
		}
	}
	a.asset.Schedules = append(a.asset.Schedules, prog)
}

// pruneSchedulesLocked drops scheduled programs that no rule matches any
// more and refreshes the rule of the others, so a removed or edited rule
// stops downloads that have not started. Programs added from Search and
// downloads in progress stay. Callers hold a.mu; active comes from activeKeys.
func (a *App) pruneSchedulesLocked(active map[string]bool) bool {
	if a.asset == nil {
		return false
	}
	changed := false
	kept := a.asset.Schedules[:0]
	for _, prog := range a.asset.Schedules {
		if a.manualInjections[prog.ID] != nil || active[downloadKey(prog.StationID, prog.Ft)] {
			kept = append(kept, prog)
			continue
		}
		rule := a.asset.Rules.FindMatchSilent(prog.StationID, prog)
		if rule == nil {
			changed = true
			continue
		}
		if prog.RuleName != rule.Name || prog.RuleFolder != rule.Folder {
			prog.RuleName, prog.RuleFolder = rule.Name, rule.Folder
			changed = true
		}
		kept = append(kept, prog)
	}
	clear(a.asset.Schedules[len(kept):])
	a.asset.Schedules = kept
	return changed
}
