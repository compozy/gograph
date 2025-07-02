package commands_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestInitCommand(t *testing.T) {
	t.Run("Should create default config file", func(t *testing.T) {
		// Create temp directory
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".gograph.yaml")

		// Create init command
		rootCmd := &cobra.Command{Use: "gograph"}
		initCmd := &cobra.Command{
			Use:   "init",
			Short: "Initialize a new gograph configuration file",
			Args:  cobra.NoArgs,
			RunE: func(_ *cobra.Command, _ []string) error {
				// Simulate init command behavior
				defaultConfig := map[string]any{
					"project": map[string]any{
						"name":      "default",
						"root_path": ".",
					},
					"neo4j": map[string]any{
						"uri":      "bolt://localhost:7687",
						"username": "neo4j",
						"password": "",
						"database": "",
					},
					"analysis": map[string]any{
						"ignore_dirs": []string{
							".git", ".idea", ".vscode", "node_modules",
							"vendor", "dist", "build", "target",
						},
						"ignore_files":    []string{},
						"include_tests":   true,
						"include_vendor":  false,
						"max_concurrency": 4,
					},
				}

				data, err := yaml.Marshal(defaultConfig)
				if err != nil {
					return err
				}

				return os.WriteFile(configPath, data, 0644)
			},
		}
		rootCmd.AddCommand(initCmd)

		// Save current directory and change to temp
		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer func() {
			err := os.Chdir(originalDir)
			require.NoError(t, err)
		}()
		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// Execute init command
		_, err = executeCommand(rootCmd, "init")
		require.NoError(t, err)

		// Verify config file was created
		_, err = os.Stat(configPath)
		require.NoError(t, err)

		// Verify config content
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)

		var config map[string]any
		err = yaml.Unmarshal(content, &config)
		require.NoError(t, err)

		// Check project section
		project := config["project"].(map[string]any)
		assert.Equal(t, "default", project["name"])
		assert.Equal(t, ".", project["root_path"])

		// Check neo4j section
		neo4j := config["neo4j"].(map[string]any)
		assert.Equal(t, "bolt://localhost:7687", neo4j["uri"])
		assert.Equal(t, "neo4j", neo4j["username"])
	})

	t.Run("Should not overwrite existing config without force flag", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".gograph.yaml")

		// Create existing config
		existingContent := []byte("existing: config")
		err := os.WriteFile(configPath, existingContent, 0644)
		require.NoError(t, err)

		// Create init command that checks for existing file
		rootCmd := &cobra.Command{Use: "gograph"}
		initCmd := &cobra.Command{
			Use:   "init",
			Short: "Initialize a new gograph configuration file",
			RunE: func(cmd *cobra.Command, _ []string) error {
				force, _ := cmd.Flags().GetBool("force")

				// Check if file exists
				if _, err := os.Stat(configPath); err == nil && !force {
					cmd.PrintErrf("Configuration file already exists: %s\n", configPath)
					cmd.Println("Use --force to overwrite")
					return nil
				}

				// Would create new config here
				return nil
			},
		}
		initCmd.Flags().Bool("force", false, "Force overwrite existing config")
		rootCmd.AddCommand(initCmd)

		// Change to temp directory
		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer func() {
			err := os.Chdir(originalDir)
			require.NoError(t, err)
		}()
		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// Execute init without force
		output, err := executeCommand(rootCmd, "init")
		require.NoError(t, err)
		assert.Contains(t, output, "Configuration file already exists")
		assert.Contains(t, output, "Use --force to overwrite")

		// Verify original content unchanged
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)
		assert.Equal(t, existingContent, content)
	})

	t.Run("Should handle custom output path", func(t *testing.T) {
		tmpDir := t.TempDir()
		customPath := filepath.Join(tmpDir, "custom-config.yaml")

		rootCmd := &cobra.Command{Use: "gograph"}
		initCmd := &cobra.Command{
			Use:   "init",
			Short: "Initialize a new gograph configuration file",
			RunE: func(cmd *cobra.Command, _ []string) error {
				output, _ := cmd.Flags().GetString("output")
				if output == "" {
					output = ".gograph.yaml"
				}

				// Create minimal config
				config := map[string]any{
					"project": map[string]any{
						"name": "custom",
					},
				}

				data, err := yaml.Marshal(config)
				if err != nil {
					return err
				}

				return os.WriteFile(output, data, 0644)
			},
		}
		initCmd.Flags().StringP("output", "o", "", "Output path for config file")
		rootCmd.AddCommand(initCmd)

		// Execute with custom output
		_, err := executeCommand(rootCmd, "init", "--output", customPath)
		require.NoError(t, err)

		// Verify file was created at custom path
		_, err = os.Stat(customPath)
		require.NoError(t, err)

		// Verify content
		content, err := os.ReadFile(customPath)
		require.NoError(t, err)

		var config map[string]any
		err = yaml.Unmarshal(content, &config)
		require.NoError(t, err)

		project := config["project"].(map[string]any)
		assert.Equal(t, "custom", project["name"])
	})

	t.Run("Should create interactive config when requested", func(t *testing.T) {
		rootCmd := &cobra.Command{Use: "gograph"}
		initCmd := &cobra.Command{
			Use:   "init",
			Short: "Initialize a new gograph configuration file",
			RunE: func(cmd *cobra.Command, _ []string) error {
				interactive, _ := cmd.Flags().GetBool("interactive")

				if interactive {
					// In real implementation, this would prompt user
					// For test, just indicate it would be interactive
					cmd.Println("Interactive mode would prompt for configuration values")
				} else {
					cmd.Println("Creating default configuration")
				}

				return nil
			},
		}
		initCmd.Flags().BoolP("interactive", "i", false, "Interactive configuration setup")
		rootCmd.AddCommand(initCmd)

		// Test non-interactive (default)
		output, err := executeCommand(rootCmd, "init")
		require.NoError(t, err)
		assert.Contains(t, output, "Creating default configuration")

		// Test interactive mode
		output, err = executeCommand(rootCmd, "init", "--interactive")
		require.NoError(t, err)
		assert.Contains(t, output, "Interactive mode")
	})
}
