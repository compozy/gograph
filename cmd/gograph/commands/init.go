package commands

import (
	"fmt"
	"os"
	"sync"

	"github.com/compozy/gograph/pkg/config"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new gograph project configuration file",
	Long: `Initialize creates a new gograph.yaml configuration file in the current
directory with project-specific settings. This file allows you to customize 
project identification, parser and analyzer behavior, Neo4j connection settings, and more.

The configuration file includes:
  • Project identification (ID, name, path)
  • Parser settings (ignore patterns, concurrency)
  • Analyzer settings (dependency depth, metrics)
  • Neo4j connection details

A unique project-id is required to isolate your project's data from other projects
in the same Neo4j database. This enables multiple projects to coexist safely.`,
	Example: `  # Initialize a new project with a unique ID
  gograph init --project-id "my-webapp"
  
  # Initialize with custom name and path
  gograph init --project-id "backend-api" --project-name "Backend API" --project-path "./src"
  
  # After creation, edit gograph.yaml to customize:
  # - Neo4j connection details
  # - Parser ignore patterns
  # - Analysis depth and metrics`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		configFile := "gograph.yaml"
		if cfgFile != "" {
			configFile = cfgFile
		}

		// Check if file exists and force flag is not set
		if _, err := os.Stat(configFile); err == nil && !forceOverwrite {
			return fmt.Errorf("config file %s already exists. Use --force to overwrite", configFile)
		}

		// Get project details from flags
		projectID, err := cmd.Flags().GetString("project-id")
		if err != nil {
			return fmt.Errorf("failed to get project-id flag: %w", err)
		}
		projectName, err := cmd.Flags().GetString("project-name")
		if err != nil {
			return fmt.Errorf("failed to get project-name flag: %w", err)
		}
		projectPath, err := cmd.Flags().GetString("project-path")
		if err != nil {
			return fmt.Errorf("failed to get project-path flag: %w", err)
		}

		// Validate required project ID
		if projectID == "" {
			return fmt.Errorf("project-id is required. Use --project-id flag to specify a unique project identifier")
		}

		// Set defaults for optional fields
		if projectName == "" {
			projectName = projectID
		}
		if projectPath == "" {
			projectPath = "."
		}

		// Create configuration with user-provided values
		cfg := config.DefaultConfig()
		cfg.Project.ID = projectID
		cfg.Project.Name = projectName
		cfg.Project.RootPath = projectPath

		// Save configuration to file
		if err := config.Save(cfg, configFile); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}

		fmt.Printf("✓ Configuration file '%s' created successfully\n", configFile)
		fmt.Printf("✓ Project ID: %s\n", projectID)
		fmt.Printf("✓ Project Name: %s\n", projectName)
		fmt.Printf("✓ Project Path: %s\n", projectPath)
		fmt.Println("\nNext steps:")
		fmt.Println("1. Edit the config file to configure Neo4j connection if needed")
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
		initCmd.Flags().String("project-id", "", "Unique project identifier (required)")
		initCmd.Flags().String("project-name", "", "Human-readable project name (defaults to project-id)")
		initCmd.Flags().String("project-path", ".", "Project root path (defaults to current directory)")
		if err := initCmd.MarkFlagRequired("project-id"); err != nil {
			panic(fmt.Sprintf("failed to mark project-id as required: %v", err))
		}
		rootCmd.AddCommand(initCmd)
	})
}
