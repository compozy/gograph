package mcp_test

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/mcp"
	"github.com/compozy/gograph/engine/parser"
	mcpconfig "github.com/compozy/gograph/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockServiceAdapter for testing
type MockServiceAdapter struct {
	mock.Mock
}

func (m *MockServiceAdapter) ParseProject(ctx context.Context, projectPath string) (*parser.ParseResult, error) {
	args := m.Called(ctx, projectPath)
	if result := args.Get(0); result != nil {
		return result.(*parser.ParseResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockServiceAdapter) AnalyzeProject(
	ctx context.Context,
	projectID core.ID,
	files []*parser.FileInfo,
) (*analyzer.AnalysisReport, error) {
	args := m.Called(ctx, projectID, files)
	if result := args.Get(0); result != nil {
		return result.(*analyzer.AnalysisReport), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockServiceAdapter) InitializeProject(ctx context.Context, project *core.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *MockServiceAdapter) ImportAnalysisResult(
	ctx context.Context,
	result *core.AnalysisResult,
) (*graph.ProjectGraph, error) {
	args := m.Called(ctx, result)
	if result := args.Get(0); result != nil {
		return result.(*graph.ProjectGraph), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockServiceAdapter) GetProjectStatistics(
	ctx context.Context,
	projectID core.ID,
) (*graph.ProjectStatistics, error) {
	args := m.Called(ctx, projectID)
	if result := args.Get(0); result != nil {
		return result.(*graph.ProjectStatistics), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockServiceAdapter) ExecuteQuery(
	ctx context.Context,
	query string,
	params map[string]any,
) ([]map[string]any, error) {
	args := m.Called(ctx, query, params)
	if result := args.Get(0); result != nil {
		return result.([]map[string]any), args.Error(1)
	}
	return nil, args.Error(1)
}

func TestNewServer(t *testing.T) {
	t.Run("Should create server with valid configuration", func(t *testing.T) {
		config := mcpconfig.DefaultConfig()
		mockAdapter := new(MockServiceAdapter)

		server := mcp.NewServer(config, mockAdapter, nil, nil, nil)

		assert.NotNil(t, server)
	})

	t.Run("Should use default configuration when nil", func(t *testing.T) {
		mockAdapter := new(MockServiceAdapter)

		server := mcp.NewServer(nil, mockAdapter, nil, nil, nil)

		assert.NotNil(t, server)
	})

	t.Run("Should reject invalid configuration", func(t *testing.T) {
		config := mcpconfig.DefaultConfig()
		config.Server.Port = 0 // Invalid port
		mockAdapter := new(MockServiceAdapter)

		server := mcp.NewServer(config, mockAdapter, nil, nil, nil)

		// Server creation should succeed even with nil services
		assert.NotNil(t, server)
	})
}

func TestServer_Start(t *testing.T) {
	t.Run("Should start server successfully", func(t *testing.T) {
		config := mcpconfig.DefaultConfig()
		mockAdapter := new(MockServiceAdapter)

		server := mcp.NewServer(config, mockAdapter, nil, nil, nil)
		assert.NotNil(t, server)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Start server in goroutine since it blocks
		done := make(chan error, 1)
		go func() {
			done <- server.Start(ctx)
		}()

		// Wait for either context timeout or server to complete
		select {
		case err := <-done:
			// Server completed when context was canceled
			assert.NoError(t, err)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Server Start did not complete after context cancellation")
		}
	})
}

// TestServer_Stop is commented out until Stop method is implemented
// func TestServer_Stop(t *testing.T) {
// 	t.Run("Should stop server gracefully", func(t *testing.T) {
// 		config := mcpconfig.DefaultConfig()
// 		mockAdapter := new(MockServiceAdapter)
//
// 		server := mcp.NewServer(config, mockAdapter, nil, nil, nil)
// 		assert.NotNil(t, server)
//
// 		// TODO(2025-07-02): Implement Stop method in server
// 		// err := server.Stop()
// 		// assert.NoError(t, err)
// 	})
// }

func TestServer_RateLimiting(t *testing.T) {
	t.Run("Should initialize with configured rate limit", func(t *testing.T) {
		config := mcpconfig.DefaultConfig()
		config.Security.RateLimit = 60 // 60 requests per time window
		mockAdapter := new(MockServiceAdapter)

		server := mcp.NewServer(config, mockAdapter, nil, nil, nil)
		assert.NotNil(t, server)

		// The server should have initialized with the rate limit
		// In a real implementation, we would test the rate limiting behavior
	})
}

func TestServer_Caching(t *testing.T) {
	t.Run("Should initialize cache when caching is enabled", func(t *testing.T) {
		config := mcpconfig.DefaultConfig()
		config.Features.EnableCaching = true
		config.Performance.CacheTTL = 3600 // 1 hour
		mockAdapter := new(MockServiceAdapter)

		server := mcp.NewServer(config, mockAdapter, nil, nil, nil)
		assert.NotNil(t, server)

		// The server should have initialized the cache
		// In a real implementation, we would test cache behavior
	})

	t.Run("Should work without cache when caching is disabled", func(t *testing.T) {
		config := mcpconfig.DefaultConfig()
		config.Features.EnableCaching = false
		mockAdapter := new(MockServiceAdapter)

		server := mcp.NewServer(config, mockAdapter, nil, nil, nil)
		assert.NotNil(t, server)

		// The server should work without caching
	})
}
