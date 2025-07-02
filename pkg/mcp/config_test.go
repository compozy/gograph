package mcp_test

import (
	"testing"

	"github.com/compozy/gograph/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	t.Run("Should return valid default configuration", func(t *testing.T) {
		config := mcp.DefaultConfig()

		assert.NotNil(t, config)
		assert.Equal(t, "localhost", config.Server.Host)
		assert.Equal(t, 8080, config.Server.Port)
		assert.Equal(t, 100, config.Server.MaxConnections)
		assert.False(t, config.Auth.Enabled)
		assert.Equal(t, "token", config.Auth.Method)
		assert.Equal(t, 1000000, config.Performance.MaxContextSize)
		assert.Equal(t, 3600, config.Performance.CacheTTL)
		assert.True(t, config.Performance.EnableStreaming)
		assert.Equal(t, 100, config.Performance.BatchSize)
		assert.NotEmpty(t, config.Security.AllowedPaths)
		assert.NotEmpty(t, config.Security.ForbiddenPaths)
		assert.Equal(t, 100, config.Security.RateLimit)
		assert.Equal(t, 30, config.Security.MaxQueryTime)
		assert.True(t, config.Features.EnableIncremental)
		assert.True(t, config.Features.EnableValidation)
		assert.True(t, config.Features.EnablePatterns)
		assert.True(t, config.Features.EnableCaching)
	})
}

func TestConfig_Validate(t *testing.T) {
	t.Run("Should validate valid configuration", func(t *testing.T) {
		config := mcp.DefaultConfig()
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should reject empty host", func(t *testing.T) {
		config := mcp.DefaultConfig()
		config.Server.Host = ""
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "host cannot be empty")
	})

	t.Run("Should reject invalid port", func(t *testing.T) {
		config := mcp.DefaultConfig()
		config.Server.Port = 0
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "port must be between 1 and 65535")
	})

	t.Run("Should reject port above 65535", func(t *testing.T) {
		config := mcp.DefaultConfig()
		config.Server.Port = 70000
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "port must be between 1 and 65535")
	})

	t.Run("Should reject zero max connections", func(t *testing.T) {
		config := mcp.DefaultConfig()
		config.Server.MaxConnections = 0
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max_connections must be positive")
	})

	t.Run("Should reject zero max context size", func(t *testing.T) {
		config := mcp.DefaultConfig()
		config.Performance.MaxContextSize = 0
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max_context_size must be positive")
	})

	t.Run("Should reject zero batch size", func(t *testing.T) {
		config := mcp.DefaultConfig()
		config.Performance.BatchSize = 0
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batch_size must be positive")
	})

	t.Run("Should reject zero rate limit", func(t *testing.T) {
		config := mcp.DefaultConfig()
		config.Security.RateLimit = 0
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate_limit must be positive")
	})

	t.Run("Should reject zero max query time", func(t *testing.T) {
		config := mcp.DefaultConfig()
		config.Security.MaxQueryTime = 0
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max_query_time must be positive")
	})
}

func TestConfig_SecurityPaths(t *testing.T) {
	t.Run("Should have sensible default allowed paths", func(t *testing.T) {
		config := mcp.DefaultConfig()

		// Check that current directory is allowed
		assert.Contains(t, config.Security.AllowedPaths, ".")
	})

	t.Run("Should have sensible default forbidden paths", func(t *testing.T) {
		config := mcp.DefaultConfig()

		// Check that common forbidden directories are included
		assert.Contains(t, config.Security.ForbiddenPaths, ".git")
		assert.Contains(t, config.Security.ForbiddenPaths, "vendor")
		assert.Contains(t, config.Security.ForbiddenPaths, "node_modules")
	})
}

func TestConfig_FeatureFlags(t *testing.T) {
	t.Run("Should enable all features by default", func(t *testing.T) {
		config := mcp.DefaultConfig()

		assert.True(t, config.Features.EnableIncremental)
		assert.True(t, config.Features.EnableValidation)
		assert.True(t, config.Features.EnablePatterns)
		assert.True(t, config.Features.EnableCaching)
	})

	t.Run("Should allow disabling individual features", func(t *testing.T) {
		config := mcp.DefaultConfig()

		// Disable some features
		config.Features.EnablePatterns = false
		config.Features.EnableCaching = false

		// Should still validate
		err := config.Validate()
		require.NoError(t, err)

		// Features should remain disabled
		assert.False(t, config.Features.EnablePatterns)
		assert.False(t, config.Features.EnableCaching)

		// Other features should remain enabled
		assert.True(t, config.Features.EnableIncremental)
		assert.True(t, config.Features.EnableValidation)
	})
}
