package main

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/iomz/radikron"
	"github.com/iomz/radikron/internal/config"
)

func testSettings(downloadDir string) Settings {
	return Settings{
		AreaID:                    "JP27",
		DownloadDir:               downloadDir,
		FileFormat:                radikron.AudioFormatMP3,
		MinimumOutputSizeMB:       2,
		ExtraStations:             []string{" TBS ", "TBS", ""},
		IgnoreStations:            []string{"QRR"},
		MaxDownloadingConcurrency: 8,
		MaxEncodingConcurrency:    1,
	}
}

func ptr[T any](v T) *T { return &v }

// settingsOf reads the settings back from a loaded config.
func settingsOf(c *config.Config) Settings {
	return Settings{
		AreaID: c.AreaID, DownloadDir: c.DownloadDir, FileFormat: c.FileFormat,
		MinimumOutputSizeMB: c.MinimumOutputSize / (radikron.Kilobytes * radikron.Kilobytes),
		ExtraStations:       c.ExtraStations, IgnoreStations: c.IgnoreStations,
		MaxDownloadingConcurrency: c.MaxDownloadingConcurrency, MaxEncodingConcurrency: c.MaxEncodingConcurrency,
	}
}

func TestSaveSettingsSavesAndKeepsRules(t *testing.T) {
	app, events := ruleTestApp(t, radikron.Rules{{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}}})
	dir := t.TempDir()
	if err := app.SaveSettings(ptr(testSettings(dir))); err != nil {
		t.Fatal(err)
	}

	saved, err := config.LoadConfigWithoutAreaLookup(app.configFile)
	if err != nil {
		t.Fatal(err)
	}
	want := testSettings(dir)
	want.ExtraStations = []string{"TBS"}
	if got := settingsOf(saved); !reflect.DeepEqual(got, want) {
		t.Errorf("saved settings = %+v, want %+v", got, want)
	}
	if len(saved.Rules) != 1 || saved.Rules[0].Name != "Show" {
		t.Errorf("saving settings changed the rules: %+v", saved.Rules)
	}
	if app.asset.OutputFormat != radikron.AudioFormatMP3 || app.asset.DownloadDir != dir {
		t.Errorf("settings not applied: format=%s dir=%s", app.asset.OutputFormat, app.asset.DownloadDir)
	}
	if !slices.Contains(events.names(), "stations-loaded") {
		t.Errorf("events = %v, want stations-loaded so the station list reloads", events.names())
	}

	got, err := app.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.MinimumOutputSizeMB != 2 || got.AreaID != "JP27" || !slices.Equal(got.ExtraStations, []string{"TBS"}) {
		t.Errorf("GetSettings() = %+v", got)
	}
}

func TestSaveSettingsRejectsInvalidValues(t *testing.T) {
	app, _ := ruleTestApp(t, nil)
	before := *app.config
	for name, change := range map[string]func(*Settings){
		"area":       func(s *Settings) { s.AreaID = "JP48" },
		"folder":     func(s *Settings) { s.DownloadDir = " " },
		"format":     func(s *Settings) { s.FileFormat = "flac" },
		"size":       func(s *Settings) { s.MinimumOutputSizeMB = -1 },
		"downloads":  func(s *Settings) { s.MaxDownloadingConcurrency = 0 },
		"encodings":  func(s *Settings) { s.MaxEncodingConcurrency = 0 },
		"no archive": func(s *Settings) { s.ExtraStations = []string{"JOAK"} },
	} {
		s := testSettings(t.TempDir())
		change(&s)
		if err := app.SaveSettings(&s); err == nil {
			t.Errorf("%s: saved invalid settings %+v", name, s)
		}
	}
	if app.config.AreaID != before.AreaID || app.config.FileFormat != before.FileFormat {
		t.Errorf("rejected settings changed the config: %+v", app.config)
	}
}

func TestSaveSettingsKeepsPreviousConfigWhenSaveFails(t *testing.T) {
	app, _ := ruleTestApp(t, nil)
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	app.configFile = filepath.Join(blocker, "config.yml")
	err := app.SaveSettings(ptr(testSettings(t.TempDir())))
	if err == nil || !strings.Contains(err.Error(), "failed to save config") {
		t.Fatalf("SaveSettings() error = %v, want a save failure", err)
	}
	if app.config.FileFormat != radikron.AudioFormatAAC || app.asset.OutputFormat != radikron.AudioFormatAAC {
		t.Errorf("failed save left the settings applied: config=%s asset=%s", app.config.FileFormat, app.asset.OutputFormat)
	}
}

func TestImportConfigReplacesSettingsAndRules(t *testing.T) {
	app, _ := ruleTestApp(t, radikron.Rules{{Name: "Old", Criteria: radikron.Criteria{Title: "Old"}}})
	ownFile := app.configFile
	dir := t.TempDir()
	other := testConfig(dir)
	other.FileFormat = radikron.AudioFormatMP3
	other.Rules = radikron.Rules{{Name: "New", Criteria: radikron.Criteria{Keyword: "jazz"}}}
	source := filepath.Join(t.TempDir(), "other.yml")
	if err := other.SaveConfig(source); err != nil {
		t.Fatal(err)
	}

	if err := app.importConfig(source); err != nil {
		t.Fatal(err)
	}
	if app.configFile != ownFile {
		t.Errorf("config file = %s, want Radikron's own %s", app.configFile, ownFile)
	}
	saved, err := loadConfigFile(ownFile)
	if err != nil {
		t.Fatal(err)
	}
	if saved.FileFormat != radikron.AudioFormatMP3 || saved.DownloadDir != dir {
		t.Errorf("saved settings = %+v, want the imported ones", settingsOf(saved))
	}
	if len(saved.Rules) != 1 || saved.Rules[0].Name != "New" {
		t.Errorf("saved rules = %+v, want the imported ones", saved.Rules)
	}
	if app.asset.OutputFormat != radikron.AudioFormatMP3 {
		t.Error("imported settings were not applied")
	}
}

func TestImportConfigKeepsConfigWhenFileIsBad(t *testing.T) {
	app, _ := ruleTestApp(t, radikron.Rules{{Name: "Old", Criteria: radikron.Criteria{Title: "Old"}}})
	bad := testConfig(t.TempDir())
	bad.FileFormat = "flac"
	source := filepath.Join(t.TempDir(), "bad.yml")
	if err := bad.SaveConfig(source); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(t.TempDir(), "broken.yml")
	if err := os.WriteFile(broken, []byte("rules: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{source, broken, filepath.Join(t.TempDir(), "missing.yml")} {
		if err := app.importConfig(path); err == nil {
			t.Errorf("importing %s succeeded", filepath.Base(path))
		}
	}
	if app.config.FileFormat != radikron.AudioFormatAAC || len(app.config.Rules) != 1 || app.config.Rules[0].Name != "Old" {
		t.Errorf("a failed import changed the config: %+v", app.config)
	}
}

func TestImportConfigRejectsInvalidRules(t *testing.T) {
	app, _ := ruleTestApp(t, radikron.Rules{{Name: "Old", Criteria: radikron.Criteria{Title: "Old"}}})
	for name, rules := range map[string]radikron.Rules{
		"no criteria": {{Name: "Empty"}},
		"bad day":     {{Name: "Bad", Criteria: radikron.Criteria{Title: "Show", DoW: []string{"someday"}}}},
		"same name":   {{Name: "Show", Criteria: radikron.Criteria{Title: "A"}}, {Name: " Show", Criteria: radikron.Criteria{Title: "B"}}},
	} {
		other := testConfig(t.TempDir())
		other.Rules = rules
		source := filepath.Join(t.TempDir(), "other.yml")
		if err := other.SaveConfig(source); err != nil {
			t.Fatal(err)
		}
		if err := app.importConfig(source); err == nil {
			t.Errorf("%s: import succeeded", name)
		}
	}
	if len(app.config.Rules) != 1 || app.config.Rules[0].Name != "Old" {
		t.Errorf("a rejected import changed the rules: %+v", app.config.Rules)
	}
}
