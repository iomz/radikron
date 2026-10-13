package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/iomz/radikron"
	"github.com/iomz/radikron/internal/config"
)

// ruleTestApp returns an App with a loaded config saved at a temporary path.
func ruleTestApp(t *testing.T, rules radikron.Rules) (*App, *recordingEventSink) {
	t.Helper()
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Rules = rules
	events := &recordingEventSink{}
	app := NewApp()
	app.events = events
	app.asset = &radikron.Asset{}
	app.config = cfg
	app.configFile = filepath.Join(dir, "config.yml")
	if err := cfg.ApplyToAsset(app.asset); err != nil {
		t.Fatal(err)
	}
	return app, events
}

func TestRuleFromProgramSuggestsUniqueName(t *testing.T) {
	app, _ := ruleTestApp(t, radikron.Rules{
		{Name: "City Pop", Criteria: radikron.Criteria{Keyword: "city pop"}},
		{Name: "City Pop (2)", Criteria: radikron.Criteria{Keyword: "citypop"}},
	})
	rule, err := app.RuleFromProgram(&radikron.Prog{Title: " City Pop ", StationID: "TBS"})
	if err != nil {
		t.Fatal(err)
	}
	if rule.Name != "City Pop (3)" || rule.Title != "City Pop" || rule.StationID != "TBS" {
		t.Errorf("rule = %+v", rule)
	}
	if _, err := app.RuleFromProgram(&radikron.Prog{StationID: "TBS"}); err == nil {
		t.Error("suggested a rule for a program without a title")
	}
}

func TestAddRuleSavesAppliesAndWakesChecks(t *testing.T) {
	app, events := ruleTestApp(t, nil)
	rule := radikron.Rule{Name: " Show ", Criteria: radikron.Criteria{Title: "Show", StationID: "TBS"}}
	if err := app.AddRule(&rule); err != nil {
		t.Fatalf("AddRule() error: %v", err)
	}

	saved, err := config.LoadConfigWithoutAreaLookup(app.configFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Rules) != 1 || saved.Rules[0].Name != "Show" || saved.Rules[0].Title != "Show" {
		t.Errorf("saved rules = %+v", saved.Rules)
	}
	if len(app.asset.Rules) != 1 || len(app.config.Rules) != 1 {
		t.Errorf("rule not applied: asset %d rules, config %d rules", len(app.asset.Rules), len(app.config.Rules))
	}
	select {
	case <-app.configChanged:
	default:
		t.Error("AddRule() did not wake automatic checks")
	}
	if got := events.names(); !slices.Equal(got, []string{"config-saved"}) {
		t.Errorf("events = %v", got)
	}
}

func TestAddRuleRejectsInvalidRulesWithoutChanges(t *testing.T) {
	existing := &radikron.Rule{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}}
	tests := []struct {
		name string
		rule radikron.Rule
		want string
	}{
		{"missing name", radikron.Rule{Criteria: radikron.Criteria{Title: "x"}}, "name is required"},
		{"no criteria", radikron.Rule{Name: "Empty", Criteria: radikron.Criteria{StationID: "TBS"}}, "needs a title"},
		{"duplicate name", radikron.Rule{Name: "Show", Criteria: radikron.Criteria{Title: "x"}}, "already exists"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, events := ruleTestApp(t, radikron.Rules{existing})
			err := app.AddRule(&tt.rule)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("AddRule() error = %v, want %q", err, tt.want)
			}
			if len(app.config.Rules) != 1 || len(events.names()) != 0 {
				t.Errorf("rejected rule changed state: rules=%d events=%v", len(app.config.Rules), events.names())
			}
		})
	}
}

func TestAddRuleKeepsPreviousConfigWhenSaveFails(t *testing.T) {
	app, _ := ruleTestApp(t, nil)
	// A regular file where the config directory should be makes the save fail.
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	app.configFile = filepath.Join(blocker, "config.yml")
	err := app.AddRule(&radikron.Rule{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}})
	if err == nil || !strings.Contains(err.Error(), "failed to save config") {
		t.Fatalf("AddRule() error = %v, want a save failure", err)
	}
	if len(app.config.Rules) != 0 || len(app.asset.Rules) != 0 {
		t.Errorf("failed save left the rule applied: config=%d asset=%d", len(app.config.Rules), len(app.asset.Rules))
	}
}

func TestUpdateRuleReplacesAndRenames(t *testing.T) {
	app, events := ruleTestApp(t, radikron.Rules{
		{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}},
		{Name: "Other", Criteria: radikron.Criteria{Title: "Other"}},
	})
	rule := radikron.Rule{
		Name:     " Renamed ",
		Criteria: radikron.Criteria{Pfm: "Host", DoW: []string{"Sun"}},
		Window:   "48h",
		Exclude:  &radikron.Criteria{},
	}
	if err := app.UpdateRule("Show", &rule); err != nil {
		t.Fatalf("UpdateRule() error: %v", err)
	}

	saved, err := config.LoadConfigWithoutAreaLookup(app.configFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Rules) != 2 || saved.Rules[0].Name != "Renamed" || saved.Rules[0].Pfm != "Host" || saved.Rules[1].Name != "Other" {
		t.Errorf("saved rules = %+v", saved.Rules)
	}
	if got := app.config.Rules[0]; !slices.Equal(got.DoW, []string{"sun"}) || got.Exclude != nil {
		t.Errorf("rule not cleaned: DoW %v, Exclude %+v", got.DoW, got.Exclude)
	}
	select {
	case <-app.configChanged:
	default:
		t.Error("UpdateRule() did not wake automatic checks")
	}
	if got := events.names(); !slices.Equal(got, []string{"config-saved"}) {
		t.Errorf("events = %v", got)
	}
}

func TestUpdateRuleRejectsInvalidChangesWithoutChanges(t *testing.T) {
	valid := radikron.Criteria{Title: "x"}
	tests := []struct {
		name   string
		target string
		rule   radikron.Rule
		want   string
	}{
		{"unknown rule", "Missing", radikron.Rule{Name: "Missing", Criteria: valid}, "no rule named"},
		{"name taken", "Show", radikron.Rule{Name: "Other", Criteria: valid}, "already exists"},
		{"bad window", "Show", radikron.Rule{Name: "Show", Criteria: valid, Window: "7d"}, "not a duration"},
		{"bad day", "Show", radikron.Rule{Name: "Show", Criteria: radikron.Criteria{Title: "x", DoW: []string{"funday"}}}, "unknown day"},
		{"no criteria", "Show", radikron.Rule{Name: "Show"}, "needs a title"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, events := ruleTestApp(t, radikron.Rules{
				{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}},
				{Name: "Other", Criteria: radikron.Criteria{Title: "Other"}},
			})
			err := app.UpdateRule(tt.target, &tt.rule)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("UpdateRule() error = %v, want %q", err, tt.want)
			}
			if app.config.Rules[0].Title != "Show" || len(events.names()) != 0 {
				t.Errorf("rejected change altered state: rules=%+v events=%v", app.config.Rules, events.names())
			}
		})
	}
}

func TestRemoveRule(t *testing.T) {
	app, _ := ruleTestApp(t, radikron.Rules{
		{Name: "Show", Criteria: radikron.Criteria{Title: "Show"}},
		{Name: "Other", Criteria: radikron.Criteria{Title: "Other"}},
	})
	if err := app.RemoveRule("Show"); err != nil {
		t.Fatalf("RemoveRule() error: %v", err)
	}
	saved, err := config.LoadConfigWithoutAreaLookup(app.configFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Rules) != 1 || saved.Rules[0].Name != "Other" || len(app.asset.Rules) != 1 {
		t.Errorf("saved rules = %+v, asset rules = %d", saved.Rules, len(app.asset.Rules))
	}
	if err := app.RemoveRule("Show"); err == nil || !strings.Contains(err.Error(), "no rule named") {
		t.Errorf("RemoveRule() of a missing rule error = %v", err)
	}
}

func TestRuleFolderForKeepsDownloadSubfoldersRelative(t *testing.T) {
	root := filepath.Join(t.TempDir(), "radiko")
	tests := []struct{ picked, want string }{
		{root, ""},
		{filepath.Join(root, "jazz", "sunday"), "jazz/sunday"},
		{filepath.Dir(root), filepath.Dir(root)},
		{root + "-other", root + "-other"},
	}
	for _, tt := range tests {
		if got := ruleFolderFor(root, tt.picked); got != tt.want {
			t.Errorf("ruleFolderFor(%q) = %q, want %q", tt.picked, got, tt.want)
		}
	}
}

func TestPreviewRuleMatchesTheWeeklySchedule(t *testing.T) {
	app, _ := ruleTestApp(t, nil)
	app.asset.AvailableStations = []string{"TBS", "FMJ"}
	app.programSnapshots = map[string]radikron.Progs{
		"TBS": {{Title: "MIDDAY LOUNGE", StationID: "TBS", Ft: "20261012130000"}, {Title: "News", StationID: "TBS", Ft: "20261012140000"}},
		"FMJ": {{Title: "MIDDAY LOUNGE", StationID: "FMJ", Ft: "20261012130000", Pfm: "GUEST"}},
	}
	rule := radikron.Rule{Criteria: radikron.Criteria{Title: "MIDDAY"}, Exclude: &radikron.Criteria{Pfm: "GUEST"}}
	got, err := app.PreviewRule(&rule)
	if err != nil {
		t.Fatalf("PreviewRule() error: %v", err)
	}
	if len(got) != 1 || got[0].StationID != "TBS" {
		t.Errorf("PreviewRule() = %+v", got)
	}
	if got, _ := app.PreviewRule(&radikron.Rule{Criteria: radikron.Criteria{StationID: "TBS"}}); len(got) != 0 {
		t.Errorf("a rule without criteria previewed %d programs", len(got))
	}
}
