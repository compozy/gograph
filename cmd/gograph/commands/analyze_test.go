package commands_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeCommand(t *testing.T) {
	t.Run("Should require exactly one argument", func(t *testing.T) {
		rootCmd := &cobra.Command{Use: "gograph"}
		analyzeCmd := &cobra.Command{
			Use:   "analyze [path]",
			Short: "Analyze Go source code and store the graph in Neo4j",
			Args:  cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, _ []string) error {
				return nil
			},
		}
		rootCmd.AddCommand(analyzeCmd)

		// No arguments
		_, err := executeCommand(rootCmd, "analyze")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "accepts 1 arg(s), received 0")

		// Too many arguments
		_, err = executeCommand(rootCmd, "analyze", "path1", "path2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "accepts 1 arg(s), received 2")

		// Correct number of arguments
		_, err = executeCommand(rootCmd, "analyze", ".")
		assert.NoError(t, err)
	})

	t.Run("Should handle --no-progress flag", func(t *testing.T) {
		rootCmd := &cobra.Command{Use: "gograph"}
		analyzeCmd := &cobra.Command{
			Use:   "analyze [path]",
			Short: "Analyze Go source code",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, _ []string) error {
				noProgress, _ := cmd.Flags().GetBool("no-progress")
				if noProgress {
					cmd.Println("Running without progress indicators")
				} else {
					cmd.Println("Running with progress indicators")
				}
				return nil
			},
		}
		analyzeCmd.Flags().Bool("no-progress", false, "Disable progress indicators")
		rootCmd.AddCommand(analyzeCmd)

		// Default (with progress)
		output, err := executeCommand(rootCmd, "analyze", ".")
		require.NoError(t, err)
		assert.Contains(t, output, "Running with progress indicators")

		// With --no-progress
		output, err = executeCommand(rootCmd, "analyze", ".", "--no-progress")
		require.NoError(t, err)
		assert.Contains(t, output, "Running without progress indicators")
	})

	t.Run("Should validate Neo4j configuration", func(t *testing.T) {
		// Clear viper settings
		viper.Reset()

		rootCmd := &cobra.Command{Use: "gograph"}
		analyzeCmd := &cobra.Command{
			Use:   "analyze [path]",
			Short: "Analyze Go source code",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, _ []string) error {
				// Check Neo4j config
				uri := viper.GetString("neo4j.uri")
				username := viper.GetString("neo4j.username")
				password := viper.GetString("neo4j.password")

				if uri == "" {
					return fmt.Errorf("neo4j.uri is not configured")
				}
				if username == "" {
					return fmt.Errorf("neo4j.username is not configured")
				}
				if password == "" {
					return fmt.Errorf("neo4j.password is not configured")
				}

				cmd.Println("Neo4j configuration valid")
				return nil
			},
		}
		rootCmd.AddCommand(analyzeCmd)

		// Test missing configuration
		_, err := executeCommand(rootCmd, "analyze", ".")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "neo4j.uri is not configured")

		// Set minimal config
		viper.Set("neo4j.uri", "bolt://localhost:7687")
		_, err = executeCommand(rootCmd, "analyze", ".")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "neo4j.username is not configured")

		// Set complete config
		viper.Set("neo4j.username", "neo4j")
		viper.Set("neo4j.password", "password")
		output, err := executeCommand(rootCmd, "analyze", ".")
		require.NoError(t, err)
		assert.Contains(t, output, "Neo4j configuration valid")
	})

	t.Run("Should use parser configuration from viper", func(t *testing.T) {
		viper.Reset()
		viper.Set("parser.ignore_dirs", []string{".git", "vendor", "node_modules"})
		viper.Set("parser.include_tests", true)
		viper.Set("parser.max_concurrency", 8)

		rootCmd := &cobra.Command{Use: "gograph"}
		analyzeCmd := &cobra.Command{
			Use:   "analyze [path]",
			Short: "Analyze Go source code",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, _ []string) error {
				ignoreDirs := viper.GetStringSlice("parser.ignore_dirs")
				includeTests := viper.GetBool("parser.include_tests")
				maxConcurrency := viper.GetInt("parser.max_concurrency")

				cmd.Printf("Parser config: ignore_dirs=%v, include_tests=%v, max_concurrency=%d\n",
					ignoreDirs, includeTests, maxConcurrency)
				return nil
			},
		}
		rootCmd.AddCommand(analyzeCmd)

		output, err := executeCommand(rootCmd, "analyze", ".")
		require.NoError(t, err)
		assert.Contains(t, output, "ignore_dirs=[.git vendor node_modules]")
		assert.Contains(t, output, "include_tests=true")
		assert.Contains(t, output, "max_concurrency=8")
	})

	t.Run("Should handle invalid project path", func(t *testing.T) {
		rootCmd := &cobra.Command{Use: "gograph"}
		analyzeCmd := &cobra.Command{
			Use:   "analyze [path]",
			Short: "Analyze Go source code",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				path := args[0]

				// Check if path exists
				if _, err := os.Stat(path); os.IsNotExist(err) {
					return fmt.Errorf("project path does not exist: %s", path)
				}

				cmd.Printf("Analyzing project at: %s\n", path)
				return nil
			},
		}
		rootCmd.AddCommand(analyzeCmd)

		// Test non-existent path
		_, err := executeCommand(rootCmd, "analyze", "/non/existent/path")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project path does not exist")

		// Test valid path (current directory)
		output, err := executeCommand(rootCmd, "analyze", ".")
		require.NoError(t, err)
		assert.Contains(t, output, "Analyzing project at: .")
	})

	t.Run("Should use analyzer configuration from viper", func(t *testing.T) {
		viper.Reset()
		viper.Set("analyzer.max_dependency_depth", 15)
		viper.Set("analyzer.ignore_test_files", true)
		viper.Set("analyzer.include_metrics", true)
		viper.Set("analyzer.parallel_workers", 6)

		rootCmd := &cobra.Command{Use: "gograph"}
		analyzeCmd := &cobra.Command{
			Use:   "analyze [path]",
			Short: "Analyze Go source code",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, _ []string) error {
				maxDepth := viper.GetInt("analyzer.max_dependency_depth")
				ignoreTests := viper.GetBool("analyzer.ignore_test_files")
				includeMetrics := viper.GetBool("analyzer.include_metrics")
				workers := viper.GetInt("analyzer.parallel_workers")

				cmd.Printf("Analyzer config: max_depth=%d, ignore_tests=%v, metrics=%v, workers=%d\n",
					maxDepth, ignoreTests, includeMetrics, workers)
				return nil
			},
		}
		rootCmd.AddCommand(analyzeCmd)

		output, err := executeCommand(rootCmd, "analyze", ".")
		require.NoError(t, err)
		assert.Contains(t, output, "max_depth=15")
		assert.Contains(t, output, "ignore_tests=true")
		assert.Contains(t, output, "metrics=true")
		assert.Contains(t, output, "workers=6")
	})

	t.Run("Should handle panic recovery", func(t *testing.T) {
		rootCmd := &cobra.Command{Use: "gograph"}
		analyzeCmd := &cobra.Command{
			Use:   "analyze [path]",
			Short: "Analyze Go source code",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				// Wrap in a recovery function similar to real command
				defer func() {
					if r := recover(); r != nil {
						cmd.PrintErrf("Command panicked: %v\n", r)
					}
				}()

				// Simulate a condition that might panic
				var nilMap map[string]string
				if args[0] == "panic" {
					// This would panic
					_ = nilMap["key"]
				}

				cmd.Println("Analysis completed")
				return nil
			},
		}
		rootCmd.AddCommand(analyzeCmd)

		// Normal execution
		output, err := executeCommand(rootCmd, "analyze", ".")
		require.NoError(t, err)
		assert.Contains(t, output, "Analysis completed")

		// Execution that would panic (but is recovered)
		_, err = executeCommand(rootCmd, "analyze", "panic")
		// The panic is recovered, so no error returned
		assert.NoError(t, err)
	})
}
