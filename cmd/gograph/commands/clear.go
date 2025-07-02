package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/infra"
	"github.com/compozy/gograph/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var clearCmd = &cobra.Command{
	Use:   "clear [project-id]",
	Short: "Clear project data from Neo4j",
	Long: `Clear removes graph data from the Neo4j database. You can either clear
data for a specific project by providing its ID, or clear all data from
the database.

This command is useful when:
  • You want to re-analyze a project from scratch
  • You need to clean up after testing
  • You want to remove outdated analysis data
  • You need to free up space in Neo4j

Safety features:
  • Confirmation prompt before deletion (bypass with --force)
  • Dry-run mode to preview what will be deleted
  • Clear error messages if something goes wrong

WARNING: This operation cannot be undone! Make sure you have backups
if the data is important.`,
	Example: `  # Clear a specific project (with confirmation)
  gograph clear project-123
  
  # Clear all data (with confirmation)
  gograph clear
  
  # Clear without confirmation prompt
  gograph clear --force
  
  # See what would be cleared without actually clearing
  gograph clear --dry-run
  
  # Clear specific project without confirmation
  gograph clear project-123 --force`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get flags
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return fmt.Errorf("failed to get force flag: %w", err)
		}
		dryRun, err := cmd.Flags().GetBool("dry-run")
		if err != nil {
			return fmt.Errorf("failed to get dry-run flag: %w", err)
		}

		// Get Neo4j configuration with fallback to defaults
		neo4jURI := viper.GetString("neo4j.uri")
		if neo4jURI == "" {
			neo4jURI = DefaultNeo4jURI // Default only if not set via env vars
		}
		neo4jUsername := viper.GetString("neo4j.username")
		if neo4jUsername == "" {
			neo4jUsername = DefaultNeo4jUsername // Default only if not set via env vars
		}
		neo4jPassword := viper.GetString("neo4j.password")
		if neo4jPassword == "" {
			neo4jPassword = DefaultNeo4jPassword // Default only if not set via env vars
		}

		neo4jConfig := &infra.Neo4jConfig{
			URI:        neo4jURI,
			Username:   neo4jUsername,
			Password:   neo4jPassword,
			Database:   viper.GetString("neo4j.database"),
			MaxRetries: 3,
			BatchSize:  1000,
		}

		// Initialize repository
		repo, err := infra.NewNeo4jRepository(neo4jConfig)
		if err != nil {
			return fmt.Errorf("failed to create Neo4j repository: %w", err)
		}
		defer repo.Close()

		ctx := context.Background()

		// Determine what to clear
		var targetDescription string
		var projectID core.ID

		if len(args) > 0 {
			// Clear specific project
			projectID = core.ID(args[0])
			targetDescription = fmt.Sprintf("project '%s'", projectID)

			// Validate project ID format (basic check)
			if projectID == "" {
				return fmt.Errorf("project ID cannot be empty")
			}
		} else {
			// Clear all data
			projectID = ""
			targetDescription = "ALL DATA in the Neo4j database"
		}

		// Show what will be cleared
		if dryRun {
			logger.Info("DRY RUN: Would clear", "target", targetDescription)
			return nil
		}

		// Confirm the operation if not forced
		if !force {
			fmt.Printf("\n⚠️  WARNING: This will permanently delete %s!\n", targetDescription)
			fmt.Print("Are you sure you want to continue? [y/N]: ")

			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}

			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				logger.Info("operation canceled")
				return nil
			}
		}

		// Perform the clear operation
		if projectID != "" {
			logger.Info("clearing project data", "project_id", projectID)

			if err := repo.ClearProject(ctx, projectID); err != nil {
				return fmt.Errorf("failed to clear project: %w", err)
			}

			logger.Info("✓ project data cleared successfully", "project_id", projectID)
		} else {
			logger.Warn("clearing ALL data from Neo4j database")

			// Use an empty ID to indicate clearing everything
			// The repository should handle this case
			if err := repo.ClearProject(ctx, ""); err != nil {
				return fmt.Errorf("failed to clear database: %w", err)
			}

			logger.Info("✓ all data cleared successfully")
		}

		return nil
	},
}

var initClearOnce sync.Once

// InitClearCommand registers the clear command
func InitClearCommand() {
	initClearOnce.Do(func() {
		rootCmd.AddCommand(clearCmd)

		// Add flags for safety
		clearCmd.Flags().BoolP("force", "f", false, "Force clear without confirmation")
		clearCmd.Flags().BoolP("dry-run", "d", false, "Show what would be cleared without actually clearing")
	})
}
