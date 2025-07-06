package commands

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gograph",
	Short: "A Go codebase analyzer that creates Neo4j graph representations",
	Long: `GoGraph is a powerful tool for analyzing Go codebases and visualizing
their structure as a graph in Neo4j. It helps developers and LLMs understand
complex project architectures by mapping out packages, files, functions, 
structs, interfaces, and their relationships.

Key Features:
  • Parse and analyze Go source code
  • Detect interface implementations and dependencies
  • Map function call chains and circular dependencies
  • Store results in Neo4j for powerful graph queries
  • Query the graph using Cypher language
  • Progress indicators for long-running operations

Example workflow:
  1. Initialize configuration:  gograph init
  2. Analyze your project:      gograph analyze /path/to/project
  3. Query the results:         gograph query "MATCH (n) RETURN n LIMIT 10"
  4. Clear data when needed:    gograph clear

For more information, visit: https://github.com/compozy/gograph`,
}

var (
	initRootOnce sync.Once
	cfgFile      string
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Initialize configuration
	InitConfig()

	// Initialize all commands
	InitAnalyzeCommand()
	InitCallChainCommand()
	InitClearCommand()
	InitHelpCommands()
	InitInitCommand()
	InitQueryCommand()
	InitVersionCommand()
	RegisterLLMCommands()
	RegisterTemplatesCommand()
	RegisterMCPCommand()

	// Set help template for better formatting
	rootCmd.SetHelpTemplate(`{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasAvailableSubCommands}}{{.UsageString}}{{end}}`)

	// Configure help command
	rootCmd.SetUsageTemplate(`Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`)

	cobra.CheckErr(rootCmd.Execute())
}

// InitConfig initializes the configuration
func InitConfig() {
	initRootOnce.Do(func() {
		// Add global config flag
		rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./gograph.yaml)")
		cobra.OnInitialize(initConfigFile)
	})
}

func initConfigFile() {
	// Set environment variable prefix and enable automatic environment variable reading
	viper.SetEnvPrefix("GOGRAPH")
	viper.AutomaticEnv()
	// Replace dots with underscores for environment variables
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in current directory
		viper.SetConfigName("gograph")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
	}

	viper.SetDefault("parser.ignore_dirs", []string{".git", "vendor"})
	viper.SetDefault("parser.ignore_files", []string{})
	viper.SetDefault("parser.include_tests", false)
	// Note: Neo4j defaults are NOT set here to allow environment variables to take precedence

	if err := viper.ReadInConfig(); err != nil {
		// Only report errors that are not "file not found" errors
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// For explicit config files that don't exist, only warn (don't exit)
			if cfgFile != "" {
				fmt.Fprintf(os.Stderr, "Warning: Could not read config file %s: %s\n", cfgFile, err)
			} else {
				fmt.Fprintf(os.Stderr, "Error reading config file: %s\n", err)
				os.Exit(1)
			}
		}
	}
}
