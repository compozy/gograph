package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/gograph/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	t.Run("Should return valid default configuration", func(t *testing.T) {
		cfg := config.DefaultConfig()

		// Project defaults
		assert.Equal(t, "default", cfg.Project.Name)
		assert.Equal(t, ".", cfg.Project.RootPath)

		// Neo4j defaults
		assert.Equal(t, "bolt://localhost:7687", cfg.Neo4j.URI)
		assert.Equal(t, "neo4j", cfg.Neo4j.Username)
		assert.Empty(t, cfg.Neo4j.Password)
		assert.Empty(t, cfg.Neo4j.Database)

		// Analysis defaults
		assert.Contains(t, cfg.Analysis.IgnoreDirs, ".git")
		assert.Contains(t, cfg.Analysis.IgnoreDirs, ".idea")
		assert.Contains(t, cfg.Analysis.IgnoreDirs, ".vscode")
		assert.Contains(t, cfg.Analysis.IgnoreDirs, "node_modules")
		assert.Empty(t, cfg.Analysis.IgnoreFiles)
		assert.True(t, cfg.Analysis.IncludeTests)
		assert.False(t, cfg.Analysis.IncludeVendor)
		assert.Equal(t, 4, cfg.Analysis.MaxConcurrency)
	})
}

func TestLoad(t *testing.T) {
	t.Run("Should return default config when file does not exist", func(t *testing.T) {
		cfg, err := config.Load("non-existent-file.yaml")

		require.NoError(t, err)
		assert.Equal(t, "default", cfg.Project.Name)
		assert.Equal(t, ".", cfg.Project.RootPath)
	})

	t.Run("Should load config from YAML file", func(t *testing.T) {
		// Create a temporary config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".gograph.yaml")

		configContent := `
project:
  name: test-project
  root_path: /test/path
neo4j:
  uri: bolt://neo4j:7687
  username: testuser
  password: testpass
  database: testdb
analysis:
  ignore_dirs:
    - .git
    - vendor
    - build
  ignore_files:
    - "*.generated.go"
  include_tests: false
  include_vendor: false
  max_concurrency: 8
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Load the config
		cfg, err := config.Load(configPath)

		require.NoError(t, err)
		assert.Equal(t, "test-project", cfg.Project.Name)
		assert.Equal(t, "/test/path", cfg.Project.RootPath)
		assert.Equal(t, "bolt://neo4j:7687", cfg.Neo4j.URI)
		assert.Equal(t, "testuser", cfg.Neo4j.Username)
		assert.Equal(t, "testpass", cfg.Neo4j.Password)
		assert.Equal(t, "testdb", cfg.Neo4j.Database)
		assert.Len(t, cfg.Analysis.IgnoreDirs, 3)
		assert.Contains(t, cfg.Analysis.IgnoreDirs, "vendor")
		assert.Contains(t, cfg.Analysis.IgnoreFiles, "*.generated.go")
		assert.False(t, cfg.Analysis.IncludeTests)
		assert.False(t, cfg.Analysis.IncludeVendor)
		assert.Equal(t, 8, cfg.Analysis.MaxConcurrency)
	})

	t.Run("Should load config from current directory when path is empty", func(t *testing.T) {
		// Save current directory and restore it after test
		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer func() {
			err := os.Chdir(originalDir)
			require.NoError(t, err)
		}()

		// Create a temporary directory and change to it
		tmpDir := t.TempDir()
		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// Create config file in the temp directory
		configContent := `
project:
  name: current-dir-project
`
		err = os.WriteFile(".gograph.yaml", []byte(configContent), 0644)
		require.NoError(t, err)

		// Load config with empty path
		cfg, err := config.Load("")

		require.NoError(t, err)
		assert.Equal(t, "current-dir-project", cfg.Project.Name)
	})

	t.Run("Should handle invalid YAML gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "invalid.yaml")

		// Write invalid YAML
		err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644)
		require.NoError(t, err)

		_, err = config.Load(configPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read config file")
	})
}

func TestSave(t *testing.T) {
	t.Run("Should save config to specified file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "test-config.yaml")

		cfg := &config.Config{
			Project: config.ProjectConfig{
				Name:     "save-test",
				RootPath: "/save/test",
			},
			Neo4j: config.Neo4jConfig{
				URI:      "bolt://saved:7687",
				Username: "saveduser",
				Password: "savedpass",
				Database: "saveddb",
			},
			Analysis: config.AnalysisConfig{
				IgnoreDirs:     []string{"target", "out"},
				IgnoreFiles:    []string{"*.tmp"},
				IncludeTests:   true,
				IncludeVendor:  true,
				MaxConcurrency: 16,
			},
		}

		err := config.Save(cfg, configPath)
		require.NoError(t, err)

		// Verify file was created
		_, err = os.Stat(configPath)
		require.NoError(t, err)

		// Load the saved config to verify
		loadedCfg, err := config.Load(configPath)
		require.NoError(t, err)

		assert.Equal(t, cfg.Project.Name, loadedCfg.Project.Name)
		assert.Equal(t, cfg.Project.RootPath, loadedCfg.Project.RootPath)
		assert.Equal(t, cfg.Neo4j.URI, loadedCfg.Neo4j.URI)
		assert.Equal(t, cfg.Neo4j.Username, loadedCfg.Neo4j.Username)
		assert.Equal(t, cfg.Neo4j.Password, loadedCfg.Neo4j.Password)
		assert.Equal(t, cfg.Neo4j.Database, loadedCfg.Neo4j.Database)
		assert.Equal(t, cfg.Analysis.IgnoreDirs, loadedCfg.Analysis.IgnoreDirs)
		assert.Equal(t, cfg.Analysis.IgnoreFiles, loadedCfg.Analysis.IgnoreFiles)
		assert.Equal(t, cfg.Analysis.IncludeTests, loadedCfg.Analysis.IncludeTests)
		assert.Equal(t, cfg.Analysis.IncludeVendor, loadedCfg.Analysis.IncludeVendor)
		assert.Equal(t, cfg.Analysis.MaxConcurrency, loadedCfg.Analysis.MaxConcurrency)
	})

	t.Run("Should save to default location when path is empty", func(t *testing.T) {
		// Save current directory and restore it after test
		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer func() {
			err := os.Chdir(originalDir)
			require.NoError(t, err)
		}()

		// Create and change to temp directory
		tmpDir := t.TempDir()
		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		cfg := config.DefaultConfig()
		cfg.Project.Name = "default-location-test"

		err = config.Save(cfg, "")
		require.NoError(t, err)

		// Verify file was created at default location
		_, err = os.Stat(".gograph.yaml")
		require.NoError(t, err)

		// Load and verify
		loadedCfg, err := config.Load("")
		require.NoError(t, err)
		assert.Equal(t, "default-location-test", loadedCfg.Project.Name)
	})

	t.Run("Should handle write errors", func(t *testing.T) {
		// Try to save to a read-only directory
		cfg := config.DefaultConfig()
		err := config.Save(cfg, "/root/impossible-to-write.yaml")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write config file")
	})
}

func TestConfig_ToProject(t *testing.T) {
	t.Run("Should convert config to core.Project", func(t *testing.T) {
		cfg := &config.Config{
			Project: config.ProjectConfig{
				Name:     "test-project",
				RootPath: "/test/root",
			},
			Neo4j: config.Neo4jConfig{
				URI:      "bolt://test:7687",
				Username: "testuser",
			},
		}

		project := cfg.ToProject("/path/to/config.yaml")

		assert.NotEmpty(t, project.ID)
		assert.Equal(t, "test-project", project.Name)
		assert.Equal(t, "/test/root", project.RootPath)
		assert.Equal(t, "bolt://test:7687", project.Neo4jURI)
		assert.Equal(t, "testuser", project.Neo4jUser)
		assert.Equal(t, "/path/to/config.yaml", project.ConfigPath)
	})

	t.Run("Should generate unique IDs", func(t *testing.T) {
		cfg := config.DefaultConfig()

		project1 := cfg.ToProject("config1.yaml")
		project2 := cfg.ToProject("config2.yaml")

		assert.NotEqual(t, project1.ID, project2.ID)
	})
}

func TestConfigIntegration(t *testing.T) {
	t.Run("Should handle full config lifecycle", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "integration-test.yaml")

		// Create a config
		originalCfg := &config.Config{
			Project: config.ProjectConfig{
				Name:     "integration-test",
				RootPath: tmpDir,
			},
			Neo4j: config.Neo4jConfig{
				URI:      "bolt://integration:7687",
				Username: "integrationuser",
				Password: "integrationpass",
				Database: "integrationdb",
			},
			Analysis: config.AnalysisConfig{
				IgnoreDirs:     []string{"dist", "coverage"},
				IgnoreFiles:    []string{"*.test.go"},
				IncludeTests:   false,
				IncludeVendor:  false,
				MaxConcurrency: 2,
			},
		}

		// Save it
		err := config.Save(originalCfg, configPath)
		require.NoError(t, err)

		// Load it back
		loadedCfg, err := config.Load(configPath)
		require.NoError(t, err)

		// Verify all fields match
		assert.Equal(t, originalCfg.Project, loadedCfg.Project)
		assert.Equal(t, originalCfg.Neo4j, loadedCfg.Neo4j)
		assert.Equal(t, originalCfg.Analysis, loadedCfg.Analysis)

		// Convert to project and verify
		project := loadedCfg.ToProject(configPath)
		assert.Equal(t, "integration-test", project.Name)
		assert.Equal(t, tmpDir, project.RootPath)
		assert.Equal(t, "bolt://integration:7687", project.Neo4jURI)
		assert.Equal(t, "integrationuser", project.Neo4jUser)
		assert.Equal(t, configPath, project.ConfigPath)
	})
}
