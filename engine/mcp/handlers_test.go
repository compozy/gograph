package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGenerateFallbackCypher(t *testing.T) {
	testCases := []struct {
		name              string
		naturalQuery      string
		projectID         string
		expectedQuery     string
		expectedParams    map[string]any
		shouldContainText []string
	}{
		{
			name:          "Should find test files with search terms",
			naturalQuery:  "show me all test files for http client",
			projectID:     "test-proj",
			expectedQuery: "MATCH (f:File {project_id: $project_id}) WHERE (f.path CONTAINS '_test.go' OR f.path CONTAINS '/test/') AND (toLower(f.path) CONTAINS $term0 OR toLower(f.name) CONTAINS $term0) AND (toLower(f.path) CONTAINS $term1 OR toLower(f.name) CONTAINS $term1) RETURN f.path, f.name LIMIT 20",
			expectedParams: map[string]any{
				"project_id": "test-proj",
				"term0":      "http",
				"term1":      "client",
			},
		},
		{
			name:          "Should find test files without extra terms",
			naturalQuery:  "show test files",
			projectID:     "test-proj",
			expectedQuery: "MATCH (f:File {project_id: $project_id}) WHERE (f.path CONTAINS '_test.go' OR f.path CONTAINS '/test/') RETURN f.path, f.name LIMIT 20",
			expectedParams: map[string]any{
				"project_id": "test-proj",
			},
		},
		{
			name:          "Should find functions with search terms",
			naturalQuery:  "list all functions in the parser package",
			projectID:     "test-proj",
			expectedQuery: "MATCH (f:Function {project_id: $project_id}) WHERE (toLower(f.name) CONTAINS $term0 OR toLower(f.package) CONTAINS $term0) AND (toLower(f.name) CONTAINS $term1 OR toLower(f.package) CONTAINS $term1) RETURN f.name, f.package, f.signature, f.file_path, f.line_start, f.is_exported LIMIT 20",
			expectedParams: map[string]any{
				"project_id": "test-proj",
				"term0":      "parser",
				"term1":      "package",
			},
		},
		{
			name:          "Should find all functions without filters",
			naturalQuery:  "show all functions",
			projectID:     "test-proj",
			expectedQuery: "MATCH (f:Function {project_id: $project_id}) RETURN f.name, f.package, f.signature, f.file_path, f.line_start, f.is_exported LIMIT 20",
			expectedParams: map[string]any{
				"project_id": "test-proj",
			},
		},
		{
			name:          "Should find packages with search terms",
			naturalQuery:  "find package with mcp handlers",
			projectID:     "test-proj",
			expectedQuery: "MATCH (p:Package {project_id: $project_id}) WHERE (toLower(p.name) CONTAINS $term0 OR toLower(p.path) CONTAINS $term0) AND (toLower(p.name) CONTAINS $term1 OR toLower(p.path) CONTAINS $term1) RETURN p.name, p.path LIMIT 20",
			expectedParams: map[string]any{
				"project_id": "test-proj",
				"term0":      "mcp",
				"term1":      "handlers",
			},
		},
		{
			name:          "Should find all packages",
			naturalQuery:  "list packages",
			projectID:     "test-proj",
			expectedQuery: "MATCH (p:Package {project_id: $project_id}) RETURN p.name, p.path LIMIT 20",
			expectedParams: map[string]any{
				"project_id": "test-proj",
			},
		},
		{
			name:          "Should find structs with search terms",
			naturalQuery:  "show me the user struct",
			projectID:     "test-proj",
			expectedQuery: "MATCH (s:Struct {project_id: $project_id}) WHERE (toLower(s.name) CONTAINS $term0 OR toLower(s.package) CONTAINS $term0) RETURN s.name, s.package LIMIT 20",
			expectedParams: map[string]any{
				"project_id": "test-proj",
				"term0":      "user",
			},
		},
		{
			name:          "Should find all types",
			naturalQuery:  "show all types",
			projectID:     "test-proj",
			expectedQuery: "MATCH (s:Struct {project_id: $project_id}) RETURN s.name, s.package LIMIT 20",
			expectedParams: map[string]any{
				"project_id": "test-proj",
			},
		},
		{
			name:          "Should find interfaces with search terms",
			naturalQuery:  "find interface for database adapter",
			projectID:     "test-proj",
			expectedQuery: "MATCH (i:Interface {project_id: $project_id}) WHERE (toLower(i.name) CONTAINS $term0 OR toLower(i.package) CONTAINS $term0) AND (toLower(i.name) CONTAINS $term1 OR toLower(i.package) CONTAINS $term1) RETURN i.name, i.package LIMIT 20",
			expectedParams: map[string]any{
				"project_id": "test-proj",
				"term0":      "database",
				"term1":      "adapter",
			},
		},
		{
			name:         "Should show project overview by default",
			naturalQuery: "what's in this project",
			projectID:    "test-proj",
			expectedQuery: "MATCH (f:Function {project_id: $project_id}) " +
				"RETURN f.name, f.package, f.signature, f.file_path, f.line_start, f.is_exported " +
				"ORDER BY f.package, f.name LIMIT 50",
			expectedParams: map[string]any{
				"project_id": "test-proj",
			},
		},
		{
			name:          "Should ignore stop words",
			naturalQuery:  "show me all the existing functions for the package",
			projectID:     "test-proj",
			expectedQuery: "MATCH (f:Function {project_id: $project_id}) WHERE (toLower(f.name) CONTAINS $term0 OR toLower(f.package) CONTAINS $term0) RETURN f.name, f.package, f.signature, f.file_path, f.line_start, f.is_exported LIMIT 20",
			expectedParams: map[string]any{
				"project_id": "test-proj",
				"term0":      "package",
			},
		},
		{
			name:          "Should ignore short words",
			naturalQuery:  "find function by id",
			projectID:     "test-proj",
			expectedQuery: "MATCH (f:Function {project_id: $project_id}) RETURN f.name, f.package, f.signature, f.file_path, f.line_start, f.is_exported LIMIT 20",
			expectedParams: map[string]any{
				"project_id": "test-proj",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query, params := GenerateFallbackCypher(tc.naturalQuery, tc.projectID)

			// Check query structure
			assert.Equal(t, tc.expectedQuery, query)

			// Check parameters
			require.Equal(t, len(tc.expectedParams), len(params), "Parameter count mismatch")
			for key, expectedValue := range tc.expectedParams {
				actualValue, exists := params[key]
				require.True(t, exists, "Missing parameter: %s", key)
				assert.Equal(t, expectedValue, actualValue, "Parameter value mismatch for %s", key)
			}

			// Additional content checks if specified
			for _, text := range tc.shouldContainText {
				assert.Contains(t, query, text)
			}
		})
	}
}

func TestGenerateFallbackCypher_SecuritySafety(t *testing.T) {
	// Test that potentially malicious input is safely parameterized
	maliciousQueries := []struct {
		name         string
		naturalQuery string
		projectID    string
	}{
		{
			name:         "Should handle injection attempt in search terms",
			naturalQuery: "find function named '); DROP DATABASE; --",
			projectID:    "test-proj",
		},
		{
			name:         "Should handle quotes in search terms",
			naturalQuery: "find function with name 'test' or name='admin'",
			projectID:    "test-proj",
		},
		{
			name:         "Should handle special characters",
			naturalQuery: "find package $1 OR 1=1",
			projectID:    "test-proj",
		},
	}

	for _, tc := range maliciousQueries {
		t.Run(tc.name, func(t *testing.T) {
			query, params := GenerateFallbackCypher(tc.naturalQuery, tc.projectID)

			// Ensure query uses parameters
			assert.Contains(t, query, "$project_id")
			assert.Contains(t, query, "project_id: $project_id")

			// Ensure no direct string interpolation of user input
			assert.NotContains(t, query, "DROP")
			assert.NotContains(t, query, "--")
			assert.NotContains(t, query, "1=1")

			// Ensure project_id is parameterized
			assert.Equal(t, tc.projectID, params["project_id"])
		})
	}
}

func TestHandleGetDatabaseSchemaInternal(t *testing.T) {
	// Create test server with mock adapter
	mockAdapter := new(MockServiceAdapter)

	// Setup mock responses
	mockAdapter.On("ExecuteQuery",
		mock.Anything,
		mock.MatchedBy(func(query string) bool {
			return strings.Contains(query, "labels(n) as labels")
		}),
		mock.Anything,
	).Return([]map[string]any{
		{
			"label": "Function",
			"property_sets": []any{
				[]any{"name", "package", "project_id", "signature"},
				[]any{"name", "package", "project_id", "is_exported"},
			},
		},
		{
			"label": "Package",
			"property_sets": []any{
				[]any{"name", "path", "project_id"},
			},
		},
	}, nil)

	mockAdapter.On("ExecuteQuery",
		mock.Anything,
		mock.MatchedBy(func(query string) bool {
			return strings.Contains(query, "type(r) as relationship_type")
		}),
		mock.Anything,
	).Return([]map[string]any{
		{
			"relationship_type": "CALLS",
			"count":             int64(42),
			"source_labels":     []any{[]any{"Function"}},
			"target_labels":     []any{[]any{"Function"}},
		},
		{
			"relationship_type": "CONTAINS",
			"count":             int64(100),
			"source_labels":     []any{[]any{"Package"}},
			"target_labels":     []any{[]any{"File", "Function"}},
		},
	}, nil)

	server := &Server{
		serviceAdapter: mockAdapter,
	}

	t.Run("Should get schema without examples", func(t *testing.T) {
		response, err := server.HandleGetDatabaseSchemaInternal(context.Background(), map[string]any{
			"project_id":       "test-project",
			"include_examples": false,
		})

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Len(t, response.Content, 2)

		// Check resource data
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		// Verify node types
		nodeTypes := data["node_types"].([]map[string]any)
		assert.Len(t, nodeTypes, 2)

		// Find Function node type
		var functionNode map[string]any
		for _, nt := range nodeTypes {
			if nt["label"] == "Function" {
				functionNode = nt
				break
			}
		}
		require.NotNil(t, functionNode)
		props := functionNode["properties"].([]string)
		assert.Contains(t, props, "name")
		assert.Contains(t, props, "package")
		assert.Contains(t, props, "project_id")

		// Verify relationship types
		relTypes := data["relationship_types"].([]map[string]any)
		assert.Len(t, relTypes, 2)
		assert.Equal(t, "CALLS", relTypes[0]["type"])
		assert.Equal(t, int64(42), relTypes[0]["count"])

		// Verify common patterns exist
		patterns := data["common_patterns"].(map[string]any)
		assert.NotNil(t, patterns["common_mistakes"])
	})

	t.Run("Should get schema with examples", func(t *testing.T) {
		response, err := server.HandleGetDatabaseSchemaInternal(context.Background(), map[string]any{
			"project_id":       "test-project",
			"include_examples": true,
		})

		require.NoError(t, err)
		require.NotNil(t, response)

		// Check resource data
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		// Verify examples are included
		examples := data["examples"].(map[string]any)
		assert.NotNil(t, examples["find_functions"])
		assert.NotNil(t, examples["function_calls"])
		assert.NotNil(t, examples["package_dependencies"])
		assert.NotNil(t, examples["interface_implementations"])
	})

	t.Run("Should filter by type", func(t *testing.T) {
		response, err := server.HandleGetDatabaseSchemaInternal(context.Background(), map[string]any{
			"project_id":  "test-project",
			"filter_type": "function",
		})

		require.NoError(t, err)

		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		// Should only include Function node type (case-insensitive filter)
		nodeTypes := data["node_types"].([]map[string]any)
		assert.Len(t, nodeTypes, 1)
		assert.Equal(t, "Function", nodeTypes[0]["label"])

		// Should not include CONTAINS relationship (doesn't contain "function")
		relTypes := data["relationship_types"].([]map[string]any)
		// CALLS doesn't contain "function" either, so should be empty
		assert.Len(t, relTypes, 0)
	})
}

func TestHandleValidateCypherQueryInternal(t *testing.T) {
	// Create test server with mock adapter
	mockAdapter := new(MockServiceAdapter)

	// Setup mock for valid query
	mockAdapter.On("ExecuteQuery",
		mock.Anything,
		"EXPLAIN MATCH (f:Function {project_id: $project_id}) WHERE f.name CONTAINS 'test' RETURN f",
		mock.Anything,
	).Return([]map[string]any{}, nil)

	// Setup mock for LIKE error
	mockAdapter.On("ExecuteQuery",
		mock.Anything,
		"EXPLAIN MATCH (f:Function) WHERE f.name LIKE '%test%' RETURN f",
		mock.Anything,
	).Return(nil, fmt.Errorf("Invalid input 'LIKE': expected CONTAINS"))

	// Setup mock for missing project_id
	mockAdapter.On("ExecuteQuery",
		mock.Anything,
		"EXPLAIN MATCH (f:Function) WHERE f.name = 'test' RETURN f",
		mock.Anything,
	).Return(nil, fmt.Errorf("undefined variable project_id"))

	server := &Server{
		serviceAdapter: mockAdapter,
	}

	t.Run("Should validate correct query", func(t *testing.T) {
		response, err := server.HandleValidateCypherQueryInternal(context.Background(), map[string]any{
			"query":      "MATCH (f:Function {project_id: $project_id}) WHERE f.name CONTAINS 'test' RETURN f",
			"project_id": "test-project",
		})

		require.NoError(t, err)
		require.NotNil(t, response)

		// Check validation result
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		assert.True(t, data["is_valid"].(bool))
		assert.Equal(t, "Query syntax is valid", data["message"])
	})

	t.Run("Should detect LIKE usage", func(t *testing.T) {
		response, err := server.HandleValidateCypherQueryInternal(context.Background(), map[string]any{
			"query":      "MATCH (f:Function) WHERE f.name LIKE '%test%' RETURN f",
			"project_id": "test-project",
		})

		require.NoError(t, err)
		require.NotNil(t, response)

		// Check validation result
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		assert.False(t, data["is_valid"].(bool))
		suggestions := data["suggestions"].([]string)
		assert.Contains(t, suggestions, "Use CONTAINS instead of LIKE for substring matching")
		assert.Contains(t, suggestions, "Add project_id filter: {project_id: $project_id}")
	})

	t.Run("Should detect missing project_id", func(t *testing.T) {
		response, err := server.HandleValidateCypherQueryInternal(context.Background(), map[string]any{
			"query":      "MATCH (f:Function) WHERE f.name = 'test' RETURN f",
			"project_id": "test-project",
		})

		require.NoError(t, err)
		require.NotNil(t, response)

		// Check validation result
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		assert.False(t, data["is_valid"].(bool))
		suggestions := data["suggestions"].([]string)
		assert.Contains(t, suggestions, "Add project_id filter: {project_id: $project_id}")
	})
}

func TestFormatSchemaForLLM(t *testing.T) {
	server := &Server{}

	schema := map[string]any{
		"node_types": []map[string]any{
			{
				"label":      "Function",
				"properties": []string{"name", "package", "signature"},
			},
			{
				"label":      "Package",
				"properties": []string{"name", "path"},
			},
		},
		"relationship_types": []map[string]any{
			{
				"type":  "CALLS",
				"count": 42,
			},
		},
		"common_patterns": map[string]any{
			"common_mistakes": []string{
				"Always include project_id filter",
				"Use CONTAINS not LIKE",
			},
		},
	}

	formatted := server.formatSchemaForLLM(schema)

	// Verify formatted output contains expected sections
	assert.Contains(t, formatted, "Neo4j Database Schema:")
	assert.Contains(t, formatted, "Node Types:")
	assert.Contains(t, formatted, "- Function (properties: name, package, signature)")
	assert.Contains(t, formatted, "- Package (properties: name, path)")
	assert.Contains(t, formatted, "Relationship Types:")
	assert.Contains(t, formatted, "- CALLS (42 occurrences)")
	assert.Contains(t, formatted, "Common Query Patterns:")
	assert.Contains(t, formatted, "- Always include project_id filter")
	assert.Contains(t, formatted, "- Use CONTAINS not LIKE")
}

func TestHandleNaturalLanguageQueryInternal_WithSchemaIntegration(t *testing.T) {
	// Create test server with mock adapter
	mockAdapter := new(MockServiceAdapter)

	// Mock schema query responses
	mockAdapter.On("ExecuteQuery",
		mock.Anything,
		mock.MatchedBy(func(query string) bool {
			return strings.Contains(query, "labels(n) as labels")
		}),
		mock.Anything,
	).Return([]map[string]any{
		{
			"label": "Function",
			"property_sets": []any{
				[]any{"name", "package", "project_id"},
			},
		},
	}, nil)

	mockAdapter.On("ExecuteQuery",
		mock.Anything,
		mock.MatchedBy(func(query string) bool {
			return strings.Contains(query, "type(r) as relationship_type")
		}),
		mock.Anything,
	).Return([]map[string]any{
		{
			"relationship_type": "CALLS",
			"count":             int64(10),
			"source_labels":     []any{[]any{"Function"}},
			"target_labels":     []any{[]any{"Function"}},
		},
	}, nil)

	// Mock successful query execution
	mockAdapter.On("ExecuteQuery",
		mock.Anything,
		mock.MatchedBy(func(query string) bool {
			return strings.Contains(query, "Function") && !strings.Contains(query, "EXPLAIN")
		}),
		mock.Anything,
	).Return([]map[string]any{
		{"name": "TestFunction", "package": "test"},
	}, nil).Once()

	// Mock validation query
	mockAdapter.On("ExecuteQuery",
		mock.Anything,
		mock.MatchedBy(func(query string) bool {
			return strings.HasPrefix(query, "EXPLAIN")
		}),
		mock.Anything,
	).Return([]map[string]any{}, nil)

	server := &Server{
		serviceAdapter: mockAdapter,
	}

	t.Run("Should execute with schema and validation", func(t *testing.T) {
		response, err := server.HandleNaturalLanguageQueryInternal(context.Background(), map[string]any{
			"query":      "find all test functions",
			"project_id": "test-project",
		})

		require.NoError(t, err)
		require.NotNil(t, response)

		// Check resource data
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		// Verify results
		assert.Equal(t, "find all test functions", data["natural_query"])
		assert.NotNil(t, data["cypher_query"])
		assert.Equal(t, 1, data["result_count"])

		// Verify all expected calls were made
		mockAdapter.AssertExpectations(t)
	})

	t.Run("Should provide enhanced error info on query failure", func(t *testing.T) {
		// Create new mock for error scenario
		errorMockAdapter := new(MockServiceAdapter)

		// Mock schema queries (same as above)
		errorMockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(query, "labels(n) as labels")
			}),
			mock.Anything,
		).Return([]map[string]any{
			{
				"label": "Function",
				"property_sets": []any{
					[]any{"name", "package", "project_id"},
				},
			},
		}, nil)

		errorMockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(query, "type(r) as relationship_type")
			}),
			mock.Anything,
		).Return([]map[string]any{
			{
				"relationship_type": "CALLS",
				"count":             int64(10),
				"source_labels":     []any{[]any{"Function"}},
				"target_labels":     []any{[]any{"Function"}},
			},
		}, nil)

		// Mock validation failure
		errorMockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.HasPrefix(query, "EXPLAIN")
			}),
			mock.Anything,
		).Return(nil, fmt.Errorf("Invalid syntax"))

		// Mock query execution failure
		errorMockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return !strings.HasPrefix(query, "EXPLAIN") && !strings.Contains(query, "labels(n)")
			}),
			mock.Anything,
		).Return(nil, fmt.Errorf("Query execution failed"))

		errorServer := &Server{
			serviceAdapter: errorMockAdapter,
		}

		response, err := errorServer.HandleNaturalLanguageQueryInternal(context.Background(), map[string]any{
			"query":      "find invalid nodes",
			"project_id": "test-project",
		})

		require.NoError(t, err) // Handler returns error info in response, not as error
		require.NotNil(t, response)

		// Check error resource data
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		errorData := resource["data"].(map[string]any)

		// Verify enhanced error information
		assert.Equal(t, "Query execution failed", errorData["original_error"])
		assert.NotNil(t, errorData["generated_query"])
		assert.True(t, errorData["validation_performed"].(bool))
		assert.False(t, errorData["query_was_valid"].(bool))
		assert.NotNil(t, errorData["suggestions"])
		assert.True(t, errorData["schema_available"].(bool))
		assert.NotNil(t, errorData["available_node_types"])
		assert.NotNil(t, errorData["available_relationships"])
	})
}
