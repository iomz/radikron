package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iomz/radikron"
	"github.com/spf13/viper"
	"github.com/yyoshiki41/go-radiko"
	"github.com/yyoshiki41/radigo"
	"gopkg.in/yaml.v3"
)

func withCwd(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("failed to restore cwd: %v", err)
		}
	})
}

func TestLoadConfig(t *testing.T) {
	// Test loading config from test file
	configPath := filepath.Join("..", "..", "cmd", "radikron", "test", "config-test.yml")
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("expected no error loading config, got: %v", err)
	}

	if cfg.AreaID != "JP13" {
		t.Errorf("expected AreaID to be JP13, got %s", cfg.AreaID)
	}

	if cfg.FileFormat != radigo.AudioFormatAAC {
		t.Errorf("expected FileFormat to be %s, got %s", radigo.AudioFormatAAC, cfg.FileFormat)
	}

	if len(cfg.Rules) == 0 {
		t.Error("expected rules to be loaded")
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")
	err := os.WriteFile(configFile, []byte("file-format: aac\n"), 0600)
	if err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Change to temp directory to test default config loading
	withCwd(t, tmpDir)

	t.Setenv(radikron.EnvRadicronHome, filepath.Join(tmpDir, "radiko_home"))
	cfg, err := LoadConfig("config.yml")
	if err != nil {
		t.Fatalf("expected no error loading config, got: %v", err)
	}

	if cfg.FileFormat != radigo.AudioFormatAAC {
		t.Errorf("expected FileFormat to be %s, got %s", radigo.AudioFormatAAC, cfg.FileFormat)
	}

	// Check that defaults are set
	if cfg.AreaID == "" {
		t.Error("expected AreaID to have default value")
	}
}

func TestLoadConfigInvalidFile(t *testing.T) {
	_, err := LoadConfig("nonexistent.yml")
	if err == nil {
		t.Error("expected error loading non-existent config file")
	}
}

func TestLoadConfigInvalidFormat(t *testing.T) {
	// Create a temporary config file with invalid format
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")
	err := os.WriteFile(configFile, []byte("file-format: invalid\n"), 0600)
	if err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	withCwd(t, tmpDir)

	t.Setenv(radikron.EnvRadicronHome, filepath.Join(tmpDir, "radiko_home"))
	_, err = LoadConfig("config.yml")
	if err == nil {
		t.Error("expected error for unsupported audio format")
	}
}

func TestApplyToAsset(t *testing.T) {
	configPath := filepath.Join("..", "..", "cmd", "radikron", "test", "config-test.yml")
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("expected no error loading config, got: %v", err)
	}

	// Create a mock asset with empty Stations map (LoadAvailableStations will return empty)
	asset := &radikron.Asset{
		AvailableStations: []string{},
		Stations:          radikron.Stations{},
	}

	err = cfg.ApplyToAsset(asset)
	if err != nil {
		t.Fatalf("expected no error applying config, got: %v", err)
	}

	if asset.OutputFormat != cfg.FileFormat {
		t.Errorf("expected OutputFormat to be %s, got %s", cfg.FileFormat, asset.OutputFormat)
	}

	if asset.MinimumOutputSize != cfg.MinimumOutputSize {
		t.Errorf("expected MinimumOutputSize to be %d, got %d", cfg.MinimumOutputSize, asset.MinimumOutputSize)
	}

	// Check that extra stations from rules are added (FMT should be added from the airship rule)
	hasFMT := false
	for _, station := range asset.AvailableStations {
		if station == "FMT" {
			hasFMT = true
			break
		}
	}
	if !hasFMT {
		t.Errorf("expected FMT station to be in available stations (from rules), got: %v", asset.AvailableStations)
	}

	// Verify that at least one station was added (from rules)
	if len(asset.AvailableStations) == 0 {
		t.Error("expected at least one station to be added from rules")
	}
}

func TestFindRulesNode(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantNode bool
		wantErr  bool
	}{
		{
			name: "normal case with rules",
			yaml: `area-id: JP13
rules:
  test:
    station-id: FMT
`,
			wantNode: true,
			wantErr:  false,
		},
		{
			name: "missing rules section",
			yaml: `area-id: JP13
file-format: aac
`,
			wantNode: false,
			wantErr:  false,
		},
		{
			name:     "empty document",
			yaml:     ``,
			wantNode: false,
			wantErr:  false,
		},
		{
			name: "rules is not a mapping",
			yaml: `area-id: JP13
rules: "not a mapping"
`,
			wantNode: true, // node exists but wrong type
			wantErr:  false,
		},
		{
			name: "nested structure",
			yaml: `area-id: JP13
other:
  nested: value
rules:
  test:
    station-id: FMT
`,
			wantNode: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rootNode yaml.Node
			err := yaml.Unmarshal([]byte(tt.yaml), &rootNode)
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("unexpected error unmarshaling YAML: %v", err)
				}
				return
			}

			rulesNode := findRulesNode(&rootNode)

			hasNode := rulesNode != nil
			if hasNode != tt.wantNode {
				t.Errorf("findRulesNode() returned node = %v, want %v", hasNode, tt.wantNode)
			}

			if tt.wantNode && rulesNode != nil {
				// Verify it's actually the rules node
				if rulesNode.Kind != yaml.MappingNode && tt.name != "rules is not a mapping" {
					t.Errorf("findRulesNode() returned node with kind %v, want MappingNode", rulesNode.Kind)
				}
			}
		})
	}

	// Test with nil root
	rulesNode := findRulesNode(nil)
	if rulesNode != nil {
		t.Errorf("findRulesNode(nil) should return nil node, got: %v", rulesNode)
	}
}

func TestParseRuleFromNode_NormalCase(t *testing.T) {
	testParseRuleFromNodeSuccess(t, `station-id: FMT
title: "Test Title"
`, "test-rule", "test-rule")
}

func TestParseRuleFromNode_AllFields(t *testing.T) {
	testParseRuleFromNodeSuccess(t, `station-id: TBS
title: "Test Title"
pfm: "Test Person"
keyword: "test"
dow:
  - mon
  - tue
window: 48h
folder: "test-folder"
`, "full-rule", "full-rule")
}

// testParseRuleFromNodeSuccess is a helper to test successful rule parsing
func testParseRuleFromNodeSuccess(t *testing.T, ruleYAML, ruleName, expectedName string) {
	t.Helper()
	viper.Reset()
	viper.SetConfigType("yaml")

	var ruleNode yaml.Node
	if err := yaml.Unmarshal([]byte(ruleYAML), &ruleNode); err != nil {
		t.Fatalf("unexpected error unmarshaling rule YAML: %v", err)
	}

	actualRuleNode := extractRuleNode(&ruleNode)
	nameNode := &yaml.Node{Kind: yaml.ScalarNode, Value: ruleName}

	rule, err := parseRuleFromNode(nameNode, actualRuleNode)
	if err != nil {
		t.Errorf("parseRuleFromNode() error = %v, want nil", err)
	}
	if rule == nil {
		t.Fatal("parseRuleFromNode() returned nil rule")
	}
	if rule.Name != expectedName {
		t.Errorf("parseRuleFromNode() rule.Name = %v, want %v", rule.Name, expectedName)
	}
}

func TestParseRuleFromNode_DecodeError(t *testing.T) {
	viper.Reset()
	viper.SetConfigType("yaml")

	// Create a scalar node instead of mapping node - this will cause decode error
	ruleNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "not a mapping",
	}
	nameNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "invalid-rule"}

	_, err := parseRuleFromNode(nameNode, ruleNode)
	if err == nil {
		t.Error("parseRuleFromNode() expected error but got nil")
	}
	if err != nil && !containsSubstring(err.Error(), "failed to decode rule") {
		t.Errorf("error message '%s' does not contain 'failed to decode rule'", err.Error())
	}
}

func TestParseRuleFromNode_NilNameNode(t *testing.T) {
	viper.Reset()
	ruleNode := &yaml.Node{Kind: yaml.MappingNode}
	_, err := parseRuleFromNode(nil, ruleNode)
	if err == nil {
		t.Error("parseRuleFromNode() with nil nameNode should return error")
	}
}

func TestParseRuleFromNode_NilRuleNode(t *testing.T) {
	viper.Reset()
	nameNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "test"}
	_, err := parseRuleFromNode(nameNode, nil)
	if err == nil {
		t.Error("parseRuleFromNode() with nil ruleNode should return error")
	}
}

func TestParseRuleFromNode_EmptyRuleName(t *testing.T) {
	viper.Reset()
	nameNode := &yaml.Node{Kind: yaml.ScalarNode, Value: ""}
	ruleNode := &yaml.Node{Kind: yaml.MappingNode}
	_, err := parseRuleFromNode(nameNode, ruleNode)
	if err == nil {
		t.Error("parseRuleFromNode() with empty rule name should return error")
	}
}

// extractRuleNode extracts the actual rule node from a YAML node
func extractRuleNode(ruleNode *yaml.Node) *yaml.Node {
	if ruleNode.Kind == yaml.DocumentNode && len(ruleNode.Content) > 0 {
		return ruleNode.Content[0]
	}
	return ruleNode
}

// containsSubstring checks if a string contains a substring
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestApplyToAssetWithExtraStations(t *testing.T) {
	// Create a temporary config file with extra stations
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")
	configContent := `area-id: JP13
file-format: aac
extra-stations:
  - EXTRA1
  - EXTRA2
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	withCwd(t, tmpDir)

	t.Setenv(radikron.EnvRadicronHome, filepath.Join(tmpDir, "radiko_home"))
	cfg, err := LoadConfig("config.yml")
	if err != nil {
		t.Fatalf("expected no error loading config, got: %v", err)
	}

	asset := &radikron.Asset{
		AvailableStations: []string{},
	}

	err = cfg.ApplyToAsset(asset)
	if err != nil {
		t.Fatalf("expected no error applying config, got: %v", err)
	}

	// Check that extra stations were added
	hasExtra1 := false
	hasExtra2 := false
	for _, station := range asset.AvailableStations {
		if station == "EXTRA1" {
			hasExtra1 = true
		}
		if station == "EXTRA2" {
			hasExtra2 = true
		}
	}
	if !hasExtra1 || !hasExtra2 {
		t.Errorf("expected extra stations to be added, got: %v", asset.AvailableStations)
	}
}

func TestApplyToAssetWithIgnoreStations(t *testing.T) {
	// Create a temporary config file with ignore stations
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")
	configContent := `area-id: JP13
file-format: aac
ignore-stations:
  - TBS
  - MBS
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	withCwd(t, tmpDir)

	t.Setenv(radikron.EnvRadicronHome, filepath.Join(tmpDir, "radiko_home"))
	cfg, err := LoadConfig("config.yml")
	if err != nil {
		t.Fatalf("expected no error loading config, got: %v", err)
	}

	// Create a properly initialized asset with stations
	// Tip: this path depends on network/remote data. Gate to avoid flakes.
	if os.Getenv("RADIKRON_NETWORK_TESTS") != "1" || testing.Short() {
		t.Skip("skipping network-dependent test; set RADIKRON_NETWORK_TESTS=1 to run")
	}
	client, err := radiko.New("")
	if err != nil {
		t.Fatalf("failed to create radiko client: %v", err)
	}

	asset, err := radikron.NewAsset(client)
	if err != nil {
		t.Fatalf("failed to create asset: %v", err)
	}

	// Manually set stations to test ignore functionality
	asset.AvailableStations = []string{"TBS", "MBS", "FMT", "FMJ"}

	// Apply config which will load stations for JP13 and then remove ignored ones
	err = cfg.ApplyToAsset(asset)
	if err != nil {
		t.Fatalf("expected no error applying config, got: %v", err)
	}

	// Check that ignored stations were removed
	for _, station := range asset.AvailableStations {
		if station == "TBS" || station == "MBS" {
			t.Errorf("expected TBS and MBS to be removed, but found %s", station)
		}
	}

	// Check that FMT is still there (it should be in JP13 stations)
	hasFMT := false
	for _, station := range asset.AvailableStations {
		if station == "FMT" {
			hasFMT = true
			break
		}
	}
	if !hasFMT {
		t.Errorf("expected FMT to still be in available stations, got: %v", asset.AvailableStations)
	}
}

func TestLoadConfigMP3Format(t *testing.T) {
	// Create a temporary config file with MP3 format
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yml")
	err := os.WriteFile(configFile, []byte("file-format: mp3\n"), 0600)
	if err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	withCwd(t, tmpDir)

	t.Setenv(radikron.EnvRadicronHome, filepath.Join(tmpDir, "radiko_home"))
	cfg, err := LoadConfig("config.yml")
	if err != nil {
		t.Fatalf("expected no error loading config, got: %v", err)
	}

	if cfg.FileFormat != radigo.AudioFormatMP3 {
		t.Errorf("expected FileFormat to be %s, got %s", radigo.AudioFormatMP3, cfg.FileFormat)
	}
}
