package commands

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/infra"
	"github.com/compozy/gograph/engine/parser"
	"github.com/compozy/gograph/pkg/config"
	"github.com/compozy/gograph/pkg/errors"
	"github.com/compozy/gograph/pkg/logger"
	"github.com/compozy/gograph/pkg/progress"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [path]",
	Short: "Analyze Go source code and store the graph in Neo4j",
	Long: `Analyze performs a comprehensive analysis of a Go project and stores the
results as a graph in Neo4j. This includes parsing all Go files, analyzing
their structure, detecting relationships, and building a complete graph
representation of your codebase.

The analysis process includes:
  • Parsing all Go source files
  • Extracting packages, files, functions, structs, and interfaces
  • Detecting interface implementations
  • Mapping import dependencies
  • Tracing function call chains
  • Identifying circular dependencies
  • Calculating code metrics (optional)

The resulting graph allows you to:
  • Visualize your project structure
  • Find dependencies between components
  • Identify architectural patterns
  • Detect code smells and circular dependencies
  • Generate insights about code complexity`,
	Example: `  # Analyze the current directory
  gograph analyze .
  
  # Analyze a specific project
  gograph analyze /path/to/my/project
  
  # Analyze without progress indicators (for CI/scripts)
  gograph analyze /path/to/project --no-progress
  
  # Analyze with custom config file
  gograph analyze /path/to/project -c custom-config.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath := args[0]

		// Wrap the entire command execution with panic recovery
		return errors.WithRecover("analyze_command", func() error {
			// Load configuration from project path
			cfg, err := config.LoadProjectConfig(projectPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Get project ID from config or override flag
			projectID := core.ID(cfg.Project.ID)
			if projectIDFlag, err := cmd.Flags().GetString("project-id"); err == nil && projectIDFlag != "" {
				projectID = core.ID(projectIDFlag)
			}

			// Check for --no-progress flag
			noProgress, err := cmd.Flags().GetBool("no-progress")
			if err != nil {
				return fmt.Errorf("failed to get no-progress flag: %w", err)
			}

			// Initialize parser configuration from config
			parserConfig := &parser.Config{
				IgnoreDirs:      cfg.Analysis.IgnoreDirs,
				IgnoreFiles:     cfg.Analysis.IgnoreFiles,
				IncludeTests:    cfg.Analysis.IncludeTests,
				IncludeVendor:   cfg.Analysis.IncludeVendor,
				EnableSSA:       true,
				EnableCallGraph: true,
			}

			// Initialize analyzer configuration with defaults
			analyzerConfig := analyzer.DefaultAnalyzerConfig()

			// Initialize Neo4j configuration from config with fallback to defaults
			neo4jURI := cfg.Neo4j.URI
			if neo4jURI == "" {
				neo4jURI = DefaultNeo4jURI
			}
			neo4jUsername := cfg.Neo4j.Username
			if neo4jUsername == "" {
				neo4jUsername = DefaultNeo4jUsername
			}
			neo4jPassword := cfg.Neo4j.Password
			if neo4jPassword == "" {
				neo4jPassword = DefaultNeo4jPassword
			}

			neo4jConfig := &infra.Neo4jConfig{
				URI:        neo4jURI,
				Username:   neo4jUsername,
				Password:   neo4jPassword,
				Database:   cfg.Neo4j.Database,
				MaxRetries: 3,
				BatchSize:  1000,
			}

			// Start the analysis
			if noProgress {
				return runAnalysisWithoutProgress(projectPath, projectID, parserConfig, analyzerConfig, neo4jConfig)
			}

			// Check if we're in TTY mode and suppress logging if so
			isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
			if isTTY {
				// Suppress all logging output to avoid conflicts with TUI
				logger.Disable()
				defer logger.Enable() // Re-enable after completion
			}

			return runAnalysisWithProgress(projectPath, projectID, parserConfig, analyzerConfig, neo4jConfig)
		})
	},
}

func runAnalysisWithoutProgress(
	projectPath string,
	projectID core.ID,
	parserConfig *parser.Config,
	analyzerConfig *analyzer.Config,
	neo4jConfig *infra.Neo4jConfig,
) error {
	ctx := context.Background()
	startTime := time.Now()

	// -----
	// Parsing Phase
	// -----
	logger.Info("parsing project", "path", projectPath)
	parserService := parser.NewService(parserConfig)
	parseResult, err := parserService.ParseProject(ctx, projectPath, parserConfig)
	if err != nil {
		return fmt.Errorf("failed to parse project: %w", err)
	}

	// Count total files from packages
	totalFiles := 0
	for _, pkg := range parseResult.Packages {
		totalFiles += len(pkg.Files)
	}

	logger.Info("parsing completed",
		"packages", len(parseResult.Packages),
		"files", totalFiles,
		"duration_ms", parseResult.ParseTime)

	// -----
	// Analysis Phase
	// -----
	logger.Info("analyzing project structure")
	analyzerService := analyzer.NewAnalyzer(analyzerConfig)
	analysisInput := &analyzer.AnalysisInput{
		ProjectID:   projectID.String(),
		ParseResult: parseResult,
	}
	report, err := analyzerService.AnalyzeProject(ctx, analysisInput)
	if err != nil {
		return fmt.Errorf("failed to analyze project: %w", err)
	}
	logger.Info("analysis completed",
		"interfaces", len(report.InterfaceImplementations),
		"call_chains", len(report.CallChains),
		"dependencies", len(report.DependencyGraph.Edges))

	// -----
	// Graph Building Phase
	// -----
	logger.Info("building graph structure")
	builder := graph.NewBuilder(nil) // Use default config
	graphResult, err := builder.BuildFromAnalysis(ctx, projectID, parseResult, report)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}
	logger.Info("graph built",
		"nodes", len(graphResult.Nodes),
		"relationships", len(graphResult.Relationships))

	// -----
	// Storage Phase
	// -----
	logger.Info("connecting to Neo4j", "uri", neo4jConfig.URI)
	repo, err := infra.NewNeo4jRepository(neo4jConfig)
	if err != nil {
		return fmt.Errorf("failed to create Neo4j repository: %w", err)
	}
	defer repo.Close()

	logger.Info("storing analysis results")
	if err := repo.StoreAnalysis(ctx, graphResult); err != nil {
		return fmt.Errorf("failed to store analysis: %w", err)
	}

	duration := time.Since(startTime)
	logger.Info("✓ analysis completed successfully",
		"duration", duration.Round(time.Millisecond),
		"project_id", projectID)

	return nil
}

func runAnalysisWithProgress(
	projectPath string,
	projectID core.ID,
	parserConfig *parser.Config,
	analyzerConfig *analyzer.Config,
	neo4jConfig *infra.Neo4jConfig,
) error {
	ctx := context.Background()

	// Initialize adaptive progress
	progressIndicator := progress.NewAdaptiveProgress(os.Stdout)
	phases := []progress.PhaseInfo{
		{Name: "Parsing", Description: "Scanning and parsing Go source files", Weight: 0.3},
		{Name: "Analysis", Description: "Analyzing code structure and relationships", Weight: 0.3},
		{Name: "Graph Building", Description: "Building graph representation", Weight: 0.2},
		{Name: "Storage", Description: "Storing results in Neo4j", Weight: 0.2},
	}
	progressIndicator.SetPhases(phases)
	progressIndicator.Start(fmt.Sprintf("Analyzing project: %s", projectPath))

	// Parse project
	parseResult, err := runParsingPhase(ctx, projectPath, parserConfig, progressIndicator)
	if err != nil {
		return err
	}

	// Analyze project
	report, err := runAnalysisPhase(ctx, projectID, parseResult, analyzerConfig, progressIndicator)
	if err != nil {
		return err
	}

	// Build graph
	graphResult, err := runGraphBuildingPhase(ctx, projectID, parseResult, report, progressIndicator)
	if err != nil {
		return err
	}

	// Store results
	err = runStoragePhase(ctx, graphResult, neo4jConfig, progressIndicator)
	if err != nil {
		return err
	}

	// Success with detailed statistics
	successMsg := "Analysis completed successfully!"

	// Count total files from packages
	totalFiles := 0
	for _, pkg := range parseResult.Packages {
		totalFiles += len(pkg.Files)
	}

	// Create detailed statistics
	stats := progress.AnalysisStats{
		Files:         totalFiles,
		Nodes:         len(graphResult.Nodes),
		Relationships: len(graphResult.Relationships),
		Interfaces:    len(report.InterfaceImplementations),
		CallChains:    len(report.CallChains),
		ProjectID:     projectID.String(),
	}

	progressIndicator.SuccessWithStats(successMsg, stats)

	return nil
}

var initAnalyzeOnce sync.Once

func runParsingPhase(
	ctx context.Context,
	projectPath string,
	parserConfig *parser.Config,
	progressIndicator *progress.AdaptiveProgress,
) (*parser.ParseResult, error) {
	progressIndicator.UpdatePhase("Parsing")
	progressIndicator.UpdateProgress(0.0, "Initializing parser")

	parserService := parser.NewService(parserConfig)
	progressIndicator.UpdateProgress(0.1, "Scanning project files")

	parseResult, err := parserService.ParseProject(ctx, projectPath, parserConfig)
	if err != nil {
		progressIndicator.Error(fmt.Errorf("failed to parse project: %w", err))
		return nil, fmt.Errorf("failed to parse project: %w", err)
	}

	// Count total files from packages
	totalFiles := 0
	for _, pkg := range parseResult.Packages {
		totalFiles += len(pkg.Files)
	}

	progressIndicator.UpdateProgress(
		0.25,
		fmt.Sprintf("Parsed %d files in %d packages", totalFiles, len(parseResult.Packages)),
	)
	return parseResult, nil
}

func runAnalysisPhase(
	ctx context.Context,
	projectID core.ID,
	parseResult *parser.ParseResult,
	analyzerConfig *analyzer.Config,
	progressIndicator *progress.AdaptiveProgress,
) (*analyzer.AnalysisReport, error) {
	progressIndicator.UpdatePhase("Analysis")
	progressIndicator.UpdateProgress(0.3, "Initializing analyzer")

	analyzerService := analyzer.NewAnalyzer(analyzerConfig)
	analysisInput := &analyzer.AnalysisInput{
		ProjectID:   projectID.String(),
		ParseResult: parseResult,
	}
	progressIndicator.UpdateProgress(0.4, "Analyzing structure and dependencies")

	report, err := analyzerService.AnalyzeProject(ctx, analysisInput)
	if err != nil {
		progressIndicator.Error(fmt.Errorf("failed to analyze project: %w", err))
		return nil, fmt.Errorf("failed to analyze project: %w", err)
	}
	progressIndicator.UpdateProgress(0.55, fmt.Sprintf("Found %d interfaces, %d call chains",
		len(report.InterfaceImplementations), len(report.CallChains)))
	return report, nil
}

func runGraphBuildingPhase(
	ctx context.Context,
	projectID core.ID,
	parseResult *parser.ParseResult,
	report *analyzer.AnalysisReport,
	progressIndicator *progress.AdaptiveProgress,
) (*core.AnalysisResult, error) {
	progressIndicator.UpdatePhase("Graph Building")
	progressIndicator.UpdateProgress(0.6, "Building graph nodes and relationships")

	builder := graph.NewBuilder(nil) // Use default config
	graphResult, err := builder.BuildFromAnalysis(ctx, projectID, parseResult, report)
	if err != nil {
		progressIndicator.Error(fmt.Errorf("failed to build graph: %w", err))
		return nil, fmt.Errorf("failed to build graph: %w", err)
	}
	progressIndicator.UpdateProgress(0.75, fmt.Sprintf("Built %d nodes, %d relationships",
		len(graphResult.Nodes), len(graphResult.Relationships)))
	return graphResult, nil
}

func runStoragePhase(
	ctx context.Context,
	graphResult *core.AnalysisResult,
	neo4jConfig *infra.Neo4jConfig,
	progressIndicator *progress.AdaptiveProgress,
) error {
	progressIndicator.UpdatePhase("Storage")
	progressIndicator.UpdateProgress(0.8, "Connecting to Neo4j database")

	repo, err := infra.NewNeo4jRepository(neo4jConfig)
	if err != nil {
		progressIndicator.Error(fmt.Errorf("failed to create Neo4j repository: %w", err))
		return fmt.Errorf("failed to create Neo4j repository: %w", err)
	}
	defer repo.Close()

	progressIndicator.UpdateProgress(0.9, "Storing nodes and relationships")
	err = repo.StoreAnalysis(ctx, graphResult)
	if err != nil {
		progressIndicator.Error(fmt.Errorf("failed to store analysis: %w", err))
		return fmt.Errorf("failed to store analysis: %w", err)
	}
	return nil
}

// InitAnalyzeCommand registers the analyze command
func InitAnalyzeCommand() {
	initAnalyzeOnce.Do(func() {
		rootCmd.AddCommand(analyzeCmd)

		// Add flags
		analyzeCmd.Flags().Bool("no-progress", false, "Disable progress indicators")
		analyzeCmd.Flags().String("project-id", "", "Override project ID from config file")
	})
}
