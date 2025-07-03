package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/infra"
	"github.com/compozy/gograph/engine/llm"
	"github.com/compozy/gograph/pkg/config"
	"github.com/compozy/gograph/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// llmCmd represents the llm command
var llmCmd = &cobra.Command{
	Use:   "llm",
	Short: "LLM integration commands",
	Long: `Commands for integrating gograph with Large Language Models.

Provides tools for:
- Translating natural language to Cypher queries
- Generating LLM context from graph data
- Analyzing project structure for AI consumption`,
}

// translateCmd translates natural language to Cypher
var translateCmd = &cobra.Command{
	Use:   "translate [natural language query]",
	Short: "Translate natural language to Cypher query",
	Long: `Translate natural language questions to Cypher queries using OpenAI.

The command takes a natural language question and converts it to a Cypher query
that can be executed against the Neo4j graph database.`,
	Args: cobra.ExactArgs(1),
	Example: `  # Translate a natural language query
  gograph llm translate "Find all functions in the main package"
  
  # Translate with specific project
  gograph llm translate "Show dependencies for user service" --project myproject`,
	RunE: runTranslate,
}

// contextCmd generates LLM context
var contextCmd = &cobra.Command{
	Use:   "context [project-id]",
	Short: "Generate LLM context from graph data",
	Long: `Generate structured context information from the graph database for LLM consumption.

This command extracts relevant nodes, relationships, and code examples from the
project graph and formats them for use with Large Language Models.`,
	Args: cobra.ExactArgs(1),
	Example: `  # Generate context for a project
  gograph llm context myproject
  
  # Generate context with query focus
  gograph llm context myproject --query "authentication"`,
	RunE: runContext,
}

// RegisterLLMCommands registers LLM commands with the root command
func RegisterLLMCommands() {
	rootCmd.AddCommand(llmCmd)
	llmCmd.AddCommand(translateCmd)
	llmCmd.AddCommand(contextCmd)

	// Translate command flags
	translateCmd.Flags().String("project", "", "Project ID to get schema from")
	translateCmd.Flags().String("openai-api-key", "", "OpenAI API key (can also use OPENAI_API_KEY env var)")
	translateCmd.Flags().String("model", "gpt-4o", "OpenAI model to use")

	// Context command flags
	contextCmd.Flags().String("query", "", "Query or topic to focus context generation on")

	// Bind flags - handle errors properly
	if err := viper.BindPFlag("llm.openai_api_key", translateCmd.Flags().Lookup("openai-api-key")); err != nil {
		panic(fmt.Sprintf("failed to bind openai-api-key flag: %v", err))
	}
	if err := viper.BindPFlag("llm.model", translateCmd.Flags().Lookup("model")); err != nil {
		panic(fmt.Sprintf("failed to bind model flag: %v", err))
	}
}

func runTranslate(_ *cobra.Command, args []string) error {
	naturalQuery := args[0]
	projectID := viper.GetString("project")

	if projectID == "" {
		return fmt.Errorf("project ID is required (use --project flag)")
	}

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Setup logging
	logger.SetDebug(viper.GetBool("debug"))
	ctx := context.Background()

	// Initialize Neo4j repository
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

	// Create simplified graph service
	graphService := &llmGraphService{repository: repo}

	// Get OpenAI API key
	openaiAPIKey := viper.GetString("llm.openai_api_key")
	if openaiAPIKey == "" {
		openaiAPIKey = os.Getenv("OPENAI_API_KEY")
	}
	if openaiAPIKey == "" {
		return fmt.Errorf("OpenAI API key is required (use OPENAI_API_KEY env var or --openai-api-key flag)")
	}

	// Initialize translator
	translatorConfig := llm.CypherTranslatorConfig{
		APIKey: openaiAPIKey,
		Model:  viper.GetString("llm.model"),
	}
	translator := llm.NewOpenAICypherTranslator(translatorConfig, graphService)

	// Get schema
	schema, err := translator.GetSchema(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get schema: %w", err)
	}

	// Translate query
	result, err := translator.Translate(ctx, naturalQuery, schema)
	if err != nil {
		return fmt.Errorf("failed to translate query: %w", err)
	}

	// Output result
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

func runContext(_ *cobra.Command, args []string) error {
	projectID := args[0]
	query := viper.GetString("query")

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Setup logging
	logger.SetDebug(viper.GetBool("debug"))
	ctx := context.Background()

	// Initialize Neo4j repository
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

	// Create simplified graph service
	graphService := &llmGraphService{repository: repo}

	// Initialize context generator
	generator := llm.NewDefaultContextGenerator(graphService)

	// Generate context
	context, err := generator.GenerateContext(ctx, projectID, query)
	if err != nil {
		return fmt.Errorf("failed to generate context: %w", err)
	}

	// Output result
	output, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

// llmGraphService is a simplified graph service implementation for LLM commands
type llmGraphService struct {
	repository graph.Repository
}

// GetProjectStatistics implements the required method for LLM functionality
func (s *llmGraphService) GetProjectStatistics(
	ctx context.Context,
	projectID core.ID,
) (*graph.ProjectStatistics, error) {
	// Execute queries to get basic statistics
	nodeCountQuery := "MATCH (n) WHERE n.project_id = $project_id RETURN count(n) as node_count"
	nodeResults, err := s.repository.ExecuteQuery(ctx, nodeCountQuery, map[string]any{
		"project_id": string(projectID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get node count: %w", err)
	}

	nodeCount := 0
	if len(nodeResults) > 0 {
		if count, ok := nodeResults[0]["node_count"].(int64); ok {
			nodeCount = int(count)
		}
	}

	// Get relationship count
	relCountQuery := "MATCH ()-[r]->() WHERE r.project_id = $project_id RETURN count(r) as rel_count"
	relResults, err := s.repository.ExecuteQuery(ctx, relCountQuery, map[string]any{
		"project_id": string(projectID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get relationship count: %w", err)
	}

	relCount := 0
	if len(relResults) > 0 {
		if count, ok := relResults[0]["rel_count"].(int64); ok {
			relCount = int(count)
		}
	}

	// Return basic statistics
	return &graph.ProjectStatistics{
		TotalNodes:          nodeCount,
		TotalRelationships:  relCount,
		NodesByType:         make(map[core.NodeType]int),
		RelationshipsByType: make(map[core.RelationType]int),
		TopPackages:         []graph.PackageStats{},
		TopFunctions:        []graph.FunctionStats{},
	}, nil
}

// GetProjectGraph implements the required method for LLM functionality
func (s *llmGraphService) GetProjectGraph(ctx context.Context, projectID core.ID) (*graph.ProjectGraph, error) {
	// Get nodes for the project
	nodeQuery := "MATCH (n) WHERE n.project_id = $project_id RETURN n LIMIT 100"
	nodeResults, err := s.repository.ExecuteQuery(ctx, nodeQuery, map[string]any{
		"project_id": string(projectID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get project nodes: %w", err)
	}

	// Get relationships for the project
	relQuery := "MATCH ()-[r]->() WHERE r.project_id = $project_id RETURN r LIMIT 100"
	relResults, err := s.repository.ExecuteQuery(ctx, relQuery, map[string]any{
		"project_id": string(projectID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get project relationships: %w", err)
	}

	// Convert results to nodes and relationships
	nodes := make([]core.Node, 0, len(nodeResults))
	relationships := make([]core.Relationship, 0, len(relResults))

	// Note: In a real implementation, you'd properly convert the Neo4j results
	// to core.Node and core.Relationship structures. For now, return empty slices.

	return &graph.ProjectGraph{
		Nodes:         nodes,
		Relationships: relationships,
	}, nil
}

// Stub implementations for other Service interface methods
func (s *llmGraphService) InitializeProject(_ context.Context, _ *core.Project) error {
	return fmt.Errorf("not implemented in LLM service")
}

func (s *llmGraphService) ImportAnalysis(_ context.Context, _ core.ID, _ *core.AnalysisResult) error {
	return fmt.Errorf("not implemented in LLM service")
}

func (s *llmGraphService) GetNodeWithRelationships(
	_ context.Context,
	_ core.ID,
) (*graph.NodeWithRelations, error) {
	return nil, fmt.Errorf("not implemented in LLM service")
}

func (s *llmGraphService) FindPath(_ context.Context, _, _ core.ID) ([]graph.PathSegment, error) {
	return nil, fmt.Errorf("not implemented in LLM service")
}

func (s *llmGraphService) GetDependencyGraph(_ context.Context, _ string) (*graph.DependencyGraph, error) {
	return nil, fmt.Errorf("not implemented in LLM service")
}

func (s *llmGraphService) GetCallGraph(_ context.Context, _ string) (*graph.CallGraph, error) {
	return nil, fmt.Errorf("not implemented in LLM service")
}

func (s *llmGraphService) ClearProject(ctx context.Context, projectID core.ID) error {
	return s.repository.ClearProject(ctx, projectID)
}

func (s *llmGraphService) ExecuteQuery(
	ctx context.Context,
	query string,
	params map[string]any,
) ([]map[string]any, error) {
	return s.repository.ExecuteQuery(ctx, query, params)
}
