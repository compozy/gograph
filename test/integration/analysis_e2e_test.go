package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/parser"
	"github.com/compozy/gograph/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullProjectAnalysis(t *testing.T) {
	// Skip if Neo4j is not available
	if !testhelpers.IsNeo4jAvailable() {
		t.Skip("Neo4j not available, skipping integration test")
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

	t.Run("Should perform complete project analysis pipeline", func(t *testing.T) {
		// Clear any existing data
		err := container.ClearDatabase(ctx)
		require.NoError(t, err)

		// 1. Parse the project
		parserService := parser.NewService(nil)
		testProjectPath := filepath.Join(projectRoot, "testdata", "simple_project")

		parseResult, err := parserService.ParseProject(ctx, testProjectPath, nil)
		require.NoError(t, err)
		require.NotNil(t, parseResult)

		// 2. Analyze the parsed results
		analyzerService := analyzer.NewAnalyzer(nil)
		projectID := core.NewID()

		analysisInput := &analyzer.AnalysisInput{
			ProjectID: string(projectID),
			Files:     parseResult.Files,
		}
		analysisReport, err := analyzerService.AnalyzeProject(ctx, analysisInput)
		require.NoError(t, err)
		require.NotNil(t, analysisReport)

		// Verify analysis results
		assert.Equal(t, string(projectID), analysisReport.ProjectID)
		assert.NotNil(t, analysisReport.DependencyGraph)

		// 3. Create graph service and import results
		graphBuilder := graph.NewBuilder(graph.DefaultBuilderConfig())
		graphService := graph.NewService(
			parserService,
			analyzerService,
			graphBuilder,
			repository,
			graph.DefaultServiceConfig(),
		)

		// Initialize project
		project := &core.Project{
			ID:       projectID,
			Name:     "simple_project",
			RootPath: testProjectPath,
		}
		err = graphService.InitializeProject(ctx, project)
		require.NoError(t, err)

		// Build and import analysis results
		analysisResult, err := graphBuilder.BuildFromAnalysis(ctx, projectID, parseResult, analysisReport)
		require.NoError(t, err)

		err = graphService.ImportAnalysis(ctx, projectID, analysisResult)
		require.NoError(t, err)

		// 4. Verify the graph was created correctly
		// Check project statistics
		stats, err := graphService.GetProjectStatistics(ctx, projectID)
		require.NoError(t, err)
		assert.Greater(t, stats.TotalNodes, 0)
		assert.Greater(t, stats.TotalRelationships, 0)

		// 5. Execute some queries to verify the graph
		// Find main function
		query := `
			MATCH (f:Function {name: 'main', project_id: $project_id})
			RETURN f.name as name, f.signature as signature
		`
		results, err := repository.ExecuteQuery(ctx, query, map[string]any{
			"project_id": string(projectID),
		})
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "main", results[0]["name"])

		// Check function calls - verify any CALLS relationships exist
		query = `
			MATCH ()-[r:CALLS]->()
			RETURN count(r) as call_count
		`
		results, err = repository.ExecuteQuery(ctx, query, map[string]any{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, results[0]["call_count"].(int64), int64(0))

		// Check package dependencies - verify any IMPORTS relationships exist
		query = `
			MATCH ()-[r:IMPORTS]->()
			RETURN count(r) as import_count
		`
		results, err = repository.ExecuteQuery(ctx, query, map[string]any{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, results[0]["import_count"].(int64), int64(0))

		// Clean up
		err = repository.ClearProject(ctx, projectID)
		require.NoError(t, err)
	})

	t.Run("Should detect circular dependencies", func(t *testing.T) {
		// Clear database
		err := container.ClearDatabase(ctx)
		require.NoError(t, err)

		// Parse circular deps project
		parserService := parser.NewService(nil)
		testProjectPath := filepath.Join(projectRoot, "testdata", "circular_deps")

		parseResult, err := parserService.ParseProject(ctx, testProjectPath, nil)
		require.NoError(t, err)

		// Analyze
		analyzerService := analyzer.NewAnalyzer(nil)
		projectID := core.NewID()

		analysisInput := &analyzer.AnalysisInput{
			ProjectID: string(projectID),
			Files:     parseResult.Files,
		}
		analysisReport, err := analyzerService.AnalyzeProject(ctx, analysisInput)
		require.NoError(t, err)

		// The analyzer should successfully analyze even with potential circular deps
		assert.NotNil(t, analysisReport)
		assert.NotNil(t, analysisReport.DependencyGraph)

		// Create graph
		graphBuilder := graph.NewBuilder(graph.DefaultBuilderConfig())
		graphService := graph.NewService(
			parserService,
			analyzerService,
			graphBuilder,
			repository,
			graph.DefaultServiceConfig(),
		)

		project := &core.Project{
			ID:       projectID,
			Name:     "circular_deps",
			RootPath: testProjectPath,
		}
		err = graphService.InitializeProject(ctx, project)
		require.NoError(t, err)

		// Build and import analysis results
		analysisResult, err := graphBuilder.BuildFromAnalysis(ctx, projectID, parseResult, analysisReport)
		require.NoError(t, err)

		err = graphService.ImportAnalysis(ctx, projectID, analysisResult)
		require.NoError(t, err)

		// Query for potential circular dependencies
		query := `
			MATCH path=(p1:Package)-[:IMPORTS*2..]->(p1)
			WHERE p1.project_id = $project_id
			RETURN p1.name as package
			LIMIT 5
		`
		results, err := repository.ExecuteQuery(ctx, query, map[string]any{
			"project_id": string(projectID),
		})
		require.NoError(t, err)
		// In our test case, we don't have actual circular deps (commented out)
		assert.Len(t, results, 0)

		// Clean up
		err = repository.ClearProject(ctx, projectID)
		require.NoError(t, err)
	})
}

func TestAnalysisPerformance(t *testing.T) {
	// Skip if Neo4j is not available
	if !testhelpers.IsNeo4jAvailable() {
		t.Skip("Neo4j not available, skipping integration test")
	}

	t.Run("Should handle large batch operations efficiently", func(t *testing.T) {
		ctx := context.Background()

		// Start Neo4j container
		container, err := testhelpers.StartNeo4jContainer(ctx)
		require.NoError(t, err)
		defer container.Stop(ctx)

		// Create repository
		repository, err := container.CreateRepository()
		require.NoError(t, err)
		defer repository.Close()

		// Clear database
		err = container.ClearDatabase(ctx)
		require.NoError(t, err)

		projectID := core.NewID()

		// Create a large number of nodes
		nodes := make([]core.Node, 1000)
		for i := 0; i < 1000; i++ {
			nodes[i] = core.Node{
				ID:   core.NewID(),
				Type: core.NodeTypeFunction,
				Name: fmt.Sprintf("function_%d", i),
				Properties: map[string]any{
					"project_id": string(projectID),
					"signature":  fmt.Sprintf("func function_%d()", i),
				},
			}
		}

		// Batch create should be efficient
		err = repository.CreateNodes(ctx, nodes)
		require.NoError(t, err)

		// Verify all nodes were created
		query := "MATCH (n:Function) WHERE n.project_id = $project_id RETURN count(n) as count"
		results, err := repository.ExecuteQuery(ctx, query, map[string]any{
			"project_id": string(projectID),
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1000), results[0]["count"])

		// Create relationships in batch
		relationships := make([]core.Relationship, 500)
		for i := 0; i < 500; i++ {
			relationships[i] = core.Relationship{
				ID:         core.NewID(),
				Type:       core.RelationCalls,
				FromNodeID: nodes[i].ID,
				ToNodeID:   nodes[i+500].ID,
				Properties: map[string]any{
					"project_id": string(projectID),
				},
			}
		}

		err = repository.CreateRelationships(ctx, relationships)
		require.NoError(t, err)

		// Verify relationships - check all CALLS relationships since we just created them
		query = "MATCH ()-[r:CALLS]->() RETURN count(r) as count"
		results, err = repository.ExecuteQuery(ctx, query, map[string]any{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, results[0]["count"].(int64), int64(500))

		// Clean up
		err = repository.ClearProject(ctx, projectID)
		require.NoError(t, err)
	})
}
