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

func TestHandleGetPackageStructureInternal_PackageNameAndImportPath(t *testing.T) {
	mockAdapter := new(MockServiceAdapter)
	server := &Server{
		serviceAdapter: mockAdapter,
	}

	// Mock response data for both package name and import path scenarios
	mockPackageResult := []map[string]any{
		{
			"pkg": map[string]any{
				"name":        "mcp",
				"import_path": "github.com/compozy/gograph/engine/mcp",
				"path":        "/engine/mcp",
				"project_id":  "test-project",
			},
			"files": []any{
				map[string]any{
					"name": "handlers.go",
					"path": "/engine/mcp/handlers.go",
				},
				map[string]any{
					"name": "server.go",
					"path": "/engine/mcp/server.go",
				},
			},
			"functions": []any{
				map[string]any{
					"name":        "HandleGetPackageStructure",
					"signature":   "func HandleGetPackageStructure(ctx context.Context, input map[string]any) (*ToolResponse, error)",
					"is_exported": true,
				},
			},
			"structs": []any{
				map[string]any{
					"name":        "Server",
					"is_exported": true,
				},
			},
			"interfaces": []any{},
		},
	}

	t.Run("Should find package by package name only", func(t *testing.T) {
		// Mock the query execution for package name search
		mockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(query, "WHERE pkg.import_path = $package OR pkg.name = $package")
			}),
			mock.MatchedBy(func(params map[string]any) bool {
				return params["project_id"] == "test-project" && params["package"] == "mcp"
			}),
		).Return(mockPackageResult, nil).Once()

		response, err := server.HandleGetPackageStructureInternal(context.Background(), map[string]any{
			"project_id":      "test-project",
			"package":         "mcp",
			"include_private": true,
		})

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Len(t, response.Content, 2)

		// Check resource structure
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		// Verify package name
		assert.Equal(t, "mcp", data["package"])

		// Verify files
		files := data["files"].([]any)
		assert.Len(t, files, 2)
		firstFile := files[0].(map[string]any)
		assert.Equal(t, "handlers.go", firstFile["name"])

		// Verify functions
		functions := data["functions"].([]any)
		assert.Len(t, functions, 1)
		firstFunction := functions[0].(map[string]any)
		assert.Equal(t, "HandleGetPackageStructure", firstFunction["name"])
		assert.True(t, firstFunction["is_exported"].(bool))

		mockAdapter.AssertExpectations(t)
	})

	t.Run("Should find package by full import path", func(t *testing.T) {
		// Mock the query execution for import path search
		mockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(query, "WHERE pkg.import_path = $package OR pkg.name = $package")
			}),
			mock.MatchedBy(func(params map[string]any) bool {
				return params["project_id"] == "test-project" &&
					params["package"] == "github.com/compozy/gograph/engine/mcp"
			}),
		).Return(mockPackageResult, nil).Once()

		response, err := server.HandleGetPackageStructureInternal(context.Background(), map[string]any{
			"project_id":      "test-project",
			"package":         "github.com/compozy/gograph/engine/mcp",
			"include_private": true,
		})

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Len(t, response.Content, 2)

		// Check resource structure
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		// Verify package name matches
		assert.Equal(t, "github.com/compozy/gograph/engine/mcp", data["package"])

		mockAdapter.AssertExpectations(t)
	})

	t.Run("Should filter private members when include_private is false", func(t *testing.T) {
		// Mock result with both public and private functions
		mockResultWithPrivate := []map[string]any{
			{
				"pkg": map[string]any{
					"name":        "mcp",
					"import_path": "github.com/compozy/gograph/engine/mcp",
					"path":        "/engine/mcp",
					"project_id":  "test-project",
				},
				"files": []any{
					map[string]any{
						"name": "handlers.go",
						"path": "/engine/mcp/handlers.go",
					},
				},
				"functions": []any{
					map[string]any{
						"name":        "HandleGetPackageStructure",
						"signature":   "func HandleGetPackageStructure(ctx context.Context, input map[string]any) (*ToolResponse, error)",
						"is_exported": true,
						"file_path":   "/engine/mcp/handlers.go",
					},
					map[string]any{
						"name":        "privateHelper",
						"signature":   "func privateHelper() error",
						"is_exported": false,
						"file_path":   "/engine/mcp/handlers.go",
					},
				},
				"structs": []any{
					map[string]any{
						"name":        "Server",
						"is_exported": true,
						"file_path":   "/engine/mcp/server.go",
					},
					map[string]any{
						"name":        "internalState",
						"is_exported": false,
						"file_path":   "/engine/mcp/server.go",
					},
				},
				"interfaces": []any{},
			},
		}

		mockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(query, "WHERE pkg.import_path = $package OR pkg.name = $package")
			}),
			mock.MatchedBy(func(params map[string]any) bool {
				return params["project_id"] == "test-project" && params["package"] == "mcp"
			}),
		).Return(mockResultWithPrivate, nil).Once()

		response, err := server.HandleGetPackageStructureInternal(context.Background(), map[string]any{
			"project_id":      "test-project",
			"package":         "mcp",
			"include_private": false,
		})

		require.NoError(t, err)
		require.NotNil(t, response)

		// Check resource structure
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		// Verify only exported functions are included
		functions := data["functions"].([]any)
		assert.Len(t, functions, 1)
		firstFunction := functions[0].(map[string]any)
		assert.Equal(t, "HandleGetPackageStructure", firstFunction["name"])
		assert.True(t, firstFunction["is_exported"].(bool))

		// Verify only exported structs are included
		types := data["types"].([]any)
		assert.Len(t, types, 1)
		firstType := types[0].(map[string]any)
		assert.Equal(t, "Server", firstType["name"])
		assert.True(t, firstType["is_exported"].(bool))

		mockAdapter.AssertExpectations(t)
	})

	t.Run("Should return not found message for nonexistent package", func(t *testing.T) {
		// Mock empty result for nonexistent package
		mockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(query, "WHERE pkg.import_path = $package OR pkg.name = $package")
			}),
			mock.MatchedBy(func(params map[string]any) bool {
				return params["project_id"] == "test-project" && params["package"] == "nonexistent"
			}),
		).Return([]map[string]any{}, nil).Once()

		response, err := server.HandleGetPackageStructureInternal(context.Background(), map[string]any{
			"project_id": "test-project",
			"package":    "nonexistent",
		})

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Len(t, response.Content, 1)

		// Check that it returns a "not found" message
		textContent := response.Content[0].(map[string]any)
		assert.Equal(t, "text", textContent["type"])
		assert.Contains(t, textContent["text"], "Package nonexistent not found")

		mockAdapter.AssertExpectations(t)
	})
}

func TestHandleVerifyCodeExistsInternal_PackageNameAndImportPath(t *testing.T) {
	mockAdapter := new(MockServiceAdapter)
	server := &Server{
		serviceAdapter: mockAdapter,
	}

	t.Run("Should verify package existence by package name", func(t *testing.T) {
		// Mock result for package verification by name
		mockPackageExistsResult := []map[string]any{
			{
				"p": map[string]any{
					"name":        "mcp",
					"import_path": "github.com/compozy/gograph/engine/mcp",
					"path":        "/engine/mcp",
					"project_id":  "test-project",
				},
				"has_file": true,
			},
		}

		mockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(
					query,
					"MATCH (p:Package {project_id: $project_id}) WHERE p.import_path = $name OR p.name = $name",
				)
			}),
			mock.MatchedBy(func(params map[string]any) bool {
				return params["project_id"] == "test-project" && params["name"] == "mcp"
			}),
		).Return(mockPackageExistsResult, nil).Once()

		response, err := server.HandleVerifyCodeExistsInternal(context.Background(), map[string]any{
			"project_id":   "test-project",
			"element_type": "package",
			"name":         "mcp",
		})

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Len(t, response.Content, 2)

		// Check resource structure
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		assert.True(t, data["exists"].(bool))
		assert.Equal(t, "package", data["element_type"])
		assert.Equal(t, "mcp", data["name"])

		// Verify details are included
		details, ok := data["details"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "mcp", details["name"])
		assert.Equal(t, "github.com/compozy/gograph/engine/mcp", details["import_path"])

		mockAdapter.AssertExpectations(t)
	})

	t.Run("Should verify package existence by full import path", func(t *testing.T) {
		// Mock result for package verification by import path
		mockPackageExistsResult := []map[string]any{
			{
				"p": map[string]any{
					"name":        "mcp",
					"import_path": "github.com/compozy/gograph/engine/mcp",
					"path":        "/engine/mcp",
					"project_id":  "test-project",
				},
				"has_file": true,
			},
		}

		mockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(
					query,
					"MATCH (p:Package {project_id: $project_id}) WHERE p.import_path = $name OR p.name = $name",
				)
			}),
			mock.MatchedBy(func(params map[string]any) bool {
				return params["project_id"] == "test-project" &&
					params["name"] == "github.com/compozy/gograph/engine/mcp"
			}),
		).Return(mockPackageExistsResult, nil).Once()

		response, err := server.HandleVerifyCodeExistsInternal(context.Background(), map[string]any{
			"project_id":   "test-project",
			"element_type": "package",
			"name":         "github.com/compozy/gograph/engine/mcp",
		})

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Len(t, response.Content, 2)

		// Check resource structure
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		assert.True(t, data["exists"].(bool))
		assert.Equal(t, "package", data["element_type"])
		assert.Equal(t, "github.com/compozy/gograph/engine/mcp", data["name"])

		// Verify details are included
		details, ok := data["details"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "mcp", details["name"])
		assert.Equal(t, "github.com/compozy/gograph/engine/mcp", details["import_path"])

		mockAdapter.AssertExpectations(t)
	})

	t.Run("Should verify function existence with package context", func(t *testing.T) {
		// Mock result for function verification
		mockFunctionExistsResult := []map[string]any{
			{
				"f": map[string]any{
					"name":        "HandleGetPackageStructure",
					"package":     "mcp",
					"signature":   "func HandleGetPackageStructure(ctx context.Context, input map[string]any) (*ToolResponse, error)",
					"is_exported": true,
					"file_path":   "/engine/mcp/handlers.go",
					"line_start":  int64(891),
					"line_end":    int64(995),
				},
				"has_file": true,
			},
		}

		mockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(
					query,
					"MATCH (f:Function {project_id: $project_id, name: $name}) WHERE f.package = $package",
				)
			}),
			mock.MatchedBy(func(params map[string]any) bool {
				return params["project_id"] == "test-project" &&
					params["name"] == "HandleGetPackageStructure" &&
					params["package"] == "mcp"
			}),
		).Return(mockFunctionExistsResult, nil).Once()

		response, err := server.HandleVerifyCodeExistsInternal(context.Background(), map[string]any{
			"project_id":   "test-project",
			"element_type": "function",
			"name":         "HandleGetPackageStructure",
			"package":      "mcp",
		})

		require.NoError(t, err)
		require.NotNil(t, response)

		// Check resource structure
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		assert.True(t, data["exists"].(bool))
		assert.Equal(t, "function", data["element_type"])
		assert.Equal(t, "HandleGetPackageStructure", data["name"])
		assert.Equal(t, "mcp", data["package"])

		// Verify function details are included
		details, ok := data["details"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "HandleGetPackageStructure", details["name"])
		assert.Equal(t, "mcp", details["package"])
		assert.True(t, details["is_exported"].(bool))

		mockAdapter.AssertExpectations(t)
	})

	t.Run("Should return false for nonexistent package", func(t *testing.T) {
		// Mock empty result for nonexistent package
		mockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(
					query,
					"MATCH (p:Package {project_id: $project_id}) WHERE p.import_path = $name OR p.name = $name",
				)
			}),
			mock.MatchedBy(func(params map[string]any) bool {
				return params["project_id"] == "test-project" && params["name"] == "nonexistent"
			}),
		).Return([]map[string]any{}, nil).Once()

		response, err := server.HandleVerifyCodeExistsInternal(context.Background(), map[string]any{
			"project_id":   "test-project",
			"element_type": "package",
			"name":         "nonexistent",
		})

		require.NoError(t, err)
		require.NotNil(t, response)

		// Check resource structure
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		assert.False(t, data["exists"].(bool))
		assert.Equal(t, "package", data["element_type"])
		assert.Equal(t, "nonexistent", data["name"])

		mockAdapter.AssertExpectations(t)
	})

	t.Run("Should return false for nonexistent function", func(t *testing.T) {
		// Mock empty result for nonexistent function
		mockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(query, "MATCH (f:Function {project_id: $project_id, name: $name}) RETURN f")
			}),
			mock.MatchedBy(func(params map[string]any) bool {
				return params["project_id"] == "test-project" && params["name"] == "NonexistentFunction"
			}),
		).Return([]map[string]any{}, nil).Once()

		response, err := server.HandleVerifyCodeExistsInternal(context.Background(), map[string]any{
			"project_id":   "test-project",
			"element_type": "function",
			"name":         "NonexistentFunction",
		})

		require.NoError(t, err)
		require.NotNil(t, response)

		// Check resource structure
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		assert.False(t, data["exists"].(bool))
		assert.Equal(t, "function", data["element_type"])
		assert.Equal(t, "NonexistentFunction", data["name"])

		mockAdapter.AssertExpectations(t)
	})

	t.Run("Should handle empty package name parameter", func(t *testing.T) {
		// Mock empty result for empty package name
		mockAdapter.On("ExecuteQuery",
			mock.Anything,
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(
					query,
					"MATCH (p:Package {project_id: $project_id}) WHERE p.import_path = $name OR p.name = $name",
				)
			}),
			mock.MatchedBy(func(params map[string]any) bool {
				return params["project_id"] == "test-project" && params["name"] == ""
			}),
		).Return([]map[string]any{}, nil).Once()

		response, err := server.HandleVerifyCodeExistsInternal(context.Background(), map[string]any{
			"project_id":   "test-project",
			"element_type": "package",
			"name":         "",
		})

		require.NoError(t, err)
		require.NotNil(t, response)

		// Check resource structure
		resource := response.Content[1].(map[string]any)["resource"].(map[string]any)
		data := resource["data"].(map[string]any)

		assert.False(t, data["exists"].(bool))
		assert.Equal(t, "package", data["element_type"])
		assert.Equal(t, "", data["name"])

		mockAdapter.AssertExpectations(t)
	})

	t.Run("Should handle invalid element type", func(t *testing.T) {
		response, err := server.HandleVerifyCodeExistsInternal(context.Background(), map[string]any{
			"project_id":   "test-project",
			"element_type": "invalid_type",
			"name":         "test",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported element type")
		assert.Nil(t, response)
	})
}
