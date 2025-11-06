package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iomz/radikron"
	"github.com/yyoshiki41/go-radiko"
	"github.com/yyoshiki41/radigo"
)

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
	err := os.WriteFile(configFile, []byte("file-format: aac\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Change to temp directory to test default config loading
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldCwd)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

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
	err := os.WriteFile(configFile, []byte("file-format: invalid\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldCwd)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

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

	// Create a mock asset
	asset := &radikron.Asset{
		AvailableStations: []string{},
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

	// Check that stations were loaded
	if len(asset.AvailableStations) == 0 {
		t.Error("expected stations to be loaded")
	}

	// Check that extra stations from rules are added
	hasFMJ := false
	for _, station := range asset.AvailableStations {
		if station == "FMT" {
			hasFMJ = true
			break
		}
	}
	if !hasFMJ {
		t.Error("expected FMT station to be in available stations")
	}
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
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldCwd)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

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
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldCwd)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cfg, err := LoadConfig("config.yml")
	if err != nil {
		t.Fatalf("expected no error loading config, got: %v", err)
	}

	// Create a properly initialized asset with stations
	// We need to load an actual asset since LoadAvailableStations requires it
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
	err := os.WriteFile(configFile, []byte("file-format: mp3\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldCwd)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cfg, err := LoadConfig("config.yml")
	if err != nil {
		t.Fatalf("expected no error loading config, got: %v", err)
	}

	if cfg.FileFormat != radigo.AudioFormatMP3 {
		t.Errorf("expected FileFormat to be %s, got %s", radigo.AudioFormatMP3, cfg.FileFormat)
	}
}
