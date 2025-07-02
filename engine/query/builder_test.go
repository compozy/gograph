package query

import (
	"testing"

	"github.com/compozy/gograph/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilder(t *testing.T) {
	t.Run("Should_build_basic_match_query", func(t *testing.T) {
		builder := NewBuilder()
		query, params, err := builder.
			Match("(n:Function)").
			Where("n.project_id = $project_id").
			Return("n").
			Build()

		require.NoError(t, err)
		assert.Equal(t, "MATCH (n:Function) WHERE n.project_id = $project_id RETURN n", query)
		assert.Empty(t, params)
	})

	t.Run("Should_build_query_with_parameters", func(t *testing.T) {
		builder := NewBuilder()
		projectID := core.NewID()
		query, params, err := builder.
			Match("(n:Function)").
			Where("n.project_id = $project_id").
			SetParameter("project_id", string(projectID)).
			Return("n").
			Build()

		require.NoError(t, err)
		assert.Contains(t, query, "MATCH (n:Function)")
		assert.Equal(t, string(projectID), params["project_id"])
	})

	t.Run("Should_build_complex_query", func(t *testing.T) {
		builder := NewBuilder()
		query, params, err := builder.
			Match("(f:Function)").
			Where("f.project_id = $project_id").
			And("f.name CONTAINS $name").
			SetParameter("project_id", "test").
			SetParameter("name", "main").
			Return("f.name, f.signature").
			OrderBy("f.name").
			Limit(10).
			Build()

		require.NoError(t, err)
		assert.Contains(t, query, "MATCH (f:Function)")
		assert.Contains(t, query, "WHERE f.project_id = $project_id")
		assert.Contains(t, query, "AND f.name CONTAINS $name")
		assert.Contains(t, query, "RETURN f.name, f.signature")
		assert.Contains(t, query, "ORDER BY f.name")
		assert.Contains(t, query, "LIMIT 10")
		assert.Equal(t, "test", params["project_id"])
		assert.Equal(t, "main", params["name"])
	})
}

func TestHighLevelBuilder(t *testing.T) {
	t.Run("Should_create_find_nodes_by_type_query", func(t *testing.T) {
		hlb := NewHighLevelBuilder()
		projectID := core.NewID()
		builder := hlb.FindNodesByType(core.NodeTypeFunction, projectID)

		query, params, err := builder.Build()
		require.NoError(t, err)
		assert.Contains(t, query, "MATCH (n:Function)")
		assert.Contains(t, query, "WHERE n.project_id = $project_id")
		assert.Equal(t, string(projectID), params["project_id"])
	})

	t.Run("Should_create_find_dependencies_query", func(t *testing.T) {
		hlb := NewHighLevelBuilder()
		nodeID := core.NewID()
		projectID := core.NewID()
		builder := hlb.FindDependencies(nodeID, projectID)

		query, params, err := builder.Build()
		require.NoError(t, err)
		assert.Contains(t, query, "MATCH (n)-[:DEPENDS_ON*1..3]->(dep)")
		assert.Equal(t, string(nodeID), params["node_id"])
		assert.Equal(t, string(projectID), params["project_id"])
	})

	t.Run("Should_create_count_nodes_by_type_query", func(t *testing.T) {
		hlb := NewHighLevelBuilder()
		projectID := core.NewID()
		builder := hlb.CountNodesByType(projectID)

		query, params, err := builder.Build()
		require.NoError(t, err)
		assert.Contains(t, query, "RETURN labels(n)[0] as node_type, count(n) as count")
		assert.Equal(t, string(projectID), params["project_id"])
	})
}
