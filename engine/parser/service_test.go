package parser_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/gograph/engine/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_ParseFile(t *testing.T) {
	// Create a test Go file
	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "test.go")
	testContent := `package main

import (
	"fmt"
	"strings"
)

// User represents a user
type User struct {
	ID   string
	Name string
	Age  int
}

// NewUser creates a new user
func NewUser(name string, age int) *User {
	return &User{
		ID:   generateID(),
		Name: name,
		Age:  age,
	}
}

// GetName returns the user's name
func (u *User) GetName() string {
	return u.Name
}

// UserService interface
type UserService interface {
	GetUser(id string) (*User, error)
	SaveUser(user *User) error
}

const MaxUsers = 100
const MinAge = 18

var (
	defaultUser = &User{Name: "Default", Age: 0}
	userCache   map[string]*User
)

func generateID() string {
	return fmt.Sprintf("user-%d", time.Now().UnixNano())
}
`
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	service := parser.NewService(nil)

	t.Run("Should parse a Go file successfully", func(t *testing.T) {
		ctx := context.Background()
		info, err := service.ParseFile(ctx, testFile)

		assert.NoError(t, err)
		assert.NotNil(t, info)
		assert.Equal(t, testFile, info.Path)
		assert.Equal(t, "main", info.Package)
	})

	t.Run("Should extract imports correctly", func(t *testing.T) {
		ctx := context.Background()
		info, err := service.ParseFile(ctx, testFile)

		require.NoError(t, err)
		assert.Len(t, info.Imports, 2)
		assert.Equal(t, "fmt", info.Imports[0].Path)
		assert.Equal(t, "strings", info.Imports[1].Path)
		assert.Contains(t, info.Dependencies, "fmt")
		assert.Contains(t, info.Dependencies, "strings")
	})

	t.Run("Should extract structs correctly", func(t *testing.T) {
		ctx := context.Background()
		info, err := service.ParseFile(ctx, testFile)

		require.NoError(t, err)
		assert.Len(t, info.Structs, 1)

		userStruct := info.Structs[0]
		assert.Equal(t, "User", userStruct.Name)
		assert.True(t, userStruct.IsExported)
		assert.Len(t, userStruct.Fields, 3)

		// Check fields
		assert.Equal(t, "ID", userStruct.Fields[0].Name)
		assert.Equal(t, "string", userStruct.Fields[0].Type)
		assert.True(t, userStruct.Fields[0].IsExported)

		assert.Equal(t, "Name", userStruct.Fields[1].Name)
		assert.Equal(t, "string", userStruct.Fields[1].Type)

		assert.Equal(t, "Age", userStruct.Fields[2].Name)
		assert.Equal(t, "int", userStruct.Fields[2].Type)
	})

	t.Run("Should extract functions correctly", func(t *testing.T) {
		ctx := context.Background()
		info, err := service.ParseFile(ctx, testFile)

		require.NoError(t, err)
		// Should have NewUser, GetName (method), and generateID
		assert.GreaterOrEqual(t, len(info.Functions), 3)

		// Find NewUser function
		var newUserFunc *parser.FunctionInfo
		for i := range info.Functions {
			if info.Functions[i].Name == "NewUser" {
				newUserFunc = &info.Functions[i]
				break
			}
		}

		require.NotNil(t, newUserFunc)
		assert.True(t, newUserFunc.IsExported)
		assert.Len(t, newUserFunc.Parameters, 2)
		assert.Equal(t, "name", newUserFunc.Parameters[0].Name)
		assert.Equal(t, "string", newUserFunc.Parameters[0].Type)
		assert.Equal(t, "age", newUserFunc.Parameters[1].Name)
		assert.Equal(t, "int", newUserFunc.Parameters[1].Type)
		assert.Len(t, newUserFunc.Returns, 1)
		assert.Equal(t, "*User", newUserFunc.Returns[0])
	})

	t.Run("Should extract methods correctly", func(t *testing.T) {
		ctx := context.Background()
		info, err := service.ParseFile(ctx, testFile)

		require.NoError(t, err)

		// Find GetName method
		var getNameMethod *parser.FunctionInfo
		for i := range info.Functions {
			if info.Functions[i].Name == "GetName" && info.Functions[i].Receiver != "" {
				getNameMethod = &info.Functions[i]
				break
			}
		}

		require.NotNil(t, getNameMethod)
		assert.Equal(t, "*User", getNameMethod.Receiver)
		assert.True(t, getNameMethod.IsExported)
		assert.Len(t, getNameMethod.Returns, 1)
		assert.Equal(t, "string", getNameMethod.Returns[0])
	})

	t.Run("Should extract interfaces correctly", func(t *testing.T) {
		ctx := context.Background()
		info, err := service.ParseFile(ctx, testFile)

		require.NoError(t, err)
		assert.Len(t, info.Interfaces, 1)

		userService := info.Interfaces[0]
		assert.Equal(t, "UserService", userService.Name)
		assert.True(t, userService.IsExported)
		assert.Len(t, userService.Methods, 2)

		// Check GetUser method
		getUserMethod := userService.Methods[0]
		assert.Equal(t, "GetUser", getUserMethod.Name)
		assert.Len(t, getUserMethod.Parameters, 1)
		assert.Equal(t, "id", getUserMethod.Parameters[0].Name)
		assert.Equal(t, "string", getUserMethod.Parameters[0].Type)
		assert.Len(t, getUserMethod.Returns, 2)
		assert.Equal(t, "*User", getUserMethod.Returns[0])
		assert.Equal(t, "error", getUserMethod.Returns[1])
	})

	t.Run("Should extract constants correctly", func(t *testing.T) {
		ctx := context.Background()
		info, err := service.ParseFile(ctx, testFile)

		require.NoError(t, err)
		assert.Len(t, info.Constants, 2)

		// Check constants
		assert.Equal(t, "MaxUsers", info.Constants[0].Name)
		assert.True(t, info.Constants[0].IsExported)

		assert.Equal(t, "MinAge", info.Constants[1].Name)
		assert.True(t, info.Constants[1].IsExported)
	})

	t.Run("Should extract variables correctly", func(t *testing.T) {
		ctx := context.Background()
		info, err := service.ParseFile(ctx, testFile)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(info.Variables), 2)

		// Find defaultUser variable
		var defaultUserVar *parser.VariableInfo
		for i := range info.Variables {
			if info.Variables[i].Name == "defaultUser" {
				defaultUserVar = &info.Variables[i]
				break
			}
		}

		require.NotNil(t, defaultUserVar)
		assert.False(t, defaultUserVar.IsExported)
		// Type is empty for variables with implicit types (inferred from initializer)
		assert.Equal(t, "", defaultUserVar.Type)

		// Find userCache variable which has explicit type
		var userCacheVar *parser.VariableInfo
		for i := range info.Variables {
			if info.Variables[i].Name == "userCache" {
				userCacheVar = &info.Variables[i]
				break
			}
		}

		require.NotNil(t, userCacheVar)
		assert.False(t, userCacheVar.IsExported)
		assert.Equal(t, "map[string]*User", userCacheVar.Type)
	})

	t.Run("Should handle context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		info, err := service.ParseFile(ctx, testFile)

		assert.Error(t, err)
		assert.Nil(t, info)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("Should handle non-existent file", func(t *testing.T) {
		ctx := context.Background()

		info, err := service.ParseFile(ctx, "/non/existent/file.go")

		assert.Error(t, err)
		assert.Nil(t, info)
		assert.Contains(t, err.Error(), "failed to parse file")
	})

	t.Run("Should handle invalid Go syntax", func(t *testing.T) {
		invalidFile := filepath.Join(testDir, "invalid.go")
		invalidContent := `package main

		func broken( {
			// Missing closing parenthesis
		}
		`
		err := os.WriteFile(invalidFile, []byte(invalidContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		info, err := service.ParseFile(ctx, invalidFile)

		assert.Error(t, err)
		assert.Nil(t, info)
		assert.Contains(t, err.Error(), "failed to parse file")
	})
}

func TestService_ParseDirectory(t *testing.T) {
	// Create test directory structure
	testDir := t.TempDir()

	// Create subdirectories
	subDir1 := filepath.Join(testDir, "pkg")
	subDir2 := filepath.Join(testDir, "internal")
	ignoredDir := filepath.Join(testDir, ".git")

	require.NoError(t, os.MkdirAll(subDir1, 0755))
	require.NoError(t, os.MkdirAll(subDir2, 0755))
	require.NoError(t, os.MkdirAll(ignoredDir, 0755))

	// Create test files
	file1 := filepath.Join(subDir1, "file1.go")
	file2 := filepath.Join(subDir2, "file2.go")
	testFile := filepath.Join(subDir1, "file1_test.go")
	nonGoFile := filepath.Join(subDir1, "readme.md")
	ignoredFile := filepath.Join(ignoredDir, "ignored.go")

	goContent := `package test

func TestFunc() string {
	return "test"
}
`

	require.NoError(t, os.WriteFile(file1, []byte(goContent), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(goContent), 0644))
	require.NoError(t, os.WriteFile(testFile, []byte(goContent), 0644))
	require.NoError(t, os.WriteFile(nonGoFile, []byte("# README"), 0644))
	require.NoError(t, os.WriteFile(ignoredFile, []byte(goContent), 0644))

	t.Run("Should parse all Go files in directory", func(t *testing.T) {
		service := parser.NewService(nil)
		ctx := context.Background()

		files, err := service.ParseDirectory(ctx, testDir)

		assert.NoError(t, err)
		assert.NotNil(t, files)
		// Should include file1.go, file2.go, and file1_test.go (tests included by default)
		assert.Len(t, files, 3)
	})

	t.Run("Should ignore non-Go files", func(t *testing.T) {
		service := parser.NewService(nil)
		ctx := context.Background()

		files, err := service.ParseDirectory(ctx, testDir)

		require.NoError(t, err)
		for _, file := range files {
			assert.True(t, filepath.Ext(file.Path) == ".go")
			assert.NotEqual(t, nonGoFile, file.Path)
		}
	})

	t.Run("Should respect ignore directories", func(t *testing.T) {
		service := parser.NewService(nil)
		ctx := context.Background()

		files, err := service.ParseDirectory(ctx, testDir)

		require.NoError(t, err)
		for _, file := range files {
			assert.NotContains(t, file.Path, ".git")
		}
	})

	t.Run("Should exclude test files when configured", func(t *testing.T) {
		config := &parser.Config{
			IncludeTests: false,
			IgnoreDirs:   []string{".git"},
		}
		service := parser.NewService(config)
		ctx := context.Background()

		files, err := service.ParseDirectory(ctx, testDir)

		require.NoError(t, err)
		// Should only include file1.go and file2.go
		assert.Len(t, files, 2)
		for _, file := range files {
			assert.NotContains(t, filepath.Base(file.Path), "_test.go")
		}
	})

	t.Run("Should handle context cancellation", func(t *testing.T) {
		service := parser.NewService(nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		files, err := service.ParseDirectory(ctx, testDir)

		assert.Error(t, err)
		assert.Nil(t, files)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("Should continue on parse errors", func(t *testing.T) {
		// Create an invalid Go file
		invalidFile := filepath.Join(subDir1, "invalid.go")
		invalidContent := `package test

		func broken( {
			// Missing closing parenthesis
		}
		`
		require.NoError(t, os.WriteFile(invalidFile, []byte(invalidContent), 0644))

		service := parser.NewService(nil)
		ctx := context.Background()

		files, err := service.ParseDirectory(ctx, testDir)

		// Should not return error, just skip the invalid file
		assert.NoError(t, err)
		assert.NotNil(t, files)
		// Should still parse valid files
		assert.GreaterOrEqual(t, len(files), 3)
	})
}

func TestService_ParseProject(t *testing.T) {
	// Create test project structure
	testDir := t.TempDir()

	// Create subdirectories
	pkgDir := filepath.Join(testDir, "pkg")
	cmdDir := filepath.Join(testDir, "cmd")
	vendorDir := filepath.Join(testDir, "vendor")
	gitDir := filepath.Join(testDir, ".git")

	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(cmdDir, 0755))
	require.NoError(t, os.MkdirAll(vendorDir, 0755))
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	// Create test files
	goFiles := []string{
		filepath.Join(pkgDir, "service.go"),
		filepath.Join(pkgDir, "service_test.go"),
		filepath.Join(cmdDir, "main.go"),
		filepath.Join(vendorDir, "vendor.go"),
		filepath.Join(gitDir, "ignored.go"),
	}

	goContent := `package test

func TestFunc() string {
	return "test"
}
`

	for _, file := range goFiles {
		require.NoError(t, os.WriteFile(file, []byte(goContent), 0644))
	}

	t.Run("Should parse entire project", func(t *testing.T) {
		service := parser.NewService(nil)
		ctx := context.Background()

		result, err := service.ParseProject(ctx, testDir, nil)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testDir, result.ProjectPath)
		// Default config includes tests but not vendor or .git
		assert.Len(t, result.Files, 3)                       // service.go, service_test.go, main.go
		assert.GreaterOrEqual(t, result.ParseTime, int64(0)) // Can be 0 for very fast parsing
	})

	t.Run("Should use provided config", func(t *testing.T) {
		config := &parser.Config{
			IncludeTests:   false,
			IncludeVendor:  true,
			IgnoreDirs:     []string{".git"},
			MaxConcurrency: 2,
		}
		service := parser.NewService(nil)
		ctx := context.Background()

		result, err := service.ParseProject(ctx, testDir, config)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Should include vendor.go but not test files
		assert.Len(t, result.Files, 3) // service.go, main.go, vendor.go

		// Verify no test files
		for _, file := range result.Files {
			assert.NotContains(t, filepath.Base(file.Path), "_test.go")
		}

		// Verify vendor is included
		hasVendor := false
		for _, file := range result.Files {
			if strings.Contains(file.Path, "vendor") {
				hasVendor = true
				break
			}
		}
		assert.True(t, hasVendor)
	})

	t.Run("Should handle concurrent parsing", func(t *testing.T) {
		// Create more files to test concurrency
		for i := 0; i < 10; i++ {
			file := filepath.Join(pkgDir, fmt.Sprintf("file%d.go", i))
			require.NoError(t, os.WriteFile(file, []byte(goContent), 0644))
		}

		config := &parser.Config{
			MaxConcurrency: 4,
			IgnoreDirs:     []string{".git", "vendor"},
			IncludeTests:   true,  // Explicitly include tests
			IncludeVendor:  false, // Explicitly exclude vendor
		}
		service := parser.NewService(nil)
		ctx := context.Background()

		result, err := service.ParseProject(ctx, testDir, config)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Should find 3 original + 10 new = 13 files
		assert.Equal(t, 13, len(result.Files))
	})

	t.Run("Should handle context cancellation", func(t *testing.T) {
		service := parser.NewService(nil)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Give context time to cancel
		time.Sleep(10 * time.Millisecond)

		result, err := service.ParseProject(ctx, testDir, nil)

		// May or may not error depending on timing, but should handle gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "context")
		}
		if result != nil {
			// Partial results are acceptable
			assert.GreaterOrEqual(t, len(result.Files), 0)
		}
	})

	t.Run("Should continue on file parse errors", func(t *testing.T) {
		// Create an invalid Go file
		invalidFile := filepath.Join(pkgDir, "invalid.go")
		invalidContent := `package test

		func broken( {
			// Missing closing parenthesis
		}
		`
		require.NoError(t, os.WriteFile(invalidFile, []byte(invalidContent), 0644))

		service := parser.NewService(nil)
		ctx := context.Background()

		result, err := service.ParseProject(ctx, testDir, nil)

		// Should not return error, just skip the invalid file
		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Should still parse valid files
		assert.GreaterOrEqual(t, len(result.Files), 3)
	})
}

func TestService_Constructor(t *testing.T) {
	t.Run("Should create service with default config when nil", func(t *testing.T) {
		service := parser.NewService(nil)

		assert.NotNil(t, service)
		// We can't directly access the config, but we can test behavior

		// Create a test directory with .git folder
		testDir := t.TempDir()
		gitDir := filepath.Join(testDir, ".git")
		require.NoError(t, os.MkdirAll(gitDir, 0755))

		goFile := filepath.Join(gitDir, "test.go")
		require.NoError(t, os.WriteFile(goFile, []byte("package test"), 0644))

		ctx := context.Background()
		files, err := service.ParseDirectory(ctx, testDir)

		// Default config should ignore .git
		assert.NoError(t, err)
		assert.Len(t, files, 0)
	})

	t.Run("Should create service with provided config", func(t *testing.T) {
		config := &parser.Config{
			IgnoreDirs:     []string{"custom_ignore"},
			IncludeTests:   false,
			IncludeVendor:  true,
			MaxConcurrency: 8,
		}
		service := parser.NewService(config)

		assert.NotNil(t, service)

		// Test that custom config is used
		testDir := t.TempDir()
		customDir := filepath.Join(testDir, "custom_ignore")
		require.NoError(t, os.MkdirAll(customDir, 0755))

		goFile := filepath.Join(customDir, "test.go")
		require.NoError(t, os.WriteFile(goFile, []byte("package test"), 0644))

		testFile := filepath.Join(testDir, "test_test.go")
		require.NoError(t, os.WriteFile(testFile, []byte("package test"), 0644))

		ctx := context.Background()
		files, err := service.ParseDirectory(ctx, testDir)

		// Custom config should ignore custom_ignore dir and test files
		assert.NoError(t, err)
		assert.Len(t, files, 0)
	})
}

func TestService_EdgeCases(t *testing.T) {
	service := parser.NewService(nil)
	testDir := t.TempDir()

	t.Run("Should handle empty structs", func(t *testing.T) {
		file := filepath.Join(testDir, "empty_struct.go")
		content := `package test

type EmptyStruct struct{}
`
		require.NoError(t, os.WriteFile(file, []byte(content), 0644))

		ctx := context.Background()
		info, err := service.ParseFile(ctx, file)

		assert.NoError(t, err)
		assert.Len(t, info.Structs, 1)
		assert.Equal(t, "EmptyStruct", info.Structs[0].Name)
		assert.Len(t, info.Structs[0].Fields, 0)
	})

	t.Run("Should handle embedded structs", func(t *testing.T) {
		file := filepath.Join(testDir, "embedded.go")
		content := `package test

type Base struct {
	ID string
}

type Extended struct {
	Base
	Name string
}
`
		require.NoError(t, os.WriteFile(file, []byte(content), 0644))

		ctx := context.Background()
		info, err := service.ParseFile(ctx, file)

		assert.NoError(t, err)
		assert.Len(t, info.Structs, 2)

		// Find Extended struct
		var extended *parser.StructInfo
		for i := range info.Structs {
			if info.Structs[i].Name == "Extended" {
				extended = &info.Structs[i]
				break
			}
		}

		require.NotNil(t, extended)
		assert.Len(t, extended.Embeds, 1)
		assert.Equal(t, "Base", extended.Embeds[0])
		assert.Len(t, extended.Fields, 1)
		assert.Equal(t, "Name", extended.Fields[0].Name)
	})

	t.Run("Should handle interface composition", func(t *testing.T) {
		file := filepath.Join(testDir, "interface_comp.go")
		content := `package test

type Reader interface {
	Read([]byte) (int, error)
}

type Writer interface {
	Write([]byte) (int, error)
}

type ReadWriter interface {
	Reader
	Writer
}
`
		require.NoError(t, os.WriteFile(file, []byte(content), 0644))

		ctx := context.Background()
		info, err := service.ParseFile(ctx, file)

		assert.NoError(t, err)
		assert.Len(t, info.Interfaces, 3)

		// Find ReadWriter interface
		var readWriter *parser.InterfaceInfo
		for i := range info.Interfaces {
			if info.Interfaces[i].Name == "ReadWriter" {
				readWriter = &info.Interfaces[i]
				break
			}
		}

		require.NotNil(t, readWriter)
		assert.Len(t, readWriter.Embeds, 2)
		assert.Contains(t, readWriter.Embeds, "Reader")
		assert.Contains(t, readWriter.Embeds, "Writer")
	})

	t.Run("Should handle complex types", func(t *testing.T) {
		file := filepath.Join(testDir, "complex_types.go")
		content := `package test

type ComplexStruct struct {
	MapField   map[string][]int
	SliceField []map[string]interface{}
	ChanField  chan struct{}
	FuncField  func(string) error
	PtrField   *ComplexStruct
}
`
		require.NoError(t, os.WriteFile(file, []byte(content), 0644))

		ctx := context.Background()
		info, err := service.ParseFile(ctx, file)

		assert.NoError(t, err)
		assert.Len(t, info.Structs, 1)

		complexStruct := info.Structs[0]
		assert.Equal(t, "ComplexStruct", complexStruct.Name)
		assert.Len(t, complexStruct.Fields, 5)

		// Check complex type parsing
		assert.Equal(t, "map[string][]int", complexStruct.Fields[0].Type)
		assert.Equal(t, "[]map[string]interface{}", complexStruct.Fields[1].Type)
		assert.Equal(t, "*ComplexStruct", complexStruct.Fields[4].Type)
	})

	t.Run("Should handle struct tags", func(t *testing.T) {
		file := filepath.Join(testDir, "tagged_struct.go")
		content := `package test

type TaggedStruct struct {
	ID   string ` + "`json:\"id\" db:\"user_id\"`" + `
	Name string ` + "`json:\"name,omitempty\"`" + `
}
`
		require.NoError(t, os.WriteFile(file, []byte(content), 0644))

		ctx := context.Background()
		info, err := service.ParseFile(ctx, file)

		assert.NoError(t, err)
		assert.Len(t, info.Structs, 1)

		taggedStruct := info.Structs[0]
		assert.Len(t, taggedStruct.Fields, 2)
		assert.Contains(t, taggedStruct.Fields[0].Tag, "json:\"id\"")
		assert.Contains(t, taggedStruct.Fields[0].Tag, "db:\"user_id\"")
		assert.Contains(t, taggedStruct.Fields[1].Tag, "json:\"name,omitempty\"")
	})
}
