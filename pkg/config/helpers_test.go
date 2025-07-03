package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProjectIDFromPath(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gograph-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test config file
	configPath := filepath.Join(tempDir, "gograph.yaml")
	configContent := `
project:
  id: "test-project-123"
  name: "Test Project"
  root_path: "."
neo4j:
  uri: "bolt://localhost:7687"
  username: "neo4j"
  password: "password"
`
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Test getting project ID from the path
	projectID, err := GetProjectIDFromPath(tempDir)
	assert.NoError(t, err)
	assert.Equal(t, "test-project-123", projectID)

	// Test with non-existent path
	_, err = GetProjectIDFromPath("/non/existent/path")
	assert.Error(t, err)
}

func TestEnsureProjectID(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gograph-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test config file
	configPath := filepath.Join(tempDir, "gograph.yaml")
	configContent := `
project:
  id: "config-project-id"
  name: "Test Project"
  root_path: "."
neo4j:
  uri: "bolt://localhost:7687"
  username: "neo4j"
  password: "password"
`
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	tests := []struct {
		name        string
		providedID  string
		projectPath string
		expectedID  string
		expectError bool
	}{
		{
			name:        "Should use provided ID when given",
			providedID:  "provided-id",
			projectPath: tempDir,
			expectedID:  "provided-id",
			expectError: false,
		},
		{
			name:        "Should load from config when ID not provided",
			providedID:  "",
			projectPath: tempDir,
			expectedID:  "config-project-id",
			expectError: false,
		},
		{
			name:        "Should error when no ID and no config",
			providedID:  "",
			projectPath: "/non/existent/path",
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectID, err := EnsureProjectID(tt.providedID, tt.projectPath)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, projectID)
			}
		})
	}
}
