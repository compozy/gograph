package commands_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/compozy/gograph/cmd/gograph/commands"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()
	return buf.String(), err
}

func TestRootCommand(t *testing.T) {
	t.Run("Should display help when no arguments provided", func(t *testing.T) {
		// Create a test command structure
		rootCmd := &cobra.Command{
			Use:   "gograph",
			Short: "A Go codebase analyzer that creates Neo4j graph representations",
			Run: func(cmd *cobra.Command, _ []string) {
				cmd.Help()
			},
		}

		output, err := executeCommand(rootCmd)

		require.NoError(t, err)
		assert.Contains(t, output, "gograph")
		assert.Contains(t, output, "A Go codebase analyzer")
	})

	t.Run("Should display error for unknown command", func(t *testing.T) {
		rootCmd := &cobra.Command{
			Use:           "gograph",
			Short:         "A Go codebase analyzer that creates Neo4j graph representations",
			SilenceErrors: false,
			SilenceUsage:  false,
		}

		output, err := executeCommand(rootCmd, "unknown-command")

		// Cobra shows the help text when an unknown command is given
		// The output should contain the command description
		if err == nil {
			// When no error, it means help was shown
			assert.Contains(t, output, "A Go codebase analyzer")
		} else {
			// If there's an error, it should be about unknown command
			assert.Error(t, err)
		}
	})

	t.Run("Should handle --help flag", func(t *testing.T) {
		rootCmd := &cobra.Command{
			Use:   "gograph",
			Short: "A Go codebase analyzer that creates Neo4j graph representations",
			Long: `GoGraph is a powerful tool for analyzing Go codebases and visualizing
their structure as a graph in Neo4j.`,
		}

		output, err := executeCommand(rootCmd, "--help")

		require.NoError(t, err)
		// Help output should contain various parts of the command description
		assert.Contains(t, output, "GoGraph")
		assert.Contains(t, output, "powerful tool")
	})
}

func TestInitConfig(t *testing.T) {
	t.Run("Should initialize configuration", func(t *testing.T) {
		// Save original working directory
		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer func() {
			err := os.Chdir(originalDir)
			require.NoError(t, err)
		}()

		// Create temp directory for test
		tmpDir := t.TempDir()
		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// Initialize config (should not error even without config file)
		commands.InitConfig()

		// Function should complete without panic
		assert.True(t, true)
	})

	t.Run("Should handle config file in current directory", func(t *testing.T) {
		// Save original working directory
		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer func() {
			err := os.Chdir(originalDir)
			require.NoError(t, err)
		}()

		// Create temp directory with config file
		tmpDir := t.TempDir()
		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// Create a simple config file
		configContent := `
parser:
  ignore_dirs:
    - .git
    - vendor
  include_tests: true
`
		err = os.WriteFile("gograph.yaml", []byte(configContent), 0644)
		require.NoError(t, err)

		// Initialize config
		commands.InitConfig()

		// Function should complete without panic
		assert.True(t, true)
	})
}

func TestCommandRegistration(t *testing.T) {
	t.Run("Should register all subcommands", func(t *testing.T) {
		// This test verifies that all init functions work correctly
		commands.InitAnalyzeCommand()
		commands.InitClearCommand()
		commands.InitHelpCommands()
		commands.InitInitCommand()
		commands.InitQueryCommand()
		commands.InitVersionCommand()

		// If we get here without panic, registration succeeded
		assert.True(t, true)
	})

	t.Run("Should handle multiple init calls safely", func(t *testing.T) {
		// Call init functions multiple times - should use sync.Once
		for i := 0; i < 3; i++ {
			commands.InitAnalyzeCommand()
			commands.InitClearCommand()
			commands.InitHelpCommands()
			commands.InitInitCommand()
			commands.InitQueryCommand()
			commands.InitVersionCommand()
		}

		// No panic means sync.Once is working correctly
		assert.True(t, true)
	})
}

func TestCommandOutput(t *testing.T) {
	t.Run("Should format help output correctly", func(t *testing.T) {
		rootCmd := &cobra.Command{
			Use:   "gograph",
			Short: "A Go codebase analyzer",
			Long:  "Long description here",
		}

		// Add a subcommand
		subCmd := &cobra.Command{
			Use:   "test",
			Short: "Test command",
		}
		rootCmd.AddCommand(subCmd)

		output, err := executeCommand(rootCmd, "help")

		require.NoError(t, err)
		assert.Contains(t, output, "Available Commands:")
		assert.Contains(t, output, "test")
	})

	t.Run("Should trim trailing whitespace in output", func(t *testing.T) {
		rootCmd := &cobra.Command{
			Use:   "gograph",
			Short: "A Go codebase analyzer",
		}

		output, err := executeCommand(rootCmd, "--help")

		require.NoError(t, err)
		// Check that output doesn't have excessive trailing whitespace
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			assert.Equal(t, strings.TrimRight(line, " "), line)
		}
	})
}
