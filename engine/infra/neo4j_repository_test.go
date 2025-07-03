package infra_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/infra"
	"github.com/compozy/gograph/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateUniqueTestID creates a unique ID for test isolation
func generateUniqueTestID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("test-%x-%d", bytes, time.Now().UnixNano())
}

// setupNeo4jTestWithProjectID creates a Neo4j test setup with a unique project ID for isolation
func setupNeo4jTestWithProjectID(t *testing.T) (*infra.Neo4jRepository, string, context.Context) {
	t.Helper()

	// Skip if running in CI without Docker
	if os.Getenv("CI") == "true" && os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration tests in CI")
	}

	container, cleanup := testhelpers.SetupNeo4jTest(t)
	defer cleanup()

	repo, err := container.CreateRepository()
	require.NoError(t, err)

	// Generate unique project ID for this test
	projectID := generateUniqueTestID()

	ctx := context.Background()

	// Clean up this specific project's data after the test
	t.Cleanup(func() {
		if repo != nil {
			container.ClearDatabaseForProject(context.Background(), projectID)
			repo.Close()
		}
	})

	return repo, projectID, ctx
}

// TestNeo4jRepository_Connect tests connection functionality
func TestNeo4jRepository_Connect(t *testing.T) {
	t.Run("Should connect successfully with valid credentials", func(t *testing.T) {
		// Skip if running in CI without Docker
		if os.Getenv("CI") == "true" && os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
			t.Skip("Skipping integration tests in CI")
		}

		container, cleanup := testhelpers.SetupNeo4jTest(t)
		defer cleanup()

		// Connection is already verified in setup
		err := container.VerifyConnection(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Should fail with invalid credentials", func(t *testing.T) {
		// Skip if running in CI without Docker
		if os.Getenv("CI") == "true" && os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
			t.Skip("Skipping integration tests in CI")
		}

		// Test invalid credentials with a new config
		config := &infra.Neo4jConfig{
			URI:        "bolt://localhost:7687",
			Username:   "invalid",
			Password:   "invalid",
			Database:   "",
			MaxRetries: 1,
			BatchSize:  1000,
		}

		_, err := infra.NewNeo4jRepository(config)
		assert.Error(t, err)
	})
}

// TestNeo4jRepository_CreateNode tests node creation
func TestNeo4jRepository_CreateNode(t *testing.T) {
	t.Run("Should create node successfully", func(t *testing.T) {
		repo, projectID, ctx := setupNeo4jTestWithProjectID(t)

		node := &core.Node{
			ID:   core.NewID(),
			Type: core.NodeType("Function"),
			Name: "TestFunction",
			Path: "/src/test.go",
			Properties: map[string]any{
				"line":       10,
				"column":     5,
				"signature":  "func TestFunction() error",
				"project_id": projectID,
			},
			CreatedAt: time.Now().UTC(),
		}

		err := repo.CreateNode(ctx, node)
		assert.NoError(t, err)

		// Verify node was created
		query := `MATCH (n {id: $id, project_id: $project_id}) RETURN n.name as name`
		result, err := repo.ExecuteQuery(ctx, query, map[string]any{
			"id":         node.ID.String(),
			"project_id": projectID,
		})
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "TestFunction", result[0]["name"])
	})

	t.Run("Should create separate nodes when called multiple times", func(t *testing.T) {
		repo, projectID, ctx := setupNeo4jTestWithProjectID(t)

		node := &core.Node{
			ID:   core.NewID(),
			Type: core.NodeType("Function"),
			Name: "TestFunction",
			Path: "/src/test.go",
			Properties: map[string]any{
				"project_id": projectID,
			},
			CreatedAt: time.Now().UTC(),
		}

		// Create node first time
		err := repo.CreateNode(ctx, node)
		require.NoError(t, err)

		// Create same node again (implementation uses CREATE, so it will create a duplicate)
		err = repo.CreateNode(ctx, node)
		assert.NoError(t, err)

		// Verify two nodes exist (since CREATE is used, not MERGE)
		query := `MATCH (n {id: $id, project_id: $project_id}) RETURN count(n) as count`
		result, err := repo.ExecuteQuery(ctx, query, map[string]any{
			"id":         node.ID.String(),
			"project_id": projectID,
		})
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, int64(2), result[0]["count"])
	})
}

// TestNeo4jRepository_CreateRelationship tests relationship creation
func TestNeo4jRepository_CreateRelationship(t *testing.T) {
	t.Run("Should create relationship between existing nodes", func(t *testing.T) {
		repo, projectID, ctx := setupNeo4jTestWithProjectID(t)

		// Create source node
		sourceNode := &core.Node{
			ID:   core.NewID(),
			Type: core.NodeType("Function"),
			Name: "SourceFunction",
			Path: "/src/source.go",
			Properties: map[string]any{
				"project_id": projectID,
			},
			CreatedAt: time.Now().UTC(),
		}
		err := repo.CreateNode(ctx, sourceNode)
		require.NoError(t, err)

		// Create target node
		targetNode := &core.Node{
			ID:   core.NewID(),
			Type: core.NodeType("Function"),
			Name: "TargetFunction",
			Path: "/src/target.go",
			Properties: map[string]any{
				"project_id": projectID,
			},
			CreatedAt: time.Now().UTC(),
		}
		err = repo.CreateNode(ctx, targetNode)
		require.NoError(t, err)

		// Create relationship
		rel := &core.Relationship{
			ID:         core.NewID(),
			Type:       core.RelationType("CALLS"),
			FromNodeID: sourceNode.ID,
			ToNodeID:   targetNode.ID,
			Properties: map[string]any{
				"project_id": projectID,
			},
			CreatedAt: time.Now().UTC(),
		}

		err = repo.CreateRelationship(ctx, rel)
		assert.NoError(t, err)

		// Verify relationship was created
		query := `MATCH ()-[r {id: $id, project_id: $project_id}]->() RETURN type(r) as type`
		result, err := repo.ExecuteQuery(ctx, query, map[string]any{
			"id":         rel.ID.String(),
			"project_id": projectID,
		})
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "CALLS", result[0]["type"])
	})

	t.Run("Should handle relationship with non-existent nodes", func(t *testing.T) {
		repo, projectID, ctx := setupNeo4jTestWithProjectID(t)

		// Note: Neo4j's MATCH clause will simply not create the relationship if nodes don't exist
		// This test verifies that behavior
		rel := &core.Relationship{
			ID:         core.NewID(),
			Type:       core.RelationType("CALLS"),
			FromNodeID: core.NewID(), // Non-existent
			ToNodeID:   core.NewID(), // Non-existent
			Properties: map[string]any{
				"project_id": projectID,
			},
			CreatedAt: time.Now().UTC(),
		}

		// The CreateRelationship uses MATCH, so it won't create anything if nodes don't exist
		err := repo.CreateRelationship(ctx, rel)
		assert.NoError(t, err) // No error is returned, but the relationship won't be created

		// Verify the relationship wasn't created
		query := `MATCH ()-[r {id: $id, project_id: $project_id}]->() RETURN r`
		result, err := repo.ExecuteQuery(ctx, query, map[string]any{
			"id":         rel.ID.String(),
			"project_id": projectID,
		})
		require.NoError(t, err)
		assert.Empty(t, result) // No relationship should exist
	})
}

// TestNeo4jRepository_ExecuteQuery tests query execution
func TestNeo4jRepository_ExecuteQuery(t *testing.T) {
	t.Run("Should execute simple query successfully", func(t *testing.T) {
		repo, _, ctx := setupNeo4jTestWithProjectID(t)

		query := "RETURN 1 as number, 'hello' as text"
		result, err := repo.ExecuteQuery(ctx, query, nil)

		assert.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, int64(1), result[0]["number"])
		assert.Equal(t, "hello", result[0]["text"])
	})

	t.Run("Should handle query with parameters", func(t *testing.T) {
		repo, _, ctx := setupNeo4jTestWithProjectID(t)

		query := "RETURN $param1 as value1, $param2 as value2"
		params := map[string]any{
			"param1": "test",
			"param2": 42,
		}

		result, err := repo.ExecuteQuery(ctx, query, params)

		assert.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "test", result[0]["value1"])
		assert.Equal(t, int64(42), result[0]["value2"])
	})

	t.Run("Should handle invalid query", func(t *testing.T) {
		repo, _, ctx := setupNeo4jTestWithProjectID(t)

		query := "INVALID CYPHER QUERY"
		_, err := repo.ExecuteQuery(ctx, query, nil)

		assert.Error(t, err)
	})
}

// TestNeo4jRepository_BulkImport tests bulk import functionality
func TestNeo4jRepository_BulkImport(t *testing.T) {
	t.Run("Should import multiple nodes in batch", func(t *testing.T) {
		repo, projectID, ctx := setupNeo4jTestWithProjectID(t)

		// Create test nodes
		projectUUID := core.ID(projectID)
		nodes := []core.Node{
			{
				ID:   core.NewID(),
				Type: core.NodeType("Function"),
				Name: "Function1",
				Path: "/src/test1.go",
				Properties: map[string]any{
					"project_id": projectID,
				},
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:   core.NewID(),
				Type: core.NodeType("Function"),
				Name: "Function2",
				Path: "/src/test2.go",
				Properties: map[string]any{
					"project_id": projectID,
				},
				CreatedAt: time.Now().UTC(),
			},
		}

		// Create relationships
		relationships := []core.Relationship{
			{
				ID:         core.NewID(),
				Type:       core.RelationType("CALLS"),
				FromNodeID: nodes[0].ID,
				ToNodeID:   nodes[1].ID,
				Properties: map[string]any{
					"project_id": projectID,
				},
				CreatedAt: time.Now().UTC(),
			},
		}

		// Import data
		result := &core.AnalysisResult{
			ProjectID:     projectUUID,
			Nodes:         nodes,
			Relationships: relationships,
		}

		err := repo.ImportAnalysisResult(ctx, result)
		assert.NoError(t, err)

		// Verify import - count function nodes only (excluding ProjectMetadata node)
		query := `MATCH (n:Function) WHERE n.project_id = $project_id RETURN count(n) as count`
		queryResult, err := repo.ExecuteQuery(ctx, query, map[string]any{
			"project_id": projectID,
		})
		require.NoError(t, err)
		require.Len(t, queryResult, 1)
		assert.Equal(t, int64(2), queryResult[0]["count"])

		// Verify ProjectMetadata node was created
		metaQuery := `MATCH (p:ProjectMetadata {project_id: $project_id}) RETURN count(p) as count`
		metaResult, err := repo.ExecuteQuery(ctx, metaQuery, map[string]any{
			"project_id": projectID,
		})
		require.NoError(t, err)
		require.Len(t, metaResult, 1)
		assert.Equal(t, int64(1), metaResult[0]["count"])

		// Verify relationships for this specific project using CALLS relationship type
		relQuery := `MATCH ()-[r:CALLS]->() WHERE r.project_id = $project_id RETURN count(r) as count`
		relResult, err := repo.ExecuteQuery(ctx, relQuery, map[string]any{
			"project_id": projectID,
		})
		require.NoError(t, err)
		require.Len(t, relResult, 1)
		assert.Equal(t, int64(1), relResult[0]["count"])
	})
}

func TestNeo4jRepository_ProjectIsolation(t *testing.T) {
	t.Run("Should completely isolate data between projects", func(t *testing.T) {
		repo, _, ctx := setupNeo4jTestWithProjectID(t)

		// Create two separate projects with unique IDs
		projectID1 := generateUniqueTestID()
		projectID2 := generateUniqueTestID()
		projectUUID1 := core.ID(projectID1)
		projectUUID2 := core.ID(projectID2)

		// Create nodes for project 1
		nodes1 := []core.Node{
			{
				ID:   core.NewID(),
				Type: core.NodeTypeFunction,
				Name: "Function1",
				Properties: map[string]any{
					"project_id": projectID1,
					"package":    "project1.pkg",
				},
			},
			{
				ID:   core.NewID(),
				Type: core.NodeTypeStruct,
				Name: "Struct1",
				Properties: map[string]any{
					"project_id": projectID1,
					"package":    "project1.pkg",
				},
			},
		}

		// Create nodes for project 2
		nodes2 := []core.Node{
			{
				ID:   core.NewID(),
				Type: core.NodeTypeFunction,
				Name: "Function2",
				Properties: map[string]any{
					"project_id": projectID2,
					"package":    "project2.pkg",
				},
			},
			{
				ID:   core.NewID(),
				Type: core.NodeTypeInterface,
				Name: "Interface2",
				Properties: map[string]any{
					"project_id": projectID2,
					"package":    "project2.pkg",
				},
			},
		}

		// Create relationships for project 1
		relationships1 := []core.Relationship{
			{
				ID:         core.NewID(),
				Type:       core.RelationType("CONTAINS"),
				FromNodeID: nodes1[0].ID,
				ToNodeID:   nodes1[1].ID,
				Properties: map[string]any{
					"project_id": projectID1,
				},
			},
		}

		// Create relationships for project 2
		relationships2 := []core.Relationship{
			{
				ID:         core.NewID(),
				Type:       core.RelationType("IMPLEMENTS"),
				FromNodeID: nodes2[0].ID,
				ToNodeID:   nodes2[1].ID,
				Properties: map[string]any{
					"project_id": projectID2,
				},
			},
		}

		// Import project 1 data
		result1 := &core.AnalysisResult{
			ProjectID:     projectUUID1,
			Nodes:         nodes1,
			Relationships: relationships1,
		}
		err := repo.ImportAnalysisResult(ctx, result1)
		require.NoError(t, err)

		// Import project 2 data
		result2 := &core.AnalysisResult{
			ProjectID:     projectUUID2,
			Nodes:         nodes2,
			Relationships: relationships2,
		}
		err = repo.ImportAnalysisResult(ctx, result2)
		require.NoError(t, err)

		// Verify project 1 data isolation
		t.Run("Project 1 should only see its own data", func(t *testing.T) {
			// Check nodes
			query := `MATCH (n) WHERE n.project_id = $project_id RETURN labels(n)[0] as type, n.name as name`
			result, err := repo.ExecuteQuery(ctx, query, map[string]any{
				"project_id": projectID1,
			})
			require.NoError(t, err)
			require.Len(t, result, 3) // 2 nodes + 1 ProjectMetadata

			// Check that only project 1 data is returned
			nodeNames := make([]string, 0)
			for _, row := range result {
				if name, ok := row["name"].(string); ok && name != "" {
					nodeNames = append(nodeNames, name)
				}
			}
			assert.Contains(t, nodeNames, "Function1")
			assert.Contains(t, nodeNames, "Struct1")
			assert.NotContains(t, nodeNames, "Function2")
			assert.NotContains(t, nodeNames, "Interface2")

			// Check relationships
			relQuery := `MATCH ()-[r]->() WHERE r.project_id = $project_id RETURN type(r) as rel_type`
			relResult, err := repo.ExecuteQuery(ctx, relQuery, map[string]any{
				"project_id": projectID1,
			})
			require.NoError(t, err)
			require.Len(t, relResult, 1)
			assert.Equal(t, "CONTAINS", relResult[0]["rel_type"])
		})

		// Verify project 2 data isolation
		t.Run("Project 2 should only see its own data", func(t *testing.T) {
			// Check nodes
			query := `MATCH (n) WHERE n.project_id = $project_id RETURN labels(n)[0] as type, n.name as name`
			result, err := repo.ExecuteQuery(ctx, query, map[string]any{
				"project_id": projectID2,
			})
			require.NoError(t, err)
			require.Len(t, result, 3) // 2 nodes + 1 ProjectMetadata

			// Check that only project 2 data is returned
			nodeNames := make([]string, 0)
			for _, row := range result {
				if name, ok := row["name"].(string); ok && name != "" {
					nodeNames = append(nodeNames, name)
				}
			}
			assert.Contains(t, nodeNames, "Function2")
			assert.Contains(t, nodeNames, "Interface2")
			assert.NotContains(t, nodeNames, "Function1")
			assert.NotContains(t, nodeNames, "Struct1")

			// Check relationships
			relQuery := `MATCH ()-[r]->() WHERE r.project_id = $project_id RETURN type(r) as rel_type`
			relResult, err := repo.ExecuteQuery(ctx, relQuery, map[string]any{
				"project_id": projectID2,
			})
			require.NoError(t, err)
			require.Len(t, relResult, 1)
			assert.Equal(t, "IMPLEMENTS", relResult[0]["rel_type"])
		})

		// Verify repository methods respect project isolation
		t.Run("Repository methods should filter by project_id", func(t *testing.T) {
			// Test FindNodesByType for project 1
			project1Functions, err := repo.FindNodesByType(ctx, core.NodeTypeFunction, projectUUID1)
			require.NoError(t, err)
			require.Len(t, project1Functions, 1)
			assert.Equal(t, "Function1", project1Functions[0].Name)

			// Test FindNodesByType for project 2
			project2Functions, err := repo.FindNodesByType(ctx, core.NodeTypeFunction, projectUUID2)
			require.NoError(t, err)
			require.Len(t, project2Functions, 1)
			assert.Equal(t, "Function2", project2Functions[0].Name)

			// Test FindNodesByName for project 1
			project1Named, err := repo.FindNodesByName(ctx, "Function1", projectUUID1)
			require.NoError(t, err)
			require.Len(t, project1Named, 1)
			assert.Equal(t, "Function1", project1Named[0].Name)

			// Test FindNodesByName should not find nodes from other projects
			crossProject, err := repo.FindNodesByName(ctx, "Function2", projectUUID1)
			require.NoError(t, err)
			assert.Len(t, crossProject, 0) // Should not find Function2 in project1
		})

		// Verify ClearProject only affects target project
		t.Run("ClearProject should only affect target project", func(t *testing.T) {
			// Clear project 1
			err := repo.ClearProject(ctx, projectUUID1)
			require.NoError(t, err)

			// Verify project 1 data is gone
			query1 := `MATCH (n) WHERE n.project_id = $project_id RETURN count(n) as count`
			result1, err := repo.ExecuteQuery(ctx, query1, map[string]any{
				"project_id": projectID1,
			})
			require.NoError(t, err)
			require.Len(t, result1, 1)
			assert.Equal(t, int64(0), result1[0]["count"])

			// Verify project 2 data is still there
			result2, err := repo.ExecuteQuery(ctx, query1, map[string]any{
				"project_id": projectID2,
			})
			require.NoError(t, err)
			require.Len(t, result2, 1)
			assert.Equal(t, int64(3), result2[0]["count"]) // 2 nodes + 1 ProjectMetadata
		})
	})
}
