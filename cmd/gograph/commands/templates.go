package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/compozy/gograph/engine/infra"
	"github.com/compozy/gograph/engine/query"
	"github.com/compozy/gograph/pkg/config"
	"github.com/compozy/gograph/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// templatesCmd represents the templates command
var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Manage and execute query templates",
	Long: `Query templates provide pre-built Cypher queries for common analysis tasks.

Available subcommands:
  list     - List all available templates
  show     - Show details of a specific template
  execute  - Execute a template with parameters
  export   - Execute a template and export results

Templates are organized by category:
  - overview: Project overview and statistics
  - functions: Function analysis
  - dependencies: Dependency analysis
  - types: Interfaces and structs
  - calls: Function call analysis
  - search: Search and find operations`,
}

// listTemplatesCmd lists all available templates
var listTemplatesCmd = &cobra.Command{
	Use:   "list [category]",
	Short: "List available query templates",
	Long: `List all available query templates, optionally filtered by category.

Categories:
  - overview: Project overview and statistics
  - functions: Function analysis queries
  - dependencies: Dependency relationship queries
  - types: Interface and struct queries
  - calls: Function call chain queries
  - search: Search and discovery queries`,
	Args: cobra.MaximumNArgs(1),
	RunE: runListTemplates,
	Example: `  # List all templates
  gograph templates list

  # List templates in a specific category
  gograph templates list functions

  # List with detailed output
  gograph templates list --detailed`,
}

// showTemplateCmd shows details of a specific template
var showTemplateCmd = &cobra.Command{
	Use:   "show <template-name>",
	Short: "Show details of a specific template",
	Long: `Show detailed information about a query template including:
  - Description and purpose
  - Required parameters
  - Example Cypher query
  - Usage examples`,
	Args: cobra.ExactArgs(1),
	RunE: runShowTemplate,
	Example: `  # Show template details
  gograph templates show project_overview

  # Show template with example parameters
  gograph templates show find_function --example`,
}

// executeTemplateCmd executes a template
var executeTemplateCmd = &cobra.Command{
	Use:   "execute <template-name>",
	Short: "Execute a query template",
	Long: `Execute a query template with the specified parameters.

Parameters can be provided as command-line flags or via a JSON file.
The project ID is automatically added based on the current configuration.`,
	Args: cobra.ExactArgs(1),
	RunE: runExecuteTemplate,
	Example: `  # Execute with inline parameters
  gograph templates execute find_function --param function_name=main

  # Execute with JSON parameters file
  gograph templates execute project_overview --params-file params.json

  # Execute with multiple parameters
  gograph templates execute find_function --param function_name=user --param project_id=myproject`,
}

// exportTemplateCmd executes and exports template results
var exportTemplateCmd = &cobra.Command{
	Use:   "export <template-name>",
	Short: "Execute template and export results",
	Long: `Execute a query template and export the results in various formats.

Supported formats:
  - json: JSON format (default)
  - csv: Comma-separated values
  - tsv: Tab-separated values`,
	Args: cobra.ExactArgs(1),
	RunE: runExportTemplate,
	Example: `  # Export to JSON
  gograph templates export project_overview --output results.json

  # Export to CSV with headers
  gograph templates export functions_by_package --format csv --output functions.csv

  # Export to stdout
  gograph templates export most_called_functions --format json`,
}

// RegisterTemplatesCommand registers the templates command with the root command
func RegisterTemplatesCommand() {
	rootCmd.AddCommand(templatesCmd)

	// Add subcommands
	templatesCmd.AddCommand(listTemplatesCmd)
	templatesCmd.AddCommand(showTemplateCmd)
	templatesCmd.AddCommand(executeTemplateCmd)
	templatesCmd.AddCommand(exportTemplateCmd)

	// List command flags
	listTemplatesCmd.Flags().Bool("detailed", false, "Show detailed template information")

	// Show command flags
	showTemplateCmd.Flags().Bool("example", false, "Show example parameter values")

	// Execute command flags
	executeTemplateCmd.Flags().StringSlice("param", []string{}, "Template parameters in key=value format")
	executeTemplateCmd.Flags().String("params-file", "", "JSON file containing template parameters")
	executeTemplateCmd.Flags().String("project", "", "Project ID (defaults to config)")

	// Export command flags
	exportTemplateCmd.Flags().StringSlice("param", []string{}, "Template parameters in key=value format")
	exportTemplateCmd.Flags().String("params-file", "", "JSON file containing template parameters")
	exportTemplateCmd.Flags().String("project", "", "Project ID (defaults to config)")
	exportTemplateCmd.Flags().String("format", "json", "Export format: json, csv, tsv")
	exportTemplateCmd.Flags().String("output", "", "Output file (defaults to stdout)")
	exportTemplateCmd.Flags().Bool("pretty", true, "Pretty print JSON output")
	exportTemplateCmd.Flags().Bool("headers", true, "Include headers in CSV/TSV output")
	exportTemplateCmd.Flags().String("delimiter", "", "Custom delimiter for CSV/TSV")
}

func runListTemplates(_ *cobra.Command, args []string) error {
	var category string
	if len(args) > 0 {
		category = args[0]
	}

	detailed := viper.GetBool("detailed")

	if category != "" {
		templates := query.GetTemplatesByCategory(category)
		if len(templates) == 0 {
			return fmt.Errorf("no templates found for category: %s", category)
		}
		displayTemplates(templates, detailed)
	} else {
		categories := query.ListTemplates()
		for cat, templates := range categories {
			caser := cases.Title(language.English)
			fmt.Printf("\n📁 %s:\n", caser.String(cat))
			displayTemplates(templates, detailed)
		}
	}

	return nil
}

func runShowTemplate(_ *cobra.Command, args []string) error {
	templateName := args[0]
	showExample := viper.GetBool("example")

	template, err := query.GetTemplate(templateName)
	if err != nil {
		return err
	}

	fmt.Printf("📋 Template: %s\n", template.Name)
	fmt.Printf("📂 Category: %s\n", template.Category)
	fmt.Printf("📝 Description: %s\n\n", template.Description)

	fmt.Println("🔧 Parameters:")
	if len(template.Parameters) == 0 {
		fmt.Println("  No parameters required")
	} else {
		for name, desc := range template.Parameters {
			fmt.Printf("  %s: %s\n", name, desc)
		}
	}

	fmt.Printf("\n🔍 Query:\n%s\n", template.Query)

	if showExample {
		fmt.Println("\n💡 Example usage:")
		fmt.Printf("gograph templates execute %s", templateName)
		for param := range template.Parameters {
			if param == "project_id" {
				fmt.Printf(" --project myproject")
			} else {
				fmt.Printf(" --param %s=example_value", param)
			}
		}
		fmt.Println()
	}

	return nil
}

func runExecuteTemplate(_ *cobra.Command, args []string) error {
	templateName := args[0]

	template, err := query.GetTemplate(templateName)
	if err != nil {
		return err
	}

	params, err := getTemplateParameters()
	if err != nil {
		return err
	}

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	logger.SetDebug(viper.GetBool("debug"))
	ctx := context.Background()

	// Setup Neo4j repository
	neo4jConfig := &infra.Neo4jConfig{
		URI:        cfg.Neo4j.URI,
		Username:   cfg.Neo4j.Username,
		Password:   cfg.Neo4j.Password,
		Database:   cfg.Neo4j.Database,
		MaxRetries: 3,
		BatchSize:  1000,
	}

	repo, err := infra.NewNeo4jRepository(neo4jConfig)
	if err != nil {
		return fmt.Errorf("failed to create Neo4j repository: %w", err)
	}
	defer repo.Close()

	// Execute query
	results, err := repo.ExecuteQuery(ctx, template.Query, params)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	// Display results
	if len(results) == 0 {
		fmt.Println("No results found")
		return nil
	}

	// Format and display results as JSON
	output, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format results: %w", err)
	}

	fmt.Printf("Results (%d rows):\n%s\n", len(results), string(output))
	return nil
}

func runExportTemplate(_ *cobra.Command, args []string) error {
	templateName := args[0]

	template, err := query.GetTemplate(templateName)
	if err != nil {
		return err
	}

	params, err := getTemplateParameters()
	if err != nil {
		return err
	}

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	logger.SetDebug(viper.GetBool("debug"))
	ctx := context.Background()

	// Setup Neo4j repository
	neo4jConfig := &infra.Neo4jConfig{
		URI:        cfg.Neo4j.URI,
		Username:   cfg.Neo4j.Username,
		Password:   cfg.Neo4j.Password,
		Database:   cfg.Neo4j.Database,
		MaxRetries: 3,
		BatchSize:  1000,
	}

	repo, err := infra.NewNeo4jRepository(neo4jConfig)
	if err != nil {
		return fmt.Errorf("failed to create Neo4j repository: %w", err)
	}
	defer repo.Close()

	// Execute query
	results, err := repo.ExecuteQuery(ctx, template.Query, params)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	// Setup export options
	format := query.ExportFormat(viper.GetString("format"))
	options := query.DefaultExportOptions(format)
	options.Pretty = viper.GetBool("pretty")
	options.Headers = viper.GetBool("headers")
	if delimiter := viper.GetString("delimiter"); delimiter != "" {
		options.Delimiter = delimiter
	}

	exporter := query.NewExporter(options)

	// Determine output destination
	outputFile := viper.GetString("output")
	var writer *os.File
	if outputFile != "" {
		writer, err = os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer writer.Close()
	} else {
		writer = os.Stdout
	}

	// Export results
	metadata, err := exporter.ExportWithMetadata(writer, results)
	if err != nil {
		return fmt.Errorf("failed to export results: %w", err)
	}

	// Print metadata to stderr (so it doesn't interfere with stdout output)
	if outputFile != "" {
		fmt.Fprintf(os.Stderr, "✅ Exported %d rows, %d columns to %s (%d bytes)\n",
			metadata.RowCount, metadata.ColumnCount, outputFile, metadata.Size)
	}

	return nil
}

func getTemplateParameters() (map[string]any, error) {
	params := make(map[string]any)

	// Add project ID from config or flag
	projectID := viper.GetString("project")
	if projectID == "" {
		// Try to get from config
		cfg, err := config.Load("")
		if err == nil && cfg.Project.Name != "" {
			projectID = cfg.Project.Name
		}
	}
	if projectID != "" {
		params["project_id"] = projectID
	}

	// Load from parameters file if specified
	paramsFile := viper.GetString("params-file")
	if paramsFile != "" {
		data, err := os.ReadFile(paramsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read parameters file: %w", err)
		}

		var fileParams map[string]any
		if err := json.Unmarshal(data, &fileParams); err != nil {
			return nil, fmt.Errorf("failed to parse parameters file: %w", err)
		}

		for key, value := range fileParams {
			params[key] = value
		}
	}

	// Add command-line parameters
	paramFlags := viper.GetStringSlice("param")
	for _, param := range paramFlags {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid parameter format: %s (expected key=value)", param)
		}
		params[parts[0]] = parts[1]
	}

	return params, nil
}

func displayTemplates(templates []*query.Template, detailed bool) {
	// Sort templates by name
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})

	for _, template := range templates {
		if detailed {
			fmt.Printf("  📋 %s\n", template.Name)
			fmt.Printf("     %s\n", template.Description)
			if len(template.Parameters) > 0 {
				fmt.Printf("     Parameters: %s\n", strings.Join(getParameterNames(template.Parameters), ", "))
			}
			fmt.Println()
		} else {
			fmt.Printf("  📋 %-25s %s\n", template.Name, template.Description)
		}
	}
}

func getParameterNames(parameters map[string]string) []string {
	names := make([]string, 0, len(parameters))
	for name := range parameters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
