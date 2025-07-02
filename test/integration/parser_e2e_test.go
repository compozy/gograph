package integration

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compozy/gograph/engine/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0) //nolint:dogsled // Need to extract filename for test path
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func TestParserEndToEnd(t *testing.T) {
	projectRoot := getProjectRoot()

	t.Run("Should parse simple project successfully", func(t *testing.T) {
		ctx := context.Background()
		parserService := parser.NewService(nil)

		testProjectPath := filepath.Join(projectRoot, "testdata", "simple_project")

		// Parse the project
		result, err := parserService.ParseProject(ctx, testProjectPath, nil)

		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify project details
		assert.Equal(t, testProjectPath, result.ProjectPath)

		// Should have parsed at least 2 files (main.go and helper.go)
		assert.GreaterOrEqual(t, len(result.Files), 2)

		// Find main.go
		var mainFile *parser.FileInfo
		for _, f := range result.Files {
			if filepath.Base(f.Path) == "main.go" {
				mainFile = f
				break
			}
		}
		require.NotNil(t, mainFile, "main.go should be parsed")

		// Verify main.go details
		assert.Equal(t, "main", mainFile.Package)
		assert.Contains(t, mainFile.Path, "cmd/main.go")

		// Should have main and process functions
		functionNames := make(map[string]bool)
		for _, fn := range mainFile.Functions {
			functionNames[fn.Name] = true
		}
		assert.True(t, functionNames["main"], "Should have main function")
		assert.True(t, functionNames["process"], "Should have process function")

		// Should have imports
		assert.Greater(t, len(mainFile.Imports), 0)
		hasUtilsImport := false
		for _, imp := range mainFile.Imports {
			if imp.Path == "github.com/test/simple/pkg/utils" {
				hasUtilsImport = true
				break
			}
		}
		assert.True(t, hasUtilsImport, "Should import utils package")

		// Find helper.go
		var helperFile *parser.FileInfo
		for _, f := range result.Files {
			if filepath.Base(f.Path) == "helper.go" {
				helperFile = f
				break
			}
		}
		require.NotNil(t, helperFile, "helper.go should be parsed")

		// Verify helper.go details
		assert.Equal(t, "utils", helperFile.Package)

		// Should have Helper, Transform, and Calculate functions
		helperFunctions := make(map[string]bool)
		for _, fn := range helperFile.Functions {
			helperFunctions[fn.Name] = true
		}
		assert.True(t, helperFunctions["Helper"], "Should have Helper function")
		assert.True(t, helperFunctions["Transform"], "Should have Transform function")
		assert.True(t, helperFunctions["Calculate"], "Should have Calculate function")
	})

	t.Run("Should handle parsing errors gracefully", func(t *testing.T) {
		ctx := context.Background()
		parserService := parser.NewService(nil)

		// Try to parse non-existent directory
		result, err := parserService.ParseProject(ctx, "/non/existent/path", nil)

		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should parse project with potential circular dependencies", func(t *testing.T) {
		ctx := context.Background()
		parserService := parser.NewService(nil)

		testProjectPath := filepath.Join(projectRoot, "testdata", "circular_deps")

		// Parse the project
		result, err := parserService.ParseProject(ctx, testProjectPath, nil)

		require.NoError(t, err)
		require.NotNil(t, result)

		// Should successfully parse even with import structure that could lead to circular deps
		assert.GreaterOrEqual(t, len(result.Files), 2)

		// Verify imports are captured correctly
		var pkgAFile *parser.FileInfo
		for _, f := range result.Files {
			if filepath.Base(f.Path) == "a.go" {
				pkgAFile = f
				break
			}
		}
		require.NotNil(t, pkgAFile)

		// Package A should import package B
		hasImportB := false
		for _, imp := range pkgAFile.Imports {
			if imp.Path == "github.com/test/circular/pkg/b" {
				hasImportB = true
				break
			}
		}
		assert.True(t, hasImportB, "Package A should import package B")
	})

	t.Run("Should capture function calls within files", func(t *testing.T) {
		ctx := context.Background()
		parserService := parser.NewService(nil)

		testProjectPath := filepath.Join(projectRoot, "testdata", "simple_project")
		result, err := parserService.ParseProject(ctx, testProjectPath, nil)

		require.NoError(t, err)

		// Find main.go
		var mainFile *parser.FileInfo
		for _, f := range result.Files {
			if filepath.Base(f.Path) == "main.go" {
				mainFile = f
				break
			}
		}
		require.NotNil(t, mainFile)

		// Find main function
		var mainFunc *parser.FunctionInfo
		for i := range mainFile.Functions {
			if mainFile.Functions[i].Name == "main" {
				mainFunc = &mainFile.Functions[i]
				break
			}
		}
		require.NotNil(t, mainFunc)

		// main function should call process and utils.Helper
		callsMap := make(map[string]bool)
		for _, call := range mainFunc.Calls {
			if call.Package != "" {
				callsMap[call.Package+"."+call.Name] = true
			} else {
				callsMap[call.Name] = true
			}
		}

		assert.True(t, callsMap["fmt.Println"], "main should call fmt.Println")
		assert.True(t, callsMap["process"], "main should call process")
		assert.True(t, callsMap["utils.Helper"], "main should call utils.Helper")
	})
}

func TestParserConcurrency(t *testing.T) {
	t.Run("Should handle concurrent parsing safely", func(t *testing.T) {
		ctx := context.Background()
		projectRoot := getProjectRoot()
		testProjectPath := filepath.Join(projectRoot, "testdata", "simple_project")

		// Run multiple parsers concurrently
		numGoroutines := 5
		results := make(chan *parser.ParseResult, numGoroutines)
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				parserService := parser.NewService(nil)
				result, err := parserService.ParseProject(ctx, testProjectPath, nil)
				if err != nil {
					errors <- err
					results <- nil
				} else {
					errors <- nil
					results <- result
				}
			}()
		}

		// Collect results
		for i := 0; i < numGoroutines; i++ {
			err := <-errors
			result := <-results

			assert.NoError(t, err)
			assert.NotNil(t, result)
			if result != nil {
				assert.GreaterOrEqual(t, len(result.Files), 2)
			}
		}
	})
}
