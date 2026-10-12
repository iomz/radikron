package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/iomz/radikron"
	"github.com/iomz/radikron/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// areaIDPattern matches radiko's area IDs, JP1 to JP47.
var areaIDPattern = regexp.MustCompile(`^JP([1-9]|[123]\d|4[0-7])$`)

// Settings are the parts of the configuration the Settings screen edits.
// Rules are edited on their own, so saving settings never overwrites them.
type Settings struct {
	AreaID                    string   `json:"areaId"`
	DownloadDir               string   `json:"downloadDir"`
	FileFormat                string   `json:"fileFormat"`
	MinimumOutputSizeMB       int64    `json:"minimumOutputSizeMb"`
	ExtraStations             []string `json:"extraStations"`
	IgnoreStations            []string `json:"ignoreStations"`
	MaxDownloadingConcurrency int      `json:"maxDownloadingConcurrency"`
	MaxEncodingConcurrency    int      `json:"maxEncodingConcurrency"`
}

// GetSettings returns the current settings.
func (a *App) GetSettings() (Settings, error) {
	if err := a.waitForStartup(); err != nil {
		return Settings{}, fmt.Errorf("startup failed: %w", err)
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.config == nil {
		return Settings{}, errors.New("config not loaded")
	}
	c := a.config
	return Settings{
		AreaID:                    c.AreaID,
		DownloadDir:               c.DownloadDir,
		FileFormat:                c.FileFormat,
		MinimumOutputSizeMB:       c.MinimumOutputSize / (radikron.Kilobytes * radikron.Kilobytes),
		ExtraStations:             slices.Clone(c.ExtraStations),
		IgnoreStations:            slices.Clone(c.IgnoreStations),
		MaxDownloadingConcurrency: c.MaxDownloadingConcurrency,
		MaxEncodingConcurrency:    c.MaxEncodingConcurrency,
	}, nil
}

// SaveSettings checks s, saves it to the config file, and applies it. If the
// save fails, the running configuration stays as it was.
func (a *App) SaveSettings(s *Settings) error {
	if s == nil {
		return errors.New("settings cannot be nil")
	}
	cleaned := *s
	if err := cleanSettings(&cleaned); err != nil {
		return err
	}
	if err := a.changeConfig(func(cfg *config.Config) error {
		cfg.AreaID = cleaned.AreaID
		cfg.DownloadDir = cleaned.DownloadDir
		cfg.FileFormat = cleaned.FileFormat
		cfg.MinimumOutputSize = cleaned.MinimumOutputSizeMB * radikron.Kilobytes * radikron.Kilobytes
		cfg.ExtraStations = cleaned.ExtraStations
		cfg.IgnoreStations = cleaned.IgnoreStations
		cfg.MaxDownloadingConcurrency = cleaned.MaxDownloadingConcurrency
		cfg.MaxEncodingConcurrency = cleaned.MaxEncodingConcurrency
		return nil
	}); err != nil {
		return err
	}
	// The area or the extra and ignored stations may have changed the list.
	a.emit("stations-loaded", nil)
	return nil
}

// cleanSettings trims s, drops repeated stations, and rejects values the
// config file cannot hold.
func cleanSettings(s *Settings) error {
	s.AreaID = strings.TrimSpace(s.AreaID)
	s.DownloadDir = strings.TrimSpace(s.DownloadDir)
	s.FileFormat = strings.TrimSpace(s.FileFormat)
	switch {
	case !areaIDPattern.MatchString(s.AreaID):
		return fmt.Errorf("unknown area %q", s.AreaID)
	case s.DownloadDir == "":
		return errors.New("choose a download folder")
	case s.FileFormat != radikron.AudioFormatAAC && s.FileFormat != radikron.AudioFormatMP3:
		return fmt.Errorf("unsupported audio format %q", s.FileFormat)
	case s.MinimumOutputSizeMB < 0:
		return errors.New("the minimum file size cannot be negative")
	case s.MaxDownloadingConcurrency < 1 || s.MaxEncodingConcurrency < 1:
		return errors.New("allow at least one download and one encoding at a time")
	}
	s.ExtraStations = uniqueStations(s.ExtraStations)
	s.IgnoreStations = uniqueStations(s.IgnoreStations)
	for _, id := range s.ExtraStations {
		if !radikron.SupportsArchive(id) {
			return fmt.Errorf("%s has no time-free recordings", id)
		}
	}
	return nil
}

// uniqueStations trims station IDs and drops empty and repeated ones,
// keeping the order.
func uniqueStations(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// ChooseDownloadFolder lets the user pick the download folder, starting from
// current. It returns nil when the user cancels.
func (a *App) ChooseDownloadFolder(current string) (*string, error) {
	if a.wails == nil {
		return nil, errors.New("folder dialog unavailable before the app starts")
	}
	start, err := radikron.GetRadikronPath(current)
	if info, statErr := os.Stat(start); err != nil || statErr != nil || !info.IsDir() {
		start = ""
	}
	picked, err := a.wails.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                "Choose the download folder",
		Directory:            start,
		CanChooseDirectories: true,
		CanCreateDirectories: true,
	}).PromptForSingleSelection()
	if err != nil || picked == "" {
		return nil, nil //nolint:nilerr // the dialog reports a cancel as an error
	}
	return &picked, nil
}

// changeConfig applies the change to a copy of the configuration, saves the
// config file, drops scheduled programs no rule matches any more, and wakes
// automatic checks. If the save fails, the running configuration stays as it
// was. Downloads in progress finish.
func (a *App) changeConfig(change func(*config.Config) error) error {
	if err := a.waitForStartup(); err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}

	active := a.activeKeys()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.config == nil || a.asset == nil {
		return errors.New("config not loaded")
	}
	if a.configFile == "" {
		return errors.New("config file path not set")
	}

	previous := a.config
	updated := *previous
	if err := change(&updated); err != nil {
		return err
	}
	if err := updated.ApplyToAsset(a.asset); err != nil {
		if restoreErr := previous.ApplyToAsset(a.asset); restoreErr != nil {
			a.logger.Error(fmt.Sprintf("Failed to restore the previous config: %v", restoreErr))
		}
		return fmt.Errorf("failed to apply the config: %w", err)
	}
	if err := updated.SaveConfig(a.configFile); err != nil {
		// Keep the running configuration in line with the file on disk.
		if restoreErr := previous.ApplyToAsset(a.asset); restoreErr != nil {
			a.logger.Error(fmt.Sprintf("Failed to restore the previous config: %v", restoreErr))
		}
		return fmt.Errorf("failed to save config: %w", err)
	}
	a.config = &updated
	pruned := a.pruneSchedulesLocked(active)
	a.signalConfigChanged()
	a.emit("config-saved", map[string]any{
		"success": true,
		"file":    a.configFile,
	})
	if pruned {
		a.emit(downloadsChangedEvent, nil)
	}
	return nil
}
