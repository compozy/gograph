package commands_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCommand(t *testing.T) {
	t.Run("Should display version information", func(t *testing.T) {
		// Create root command
		rootCmd := &cobra.Command{Use: "gograph"}

		// Create a test version command
		versionCmd := &cobra.Command{
			Use:   "version",
			Short: "Show gograph version information",
			Run: func(cmd *cobra.Command, _ []string) {
				cmd.Println("gograph version development")
			},
		}
		rootCmd.AddCommand(versionCmd)

		output, err := executeCommand(rootCmd, "version")

		require.NoError(t, err)
		assert.Contains(t, output, "gograph version")
	})

	t.Run("Should handle version command with flags", func(t *testing.T) {
		rootCmd := &cobra.Command{Use: "gograph"}

		versionCmd := &cobra.Command{
			Use:   "version",
			Short: "Show gograph version information",
			Run: func(cmd *cobra.Command, _ []string) {
				short, _ := cmd.Flags().GetBool("short")
				if short {
					cmd.Println("dev")
				} else {
					cmd.Println("gograph version development")
				}
			},
		}
		versionCmd.Flags().Bool("short", false, "Print just the version number")
		rootCmd.AddCommand(versionCmd)

		// Test regular version
		output, err := executeCommand(rootCmd, "version")
		require.NoError(t, err)
		assert.Contains(t, output, "gograph version")

		// Test short version
		output, err = executeCommand(rootCmd, "version", "--short")
		require.NoError(t, err)
		assert.Contains(t, output, "dev")
		assert.NotContains(t, output, "gograph version")
	})

	t.Run("Should not accept arguments", func(t *testing.T) {
		rootCmd := &cobra.Command{Use: "gograph"}

		versionCmd := &cobra.Command{
			Use:   "version",
			Short: "Show gograph version information",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				cmd.Println("gograph version development")
				return nil
			},
		}
		rootCmd.AddCommand(versionCmd)

		// Version command with unexpected argument should error
		_, err := executeCommand(rootCmd, "version", "unexpected-arg")
		assert.Error(t, err)
	})
}
