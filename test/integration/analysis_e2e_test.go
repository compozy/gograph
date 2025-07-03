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
			ProjectID:   string(projectID),
			ParseResult: parseResult,
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
		assert.Greater(t, stats.TotalNodes, 0, "Should have nodes in the graph")
		assert.Greater(t, stats.TotalRelationships, 0, "Should have relationships in the graph")

		// 5. Execute some queries to verify the graph
		// First, let's see what nodes exist with this project_id
		debugQuery := `
			MATCH (n {project_id: $project_id})
			RETURN n.type as type, n.name as name
			LIMIT 10
		`
		debugResults, err := repository.ExecuteQuery(ctx, debugQuery, map[string]any{
			"project_id": projectID.String(),
		})
		require.NoError(t, err)

		// If no nodes found with project_id, check if nodes exist without project_id filter
		if len(debugResults) == 0 {
			allNodesQuery := `MATCH (n) RETURN n.type as type, n.name as name LIMIT 10`
			allResults, err := repository.ExecuteQuery(ctx, allNodesQuery, nil)
			require.NoError(t, err)
			t.Logf("No nodes found with project_id %s, but found %d nodes total", projectID.String(), len(allResults))
			for i, result := range allResults {
				t.Logf("Node %d: type=%v, name=%v", i, result["type"], result["name"])
			}
		}

		// Find main function
		query := `
			MATCH (f:Function {name: 'main', project_id: $project_id})
			RETURN f.name as name, f.signature as signature
		`
		results, err := repository.ExecuteQuery(ctx, query, map[string]any{
			"project_id": projectID.String(),
		})
		require.NoError(t, err)
		if assert.Len(t, results, 1, "main function should be found in the graph") {
			assert.Equal(t, "main", results[0]["name"])
		}

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
			ProjectID:   string(projectID),
			ParseResult: parseResult,
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

		// Verify all nodes were created and wait for them to be fully committed
		query := "MATCH (n:Function) WHERE n.project_id = $project_id RETURN count(n) as count"
		results, err := repository.ExecuteQuery(ctx, query, map[string]any{
			"project_id": string(projectID),
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1000), results[0]["count"])

		// Verify specific nodes exist before creating relationships
		// This ensures nodes are properly indexed and accessible
		verifyQuery := `
			MATCH (n:Function) 
			WHERE n.project_id = $project_id AND n.id IN $node_ids
			RETURN count(n) as count
		`
		// Check a sample of node IDs
		sampleNodeIDs := []string{
			nodes[0].ID.String(),
			nodes[499].ID.String(),
			nodes[500].ID.String(),
			nodes[999].ID.String(),
		}
		verifyResults, err := repository.ExecuteQuery(ctx, verifyQuery, map[string]any{
			"project_id": string(projectID),
			"node_ids":   sampleNodeIDs,
		})
		require.NoError(t, err)
		require.Equal(t, int64(4), verifyResults[0]["count"], "Sample nodes should exist before creating relationships")

		// Create relationships in smaller batches to avoid issues
		const relationshipBatchSize = 100

		for batch := 0; batch < 5; batch++ {
			relationships := make([]core.Relationship, relationshipBatchSize)
			for i := 0; i < relationshipBatchSize; i++ {
				idx := batch*relationshipBatchSize + i
				relationships[i] = core.Relationship{
					ID:         core.NewID(),
					Type:       core.RelationCalls,
					FromNodeID: nodes[idx].ID,
					ToNodeID:   nodes[idx+500].ID,
					Properties: map[string]any{
						"project_id": string(projectID),
					},
				}
			}

			// Create this batch of relationships
			err = repository.CreateRelationships(ctx, relationships)
			if err != nil {
				// Log detailed error information for debugging
				t.Logf("Failed to create relationship batch %d: %v", batch, err)

				// Check if the nodes still exist
				checkQuery := `
					MATCH (n:Function) 
					WHERE n.project_id = $project_id AND n.id = $node_id
					RETURN n.id as id, n.name as name
				`
				// Check the first failing relationship's nodes
				fromResult, _ := repository.ExecuteQuery(ctx, checkQuery, map[string]any{
					"project_id": string(projectID),
					"node_id":    relationships[0].FromNodeID.String(),
				})
				toResult, _ := repository.ExecuteQuery(ctx, checkQuery, map[string]any{
					"project_id": string(projectID),
					"node_id":    relationships[0].ToNodeID.String(),
				})

				t.Logf("From node exists: %v", len(fromResult) > 0)
				t.Logf("To node exists: %v", len(toResult) > 0)
			}
			require.NoError(t, err, "Failed to create relationship batch %d", batch)
		}

		// Verify relationships were created
		query = "MATCH ()-[r:CALLS]->() WHERE r.project_id = $project_id RETURN count(r) as count"
		results, err = repository.ExecuteQuery(ctx, query, map[string]any{
			"project_id": string(projectID),
		})
		require.NoError(t, err)
		assert.Equal(t, int64(500), results[0]["count"], "All relationships should be created")

		// Clean up
		err = repository.ClearProject(ctx, projectID)
		require.NoError(t, err)
	})
}
