package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iomz/radikron"
	"github.com/spf13/viper"
	"github.com/yyoshiki41/go-radiko"
	"github.com/yyoshiki41/radigo"
	"gopkg.in/yaml.v3"
)

const (
	// DirPermissions is the file system permissions for directories (rwxr-xr-x)
	DirPermissions = 0755
	// FilePermissions is the file system permissions for files (rw-------)
	FilePermissions = 0600
)

// Config holds the application configuration
type Config struct {
	AreaID                    string
	ExtraStations             []string
	IgnoreStations            []string
	FileFormat                string
	MinimumOutputSize         int64
	DownloadDir               string
	Rules                     radikron.Rules
	MaxDownloadingConcurrency int
	MaxEncodingConcurrency    int
}

// LoadConfig loads and validates configuration from the specified file
func LoadConfig(filename string) (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
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
	if err := c.validateArchiveStations(); err != nil {
		return err
	}

	asset.OutputFormat = c.FileFormat
	asset.MinimumOutputSize = c.MinimumOutputSize
	asset.DownloadDir = c.DownloadDir
	asset.MaxDownloadingConcurrency = c.MaxDownloadingConcurrency
	asset.MaxEncodingConcurrency = c.MaxEncodingConcurrency
	asset.LoadAvailableStations(c.AreaID)
	asset.AddExtraStations(c.ExtraStations)
	asset.RemoveIgnoreStations(c.IgnoreStations)
	asset.Rules = c.Rules

	// Initialize semaphores with the configured concurrency values
	radikron.InitSemaphores(asset)

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

	// Get default downloads directory (cross-platform: $HOME/Downloads/radiko)
	var defaultDownloads string
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current working directory if home directory can't be determined
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			// Last resort: use relative path
			defaultDownloads = "radiko"
		} else {
			defaultDownloads = filepath.Join(cwd, "radiko")
		}
	} else {
		defaultDownloads = filepath.Join(homeDir, "Downloads", "radiko")
	}

	viper.SetDefault("area-id", currentAreaID)
	viper.SetDefault("extra-stations", []string{})
	viper.SetDefault("ignore-stations", []string{})
	viper.SetDefault("file-format", radigo.AudioFormatAAC)
	viper.SetDefault("minimum-output-size", radikron.DefaultMinimumOutputSize)
	viper.SetDefault("downloads", defaultDownloads)
	viper.SetDefault("max-downloading-concurrency", radikron.MaxDownloadingConcurrency)
	viper.SetDefault("max-encoding-concurrency", radikron.MaxEncodingConcurrency)
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
	c.MaxDownloadingConcurrency = viper.GetInt("max-downloading-concurrency")
	c.MaxEncodingConcurrency = viper.GetInt("max-encoding-concurrency")

	// Load rules
	rules, err := loadRules()
	if err != nil {
		return fmt.Errorf("error loading rules: %w", err)
	}
	c.Rules = rules
	if err := c.validateArchiveStations(); err != nil {
		return err
	}

	return nil
}

func (c *Config) validateArchiveStations() error {
	unsupported := make([]string, 0)
	for _, stationID := range c.ExtraStations {
		if !radikron.SupportsArchive(stationID) {
			unsupported = append(unsupported, fmt.Sprintf("extra-stations: %s", stationID))
		}
	}
	for _, rule := range c.Rules {
		if rule.HasStationID() && !radikron.SupportsArchive(rule.StationID) {
			unsupported = append(unsupported, fmt.Sprintf("rules.%s.station-id: %s", rule.Name, rule.StationID))
		}
	}
	if len(unsupported) == 0 {
		return nil
	}

	sort.Strings(unsupported)
	return fmt.Errorf(
		"stations do not support Radiko archive/timeshift downloads: %s",
		strings.Join(unsupported, ", "),
	)
}

// configYAML represents the YAML structure for saving configuration
type configYAML struct {
	AreaID                    string               `yaml:"area-id"`
	ExtraStations             []string             `yaml:"extra-stations,omitempty"`
	IgnoreStations            []string             `yaml:"ignore-stations,omitempty"`
	FileFormat                string               `yaml:"file-format"`
	MinimumOutputSize         int64                `yaml:"minimum-output-size"`
	DownloadDir               string               `yaml:"downloads"`
	MaxDownloadingConcurrency *int                 `yaml:"max-downloading-concurrency,omitempty"`
	MaxEncodingConcurrency    *int                 `yaml:"max-encoding-concurrency,omitempty"`
	Rules                     map[string]*ruleYAML `yaml:"rules,omitempty"`
	// rulesOrder stores the order of rule names for custom marshaling
	rulesOrder []string
}

// ruleYAML represents a rule in YAML format
type ruleYAML struct {
	StationID string   `yaml:"station-id,omitempty"`
	Title     string   `yaml:"title,omitempty"`
	DoW       []string `yaml:"dow,omitempty"`
	Keyword   string   `yaml:"keyword,omitempty"`
	Pfm       string   `yaml:"pfm,omitempty"`
	Window    string   `yaml:"window,omitempty"`
	Folder    string   `yaml:"folder,omitempty"`
}

// convertRulesToYAML converts rules to YAML format, preserving order
// Returns both the map and the order of rule names
func convertRulesToYAML(rules radikron.Rules) (rulesMap map[string]*ruleYAML, order []string) {
	if len(rules) == 0 {
		return nil, nil
	}
	rulesMap = make(map[string]*ruleYAML, len(rules))
	order = make([]string, 0, len(rules))
	for _, rule := range rules {
		ruleYAMLObj := &ruleYAML{
			Folder: rule.Folder,
		}
		if rule.HasStationID() {
			ruleYAMLObj.StationID = rule.StationID
		}
		if rule.HasTitle() {
			ruleYAMLObj.Title = rule.Title
		}
		if rule.HasDoW() {
			ruleYAMLObj.DoW = rule.DoW
		}
		if rule.HasKeyword() {
			ruleYAMLObj.Keyword = rule.Keyword
		}
		if rule.HasPfm() {
			ruleYAMLObj.Pfm = rule.Pfm
		}
		if rule.HasWindow() {
			ruleYAMLObj.Window = rule.Window
		}
		rulesMap[rule.Name] = ruleYAMLObj
		order = append(order, rule.Name)
	}
	return rulesMap, order
}

// SaveConfig saves the configuration to a file in YAML format
func (c *Config) SaveConfig(filename string) error {
	// Convert absolute path
	configPath, err := filepath.Abs(filename)
	if err != nil {
		return fmt.Errorf("invalid config path: %w", err)
	}

	// Create directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, DirPermissions); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Convert config to YAML structure
	cfgYAML := configYAML{
		AreaID:            c.AreaID,
		ExtraStations:     c.ExtraStations,
		IgnoreStations:    c.IgnoreStations,
		FileFormat:        c.FileFormat,
		MinimumOutputSize: c.MinimumOutputSize / (radikron.Kilobytes * radikron.Kilobytes), // Convert bytes to MB
		DownloadDir:       c.DownloadDir,
	}

	// Only include concurrency settings if they differ from defaults
	if c.MaxDownloadingConcurrency != radikron.MaxDownloadingConcurrency {
		cfgYAML.MaxDownloadingConcurrency = &c.MaxDownloadingConcurrency
	}
	if c.MaxEncodingConcurrency != radikron.MaxEncodingConcurrency {
		cfgYAML.MaxEncodingConcurrency = &c.MaxEncodingConcurrency
	}

	// Convert rules to YAML format, preserving order
	cfgYAML.Rules, cfgYAML.rulesOrder = convertRulesToYAML(c.Rules)

	// Marshal to YAML using custom marshaler to preserve order
	data, err := marshalConfigWithOrder(&cfgYAML)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write atomically to file
	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, FilePermissions); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath) // Clean up on failure
		return fmt.Errorf("failed to rename config file: %w", err)
	}
	return nil
}

// marshalConfigWithOrder marshals config to YAML while preserving the order of rules
func marshalConfigWithOrder(cfg *configYAML) ([]byte, error) {
	// First, marshal the config without rules to get the base structure
	cfgWithoutRules := *cfg
	cfgWithoutRules.Rules = nil
	cfgWithoutRules.rulesOrder = nil

	baseData, err := yaml.Marshal(&cfgWithoutRules)
	if err != nil {
		return nil, err
	}

	// Parse the base YAML to get a node structure
	var baseNode yaml.Node
	if err := yaml.Unmarshal(baseData, &baseNode); err != nil {
		return nil, fmt.Errorf("failed to parse base YAML: %w", err)
	}

	// Find the root mapping node
	var rootMapping *yaml.Node
	if baseNode.Kind == yaml.DocumentNode && len(baseNode.Content) > 0 {
		rootMapping = baseNode.Content[0]
	} else {
		rootMapping = &baseNode
	}

	if rootMapping.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected mapping node")
	}

	// Add rules section with preserved order
	if len(cfg.Rules) > 0 {
		// Create key node for "rules"
		rulesKeyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "rules",
		}

		// Create mapping node for rules
		rulesMappingNode := &yaml.Node{
			Kind: yaml.MappingNode,
		}

		// Add rules in order
		for _, ruleName := range cfg.rulesOrder {
			rule, exists := cfg.Rules[ruleName]
			if !exists {
				continue
			}

			// Create key node for rule name
			ruleKeyNode := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: ruleName,
			}

			// Marshal rule to get its node structure
			ruleData, err := yaml.Marshal(rule)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal rule %s: %w", ruleName, err)
			}

			var ruleDocNode yaml.Node
			if err := yaml.Unmarshal(ruleData, &ruleDocNode); err != nil {
				return nil, fmt.Errorf("failed to parse rule %s: %w", ruleName, err)
			}

			// Get the mapping node from the rule document
			var ruleMapping *yaml.Node
			if ruleDocNode.Kind == yaml.DocumentNode && len(ruleDocNode.Content) > 0 {
				ruleMapping = ruleDocNode.Content[0]
			} else {
				ruleMapping = &ruleDocNode
			}

			// Ensure it's a mapping node
			if ruleMapping.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("expected mapping node for rule %s", ruleName)
			}

			// Add key-value pair to rules mapping
			rulesMappingNode.Content = append(rulesMappingNode.Content, ruleKeyNode, ruleMapping)
		}

		// Add rules key-value pair to root mapping
		rootMapping.Content = append(rootMapping.Content, rulesKeyNode, rulesMappingNode)
	}

	// Marshal the complete node structure
	return yaml.Marshal(&baseNode)
}

// findRulesNode finds the "rules" mapping node in the YAML document.
// It handles DocumentNode -> root mapping and iterates mapping pairs to find the "rules" key.
// Returns the rules node if found, or nil if not found or invalid.
func findRulesNode(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}

	// Handle DocumentNode -> get the root mapping
	var rootMapping *yaml.Node
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil
		}
		rootMapping = root.Content[0]
	} else {
		rootMapping = root
	}

	// Must be a mapping node
	if rootMapping.Kind != yaml.MappingNode {
		return nil
	}

	// Iterate through key-value pairs to find "rules"
	for i := 0; i < len(rootMapping.Content); i += 2 {
		if i+1 >= len(rootMapping.Content) {
			continue
		}
		keyNode := rootMapping.Content[i]
		if keyNode.Value == "rules" {
			return rootMapping.Content[i+1]
		}
	}

	return nil
}

// parseRuleFromNode decodes a rule node into a radikron.Rule.
// It decodes the ruleNode into a map, sets temporary viper keys for that rule,
// unmarshals into a radikron.Rule, sets its name from nameNode, and returns it.
func parseRuleFromNode(nameNode, ruleNode *yaml.Node) (*radikron.Rule, error) {
	if nameNode == nil || ruleNode == nil {
		return nil, fmt.Errorf("nameNode and ruleNode must not be nil")
	}

	name := nameNode.Value
	if name == "" {
		return nil, fmt.Errorf("rule name cannot be empty")
	}

	// Convert rule node to a map for viper to process
	// Viper's UnmarshalKey respects mapstructure tags
	var ruleMap map[string]any
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

	return rule, nil
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

	// Validate config file exists before attempting to read
	if _, err := os.Stat(configFile); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file does not exist: %s", configFile)
		}
		return nil, fmt.Errorf("failed to access config file: %w", err)
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
	rulesNode := findRulesNode(&rootNode)

	if rulesNode == nil || rulesNode.Kind != yaml.MappingNode {
		return rules, nil
	}

	// Iterate through rules in order (yaml.Node.Content preserves order)
	for i := 0; i < len(rulesNode.Content); i += 2 {
		if i+1 >= len(rulesNode.Content) {
			continue
		}
		nameNode := rulesNode.Content[i]
		ruleNode := rulesNode.Content[i+1]

		rule, err := parseRuleFromNode(nameNode, ruleNode)
		if err != nil {
			return nil, err
		}
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
