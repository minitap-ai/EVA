package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	NotionAPIKey     string       `yaml:"notion_api_key"`
	NotionDatabaseID string       `yaml:"notion_database_id"`
	GitHubToken      string       `yaml:"github_token"`
	Devops           DevopsConfig `yaml:"devops"`
}

// Load reads the config file and returns the parsed config.
// It does NOT enforce any required fields — callers validate what they need.
func Load() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".eva", "config.yaml")

	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file at %s: %w", configPath, err)
	}
	defer file.Close()

	var cfg Config
	if err := yaml.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config format: %w", err)
	}

	return &cfg, nil
}

// RequireNotion validates that Notion fields are present in the config.
func (c *Config) RequireNotion() error {
	if c.NotionAPIKey == "" || c.NotionDatabaseID == "" {
		return fmt.Errorf("missing required fields in config (notion_api_key or notion_database_id)")
	}
	return nil
}
