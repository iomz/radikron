package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/iomz/radikron"
)

const (
	// downloadsFileVersion is the format of the downloads file.
	downloadsFileVersion = 1
	// completedRetention is how long finished downloads stay in the history.
	completedRetention = 90 * 24 * time.Hour
	// failedRetention is how long failed downloads stay in the history; rules
	// try them again, and radiko drops them after a week anyway.
	failedRetention = archiveRetention
)

// skippedProgram is a program the user skipped or canceled. Checks leave it
// alone, so a rule does not download it again, until the user undoes the skip
// or radiko no longer has it.
type skippedProgram struct {
	ProgramID  string    `json:"programId"`
	StationID  string    `json:"stationId"`
	Start      string    `json:"start"`
	End        string    `json:"end"`
	Title      string    `json:"title"`
	RuleName   string    `json:"ruleName"`
	RuleFolder string    `json:"ruleFolder"`
	Manual     bool      `json:"manual"`
	SkippedAt  time.Time `json:"skippedAt"`
}

func (s *skippedProgram) item() DownloadItem {
	return DownloadItem{
		ID:        s.ProgramID,
		StationID: s.StationID,
		Title:     s.Title,
		Start:     s.Start,
		End:       s.End,
		RuleName:  s.RuleName,
		Manual:    s.Manual,
		State:     DownloadSkipped,
		UpdatedAt: formatStatusTime(s.SkippedAt),
	}
}

// historyEntry is a finished download in the downloads file.
type historyEntry struct {
	Item      DownloadItem `json:"item"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

// downloadsFile is what Radikron remembers about downloads between runs.
type downloadsFile struct {
	Version int              `json:"version"`
	History []historyEntry   `json:"history"`
	Skipped []skippedProgram `json:"skipped"`
}

// loadDownloadsFile restores the download history and skipped programs. A
// missing file means there is nothing to restore.
func (a *App) loadDownloadsFile(path string, now time.Time) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var file downloadsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	a.downloadsMu.Lock()
	defer a.downloadsMu.Unlock()
	if a.downloads == nil {
		a.downloads = make(map[string]*downloadRecord)
	}
	if a.skipped == nil {
		a.skipped = make(map[string]*skippedProgram)
	}
	for i := range file.History {
		entry := &file.History[i]
		if entry.Item.State != DownloadCompleted && entry.Item.State != DownloadFailed {
			continue
		}
		key := downloadKey(entry.Item.StationID, entry.Item.Start)
		if _, seen := a.downloads[key]; !seen {
			a.downloads[key] = &downloadRecord{item: entry.Item, updatedAt: entry.UpdatedAt}
		}
	}
	for i := range file.Skipped {
		skip := &file.Skipped[i]
		a.skipped[downloadKey(skip.StationID, skip.Start)] = skip
	}
	a.pruneDownloadsLocked(now)
	return nil
}

// saveDownloadsFile writes the download history and skipped programs.
func (a *App) saveDownloadsFile() error {
	a.downloadsSaveMu.Lock()
	defer a.downloadsSaveMu.Unlock()

	a.downloadsMu.Lock()
	path := a.downloadsFile
	file := downloadsFile{Version: downloadsFileVersion, History: []historyEntry{}, Skipped: []skippedProgram{}}
	for _, rec := range a.downloads {
		if rec.item.State == DownloadCompleted || rec.item.State == DownloadFailed {
			file.History = append(file.History, historyEntry{Item: rec.item, UpdatedAt: rec.updatedAt})
		}
	}
	for _, skip := range a.skipped {
		file.Skipped = append(file.Skipped, *skip)
	}
	a.downloadsMu.Unlock()
	if path == "" {
		return nil
	}

	sort.Slice(file.History, func(i, j int) bool { return file.History[i].UpdatedAt.After(file.History[j].UpdatedAt) })
	sort.Slice(file.Skipped, func(i, j int) bool { return file.Skipped[i].Start < file.Skipped[j].Start })
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), defaultDirPerm); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, manualInjectionsFilePerm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// saveDownloadsFileInBackground saves without blocking the caller, logging a
// failure.
func (a *App) saveDownloadsFileInBackground() {
	go func() {
		if err := a.saveDownloadsFile(); err != nil {
			a.logger.Error(fmt.Sprintf("Failed to save download history: %v", err))
		}
	}()
}

// findProgramLocked finds a scheduled program by station and start time.
// Callers hold a.mu.
func (a *App) findProgramLocked(stationID, start string) *radikron.Prog {
	if a.asset == nil {
		return nil
	}
	for _, prog := range a.asset.Schedules {
		if prog.StationID == stationID && prog.Ft == start {
			return prog
		}
	}
	return nil
}

// SkipDownload skips a scheduled program, or cancels it while it downloads.
// Rules leave it alone afterwards, even after a restart, until UndoSkip.
func (a *App) SkipDownload(stationID, start string) error {
	key := downloadKey(stationID, start)
	skip := &skippedProgram{StationID: stationID, Start: start, SkippedAt: time.Now()}

	a.mu.RLock()
	prog := a.findProgramLocked(stationID, start)
	if prog != nil {
		skip.ProgramID, skip.End, skip.Title = prog.ID, prog.To, prog.Title
		skip.RuleName, skip.RuleFolder = prog.RuleName, prog.RuleFolder
		skip.Manual = a.manualInjections[key] != nil
	}
	a.mu.RUnlock()

	a.downloadsMu.Lock()
	rec := a.downloads[key]
	if prog == nil && rec == nil {
		a.downloadsMu.Unlock()
		return fmt.Errorf("no download for %s at %s", stationID, start)
	}
	if prog == nil {
		skip.ProgramID, skip.End, skip.Title = rec.item.ID, rec.item.End, rec.item.Title
		skip.RuleName, skip.Manual = rec.item.RuleName, rec.item.Manual
	}
	if a.skipped == nil {
		a.skipped = make(map[string]*skippedProgram)
	}
	a.skipped[key] = skip
	if d := a.active[key]; d != nil {
		// The core reports the cancel as a failure, which
		// recordDownloadEvent drops for skipped programs.
		d.cancel()
	}
	if rec != nil && rec.item.State != DownloadDownloading {
		delete(a.downloads, key)
	}
	a.downloadsMu.Unlock()

	if skip.Manual {
		// A skipped program added from Search is not downloaded later; UndoSkip
		// adds it again.
		a.removeManualInjectionAndSave(key, stationID, skip.Title)
	}
	a.saveDownloadsFileInBackground()
	a.emit(downloadsChangedEvent, nil)
	return nil
}

// UndoSkip lets Radikron download a skipped program again: rules pick it up
// at the next check, which starts now, and a program added from Search is
// added again.
func (a *App) UndoSkip(stationID, start string) error {
	key := downloadKey(stationID, start)
	a.downloadsMu.Lock()
	skip, ok := a.skipped[key]
	delete(a.skipped, key)
	a.downloadsMu.Unlock()
	if !ok {
		return fmt.Errorf("%s at %s is not skipped", stationID, start)
	}

	a.saveDownloadsFileInBackground()
	if skip.Manual {
		prog := &radikron.Prog{
			ID: skip.ProgramID, StationID: skip.StationID, Ft: skip.Start, To: skip.End,
			Title: skip.Title, RuleName: skip.RuleName, RuleFolder: skip.RuleFolder,
		}
		if err := a.InjectProgram(prog, skip.RuleName); err != nil {
			return err
		}
	} else {
		a.signalConfigChanged()
	}
	a.emit(downloadsChangedEvent, nil)
	return nil
}

// ClearHistory forgets finished downloads and skipped programs. Downloads in
// progress go on, and files stay on disk. A check starts now: rules may
// download programs that were skipped, and programs whose files are already
// there show as downloaded again.
func (a *App) ClearHistory() error {
	a.downloadsMu.Lock()
	for key, rec := range a.downloads {
		if rec.item.State != DownloadDownloading {
			delete(a.downloads, key)
		}
	}
	a.skipped = make(map[string]*skippedProgram)
	a.downloadsMu.Unlock()

	if err := a.saveDownloadsFile(); err != nil {
		return fmt.Errorf("failed to save download history: %w", err)
	}
	a.signalConfigChanged()
	a.emit(downloadsChangedEvent, nil)
	return nil
}
