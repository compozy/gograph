package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/gograph/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLICommands tests critical CLI commands end-to-end
func TestCLICommands(t *testing.T) {
	// Skip if Neo4j is not available
	if !testhelpers.IsNeo4jAvailable() {
		t.Skip("Neo4j not available, skipping CLI integration test")
	}

	projectRoot := getProjectRoot()
	ctx := context.Background()

	// Start Neo4j container
	container, err := testhelpers.StartNeo4jContainer(ctx)
	require.NoError(t, err)
	defer container.Stop(ctx)

	// Build the CLI binary if needed
	gographBinary := buildCLIBinary(t, projectRoot)

	t.Run("Should execute init command", func(t *testing.T) {
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "test-config.yaml")

		cmd := exec.Command(
			gographBinary,
			"init",
			"--config",
			configFile,
			"--project-id",
			"cli-test-project",
			"--force",
		)
		output, err := cmd.CombinedOutput()

		assert.NoError(t, err, "init command should succeed: %s", string(output))
		assert.Contains(t, string(output), "Configuration file")

		// Verify config file was created
		_, err = os.Stat(configFile)
		assert.NoError(t, err, "config file should be created")
	})

	t.Run("Should execute analyze command", func(t *testing.T) {
		tempDir := t.TempDir()
		testProjectPath := filepath.Join(projectRoot, "testdata", "simple_project")

		// Set environment variables for Neo4j connection - no config file needed
		env := append(os.Environ(),
			"GOGRAPH_NEO4J_URI="+container.URI,
			"GOGRAPH_NEO4J_USERNAME="+container.Username,
			"GOGRAPH_NEO4J_PASSWORD="+container.Password)

		// Analyze the test project without config file (using env vars only)
		analyzeCmd := exec.Command(gographBinary, "analyze",
			"--no-progress",
			testProjectPath)
		analyzeCmd.Env = env
		analyzeCmd.Dir = tempDir // Run from temp dir so no config file is found

		output, err := analyzeCmd.CombinedOutput()
		assert.NoError(t, err, "analyze command should succeed: %s", string(output))
		assert.Contains(t, string(output), "analysis completed successfully")
	})

	t.Run("Should execute query command", func(t *testing.T) {
		tempDir := t.TempDir()

		// Set environment variables for Neo4j connection
		env := append(os.Environ(),
			"GOGRAPH_NEO4J_URI="+container.URI,
			"GOGRAPH_NEO4J_USERNAME="+container.Username,
			"GOGRAPH_NEO4J_PASSWORD="+container.Password)

		// Execute a simple query - without project-specific filter since we don't know the generated ID
		queryCmd := exec.Command(gographBinary, "query",
			"--no-progress",
			"MATCH (n) RETURN count(n) as total_nodes LIMIT 1")
		queryCmd.Env = env
		queryCmd.Dir = tempDir // Run from temp dir so no config file is found

		output, err := queryCmd.CombinedOutput()
		assert.NoError(t, err, "query command should succeed: %s", string(output))
		assert.Contains(t, string(output), "total_nodes")
	})

	t.Run("Should execute clear command", func(t *testing.T) {
		tempDir := t.TempDir()

		// Set environment variables for Neo4j connection
		env := append(os.Environ(),
			"GOGRAPH_NEO4J_URI="+container.URI,
			"GOGRAPH_NEO4J_USERNAME="+container.Username,
			"GOGRAPH_NEO4J_PASSWORD="+container.Password)

		// Clear all data with force flag (since we don't know the specific project ID)
		clearCmd := exec.Command(gographBinary, "clear",
			"--force")
		clearCmd.Env = env
		clearCmd.Dir = tempDir // Run from temp dir so no config file is found

		output, err := clearCmd.CombinedOutput()
		assert.NoError(t, err, "clear command should succeed: %s", string(output))
		// Check that the command ran without error (output may vary)
	})

	t.Run("Should execute version command", func(t *testing.T) {
		cmd := exec.Command(gographBinary, "version")
		output, err := cmd.CombinedOutput()

		assert.NoError(t, err, "version command should succeed")
		outputStr := string(output)
		assert.True(t,
			strings.Contains(outputStr, "gograph version") ||
				strings.Contains(outputStr, "dev") ||
				strings.Contains(outputStr, "unknown"),
			"version output should contain version info: %s", outputStr)
	})

	t.Run("Should execute help command", func(t *testing.T) {
		cmd := exec.Command(gographBinary, "help")
		output, err := cmd.CombinedOutput()

		assert.NoError(t, err, "help command should succeed")
		assert.Contains(t, string(output), "Available Commands")
		assert.Contains(t, string(output), "analyze")
		assert.Contains(t, string(output), "query")
		assert.Contains(t, string(output), "clear")
	})
}

// TestMCPServerCommand tests the MCP server CLI command
func TestMCPServerCommand(t *testing.T) {
	projectRoot := getProjectRoot()

	// Build the CLI binary if needed
	gographBinary := buildCLIBinary(t, projectRoot)

	t.Run("Should show MCP serve help", func(t *testing.T) {
		cmd := exec.Command(gographBinary, "serve-mcp", "--help")
		output, err := cmd.CombinedOutput()

		assert.NoError(t, err, "serve-mcp help should succeed")
		outputStr := string(output)
		assert.Contains(t, outputStr, "Start MCP server")
		assert.Contains(t, outputStr, "Model Context Protocol")
		assert.Contains(t, outputStr, "--port")
		assert.Contains(t, outputStr, "--auth")
	})

	t.Run("Should start MCP server briefly", func(t *testing.T) {
		// Test that the MCP server can start (we'll stop it quickly)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, gographBinary, "serve-mcp")
		err := cmd.Start()
		assert.NoError(t, err, "MCP server should start")

		// Let it run briefly then kill it
		time.Sleep(500 * time.Millisecond)
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		cmd.Wait()

		// The command should have started successfully (exit code depends on how it was killed)
	})
}

// buildCLIBinary builds the CLI binary for testing
func buildCLIBinary(t *testing.T, projectRoot string) string {
	t.Helper()

	binaryPath := filepath.Join(projectRoot, "bin", "gograph")

	// Check if binary already exists and is recent
	if stat, err := os.Stat(binaryPath); err == nil {
		// If binary is less than 5 minutes old, use it
		if time.Since(stat.ModTime()) < 5*time.Minute {
			return binaryPath
		}
	}

	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/gograph")
	buildCmd.Dir = projectRoot

	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "CLI binary build should succeed: %s", string(output))

	// Verify binary was created
	_, err = os.Stat(binaryPath)
	require.NoError(t, err, "CLI binary should be created")

	return binaryPath
}
