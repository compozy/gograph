package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/infra"
	"github.com/compozy/gograph/engine/llm"
	"github.com/compozy/gograph/engine/mcp"
	"github.com/compozy/gograph/engine/parser"
	"github.com/compozy/gograph/pkg/logger"
	mcpconfig "github.com/compozy/gograph/pkg/mcp"
	"github.com/joho/godotenv"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	mcpHost       string
	mcpPort       int
	mcpAuth       bool
	mcpHTTP       bool
	mcpConfigFile string
)

// serveMCPCmd represents the serve-mcp command
var serveMCPCmd = &cobra.Command{
	Use:   "serve-mcp",
	Short: "Start MCP server to expose GoGraph capabilities to LLM applications",
	Long: `Start the Model Context Protocol (MCP) server to expose GoGraph's code analysis
capabilities to LLM applications. This allows AI assistants to query and analyze
your Go codebase through standardized MCP tools.

The MCP server provides:
  • Code analysis tools for project understanding
  • Navigation tools for exploring code structure
  • Query tools for executing Cypher queries
  • Verification tools to prevent hallucinations
  • Pattern detection for design patterns
  • Test integration for coverage analysis

Examples:
  # Start MCP server with default settings (stdio transport)
  gograph serve-mcp

  # Start with HTTP transport on custom port
  gograph serve-mcp --http --port 8080

  # Enable authentication
  gograph serve-mcp --auth --http

  # Use custom configuration file
  gograph serve-mcp --config mcp-config.yaml`,
	RunE: runServeMCP,
}

// RegisterMCPCommand registers the MCP command with the root command
func RegisterMCPCommand() {
	// Setup flags
	serveMCPCmd.Flags().StringVar(&mcpHost, "host", "localhost", "Host to bind the server")
	serveMCPCmd.Flags().IntVar(&mcpPort, "port", 3333, "Port to bind the server (for HTTP transport)")
	serveMCPCmd.Flags().BoolVar(&mcpAuth, "auth", false, "Enable authentication")
	serveMCPCmd.Flags().BoolVar(&mcpHTTP, "http", false, "Use HTTP transport instead of stdio")
	serveMCPCmd.Flags().StringVar(&mcpConfigFile, "config", "", "Path to MCP configuration file")

	// Add to root command
	rootCmd.AddCommand(serveMCPCmd)
}

func runServeMCP(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load .env file if it exists
	if err := loadEnvFile(); err != nil {
		logger.Debug("Could not load .env file", "error", err)
	}

	config, err := prepareMCPConfiguration(cmd)
	if err != nil {
		return err
	}

	driver, err := initializeNeo4jConnection(ctx)
	if err != nil {
		return err
	}
	defer driver.Close(ctx)

	server := createMCPServer(config)

	runMCPServerWithGracefulShutdown(ctx, cancel, server)
	return nil
}

func prepareMCPConfiguration(cmd *cobra.Command) (*mcpconfig.Config, error) {
	config, err := loadMCPConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load MCP configuration: %w", err)
	}

	applyCommandLineFlagOverrides(cmd, config)
	return config, nil
}

func applyCommandLineFlagOverrides(cmd *cobra.Command, config *mcpconfig.Config) {
	if cmd.Flags().Changed("host") {
		config.Server.Host = mcpHost
	}
	if cmd.Flags().Changed("port") {
		config.Server.Port = mcpPort
	}
	if cmd.Flags().Changed("auth") {
		config.Auth.Enabled = mcpAuth
	}
}

func initializeNeo4jConnection(_ context.Context) (neo4j.DriverWithContext, error) {
	neo4jConfig := &infra.Neo4jConfig{
		URI:        viper.GetString("neo4j.uri"),
		Username:   viper.GetString("neo4j.username"),
		Password:   viper.GetString("neo4j.password"),
		Database:   viper.GetString("neo4j.database"),
		MaxRetries: 3,
		BatchSize:  1000,
	}

	driver, err := neo4j.NewDriverWithContext(
		neo4jConfig.URI,
		neo4j.BasicAuth(neo4jConfig.Username, neo4jConfig.Password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Neo4j: %w", err)
	}

	return driver, nil
}

func createMCPServer(config *mcpconfig.Config) *mcp.Server {
	logger.Info("Creating MCP server with full service configuration")

	// Initialize Neo4j repository
	neo4jConfig := &infra.Neo4jConfig{
		URI:        viper.GetString("neo4j.uri"),
		Username:   viper.GetString("neo4j.username"),
		Password:   viper.GetString("neo4j.password"),
		Database:   viper.GetString("neo4j.database"),
		MaxRetries: 3,
		BatchSize:  1000,
	}

	repository, err := infra.NewNeo4jRepository(neo4jConfig)
	if err != nil {
		logger.Error("Failed to create Neo4j repository", "error", err)
		// Return server with nil services as fallback, but log the error
		return mcp.NewServer(config, nil, nil, nil, nil)
	}

	// Create Neo4j driver for service adapter
	driver, err := neo4j.NewDriverWithContext(
		neo4jConfig.URI,
		neo4j.BasicAuth(neo4jConfig.Username, neo4jConfig.Password, ""),
	)
	if err != nil {
		logger.Error("Failed to create Neo4j driver", "error", err)
		return mcp.NewServer(config, nil, nil, nil, nil)
	}

	// Create core services
	parserService := parser.NewService(nil) // Uses default config internally
	analyzerService := analyzer.NewAnalyzer(analyzer.DefaultAnalyzerConfig())

	// Create graph builder and service
	graphBuilder := graph.NewBuilder(graph.DefaultBuilderConfig())
	graphService := graph.NewService(
		parserService,
		analyzerService,
		graphBuilder,
		repository,
		graph.DefaultServiceConfig(),
	)

	// Create service adapter
	serviceAdapter := mcp.NewServiceAdapter(driver, graphService, parserService, analyzerService)

	// Initialize LLM service if OpenAI API key is available
	var llmService llm.CypherTranslator
	openAIKey := viper.GetString("openai.api_key")
	if openAIKey == "" {
		openAIKey = os.Getenv("OPENAI_API_KEY")
	}

	if openAIKey != "" {
		// Validate API key format
		if !strings.HasPrefix(openAIKey, "sk-") {
			logger.Warn("OpenAI API key does not have the expected 'sk-' prefix. It may be invalid.")
		}
		logger.Info("Initializing OpenAI LLM service for natural language queries")
		llmConfig := llm.CypherTranslatorConfig{
			APIKey: openAIKey,
			Model:  viper.GetString("openai.model"), // Will use default if not set
		}
		llmService = llm.NewOpenAICypherTranslator(llmConfig, graphService)
	} else {
		logger.Warn("OpenAI API key not found - natural language queries will use basic fallback")
	}

	// TODO: Initialize context generator and query builder when implemented
	return mcp.NewServer(config, serviceAdapter, llmService, nil, nil)
}

func runMCPServerWithGracefulShutdown(ctx context.Context, cancel context.CancelFunc, server *mcp.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go startMCPServer(ctx, cancel, server)

	select {
	case <-sigChan:
		logger.Info("Received shutdown signal")
	case <-ctx.Done():
		logger.Info("Context canceled")
	}

	logger.Info("Shutting down MCP server...")
	// TODO(2025-07-02): Implement graceful shutdown when mcp-go supports it
}

func startMCPServer(ctx context.Context, cancel context.CancelFunc, server *mcp.Server) {
	var serverErr error
	if mcpHTTP {
		logger.Info("Starting MCP server with HTTP transport", "host", server, "port", mcpPort)
		serverErr = server.Start(ctx) // Use stdio for now, HTTP TODO(2025-07-02)
	} else {
		logger.Info("Starting MCP server with stdio transport")
		serverErr = server.Start(ctx)
	}
	if serverErr != nil {
		logger.Error("MCP server error", "error", serverErr)
		cancel()
	}
}

func loadMCPConfig() (*mcpconfig.Config, error) {
	if mcpConfigFile != "" {
		// Load from specified file
		viper.SetConfigFile(mcpConfigFile)
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		var config mcpconfig.Config
		if err := viper.UnmarshalKey("mcp", &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal MCP config: %w", err)
		}
		return &config, nil
	}

	// Use default config
	config := mcpconfig.DefaultConfig()

	// Check for MCP settings in main config
	if viper.IsSet("mcp.server.host") {
		config.Server.Host = viper.GetString("mcp.server.host")
	}
	if viper.IsSet("mcp.server.port") {
		config.Server.Port = viper.GetInt("mcp.server.port")
	}
	if viper.IsSet("mcp.auth.enabled") {
		config.Auth.Enabled = viper.GetBool("mcp.auth.enabled")
	}
	if viper.IsSet("mcp.auth.token") {
		config.Auth.Token = viper.GetString("mcp.auth.token")
	}

	return config, nil
}

// loadEnvFile loads the .env file from the current directory
func loadEnvFile() error {
	err := godotenv.Load()
	if err != nil {
		// It's acceptable if the file doesn't exist
		return fmt.Errorf(".env file not found or failed to load: %w", err)
	}
	logger.Info("Loaded .env file from current directory")
	return nil
}
