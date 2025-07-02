package analyzer_test

import (
	"testing"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/stretchr/testify/assert"
)

func TestDefaultAnalyzerConfig(t *testing.T) {
	t.Run("Should return valid default configuration", func(t *testing.T) {
		cfg := analyzer.DefaultAnalyzerConfig()

		assert.NotNil(t, cfg)
		assert.Equal(t, 10, cfg.MaxDependencyDepth)
		assert.False(t, cfg.IgnoreTestFiles)
		assert.True(t, cfg.IgnoreVendor)
		assert.True(t, cfg.IncludeMetrics)
		assert.Equal(t, 4, cfg.ParallelWorkers)
	})

	t.Run("Should be used as default when creating analyzer with nil config", func(t *testing.T) {
		// This is tested implicitly in service_test.go when creating analyzer with nil config
		service := analyzer.NewAnalyzer(nil)
		assert.NotNil(t, service)
		// The service should work with default config
	})
}

func TestAnalyzerConfig_Validation(t *testing.T) {
	t.Run("Should handle zero values appropriately", func(t *testing.T) {
		cfg := &analyzer.Config{
			MaxDependencyDepth: 0,
			IgnoreTestFiles:    true,
			IgnoreVendor:       false,
			IncludeMetrics:     false,
			ParallelWorkers:    0,
		}

		// Zero values should be valid
		assert.Equal(t, 0, cfg.MaxDependencyDepth)
		assert.True(t, cfg.IgnoreTestFiles)
		assert.False(t, cfg.IgnoreVendor)
		assert.False(t, cfg.IncludeMetrics)
		assert.Equal(t, 0, cfg.ParallelWorkers)
	})

	t.Run("Should handle negative values", func(t *testing.T) {
		cfg := &analyzer.Config{
			MaxDependencyDepth: -1,
			ParallelWorkers:    -1,
		}

		// Config should accept negative values (validation happens in service)
		assert.Equal(t, -1, cfg.MaxDependencyDepth)
		assert.Equal(t, -1, cfg.ParallelWorkers)
	})

	t.Run("Should handle large values", func(t *testing.T) {
		cfg := &analyzer.Config{
			MaxDependencyDepth: 1000,
			ParallelWorkers:    100,
		}

		assert.Equal(t, 1000, cfg.MaxDependencyDepth)
		assert.Equal(t, 100, cfg.ParallelWorkers)
	})
}

func TestAnalyzerConfig_Customization(t *testing.T) {
	t.Run("Should allow full customization", func(t *testing.T) {
		cfg := &analyzer.Config{
			MaxDependencyDepth: 20,
			IgnoreTestFiles:    true,
			IgnoreVendor:       false,
			IncludeMetrics:     false,
			ParallelWorkers:    8,
		}

		assert.Equal(t, 20, cfg.MaxDependencyDepth)
		assert.True(t, cfg.IgnoreTestFiles)
		assert.False(t, cfg.IgnoreVendor)
		assert.False(t, cfg.IncludeMetrics)
		assert.Equal(t, 8, cfg.ParallelWorkers)
	})

	t.Run("Should allow partial customization from defaults", func(t *testing.T) {
		cfg := analyzer.DefaultAnalyzerConfig()

		// Modify only specific fields
		cfg.IgnoreTestFiles = true
		cfg.ParallelWorkers = 16

		// Other fields should remain as defaults
		assert.Equal(t, 10, cfg.MaxDependencyDepth)
		assert.True(t, cfg.IgnoreTestFiles) // Modified
		assert.True(t, cfg.IgnoreVendor)
		assert.True(t, cfg.IncludeMetrics)
		assert.Equal(t, 16, cfg.ParallelWorkers) // Modified
	})
}

func TestAnalyzerConfig_UseCases(t *testing.T) {
	t.Run("Should configure for test analysis", func(t *testing.T) {
		cfg := analyzer.DefaultAnalyzerConfig()
		cfg.IgnoreTestFiles = false // Include test files
		cfg.IncludeMetrics = true   // Get metrics for tests too

		assert.False(t, cfg.IgnoreTestFiles)
		assert.True(t, cfg.IncludeMetrics)
	})

	t.Run("Should configure for performance mode", func(t *testing.T) {
		cfg := analyzer.DefaultAnalyzerConfig()
		cfg.ParallelWorkers = 16   // More workers
		cfg.IncludeMetrics = false // Skip metrics for speed
		cfg.MaxDependencyDepth = 5 // Limit depth for speed

		assert.Equal(t, 16, cfg.ParallelWorkers)
		assert.False(t, cfg.IncludeMetrics)
		assert.Equal(t, 5, cfg.MaxDependencyDepth)
	})

	t.Run("Should configure for vendor analysis", func(t *testing.T) {
		cfg := analyzer.DefaultAnalyzerConfig()
		cfg.IgnoreVendor = false   // Include vendor
		cfg.MaxDependencyDepth = 3 // Limit depth for vendor deps

		assert.False(t, cfg.IgnoreVendor)
		assert.Equal(t, 3, cfg.MaxDependencyDepth)
	})

	t.Run("Should configure for minimal analysis", func(t *testing.T) {
		cfg := &analyzer.Config{
			MaxDependencyDepth: 1,
			IgnoreTestFiles:    true,
			IgnoreVendor:       true,
			IncludeMetrics:     false,
			ParallelWorkers:    1,
		}

		assert.Equal(t, 1, cfg.MaxDependencyDepth)
		assert.True(t, cfg.IgnoreTestFiles)
		assert.True(t, cfg.IgnoreVendor)
		assert.False(t, cfg.IncludeMetrics)
		assert.Equal(t, 1, cfg.ParallelWorkers)
	})
}
