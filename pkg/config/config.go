package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/gograph/engine/core"
	"github.com/spf13/viper"
)

const (
	defaultConfigFileName = ".gograph"
	defaultConfigType     = "yaml"
	defaultNeo4jURI       = "bolt://localhost:7687"
	defaultNeo4jUser      = "neo4j"
)

// Config represents the application configuration
type Config struct {
	Project  ProjectConfig  `mapstructure:"project"`
	Neo4j    Neo4jConfig    `mapstructure:"neo4j"`
	Analysis AnalysisConfig `mapstructure:"analysis"`
}

// ProjectConfig represents project-specific configuration
type ProjectConfig struct {
	ID       string `mapstructure:"id"`
	Name     string `mapstructure:"name"`
	RootPath string `mapstructure:"root_path"`
}

// Neo4jConfig represents Neo4j connection configuration
type Neo4jConfig struct {
	URI      string `mapstructure:"uri"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

// AnalysisConfig represents analysis configuration
type AnalysisConfig struct {
	IgnoreDirs     []string `mapstructure:"ignore_dirs"`
	IgnoreFiles    []string `mapstructure:"ignore_files"`
	IncludeTests   bool     `mapstructure:"include_tests"`
	IncludeVendor  bool     `mapstructure:"include_vendor"`
	MaxConcurrency int      `mapstructure:"max_concurrency"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Project: ProjectConfig{
			ID:       "",
			Name:     "default",
			RootPath: ".",
		},
		Neo4j: Neo4jConfig{
			URI:      defaultNeo4jURI,
			Username: defaultNeo4jUser,
			Password: "",
			Database: "",
		},
		Analysis: AnalysisConfig{
			IgnoreDirs:     []string{".git", ".idea", ".vscode", "node_modules"},
			IgnoreFiles:    []string{},
			IncludeTests:   true,
			IncludeVendor:  false,
			MaxConcurrency: 4,
		},
	}
}

// Load loads configuration from a file
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath == "" {
		// Look for config file in current directory - try both formats
		possiblePaths := []string{
			filepath.Join(".", "gograph.yaml"),                              // New format
			filepath.Join(".", defaultConfigFileName+"."+defaultConfigType), // Legacy format (.gograph.yaml)
		}

		configPath = ""
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}

		if configPath == "" {
			return cfg, nil // Return default config if no config file found
		}
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil // Return default config
	}

	viper.SetConfigFile(configPath)
	viper.SetConfigType(defaultConfigType)

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal config
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// Save saves the configuration to a file
func Save(cfg *Config, configPath string) error {
	if configPath == "" {
		configPath = filepath.Join(".", defaultConfigFileName+"."+defaultConfigType)
	}

	viper.SetConfigFile(configPath)
	viper.SetConfigType(defaultConfigType)

	// Set all values
	viper.Set("project", cfg.Project)
	viper.Set("neo4j", cfg.Neo4j)
	viper.Set("analysis", cfg.Analysis)

	// Write config file
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ToProject converts the config to a core.Project
func (c *Config) ToProject(configPath string) *core.Project {
	return &core.Project{
		ID:         core.ID(c.Project.ID),
		Name:       c.Project.Name,
		RootPath:   c.Project.RootPath,
		Neo4jURI:   c.Neo4j.URI,
		Neo4jUser:  c.Neo4j.Username,
		ConfigPath: configPath,
	}
}

// Validate ensures the configuration is valid
func (c *Config) Validate() error {
	// Project ID is required
	if c.Project.ID == "" {
		return fmt.Errorf("project.id is required - run 'gograph init --project-id <your-project-id>' to initialize")
	}

	// Set defaults for optional fields
	if c.Project.Name == "" {
		c.Project.Name = c.Project.ID
	}

	if c.Project.RootPath == "" {
		c.Project.RootPath = "."
	}

	return nil
}
