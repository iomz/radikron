package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/iomz/radikron"
	"github.com/iomz/radikron/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// RuleFromProgram suggests a rule that matches every episode of prog: the
// same title on the same station. Its name is the title, made unique among
// the configured rules.
func (a *App) RuleFromProgram(prog *radikron.Prog) (radikron.Rule, error) {
	if prog == nil || strings.TrimSpace(prog.Title) == "" {
		return radikron.Rule{}, errors.New("program has no title")
	}
	if err := a.waitForStartup(); err != nil {
		return radikron.Rule{}, fmt.Errorf("startup failed: %w", err)
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.config == nil {
		return radikron.Rule{}, errors.New("config not loaded")
	}
	title := strings.TrimSpace(prog.Title)
	return radikron.Rule{
		Name:     uniqueRuleName(a.config.Rules, title),
		Criteria: radikron.Criteria{Title: title, StationID: prog.StationID},
	}, nil
}

func uniqueRuleName(rules radikron.Rules, base string) string {
	taken := func(name string) bool {
		return slices.ContainsFunc(rules, func(r *radikron.Rule) bool { return r != nil && r.Name == name })
	}
	name := base
	for i := 2; taken(name); i++ {
		name = fmt.Sprintf("%s (%d)", base, i)
	}
	return name
}

// AddRule adds newRule to the configuration and saves the config file. It wakes
// automatic checks, so programs the rule matches, including episodes from the
// past week, are downloaded right away.
func (a *App) AddRule(newRule *radikron.Rule) error {
	rule, err := cleanRule(newRule)
	if err != nil {
		return err
	}
	return a.changeRules(func(rules radikron.Rules) (radikron.Rules, error) {
		if uniqueRuleName(rules, rule.Name) != rule.Name {
			return nil, fmt.Errorf("a rule named %q already exists", rule.Name)
		}
		return append(slices.Clone(rules), rule), nil
	})
}

// UpdateRule replaces the rule called name with rule, which may rename it, and
// saves the config file. It wakes automatic checks so the changed rule applies
// right away.
func (a *App) UpdateRule(name string, rule *radikron.Rule) error {
	updated, err := cleanRule(rule)
	if err != nil {
		return err
	}
	return a.changeRules(func(rules radikron.Rules) (radikron.Rules, error) {
		i := ruleIndex(rules, name)
		if i < 0 {
			return nil, fmt.Errorf("no rule named %q", name)
		}
		if updated.Name != name && ruleIndex(rules, updated.Name) >= 0 {
			return nil, fmt.Errorf("a rule named %q already exists", updated.Name)
		}
		changed := slices.Clone(rules)
		changed[i] = updated
		return changed, nil
	})
}

// RemoveRule removes the rule called name and saves the config file. Programs
// it already scheduled stay scheduled.
func (a *App) RemoveRule(name string) error {
	return a.changeRules(func(rules radikron.Rules) (radikron.Rules, error) {
		i := ruleIndex(rules, name)
		if i < 0 {
			return nil, fmt.Errorf("no rule named %q", name)
		}
		return slices.Delete(slices.Clone(rules), i, i+1), nil
	})
}

// ChooseRuleFolder lets the user pick a rule folder, starting from current.
// A folder inside the download folder is returned relative to it ("" for the
// download folder itself) so the config stays portable; any other folder is
// returned as an absolute path. It returns nil when the user cancels.
func (a *App) ChooseRuleFolder(current string) (*string, error) {
	if a.wails == nil {
		return nil, errors.New("folder dialog unavailable before the app starts")
	}
	a.mu.RLock()
	downloadDir := ""
	if a.config != nil {
		downloadDir = a.config.DownloadDir
	}
	a.mu.RUnlock()
	root, err := radikron.GetRadikronPath(downloadDir)
	if err != nil {
		return nil, fmt.Errorf("download folder: %w", err)
	}
	if err := os.MkdirAll(root, radikron.DirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create the download folder: %w", err)
	}
	start := radikron.RuleFolderPath(root, filepath.FromSlash(current))
	if info, err := os.Stat(start); err != nil || !info.IsDir() {
		start = root
	}
	picked, err := a.wails.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                "Choose a folder for this rule",
		Directory:            start,
		CanChooseDirectories: true,
		CanCreateDirectories: true,
	}).PromptForSingleSelection()
	if err != nil || picked == "" {
		return nil, nil //nolint:nilerr // the dialog reports a cancel as an error
	}
	folder := ruleFolderFor(root, picked)
	return &folder, nil
}

// ruleFolderFor returns picked relative to root with forward slashes when it
// is inside root, and picked itself otherwise.
func ruleFolderFor(root, picked string) string {
	picked = filepath.Clean(picked)
	rel, err := filepath.Rel(filepath.Clean(root), picked)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return picked
	}
	if rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

// PreviewRule lists the programs in radiko's weekly schedule that rule
// matches, so the user can check a rule before saving it. The rule's name is
// not needed.
func (a *App) PreviewRule(rule *radikron.Rule) (radikron.Progs, error) {
	if rule == nil || (!rule.HasTitle() && !rule.HasPfm() && !rule.HasKeyword()) {
		return nil, nil
	}
	if err := a.waitForStartup(); err != nil {
		return nil, fmt.Errorf("startup failed: %w", err)
	}
	// Right after startup the first check may still be fetching the schedule;
	// this waits for it instead of fetching again.
	a.loadProgramSnapshots(radikron.FetchWeeklyPrograms, false)

	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.asset == nil {
		return nil, errors.New("asset not initialized")
	}
	var matched radikron.Progs
	for _, stationID := range a.asset.AvailableStations {
		for _, prog := range a.programSnapshots[stationID] {
			if prog != nil && rule.MatchSilent(stationID, prog) {
				matched = append(matched, prog)
			}
		}
	}
	return matched, nil
}

var weekdays = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// cleanRule returns a trimmed copy of rule, or an error when the rule could
// never match or its window or days are not understood.
func cleanRule(rule *radikron.Rule) (*radikron.Rule, error) {
	if rule == nil {
		return nil, errors.New("rule is required")
	}
	r := *rule
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return nil, errors.New("rule name is required")
	}
	if !r.HasTitle() && !r.HasPfm() && !r.HasKeyword() {
		return nil, errors.New("rule needs a title, performer, or keyword to match programs")
	}
	if r.Window != "" {
		if d, err := time.ParseDuration(r.Window); err != nil || d <= 0 {
			return nil, fmt.Errorf("window %q is not a duration such as 48h", r.Window)
		}
	}
	days := make([]string, 0, len(r.DoW))
	for _, day := range r.DoW {
		day = strings.ToLower(strings.TrimSpace(day))
		if !slices.Contains(weekdays, day) {
			return nil, fmt.Errorf("unknown day %q", day)
		}
		days = append(days, day)
	}
	r.DoW = days
	if r.Exclude != nil {
		exclude := *r.Exclude
		r.Exclude = &exclude
		if !r.HasExclude() {
			r.Exclude = nil
		}
	}
	return &r, nil
}

func ruleIndex(rules radikron.Rules, name string) int {
	return slices.IndexFunc(rules, func(r *radikron.Rule) bool { return r != nil && r.Name == name })
}

// changeRules applies the rules that change returns, as changeConfig does.
func (a *App) changeRules(change func(radikron.Rules) (radikron.Rules, error)) error {
	return a.changeConfig(func(cfg *config.Config) error {
		rules, err := change(cfg.Rules)
		if err != nil {
			return err
		}
		cfg.Rules = rules
		return nil
	})
}
