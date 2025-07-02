package commands

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new gograph configuration file",
	Long: `Initialize creates a new gograph.yaml configuration file in the current
directory with default settings. This file allows you to customize parser
and analyzer behavior, Neo4j connection settings, and more.

The configuration file includes:
  • Parser settings (ignore patterns, concurrency)
  • Analyzer settings (dependency depth, metrics)
  • Neo4j connection details
  • Logging preferences

Example:
  gograph init

This will create a gograph.yaml file with sensible defaults that you can
then customize according to your needs.`,
	Example: `  # Create a default configuration file
  gograph init
  
  # After creation, edit gograph.yaml to customize:
  # - Neo4j connection details
  # - Parser ignore patterns
  # - Analysis depth and metrics`,
	RunE: func(_ *cobra.Command, _ []string) error {
		configFile := "gograph.yaml"
		if cfgFile != "" {
			configFile = cfgFile
		}

		// Check if file exists and force flag is not set
		if _, err := os.Stat(configFile); err == nil && !forceOverwrite {
			return fmt.Errorf("config file %s already exists. Use --force to overwrite", configFile)
		}

		// Set default configuration values
		viper.Set("neo4j.uri", DefaultNeo4jURI)
		viper.Set("neo4j.username", DefaultNeo4jUsername)
		viper.Set("neo4j.password", DefaultNeo4jPassword)
		viper.Set("parser.ignore_dirs", []string{".git", "vendor", "node_modules"})
		viper.Set("parser.ignore_files", []string{})
		viper.Set("parser.include_tests", false)
		viper.Set("parser.max_concurrency", 4)
		viper.Set("analyzer.max_depth", 10)
		viper.Set("analyzer.detect_circular_deps", true)

		// Set configuration file path
		viper.SetConfigFile(configFile)

		// Write config file
		err := viper.WriteConfigAs(configFile)

		if err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}

		fmt.Printf("✓ Configuration file '%s' created successfully\n", configFile)
		fmt.Println("\nNext steps:")
		fmt.Println("1. Edit the config file to configure Neo4j connection")
		fmt.Println("2. Run 'gograph analyze <path>' to analyze your project")
		return nil
	},
}

var (
	initInitOnce   sync.Once
	forceOverwrite bool
)

// InitInitCommand registers the init command
func InitInitCommand() {
	initInitOnce.Do(func() {
		initCmd.Flags().BoolVar(&forceOverwrite, "force", false, "Force overwrite existing config file")
		rootCmd.AddCommand(initCmd)
	})
}
