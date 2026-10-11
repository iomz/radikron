package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/iomz/radikron"
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
	if newRule == nil {
		return errors.New("rule is required")
	}
	rule := *newRule
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" {
		return errors.New("rule name is required")
	}
	if !rule.HasTitle() && !rule.HasPfm() && !rule.HasKeyword() {
		return errors.New("rule needs a title, performer, or keyword to match programs")
	}
	if err := a.waitForStartup(); err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.config == nil || a.asset == nil {
		return errors.New("config not loaded")
	}
	if a.configFile == "" {
		return errors.New("config file path not set")
	}
	if uniqueRuleName(a.config.Rules, rule.Name) != rule.Name {
		return fmt.Errorf("a rule named %q already exists", rule.Name)
	}

	previous := a.config
	updated := *previous
	updated.Rules = append(slices.Clone(previous.Rules), &rule)
	if err := updated.ApplyToAsset(a.asset); err != nil {
		return fmt.Errorf("failed to apply rule: %w", err)
	}
	if err := updated.SaveConfig(a.configFile); err != nil {
		// Keep the running configuration in line with the file on disk.
		if restoreErr := previous.ApplyToAsset(a.asset); restoreErr != nil {
			a.logger.Error(fmt.Sprintf("Failed to restore the previous config: %v", restoreErr))
		}
		return fmt.Errorf("failed to save config: %w", err)
	}
	a.config = &updated
	a.signalConfigChanged()
	a.emit("config-saved", map[string]any{
		"success": true,
		"file":    a.configFile,
	})
	return nil
}
