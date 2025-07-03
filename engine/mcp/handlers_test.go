package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
