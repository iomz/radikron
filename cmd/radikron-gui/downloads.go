package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/iomz/radikron"
)

// downloadsChangedEvent tells the GUI to reload GetDownloads.
const downloadsChangedEvent = "downloads-changed"

const (
	// archiveRetention is how long radiko keeps time-free recordings after a
	// program starts; older programs cannot be downloaded any more.
	archiveRetention = 7 * 24 * time.Hour
	// maxRecentDownloads bounds the history of finished downloads.
	maxRecentDownloads = 500
)

// DownloadState is where a program is in the download lifecycle.
type DownloadState string

const (
	// DownloadWaiting means the program has not finished airing, or its
	// recording is not available yet.
	DownloadWaiting DownloadState = "waiting"
	// DownloadAvailable means the recording is available and the next check
	// downloads it.
	DownloadAvailable   DownloadState = "available"
	DownloadDownloading DownloadState = "downloading"
	DownloadCompleted   DownloadState = "completed"
	DownloadFailed      DownloadState = "failed"
	// DownloadExpired means radiko no longer offers the recording.
	DownloadExpired DownloadState = "expired"
	// DownloadSkipped means the user skipped or canceled the program.
	DownloadSkipped DownloadState = "skipped"
)

// DownloadItem is a program Radikron will download, is downloading, has
// tried to download, or was told to skip.
type DownloadItem struct {
	ID        string `json:"id"`
	StationID string `json:"stationId"`
	Title     string `json:"title"`
	// Start and End use radiko's YYYYMMDDHHmmss format in Japan time, like
	// Prog.Ft and Prog.To. End is "" when unknown.
	Start    string `json:"start"`
	End      string `json:"end"`
	RuleName string `json:"ruleName"`
	// Manual is true for programs added from Program Search rather than
	// matched by a rule.
	Manual bool          `json:"manual"`
	State  DownloadState `json:"state"`
	// Stage names the current step while downloading, or the step that
	// failed.
	Stage     string `json:"stage"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Error     string `json:"error"`
	// UpdatedAt is when the state last changed, as RFC 3339 in local time;
	// "" for programs that have not started downloading.
	UpdatedAt string `json:"updatedAt"`
}

// Downloads groups the programs the GUI shows.
type Downloads struct {
	// Scheduled lists programs that are not on disk yet, ordered by start.
	Scheduled []DownloadItem `json:"scheduled"`
	// Recent lists completed and failed downloads, newest first. They are
	// kept between runs.
	Recent []DownloadItem `json:"recent"`
	// Skipped lists programs the user skipped or canceled, ordered by start.
	Skipped []DownloadItem `json:"skipped"`
}

// downloadRecord is the last download event seen for a program.
type downloadRecord struct {
	item      DownloadItem
	updatedAt time.Time
}

func downloadKey(stationID, start string) string {
	return stationID + "|" + start
}

// emitDownloadEvent forwards a download event from the core to the GUI and
// records the program's download state.
func (a *App) emitDownloadEvent(ctx context.Context, name string, data any) {
	if a.events != nil {
		a.events.Emit(ctx, name, data)
	}
	changed, finished := a.recordDownloadEvent(name, data, time.Now())
	if finished {
		a.saveDownloadsFileInBackground()
	}
	if changed {
		a.emit(downloadsChangedEvent, nil)
	}
}

// recordDownloadEvent updates the download state from a core event. It
// reports whether anything changed and whether a download finished, which
// changes the saved history.
func (a *App) recordDownloadEvent(name string, data any, now time.Time) (changed, finished bool) {
	fields, ok := data.(map[string]any)
	if !ok {
		return false, false
	}
	text := func(key string) string {
		s, _ := fields[key].(string)
		return s
	}
	number := func(key string) int {
		n, _ := fields[key].(int)
		return n
	}
	stationID, start := text("station"), text("start")
	if stationID == "" || start == "" {
		return false, false
	}

	var state DownloadState
	switch name {
	case "download-started", "download-progress":
		state = DownloadDownloading
	case "download-completed":
		state = DownloadCompleted
	case "download-failed":
		state = DownloadFailed
	default:
		return false, false
	}

	a.downloadsMu.Lock()
	defer a.downloadsMu.Unlock()
	if a.downloads == nil {
		a.downloads = make(map[string]*downloadRecord)
	}
	key := downloadKey(stationID, start)
	a.trackDownloadEventLocked(key, state)
	_, skipped := a.skipped[key]
	// A skipped program's download was canceled on purpose, and a download
	// interrupted by quitting is tried again later; neither is a failure.
	interrupted := state == DownloadFailed && strings.Contains(text("error"), context.Canceled.Error())
	if skipped || interrupted {
		_, had := a.downloads[key]
		delete(a.downloads, key)
		return had, had
	}
	rec := a.downloads[key]
	if rec == nil {
		rec = &downloadRecord{item: DownloadItem{StationID: stationID, Start: start}}
		a.downloads[key] = rec
	}
	if title := text("title"); title != "" {
		rec.item.Title = title
	}
	rec.item.State = state
	rec.item.Stage = text("stage")
	rec.item.Completed = number("completed")
	rec.item.Total = number("total")
	rec.item.Error = text("error")
	rec.updatedAt = now
	a.pruneDownloadsLocked(now)
	return true, state == DownloadCompleted || state == DownloadFailed
}

// pruneDownloadsLocked forgets finished downloads past their retention,
// keeping at most maxRecentDownloads, and skipped programs radiko no longer
// has. Callers hold a.downloadsMu.
func (a *App) pruneDownloadsLocked(now time.Time) {
	finished := make([]string, 0, len(a.downloads))
	for key, rec := range a.downloads {
		retention := completedRetention
		switch rec.item.State {
		case DownloadDownloading:
			continue
		case DownloadFailed:
			retention = failedRetention
		}
		if now.Sub(rec.updatedAt) > retention {
			delete(a.downloads, key)
			continue
		}
		finished = append(finished, key)
	}
	for key, skip := range a.skipped {
		start, err := time.ParseInLocation(radikron.DatetimeLayout, skip.Start, radikron.Location)
		if err == nil && now.Sub(start) > archiveRetention {
			delete(a.skipped, key)
		}
	}
	if len(finished) <= maxRecentDownloads {
		return
	}
	sort.Slice(finished, func(i, j int) bool {
		return a.downloads[finished[i]].updatedAt.Before(a.downloads[finished[j]].updatedAt)
	})
	for _, key := range finished[:len(finished)-maxRecentDownloads] {
		delete(a.downloads, key)
	}
}

// GetDownloads returns scheduled programs with their download state, the
// finished downloads, and the skipped programs.
func (a *App) GetDownloads() (Downloads, error) {
	a.downloadsMu.Lock()
	records := make(map[string]*downloadRecord, len(a.downloads))
	for key, rec := range a.downloads {
		snapshot := *rec
		records[key] = &snapshot
	}
	skipped := make(map[string]skippedProgram, len(a.skipped))
	for key, skip := range a.skipped {
		skipped[key] = *skip
	}
	a.downloadsMu.Unlock()

	scheduled, err := a.GetSchedules()
	if err != nil {
		return Downloads{}, err
	}
	now := time.Now()
	result := Downloads{Scheduled: []DownloadItem{}, Recent: []DownloadItem{}, Skipped: []DownloadItem{}}
	for _, prog := range scheduled {
		if _, ok := skipped[downloadKey(prog.StationID, prog.Ft)]; ok {
			continue
		}
		item := DownloadItem{
			ID:        prog.ID,
			StationID: prog.StationID,
			Title:     prog.Title,
			Start:     prog.Ft,
			End:       prog.To,
			RuleName:  prog.RuleName,
			Manual:    prog.IsManualInjection,
			State:     scheduledState(prog, now),
		}
		if rec, ok := records[downloadKey(prog.StationID, prog.Ft)]; ok {
			applyRecord(&item, rec)
		}
		result.Scheduled = append(result.Scheduled, item)
	}
	a.rememberScheduleDetails(result.Scheduled)
	sort.SliceStable(result.Scheduled, func(i, j int) bool {
		return result.Scheduled[i].Start < result.Scheduled[j].Start
	})

	recent := make([]*downloadRecord, 0, len(records))
	for _, rec := range records {
		if rec.item.State == DownloadCompleted || rec.item.State == DownloadFailed {
			recent = append(recent, rec)
		}
	}
	for key := range skipped {
		skip := skipped[key]
		result.Skipped = append(result.Skipped, skip.item())
	}
	sort.Slice(result.Skipped, func(i, j int) bool { return result.Skipped[i].Start < result.Skipped[j].Start })

	sort.Slice(recent, func(i, j int) bool { return recent[i].updatedAt.After(recent[j].updatedAt) })
	for _, rec := range recent {
		item := rec.item
		item.UpdatedAt = formatStatusTime(rec.updatedAt)
		result.Recent = append(result.Recent, item)
	}
	return result, nil
}

// rememberScheduleDetails copies what only the schedule knows (end time,
// rule, source) into the download records, so a program keeps them in the
// recent results after it leaves the schedule.
func (a *App) rememberScheduleDetails(items []DownloadItem) {
	a.downloadsMu.Lock()
	defer a.downloadsMu.Unlock()
	for i := range items {
		item := &items[i]
		if rec := a.downloads[downloadKey(item.StationID, item.Start)]; rec != nil {
			rec.item.ID = item.ID
			rec.item.End = item.End
			rec.item.RuleName = item.RuleName
			rec.item.Manual = item.Manual
		}
	}
}

func applyRecord(item *DownloadItem, rec *downloadRecord) {
	item.State = rec.item.State
	item.Stage = rec.item.Stage
	item.Completed = rec.item.Completed
	item.Total = rec.item.Total
	item.Error = rec.item.Error
	item.UpdatedAt = formatStatusTime(rec.updatedAt)
}

// scheduledState derives the state of a scheduled program that has not
// started downloading from its air time.
func scheduledState(prog *radikron.Prog, now time.Time) DownloadState {
	start, err := time.ParseInLocation(radikron.DatetimeLayout, prog.Ft, radikron.Location)
	if err != nil {
		return DownloadWaiting
	}
	if now.Sub(start) > archiveRetention {
		return DownloadExpired
	}
	end, err := time.ParseInLocation(radikron.DatetimeLayout, prog.To, radikron.Location)
	if err != nil || now.Before(end.Add(radikron.BufferMinutes*time.Minute)) {
		return DownloadWaiting
	}
	return DownloadAvailable
}

// downloadFailedEvent is the event for a download that failed before the core
// started it.
func downloadFailedEvent(prog *radikron.Prog, err error) map[string]any {
	return map[string]any{
		"station": prog.StationID,
		"title":   prog.Title,
		"start":   prog.Ft,
		"error":   fmt.Sprint(err),
	}
}
