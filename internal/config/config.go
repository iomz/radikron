package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iomz/radikron"
	"github.com/spf13/viper"
	"github.com/yyoshiki41/go-radiko"
	"github.com/yyoshiki41/radigo"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	AreaID            string
	ExtraStations     []string
	IgnoreStations    []string
	FileFormat        string
	MinimumOutputSize int64
	DownloadDir       string
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
	asset.DownloadDir = c.DownloadDir
	asset.LoadAvailableStations(c.AreaID)
	asset.AddExtraStations(c.ExtraStations)
	asset.RemoveIgnoreStations(c.IgnoreStations)
	asset.Rules = c.Rules

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
	viper.SetDefault("downloads", "downloads")
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
	c.DownloadDir = viper.GetString("downloads")

	// Load rules
	rules, err := loadRules()
	if err != nil {
		return fmt.Errorf("error loading rules: %w", err)
	}
	c.Rules = rules

	return nil
}

// loadRules loads rules from the configuration, preserving the order from the config file
func loadRules() (radikron.Rules, error) {
	rules := radikron.Rules{}

	// Get the config file path
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		// Fallback to viper's method if config file path is not available
		return loadRulesFromViper()
	}

	// Read the YAML file directly to preserve order
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML to yaml.Node to preserve order
	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Find the rules section in the YAML node
	var rulesNode *yaml.Node
	if rootNode.Kind == yaml.DocumentNode && len(rootNode.Content) > 0 {
		root := rootNode.Content[0]
		if root.Kind == yaml.MappingNode {
			for i := 0; i < len(root.Content); i += 2 {
				keyNode := root.Content[i]
				if keyNode.Value == "rules" && i+1 < len(root.Content) {
					rulesNode = root.Content[i+1]
					break
				}
			}
		}
	}

	if rulesNode == nil || rulesNode.Kind != yaml.MappingNode {
		return rules, nil
	}

	// Iterate through rules in order (yaml.Node.Content preserves order)
	for i := 0; i < len(rulesNode.Content); i += 2 {
		nameNode := rulesNode.Content[i]
		ruleNode := rulesNode.Content[i+1]
		name := nameNode.Value

		// Convert rule node to a map for viper to process
		// Viper's UnmarshalKey respects mapstructure tags
		var ruleMap map[string]interface{}
		if err := ruleNode.Decode(&ruleMap); err != nil {
			return nil, fmt.Errorf("failed to decode rule '%s': %w", name, err)
		}

		// Set the rule data in viper temporarily
		ruleKey := fmt.Sprintf("rules.%s", name)
		for k, v := range ruleMap {
			viper.Set(fmt.Sprintf("%s.%s", ruleKey, k), v)
		}

		// Use viper's UnmarshalKey which respects mapstructure tags
		rule := &radikron.Rule{}
		if err := viper.UnmarshalKey(ruleKey, rule); err != nil {
			return nil, fmt.Errorf("error reading the rule '%s': %w", name, err)
		}
		rule.SetName(name)
		rules = append(rules, rule)
	}

	return rules, nil
}

// loadRulesFromViper is a fallback method when config file path is not available
func loadRulesFromViper() (radikron.Rules, error) {
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
