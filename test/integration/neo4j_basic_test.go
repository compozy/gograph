package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNeo4jComplexPropertySerialization(t *testing.T) {
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

	// Create repository - using concrete type directly
	repository, err := container.CreateRepository()
	require.NoError(t, err)
	defer repository.Close()

	// Clear database
	err = container.ClearDatabase(ctx)
	require.NoError(t, err)

	projectID := core.NewID()

	t.Run("Should serialize complex properties to JSON strings", func(t *testing.T) {
		// Create a node with complex properties that would cause Neo4j errors
		complexProps := map[string]any{
			// Simple properties (should remain unchanged)
			"simple_string": "test",
			"simple_int":    42,
			"simple_bool":   true,
			"simple_float":  3.14,

			// Complex properties (should be serialized to JSON)
			"struct_field": map[string]any{
				"name": "ID",
				"tag":  "`json:\"id\" yaml:\"id\" mapstructure:\"id\"`",
				"type": "string",
			},
			"slice_field": []map[string]any{
				{"name": "field1", "type": "string"},
				{"name": "field2", "type": "int"},
			},
			"nested_map": map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"value": "deeply nested",
					},
				},
			},
			"array_of_primitives": []string{"a", "b", "c"},
			"mixed_array":         []any{"string", 123, true},
			"project_id":          projectID.String(),
		}

		node := &core.Node{
			ID:         core.NewID(),
			Type:       "TestStruct",
			Name:       "ComplexPropsTest",
			Properties: complexProps,
		}

		// This should not fail with property type errors
		err := repository.CreateNode(ctx, node)
		assert.NoError(t, err, "Creating node with complex properties should not fail")

		// Retrieve the node and verify properties were stored correctly
		retrievedNode, err := repository.GetNode(ctx, node.ID)
		assert.NoError(t, err, "Should be able to retrieve the node")
		assert.NotNil(t, retrievedNode, "Retrieved node should not be nil")

		// Simple properties should remain unchanged
		assert.Equal(t, "test", retrievedNode.Properties["simple_string"])
		assert.Equal(t, int64(42), retrievedNode.Properties["simple_int"]) // Neo4j returns int64
		assert.Equal(t, true, retrievedNode.Properties["simple_bool"])
		assert.InDelta(t, 3.14, retrievedNode.Properties["simple_float"], 0.001)

		// Complex properties should be JSON strings
		structFieldJSON, ok := retrievedNode.Properties["struct_field"].(string)
		assert.True(t, ok, "Complex struct_field should be stored as JSON string")

		// Verify we can unmarshal the JSON back to the original structure
		var structField map[string]any
		err = json.Unmarshal([]byte(structFieldJSON), &structField)
		assert.NoError(t, err, "Should be able to unmarshal struct_field JSON")
		assert.Equal(t, "ID", structField["name"])
		assert.Equal(t, "`json:\"id\" yaml:\"id\" mapstructure:\"id\"`", structField["tag"])
		assert.Equal(t, "string", structField["type"])

		// Verify slice was serialized
		sliceFieldJSON, ok := retrievedNode.Properties["slice_field"].(string)
		assert.True(t, ok, "Complex slice_field should be stored as JSON string")

		var sliceField []map[string]any
		err = json.Unmarshal([]byte(sliceFieldJSON), &sliceField)
		assert.NoError(t, err, "Should be able to unmarshal slice_field JSON")
		assert.Len(t, sliceField, 2)
		assert.Equal(t, "field1", sliceField[0]["name"])
		assert.Equal(t, "string", sliceField[0]["type"])

		// Verify nested map was serialized
		nestedMapJSON, ok := retrievedNode.Properties["nested_map"].(string)
		assert.True(t, ok, "Complex nested_map should be stored as JSON string")

		var nestedMap map[string]any
		err = json.Unmarshal([]byte(nestedMapJSON), &nestedMap)
		assert.NoError(t, err, "Should be able to unmarshal nested_map JSON")
		level1, ok := nestedMap["level1"].(map[string]any)
		assert.True(t, ok, "Should have level1 map")
		level2, ok := level1["level2"].(map[string]any)
		assert.True(t, ok, "Should have level2 map")
		assert.Equal(t, "deeply nested", level2["value"])
	})

	t.Run("Should handle batch operations with complex properties", func(t *testing.T) {
		nodes := []core.Node{
			{
				ID:   core.NewID(),
				Type: "TestStruct",
				Name: "BatchTest1",
				Properties: map[string]any{
					"project_id": projectID.String(),
					"complex_field": map[string]any{
						"name": "TestField",
						"metadata": map[string]any{
							"exported": true,
							"tags":     []string{"json", "yaml"},
						},
					},
				},
			},
			{
				ID:   core.NewID(),
				Type: "TestStruct",
				Name: "BatchTest2",
				Properties: map[string]any{
					"project_id": projectID.String(),
					"array_field": []map[string]any{
						{"key": "value1"},
						{"key": "value2"},
					},
				},
			},
		}

		// Batch creation should not fail with property type errors
		err := repository.CreateNodes(ctx, nodes)
		assert.NoError(t, err, "Batch creating nodes with complex properties should not fail")

		// Verify both nodes were created and properties serialized
		for _, node := range nodes {
			retrievedNode, err := repository.GetNode(ctx, node.ID)
			assert.NoError(t, err, "Should be able to retrieve batch-created node")
			assert.NotNil(t, retrievedNode.Properties, "Node should have properties")
		}
	})

	t.Run("Should handle relationships with complex properties", func(t *testing.T) {
		// Create two nodes first
		fromNode := &core.Node{
			ID:   core.NewID(),
			Type: "TestStruct",
			Name: "FromNode",
			Properties: map[string]any{
				"project_id": projectID.String(),
			},
		}
		toNode := &core.Node{
			ID:   core.NewID(),
			Type: "TestStruct",
			Name: "ToNode",
			Properties: map[string]any{
				"project_id": projectID.String(),
			},
		}

		err := repository.CreateNode(ctx, fromNode)
		assert.NoError(t, err)
		err = repository.CreateNode(ctx, toNode)
		assert.NoError(t, err)

		// Create relationship with complex properties
		rel := &core.Relationship{
			ID:         core.NewID(),
			Type:       "USES",
			FromNodeID: fromNode.ID,
			ToNodeID:   toNode.ID,
			Properties: map[string]any{
				"project_id": projectID.String(),
				"call_info": map[string]any{
					"function_name": "TestFunction",
					"parameters": []map[string]any{
						{"name": "param1", "type": "string"},
						{"name": "param2", "type": "int"},
					},
					"return_type": "error",
				},
				"metadata": map[string]any{
					"line_number": 42,
					"file_path":   "/test/file.go",
				},
			},
		}

		// This should not fail with property type errors
		err = repository.CreateRelationship(ctx, rel)
		assert.NoError(t, err, "Creating relationship with complex properties should not fail")

		// Retrieve and verify
		retrievedRel, err := repository.GetRelationship(ctx, rel.ID)
		assert.NoError(t, err, "Should be able to retrieve the relationship")
		assert.NotNil(t, retrievedRel, "Retrieved relationship should not be nil")

		// Verify complex properties were serialized
		callInfoJSON, ok := retrievedRel.Properties["call_info"].(string)
		assert.True(t, ok, "Complex call_info should be stored as JSON string")

		var callInfo map[string]any
		err = json.Unmarshal([]byte(callInfoJSON), &callInfo)
		assert.NoError(t, err, "Should be able to unmarshal call_info JSON")
		assert.Equal(t, "TestFunction", callInfo["function_name"])
	})

	t.Run("Should handle nil and empty properties gracefully", func(t *testing.T) {
		testCases := []struct {
			name       string
			properties map[string]any
		}{
			{"nil properties", nil},
			{"empty properties", map[string]any{}},
			{"properties with nil values", map[string]any{
				"nil_field":   nil,
				"empty_slice": []any{},
				"empty_map":   map[string]any{},
			}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Add project_id for test isolation
				if tc.properties == nil {
					tc.properties = map[string]any{"project_id": projectID.String()}
				} else {
					tc.properties["project_id"] = projectID.String()
				}

				node := &core.Node{
					ID:         core.NewID(),
					Type:       "TestStruct",
					Name:       "NilPropsTest",
					Properties: tc.properties,
				}

				err := repository.CreateNode(ctx, node)
				assert.NoError(t, err, "Creating node with %s should not fail", tc.name)

				retrievedNode, err := repository.GetNode(ctx, node.ID)
				assert.NoError(t, err, "Should be able to retrieve node with %s", tc.name)
				assert.NotNil(t, retrievedNode, "Retrieved node should not be nil")
			})
		}
	})

	t.Run("Should handle time properties correctly", func(t *testing.T) {
		now := time.Now()
		node := &core.Node{
			ID:   core.NewID(),
			Type: "TestStruct",
			Name: "TimePropsTest",
			Properties: map[string]any{
				"project_id": projectID.String(),
				"created_at": now,
				"metadata": map[string]any{
					"timestamp": now,
					"info":      "test with time",
				},
			},
		}

		// Create node
		err := repository.CreateNode(ctx, node)
		require.NoError(t, err)

		// Retrieve and verify
		retrievedNode, err := repository.GetNode(ctx, node.ID)
		require.NoError(t, err)

		// Time should be stored as UTC time (Neo4j returns time as time.Time)
		retrievedTime, ok := retrievedNode.Properties["created_at"].(time.Time)
		if !ok {
			// Neo4j might return time as a different type, check for string representation
			timeStr, isStr := retrievedNode.Properties["created_at"].(string)
			if isStr {
				parsedTime, err := time.Parse(time.RFC3339Nano, timeStr)
				assert.NoError(t, err, "Should be able to parse time string")
				assert.True(t, parsedTime.UTC().Equal(now.UTC()), "Time should be stored as UTC")
			} else {
				t.Logf("Time property type: %T, value: %v", retrievedNode.Properties["created_at"], retrievedNode.Properties["created_at"])
				t.Logf("All properties: %+v", retrievedNode.Properties)
				// Check if created_at exists at all
				if retrievedNode.Properties["created_at"] == nil {
					t.Error("created_at property is nil")
				} else {
					t.Errorf("Unexpected time type: %T", retrievedNode.Properties["created_at"])
				}
			}
		} else {
			assert.True(t, retrievedTime.UTC().Equal(now.UTC()), "Time should be stored as UTC")
		}

		// Complex property should be serialized
		metadataJSON, ok := retrievedNode.Properties["metadata"].(string)
		assert.True(t, ok, "Metadata should be serialized to JSON string")

		var metadata map[string]any
		err = json.Unmarshal([]byte(metadataJSON), &metadata)
		assert.NoError(t, err, "Should be able to unmarshal metadata")
		assert.Equal(t, "test with time", metadata["info"])
	})

	// Clean up
	err = repository.ClearProject(ctx, projectID)
	require.NoError(t, err)
}

func TestNeo4jDeepNestedSerialization(t *testing.T) {
	// Skip if Neo4j is not available
	if !testhelpers.IsNeo4jAvailable() {
		t.Skip("Neo4j not available, skipping integration test")
	}

	ctx := context.Background()

	// Start Neo4j container
	container, err := testhelpers.StartNeo4jContainer(ctx)
	require.NoError(t, err)
	defer container.Stop(ctx)

	// Create repository
	repository, err := container.CreateRepository()
	require.NoError(t, err)
	defer repository.Close()

	projectID := core.NewID()

	t.Run("Should serialize interface methods with complex parameters", func(t *testing.T) {
		// This test case matches the exact error from the user's report
		interfaceNode := &core.Node{
			ID:   core.NewID(),
			Type: "Interface",
			Name: "ToolExecutor",
			Properties: map[string]any{
				"project_id": projectID.String(),
				"methods": []map[string]any{
					{
						"name":    "ExecuteTool",
						"returns": []string{"*core.Output", "error"},
						"parameters": []map[string]any{
							{"name": "ctx", "type": "context.Context"},
							{"name": "toolID", "type": "string"},
							{"name": "toolExecID", "type": "core.ID"},
							{"name": "input", "type": "*core.Input"},
							{"name": "env", "type": "core.EnvMap"},
						},
					},
				},
			},
		}

		// Create node - this should not fail with the Neo4j type error
		err := repository.CreateNode(ctx, interfaceNode)
		require.NoError(t, err, "Should handle interface with complex method definitions")

		// Retrieve and verify
		retrievedNode, err := repository.GetNode(ctx, interfaceNode.ID)
		require.NoError(t, err)

		// Methods should be serialized to JSON
		methodsJSON, ok := retrievedNode.Properties["methods"].(string)
		assert.True(t, ok, "Methods should be serialized to JSON string")

		var methods []map[string]any
		err = json.Unmarshal([]byte(methodsJSON), &methods)
		assert.NoError(t, err, "Should be able to unmarshal methods")
		assert.Len(t, methods, 1)
		assert.Equal(t, "ExecuteTool", methods[0]["name"])

		// Verify parameters were preserved
		params, ok := methods[0]["parameters"].([]any)
		assert.True(t, ok, "Parameters should be an array")
		assert.Len(t, params, 5)
	})

	t.Run("Should handle deeply nested structures", func(t *testing.T) {
		deepNode := &core.Node{
			ID:   core.NewID(),
			Type: "ComplexStruct",
			Name: "DeepNested",
			Properties: map[string]any{
				"project_id": projectID.String(),
				"config": map[string]any{
					"server": map[string]any{
						"endpoints": []map[string]any{
							{
								"path":    "/api/v1",
								"methods": []string{"GET", "POST"},
								"auth": map[string]any{
									"type":   "oauth2",
									"scopes": []string{"read", "write"},
									"providers": []map[string]any{
										{"name": "google", "client_id": "abc123"},
										{"name": "github", "client_id": "def456"},
									},
								},
							},
						},
					},
				},
			},
		}

		// Create node
		err := repository.CreateNode(ctx, deepNode)
		require.NoError(t, err, "Should handle deeply nested structures")

		// Retrieve and verify
		retrievedNode, err := repository.GetNode(ctx, deepNode.ID)
		require.NoError(t, err)

		// Config should be serialized
		configJSON, ok := retrievedNode.Properties["config"].(string)
		assert.True(t, ok, "Config should be serialized to JSON string")

		var config map[string]any
		err = json.Unmarshal([]byte(configJSON), &config)
		assert.NoError(t, err, "Should be able to unmarshal config")

		// Verify deep structure is preserved
		server := config["server"].(map[string]any)
		endpoints := server["endpoints"].([]any)
		endpoint := endpoints[0].(map[string]any)
		auth := endpoint["auth"].(map[string]any)
		providers := auth["providers"].([]any)

		assert.Len(t, providers, 2)
		provider1 := providers[0].(map[string]any)
		assert.Equal(t, "google", provider1["name"])
	})

	t.Run("Should handle mixed primitive and complex arrays", func(t *testing.T) {
		mixedNode := &core.Node{
			ID:   core.NewID(),
			Type: "MixedArrays",
			Name: "ArrayTest",
			Properties: map[string]any{
				"project_id": projectID.String(),
				// Primitive arrays - should NOT be serialized
				"tags":   []string{"go", "neo4j", "graph"},
				"scores": []int{100, 95, 87},
				"flags":  []bool{true, false, true},

				// Complex arrays - should be serialized
				"structs": []map[string]any{
					{"field": "value1", "nested": map[string]any{"key": "val"}},
					{"field": "value2", "nested": map[string]any{"key": "val2"}},
				},

				// Empty arrays
				"empty_strings": []string{},
				"empty_maps":    []map[string]any{},
			},
		}

		// Create node
		err := repository.CreateNode(ctx, mixedNode)
		require.NoError(t, err, "Should handle mixed arrays")

		// Retrieve and verify
		retrievedNode, err := repository.GetNode(ctx, mixedNode.ID)
		require.NoError(t, err)

		// Primitive arrays should remain as arrays
		tags, ok := retrievedNode.Properties["tags"].([]any)
		assert.True(t, ok, "Tags should remain as array, got type: %T", retrievedNode.Properties["tags"])
		assert.Len(t, tags, 3)

		scores, ok := retrievedNode.Properties["scores"].([]any)
		assert.True(t, ok, "Scores should remain as array")
		assert.Len(t, scores, 3)

		flags, ok := retrievedNode.Properties["flags"].([]any)
		assert.True(t, ok, "Flags should remain as array")
		assert.Len(t, flags, 3)

		// Complex arrays should be serialized
		structsJSON, ok := retrievedNode.Properties["structs"].(string)
		assert.True(t, ok, "Structs should be serialized to JSON string")

		var structs []map[string]any
		err = json.Unmarshal([]byte(structsJSON), &structs)
		assert.NoError(t, err, "Should be able to unmarshal structs")
		assert.Len(t, structs, 2)
		assert.Equal(t, "value1", structs[0]["field"])
	})

	t.Run("Should handle batch operations with deeply nested properties", func(t *testing.T) {
		nodes := []core.Node{
			{
				ID:   core.NewID(),
				Type: "BatchInterface",
				Name: "Interface1",
				Properties: map[string]any{
					"project_id": projectID.String(),
					"methods": []map[string]any{
						{
							"name": "Method1",
							"params": []map[string]any{
								{"name": "p1", "type": "string"},
								{"name": "p2", "type": "int"},
							},
						},
					},
				},
			},
			{
				ID:   core.NewID(),
				Type: "BatchInterface",
				Name: "Interface2",
				Properties: map[string]any{
					"project_id": projectID.String(),
					"methods": []map[string]any{
						{
							"name": "Method2",
							"params": []map[string]any{
								{"name": "ctx", "type": "context.Context"},
								{"name": "data", "type": "[]byte"},
							},
						},
					},
				},
			},
		}

		// Create nodes in batch
		err := repository.CreateNodes(ctx, nodes)
		require.NoError(t, err, "Batch operation should handle complex nested properties")

		// Verify both nodes
		for _, node := range nodes {
			retrievedNode, err := repository.GetNode(ctx, node.ID)
			require.NoError(t, err)

			methodsJSON, ok := retrievedNode.Properties["methods"].(string)
			assert.True(t, ok, "Methods should be serialized for node %s", node.Name)

			var methods []map[string]any
			err = json.Unmarshal([]byte(methodsJSON), &methods)
			assert.NoError(t, err)
			assert.Len(t, methods, 1)
		}
	})
}
