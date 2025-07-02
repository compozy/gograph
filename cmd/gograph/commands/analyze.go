package commands

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/infra"
	"github.com/compozy/gograph/engine/parser"
	"github.com/compozy/gograph/pkg/errors"
	"github.com/compozy/gograph/pkg/logger"
	"github.com/compozy/gograph/pkg/progress"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
			// Check for --no-progress flag
			noProgress, err := cmd.Flags().GetBool("no-progress")
			if err != nil {
				return fmt.Errorf("failed to get no-progress flag: %w", err)
			}

			// Initialize parser configuration from Viper
			parserConfig := &parser.Config{
				IgnoreDirs:     viper.GetStringSlice("parser.ignore_dirs"),
				IgnoreFiles:    viper.GetStringSlice("parser.ignore_files"),
				IncludeTests:   viper.GetBool("parser.include_tests"),
				IncludeVendor:  viper.GetBool("parser.include_vendor"),
				MaxConcurrency: viper.GetInt("parser.max_concurrency"),
			}

			// Initialize analyzer configuration from Viper
			analyzerConfig := analyzer.DefaultAnalyzerConfig()
			if maxDepth := viper.GetInt("analyzer.max_dependency_depth"); maxDepth > 0 {
				analyzerConfig.MaxDependencyDepth = maxDepth
			}
			if ignoreTest := viper.GetBool("analyzer.ignore_test_files"); ignoreTest {
				analyzerConfig.IgnoreTestFiles = ignoreTest
			}
			if ignoreVendor := viper.GetBool("analyzer.ignore_vendor"); ignoreVendor {
				analyzerConfig.IgnoreVendor = ignoreVendor
			}
			if includeMetrics := viper.GetBool("analyzer.include_metrics"); includeMetrics {
				analyzerConfig.IncludeMetrics = includeMetrics
			}
			if workers := viper.GetInt("analyzer.parallel_workers"); workers > 0 {
				analyzerConfig.ParallelWorkers = workers
			}

			// Initialize Neo4j configuration from Viper with fallback to defaults
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

			// Start the analysis
			if noProgress {
				return runAnalysisWithoutProgress(projectPath, parserConfig, analyzerConfig, neo4jConfig)
			}

			return runAnalysisWithProgress(projectPath, parserConfig, analyzerConfig, neo4jConfig)
		})
	},
}

func runAnalysisWithoutProgress(
	projectPath string,
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
	logger.Info("parsing completed",
		"files", len(parseResult.Files),
		"duration_ms", parseResult.ParseTime)

	// -----
	// Analysis Phase
	// -----
	logger.Info("analyzing project structure")
	analyzerService := analyzer.NewAnalyzer(analyzerConfig)
	projectID := core.NewID()
	analysisInput := &analyzer.AnalysisInput{
		ProjectID: projectID.String(),
		Files:     parseResult.Files,
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
	parserConfig *parser.Config,
	analyzerConfig *analyzer.Config,
	neo4jConfig *infra.Neo4jConfig,
) error {
	ctx := context.Background()
	startTime := time.Now()

	var parseResult *parser.ParseResult
	var report *analyzer.AnalysisReport
	var graphResult *core.AnalysisResult
	var projectID core.ID

	// -----
	// Parsing Phase
	// -----
	err := progress.WithProgress("Parsing project files", func() error {
		parserService := parser.NewService(parserConfig)
		var err error
		parseResult, err = parserService.ParseProject(ctx, projectPath, parserConfig)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to parse project: %w", err)
	}

	// -----
	// Analysis Phase
	// -----
	err = progress.WithProgress("Analyzing code structure", func() error {
		analyzerService := analyzer.NewAnalyzer(analyzerConfig)
		projectID = core.NewID()
		analysisInput := &analyzer.AnalysisInput{
			ProjectID: projectID.String(),
			Files:     parseResult.Files,
		}
		var err error
		report, err = analyzerService.AnalyzeProject(ctx, analysisInput)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to analyze project: %w", err)
	}

	// -----
	// Graph Building Phase
	// -----
	err = progress.WithProgress("Building graph representation", func() error {
		builder := graph.NewBuilder(nil) // Use default config
		var err error
		graphResult, err = builder.BuildFromAnalysis(ctx, projectID, parseResult, report)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// -----
	// Storage Phase
	// -----
	err = progress.WithProgressSteps("Storing in Neo4j", func(update func(string), _ func(int, int)) error {
		update("Connecting to database")
		repo, err := infra.NewNeo4jRepository(neo4jConfig)
		if err != nil {
			return fmt.Errorf("failed to create Neo4j repository: %w", err)
		}
		defer repo.Close()

		update("Storing nodes and relationships")
		return repo.StoreAnalysis(ctx, graphResult)
	})
	if err != nil {
		return fmt.Errorf("failed to store analysis: %w", err)
	}

	duration := time.Since(startTime)

	// Final summary
	logger.Info("✓ Analysis completed successfully")
	logger.Info("Summary:",
		"files", len(parseResult.Files),
		"nodes", len(graphResult.Nodes),
		"relationships", len(graphResult.Relationships),
		"duration", duration.Round(time.Millisecond),
		"project_id", projectID)

	return nil
}

var initAnalyzeOnce sync.Once

// InitAnalyzeCommand registers the analyze command
func InitAnalyzeCommand() {
	initAnalyzeOnce.Do(func() {
		rootCmd.AddCommand(analyzeCmd)

		// Add progress flag
		analyzeCmd.Flags().Bool("no-progress", false, "Disable progress indicators")
	})
}
