package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iomz/radikron"
	"github.com/spf13/viper"
	"github.com/yyoshiki41/go-radiko"
	"github.com/yyoshiki41/radigo"
)

// Config holds the application configuration
type Config struct {
	AreaID            string
	ExtraStations     []string
	IgnoreStations    []string
	FileFormat        string
	MinimumOutputSize int64
	Rules             radikron.Rules
}

// LoadConfig loads and validates configuration from the specified file
func LoadConfig(filename string) (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Set up RADICRON_HOME if not set
	if os.Getenv(radikron.EnvRadicronHome) == "" {
		os.Setenv(radikron.EnvRadicronHome, filepath.Join(cwd, "radiko"))
	}

	// Configure viper
	if err := setupViper(filename, cwd); err != nil {
		return nil, err
	}

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	// Set defaults
	setDefaults()

	// Validate and build config
	cfg := &Config{}
	if err := cfg.buildConfig(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ApplyToAsset applies the configuration to an asset
func (c *Config) ApplyToAsset(asset *radikron.Asset) error {
	asset.OutputFormat = c.FileFormat
	asset.MinimumOutputSize = c.MinimumOutputSize
	asset.LoadAvailableStations(c.AreaID)
	asset.AddExtraStations(c.ExtraStations)
	asset.RemoveIgnoreStations(c.IgnoreStations)

	// Build a set of existing stations for faster lookup
	existingStations := make(map[string]bool)
	for _, as := range asset.AvailableStations {
		existingStations[as] = true
	}

	// Add station IDs from rules that aren't already in available stations
	for _, rule := range c.Rules {
		if rule.HasStationID() {
			if !existingStations[rule.StationID] {
				asset.AddExtraStations([]string{rule.StationID})
				existingStations[rule.StationID] = true
			}
		}
	}

	return nil
}

// setupViper configures the viper instance with the config file path
func setupViper(filename, cwd string) error {
	if filename != "config.yml" && filename != "config.toml" {
		configPath, err := filepath.Abs(filename)
		if err != nil {
			return fmt.Errorf("invalid config path: %w", err)
		}
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath(cwd)
	}
	return nil
}

// setDefaults sets default values for configuration
func setDefaults() {
	currentAreaID, err := radiko.AreaID()
	if err != nil {
		// If we can't get the area ID, use the default
		currentAreaID = radikron.DefaultArea
	}

	viper.SetDefault("area-id", currentAreaID)
	viper.SetDefault("extra-stations", []string{})
	viper.SetDefault("ignore-stations", []string{})
	viper.SetDefault("file-format", radigo.AudioFormatAAC)
	viper.SetDefault("minimum-output-size", radikron.DefaultMinimumOutputSize)
}

// buildConfig builds the Config struct from viper values
func (c *Config) buildConfig() error {
	// Validate file format
	fileFormat := viper.GetString("file-format")
	if fileFormat != radigo.AudioFormatAAC && fileFormat != radigo.AudioFormatMP3 {
		return fmt.Errorf("unsupported audio format: %s", fileFormat)
	}

	c.FileFormat = fileFormat
	c.AreaID = viper.GetString("area-id")
	c.ExtraStations = viper.GetStringSlice("extra-stations")
	c.IgnoreStations = viper.GetStringSlice("ignore-stations")
	c.MinimumOutputSize = viper.GetInt64("minimum-output-size") * radikron.Kilobytes * radikron.Kilobytes

	// Load rules
	rules, err := loadRules()
	if err != nil {
		return fmt.Errorf("error loading rules: %w", err)
	}
	c.Rules = rules

	return nil
}

// loadRules loads rules from the configuration
func loadRules() (radikron.Rules, error) {
	rules := radikron.Rules{}
	ruleMap := viper.GetStringMap("rules")

	for name := range ruleMap {
		rule := &radikron.Rule{}
		err := viper.UnmarshalKey(fmt.Sprintf("rules.%s", name), rule)
		if err != nil {
			return nil, fmt.Errorf("error reading the rule '%s': %w", name, err)
		}
		rule.SetName(name)
		rules = append(rules, rule)
	}

	return rules, nil
}
