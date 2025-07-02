package integration

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/infra"
	"github.com/compozy/gograph/engine/mcp"
	"github.com/compozy/gograph/engine/parser"
	"github.com/compozy/gograph/engine/query"
	mcpconfig "github.com/compozy/gograph/pkg/mcp"
	"github.com/compozy/gograph/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPServerIntegration tests the MCP server with real services
func TestMCPServerIntegration(t *testing.T) {
	// Skip if Neo4j is not available
	if !testhelpers.IsNeo4jAvailable() {
		t.Skip("Neo4j not available, skipping MCP integration test")
	}

	projectRoot := getProjectRoot()
	ctx := context.Background()

	// Start Neo4j container
	container, err := testhelpers.StartNeo4jContainer(ctx)
	require.NoError(t, err)
	defer container.Stop(ctx)

	// Create repository
	repository, err := container.CreateRepository()
	require.NoError(t, err)
	defer repository.Close()

	// Create real services
	parserService := parser.NewService(nil)
	analyzerService := analyzer.NewAnalyzer(nil)
	graphBuilder := graph.NewBuilder(graph.DefaultBuilderConfig())
	graphService := graph.NewService(
		parserService,
		analyzerService,
		graphBuilder,
		repository,
		graph.DefaultServiceConfig(),
	)
	queryBuilder := query.NewHighLevelBuilder()

	// Create service adapter
	serviceAdapter := &realServiceAdapter{
		graphService: graphService,
		repository:   repository,
	}

	// Create MCP server with test-friendly config
	config := mcpconfig.DefaultConfig()
	// Allow the project root directory for testing
	config.Security.AllowedPaths = []string{projectRoot, "/tmp"}
	server := mcp.NewServer(config, serviceAdapter, nil, nil, queryBuilder)

	t.Run("Should analyze project via MCP tool", func(t *testing.T) {
		// Clear database
		err := container.ClearDatabase(ctx)
		require.NoError(t, err)

		// Test project path
		testProjectPath := filepath.Join(projectRoot, "testdata", "simple_project")
		projectID := "test-project-1"

		// Call analyze_project tool through internal handler
		response, err := server.HandleAnalyzeProjectInternal(ctx, map[string]any{
			"project_path": testProjectPath,
			"project_id":   projectID,
		})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.NotEmpty(t, response.Content)

		// Verify data was created in database - check for any nodes with our project_id
		query := "MATCH (n {project_id: $project_id}) RETURN count(n) as node_count"
		results, err := repository.ExecuteQuery(ctx, query, map[string]any{
			"project_id": projectID,
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Greater(t, results[0]["node_count"], int64(0))

		// Clean up
		err = repository.ClearProject(ctx, core.ID(projectID))
		require.NoError(t, err)
	})

	t.Run("Should execute Cypher queries via MCP tool", func(t *testing.T) {
		// Clear database
		err := container.ClearDatabase(ctx)
		require.NoError(t, err)

		// First analyze a project
		testProjectPath := filepath.Join(projectRoot, "testdata", "simple_project")
		projectID := "test-project-2"

		_, err = server.HandleAnalyzeProjectInternal(ctx, map[string]any{
			"project_path": testProjectPath,
			"project_id":   projectID,
		})
		require.NoError(t, err)

		// Execute Cypher query
		cypherQuery := "MATCH (f:Function {project_id: $project_id}) RETURN f.name as name LIMIT 5"
		response, err := server.HandleExecuteCypherInternal(ctx, map[string]any{
			"project_id": projectID,
			"query":      cypherQuery,
			"parameters": map[string]any{"project_id": projectID},
		})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.NotEmpty(t, response.Content)

		// Clean up
		err = repository.ClearProject(ctx, core.ID(projectID))
		require.NoError(t, err)
	})

	t.Run("Should get function info via MCP tool", func(t *testing.T) {
		// Clear database
		err := container.ClearDatabase(ctx)
		require.NoError(t, err)

		// Analyze project first
		testProjectPath := filepath.Join(projectRoot, "testdata", "simple_project")
		projectID := "test-project-3"

		_, err = server.HandleAnalyzeProjectInternal(ctx, map[string]any{
			"project_path": testProjectPath,
			"project_id":   projectID,
		})
		require.NoError(t, err)

		// Get function info for main function
		response, err := server.HandleGetFunctionInfoInternal(ctx, map[string]any{
			"project_id":      projectID,
			"function_name":   "main",
			"include_calls":   true,
			"include_callers": false,
		})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.NotEmpty(t, response.Content)

		// Clean up
		err = repository.ClearProject(ctx, core.ID(projectID))
		require.NoError(t, err)
	})

	t.Run("Should verify code exists via MCP tool", func(t *testing.T) {
		// Clear database
		err := container.ClearDatabase(ctx)
		require.NoError(t, err)

		// Analyze project first
		testProjectPath := filepath.Join(projectRoot, "testdata", "simple_project")
		projectID := "test-project-4"

		_, err = server.HandleAnalyzeProjectInternal(ctx, map[string]any{
			"project_path": testProjectPath,
			"project_id":   projectID,
		})
		require.NoError(t, err)

		// Verify main function exists
		response, err := server.HandleVerifyCodeExistsInternal(ctx, map[string]any{
			"project_id":   projectID,
			"element_type": "function",
			"name":         "main",
		})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.NotEmpty(t, response.Content)

		// Verify non-existent function
		response, err = server.HandleVerifyCodeExistsInternal(ctx, map[string]any{
			"project_id":   projectID,
			"element_type": "function",
			"name":         "nonExistentFunction",
		})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.NotEmpty(t, response.Content)

		// Clean up
		err = repository.ClearProject(ctx, core.ID(projectID))
		require.NoError(t, err)
	})

	t.Run("Should handle multiple concurrent MCP operations", func(t *testing.T) {
		// Clear database
		err := container.ClearDatabase(ctx)
		require.NoError(t, err)

		// Analyze project first
		testProjectPath := filepath.Join(projectRoot, "testdata", "simple_project")
		projectID := "test-project-concurrent"

		_, err = server.HandleAnalyzeProjectInternal(ctx, map[string]any{
			"project_path": testProjectPath,
			"project_id":   projectID,
		})
		require.NoError(t, err)

		// Run multiple operations concurrently
		numOperations := 5
		results := make(chan error, numOperations)

		for i := 0; i < numOperations; i++ {
			go func() {
				_, err := server.HandleGetFunctionInfoInternal(ctx, map[string]any{
					"project_id":    projectID,
					"function_name": "main",
				})
				results <- err
			}()
		}

		// Check all operations completed successfully
		for i := 0; i < numOperations; i++ {
			err := <-results
			assert.NoError(t, err)
		}

		// Clean up
		err = repository.ClearProject(ctx, core.ID(projectID))
		require.NoError(t, err)
	})
}

// TestMCPResourceProviders tests MCP resource providers
func TestMCPResourceProviders(t *testing.T) {
	// Skip if Neo4j is not available
	if !testhelpers.IsNeo4jAvailable() {
		t.Skip("Neo4j not available, skipping MCP resource test")
	}

	projectRoot := getProjectRoot()
	ctx := context.Background()

	// Start Neo4j container
	container, err := testhelpers.StartNeo4jContainer(ctx)
	require.NoError(t, err)
	defer container.Stop(ctx)

	// Create repository
	repository, err := container.CreateRepository()
	require.NoError(t, err)
	defer repository.Close()

	// Create services
	parserService := parser.NewService(nil)
	analyzerService := analyzer.NewAnalyzer(nil)
	graphBuilder := graph.NewBuilder(graph.DefaultBuilderConfig())
	graphService := graph.NewService(
		parserService,
		analyzerService,
		graphBuilder,
		repository,
		graph.DefaultServiceConfig(),
	)

	// Create service adapter
	serviceAdapter := &realServiceAdapter{
		graphService: graphService,
		repository:   repository,
	}

	// Create MCP server with test-friendly config
	config := mcpconfig.DefaultConfig()
	// Allow the project root directory for testing
	config.Security.AllowedPaths = []string{projectRoot, "/tmp"}
	server := mcp.NewServer(config, serviceAdapter, nil, nil, nil)

	t.Run("Should provide project metadata resource", func(t *testing.T) {
		// Clear database
		err := container.ClearDatabase(ctx)
		require.NoError(t, err)

		projectID := "test-project-resource"

		// Test project metadata resource
		data, err := server.HandleProjectMetadataResource(ctx, map[string]string{
			"project_id": projectID,
		})

		require.NoError(t, err)
		assert.NotNil(t, data)
		assert.Greater(t, len(data), 0)
	})

	t.Run("Should provide query templates resource", func(t *testing.T) {
		data, err := server.HandleQueryTemplatesResource(ctx, map[string]string{})

		require.NoError(t, err)
		assert.NotNil(t, data)
		assert.Greater(t, len(data), 0)
	})

	t.Run("Should provide code patterns resource", func(t *testing.T) {
		data, err := server.HandleCodePatternsResource(ctx, map[string]string{})

		require.NoError(t, err)
		assert.NotNil(t, data)
		assert.Greater(t, len(data), 0)
	})

	t.Run("Should provide project invariants resource", func(t *testing.T) {
		projectID := "test-project-invariants"

		data, err := server.HandleProjectInvariantsResource(ctx, map[string]string{
			"project_id": projectID,
		})

		require.NoError(t, err)
		assert.NotNil(t, data)
		assert.Greater(t, len(data), 0)
	})
}

// realServiceAdapter implements mcp.ServiceAdapter for testing
type realServiceAdapter struct {
	graphService graph.Service
	repository   *infra.Neo4jRepository
}

func (r *realServiceAdapter) ParseProject(ctx context.Context, projectPath string) (*parser.ParseResult, error) {
	parserService := parser.NewService(nil)
	return parserService.ParseProject(ctx, projectPath, nil)
}

func (r *realServiceAdapter) AnalyzeProject(
	ctx context.Context,
	projectID core.ID,
	files []*parser.FileInfo,
) (*analyzer.AnalysisReport, error) {
	analyzerService := analyzer.NewAnalyzer(nil)
	input := &analyzer.AnalysisInput{
		ProjectID: string(projectID),
		Files:     files,
	}
	return analyzerService.AnalyzeProject(ctx, input)
}

func (r *realServiceAdapter) InitializeProject(ctx context.Context, project *core.Project) error {
	return r.graphService.InitializeProject(ctx, project)
}

func (r *realServiceAdapter) ImportAnalysisResult(
	ctx context.Context,
	result *core.AnalysisResult,
) (*graph.ProjectGraph, error) {
	// First import the analysis
	err := r.graphService.ImportAnalysis(ctx, result.ProjectID, result)
	if err != nil {
		return nil, err
	}

	// Then get the project graph
	return r.graphService.GetProjectGraph(ctx, result.ProjectID)
}

func (r *realServiceAdapter) GetProjectStatistics(
	ctx context.Context,
	projectID core.ID,
) (*graph.ProjectStatistics, error) {
	return r.graphService.GetProjectStatistics(ctx, projectID)
}

func (r *realServiceAdapter) ExecuteQuery(
	ctx context.Context,
	query string,
	params map[string]any,
) ([]map[string]any, error) {
	return r.repository.ExecuteQuery(ctx, query, params)
}
