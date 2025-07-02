package commands

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/spf13/cobra"
)

// Version information
var (
	Version   = "0.1.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long: `Display detailed version information about gograph including the version
number, build time, Git commit hash, and Go runtime version.`,
	Example: `  # Show version information
  gograph version
  
  # Example output:
  # GoGraph - Go Codebase Analyzer
  # Version:    0.1.0
  # Build Time: 2024-01-01T12:00:00Z
  # Git Commit: abc123def
  # Go Version: go1.22.0
  # OS/Arch:    darwin/arm64`,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("GoGraph - Go Codebase Analyzer")
		fmt.Printf("Version:    %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

var initVersionOnce sync.Once

// InitVersionCommand registers the version command
func InitVersionCommand() {
	initVersionOnce.Do(func() {
		rootCmd.AddCommand(versionCmd)
	})
}
