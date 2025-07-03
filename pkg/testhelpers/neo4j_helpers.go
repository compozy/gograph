package testhelpers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/compozy/gograph/engine/infra"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/config"
)

// Neo4jTestContainer holds the connection details for docker-compose Neo4j
type Neo4jTestContainer struct {
	URI      string
	Username string
	Password string
}

var (
	// Default test connection details matching docker-compose.yml
	defaultTestURI      = "bolt://localhost:7687"
	defaultTestUsername = "neo4j"
	defaultTestPassword = "password"
)

// StartNeo4jContainer ensures Neo4j is running via docker-compose or uses existing service in CI
func StartNeo4jContainer(ctx context.Context) (*Neo4jTestContainer, error) {
	container := &Neo4jTestContainer{
		URI:      getEnvOrDefault("NEO4J_TEST_URI", defaultTestURI),
		Username: getEnvOrDefault("NEO4J_TEST_USERNAME", defaultTestUsername),
		Password: getEnvOrDefault("NEO4J_TEST_PASSWORD", defaultTestPassword),
	}

	// In CI environments, Neo4j should already be running as a service
	if os.Getenv("CI") == "true" {
		// Just verify connection in CI
		if err := waitForNeo4j(ctx, container); err != nil {
			return nil, fmt.Errorf("Neo4j service not ready in CI: %w", err)
		}
		return container, nil
	}

	// For local development, manage docker-compose
	if err := checkDockerComposeStatus(); err != nil {
		// Start docker-compose if not running
		if err := startDockerCompose(); err != nil {
			return nil, fmt.Errorf("failed to start docker-compose: %w", err)
		}
	}

	// Wait for Neo4j to be ready
	if err := waitForNeo4j(ctx, container); err != nil {
		return nil, fmt.Errorf("Neo4j did not become ready: %w", err)
	}

	return container, nil
}

// Stop is a no-op since docker-compose keeps running between tests
func (tc *Neo4jTestContainer) Stop(_ context.Context) error {
	// We don't stop docker-compose between tests for speed
	// Use 'make test-cleanup' to stop it manually
	return nil
}

// CreateDriver creates a new Neo4j driver for the test container
func (tc *Neo4jTestContainer) CreateDriver() (neo4j.DriverWithContext, error) {
	return neo4j.NewDriverWithContext(
		tc.URI,
		neo4j.BasicAuth(tc.Username, tc.Password, ""),
		func(c *config.Config) {
			c.MaxConnectionPoolSize = 25
			c.MaxConnectionLifetime = 5 * time.Minute
		},
	)
}

// CreateRepository creates a new Neo4j repository using the test container
func (tc *Neo4jTestContainer) CreateRepository() (*infra.Neo4jRepository, error) {
	config := &infra.Neo4jConfig{
		URI:        tc.URI,
		Username:   tc.Username,
		Password:   tc.Password,
		Database:   "",
		MaxRetries: 3,
		BatchSize:  1000,
	}

	repo, err := infra.NewNeo4jRepository(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	// Type assert to get the concrete type for tests
	neo4jRepo, ok := repo.(*infra.Neo4jRepository)
	if !ok {
		return nil, fmt.Errorf("repository is not a Neo4jRepository")
	}

	return neo4jRepo, nil
}

// SetupNeo4jTest is a convenience function for setting up Neo4j in tests
// It ensures Neo4j is running and returns a cleanup function
func SetupNeo4jTest(t *testing.T) (*Neo4jTestContainer, func()) {
	t.Helper()

	ctx := context.Background()
	container, err := StartNeo4jContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to start Neo4j container: %v", err)
	}

	// Clear database before test
	if err := container.ClearDatabase(ctx); err != nil {
		t.Fatalf("Failed to clear database: %v", err)
	}

	cleanup := func() {
		// Clear database after test
		if err := container.ClearDatabase(ctx); err != nil {
			t.Errorf("Failed to clear database after test: %v", err)
		}
	}

	return container, cleanup
}

// ClearDatabase removes all nodes and relationships from the database
func (tc *Neo4jTestContainer) ClearDatabase(ctx context.Context) error {
	driver, err := tc.CreateDriver()
	if err != nil {
		return fmt.Errorf("failed to create driver: %w", err)
	}
	defer driver.Close(ctx)

	session := driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeWrite,
	})
	defer session.Close(ctx)

	// Delete all nodes and relationships with a retry mechanism to handle deadlocks
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		_, err = session.Run(ctx, "MATCH (n) DETACH DELETE n", nil)
		if err == nil {
			return nil
		}

		// Check if it's a deadlock error and retry
		if i < maxRetries-1 && isDeadlockError(err) {
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond) // Exponential backoff
			continue
		}

		return err
	}

	return err
}

// ClearDatabaseForProject removes all nodes and relationships for a specific project
func (tc *Neo4jTestContainer) ClearDatabaseForProject(ctx context.Context, projectID string) error {
	driver, err := tc.CreateDriver()
	if err != nil {
		return fmt.Errorf("failed to create driver: %w", err)
	}
	defer driver.Close(ctx)

	session := driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeWrite,
	})
	defer session.Close(ctx)

	// Delete only nodes and relationships for this specific project
	_, err = session.Run(ctx, "MATCH (n {project_id: $project_id}) DETACH DELETE n", map[string]any{
		"project_id": projectID,
	})
	return err
}

// isDeadlockError checks if the error is a Neo4j deadlock error
func isDeadlockError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "DeadlockDetected") ||
		strings.Contains(err.Error(), "deadlock")
}

// ExecuteCypher executes a Cypher query against the test database
func (tc *Neo4jTestContainer) ExecuteCypher(ctx context.Context, query string, params map[string]any) error {
	driver, err := tc.CreateDriver()
	if err != nil {
		return fmt.Errorf("failed to create driver: %w", err)
	}
	defer driver.Close(ctx)

	session := driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeWrite,
	})
	defer session.Close(ctx)

	_, err = session.Run(ctx, query, params)
	return err
}

// VerifyConnection checks if the Neo4j connection is working
func (tc *Neo4jTestContainer) VerifyConnection(ctx context.Context) error {
	driver, err := tc.CreateDriver()
	if err != nil {
		return fmt.Errorf("failed to create driver: %w", err)
	}
	defer driver.Close(ctx)

	err = driver.VerifyConnectivity(ctx)
	if err != nil {
		return fmt.Errorf("failed to verify connectivity: %w", err)
	}

	return nil
}

// IsNeo4jAvailable checks if Neo4j is available for testing
func IsNeo4jAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	container := &Neo4jTestContainer{
		URI:      getEnvOrDefault("NEO4J_TEST_URI", defaultTestURI),
		Username: getEnvOrDefault("NEO4J_TEST_USERNAME", defaultTestUsername),
		Password: getEnvOrDefault("NEO4J_TEST_PASSWORD", defaultTestPassword),
	}

	return container.VerifyConnection(ctx) == nil
}

// TestConfig represents test configuration
type TestConfig struct {
	Neo4j struct {
		URI      string
		Username string
		Password string
		Database string
	}
}

// GetTestConfig returns a default test configuration
func GetTestConfig() *TestConfig {
	return &TestConfig{
		Neo4j: struct {
			URI      string
			Username string
			Password string
			Database string
		}{
			URI:      getEnvOrDefault("NEO4J_TEST_URI", defaultTestURI),
			Username: getEnvOrDefault("NEO4J_TEST_USERNAME", defaultTestUsername),
			Password: getEnvOrDefault("NEO4J_TEST_PASSWORD", defaultTestPassword),
			Database: "",
		},
	}
}

// Helper functions

func checkDockerComposeStatus() error {
	composeFile := findDockerComposeFile()
	cmd := exec.Command("docker-compose", "-f", composeFile, "ps", "-q", "neo4j-test")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	if len(output) == 0 {
		return fmt.Errorf("neo4j-test container not running")
	}
	return nil
}

func startDockerCompose() error {
	composeFile := findDockerComposeFile()
	cmd := exec.Command("docker-compose", "-f", composeFile, "up", "-d", "neo4j-test")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitForNeo4j(ctx context.Context, container *Neo4jTestContainer) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(60 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for Neo4j to be ready")
		case <-ticker.C:
			if err := container.VerifyConnection(ctx); err == nil {
				return nil
			}
		}
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func findDockerComposeFile() string {
	// Check for docker-compose.yml in common locations
	candidates := []string{
		"docker-compose.yml",
		"../docker-compose.yml",
		"../../docker-compose.yml",
		"../../../docker-compose.yml",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Default to the project root
	return "../../docker-compose.yml"
}
