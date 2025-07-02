package integration

import (
	"context"
	"testing"

	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNeo4jBasicIntegration(t *testing.T) {
	// Skip if Neo4j is not available
	if !testhelpers.IsNeo4jAvailable() {
		t.Skip("Neo4j not available, skipping integration test")
	}
	ctx := context.Background()
	// Start Neo4j container
	container, err := testhelpers.StartNeo4jContainer(ctx)
	require.NoError(t, err)
	defer container.Stop(ctx)
	// Test basic connectivity
	err = container.VerifyConnection(ctx)
	require.NoError(t, err)
	// Create repository
	repository, err := container.CreateRepository()
	require.NoError(t, err)
	defer repository.Close()
	// Test basic operations
	projectID := core.NewID()
	// Clear any existing data
	err = repository.ClearProject(ctx, projectID)
	require.NoError(t, err)
	// Create test nodes
	nodes := []core.Node{
		{
			ID:   core.NewID(),
			Type: core.NodeTypeFile,
			Name: "main.go",
			Properties: map[string]any{
				"path":       "/test/main.go",
				"project_id": string(projectID),
			},
		},
		{
			ID:   core.NewID(),
			Type: core.NodeTypeFunction,
			Name: "main",
			Properties: map[string]any{
				"signature":  "func main()",
				"project_id": string(projectID),
			},
		},
	}
	// Create nodes
	err = repository.CreateNodes(ctx, nodes)
	require.NoError(t, err)
	// Verify nodes were created
	query := "MATCH (n) WHERE n.project_id = $project_id RETURN count(n) as node_count"
	result, err := repository.ExecuteQuery(ctx, query, map[string]any{
		"project_id": string(projectID),
	})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, int64(2), result[0]["node_count"])
	// Create relationship
	relationships := []core.Relationship{
		{
			ID:         core.NewID(),
			Type:       core.RelationContains,
			FromNodeID: nodes[0].ID,
			ToNodeID:   nodes[1].ID,
			Properties: map[string]any{
				"project_id": string(projectID),
			},
		},
	}
	err = repository.CreateRelationships(ctx, relationships)
	require.NoError(t, err)
	// Verify relationship was created - check all relationships since we just created one
	query = "MATCH ()-[r]->() RETURN count(r) as rel_count"
	result, err = repository.ExecuteQuery(ctx, query, map[string]any{})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.GreaterOrEqual(t, result[0]["rel_count"].(int64), int64(1))
	// Clean up
	err = repository.ClearProject(ctx, projectID)
	require.NoError(t, err)
}
