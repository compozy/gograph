package parser_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/gograph/engine/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_ParseProject(t *testing.T) {
	// Create a test Go file
	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "test.go")
	testContent := `package main

import (
	"fmt"
	"time"
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
	// Create go.mod file
	goModContent := `module testproject

go 1.21
`
	err := os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goModContent), 0644)
	require.NoError(t, err)
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	service := parser.NewService(nil)

	t.Run("Should parse a Go project successfully", func(t *testing.T) {
		ctx := context.Background()
		result, err := service.ParseProject(ctx, testDir, nil)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testDir, result.ProjectPath)
		assert.GreaterOrEqual(t, len(result.Packages), 1)

		// Find main package
		var mainPkg *parser.PackageInfo
		for _, pkg := range result.Packages {
			if pkg.Name == "main" {
				mainPkg = pkg
				break
			}
		}
		assert.NotNil(t, mainPkg)
	})

	t.Run("Should extract imports correctly", func(t *testing.T) {
		ctx := context.Background()
		result, err := service.ParseProject(ctx, testDir, nil)

		require.NoError(t, err)
		// Find main package
		var mainPkg *parser.PackageInfo
		for _, pkg := range result.Packages {
			if pkg.Name == "main" {
				mainPkg = pkg
				break
			}
		}
		require.NotNil(t, mainPkg)
		assert.GreaterOrEqual(t, len(mainPkg.Files), 1)

		// Check imports in first file
		file := mainPkg.Files[0]
		assert.Len(t, file.Imports, 2)
		assert.Equal(t, "fmt", file.Imports[0].Path)
		assert.Equal(t, "time", file.Imports[1].Path)
	})

	t.Run("Should extract types correctly", func(t *testing.T) {
		ctx := context.Background()
		result, err := service.ParseProject(ctx, testDir, nil)

		require.NoError(t, err)
		// Find main package
		var mainPkg *parser.PackageInfo
		for _, pkg := range result.Packages {
			if pkg.Name == "main" {
				mainPkg = pkg
				break
			}
		}
		require.NotNil(t, mainPkg)

		// Should have User struct in types
		var userType *parser.TypeInfo
		for _, t := range mainPkg.Types {
			if t.Name == "User" {
				userType = t
				break
			}
		}
		require.NotNil(t, userType)
		assert.True(t, userType.IsExported)
		assert.Len(t, userType.Fields, 3)

		// Check fields
		assert.Equal(t, "ID", userType.Fields[0].Name)
		assert.True(t, userType.Fields[0].IsExported)

		assert.Equal(t, "Name", userType.Fields[1].Name)
		assert.Equal(t, "Age", userType.Fields[2].Name)
	})

	t.Run("Should extract functions correctly", func(t *testing.T) {
		ctx := context.Background()
		result, err := service.ParseProject(ctx, testDir, nil)

		require.NoError(t, err)
		// Find main package
		var mainPkg *parser.PackageInfo
		for _, pkg := range result.Packages {
			if pkg.Name == "main" {
				mainPkg = pkg
				break
			}
		}
		require.NotNil(t, mainPkg)

		// Should have NewUser, GetName (method), and generateID
		assert.GreaterOrEqual(t, len(mainPkg.Functions), 3)

		// Find NewUser function
		var newUserFunc *parser.FunctionInfo
		for _, fn := range mainPkg.Functions {
			if fn.Name == "NewUser" {
				newUserFunc = fn
				break
			}
		}

		require.NotNil(t, newUserFunc)
		assert.True(t, newUserFunc.IsExported)
		assert.NotNil(t, newUserFunc.Signature)
	})

	t.Run("Should extract methods correctly", func(t *testing.T) {
		ctx := context.Background()
		result, err := service.ParseProject(ctx, testDir, nil)

		require.NoError(t, err)
		// Find main package
		var mainPkg *parser.PackageInfo
		for _, pkg := range result.Packages {
			if pkg.Name == "main" {
				mainPkg = pkg
				break
			}
		}
		require.NotNil(t, mainPkg)

		// Find GetName method
		var getNameMethod *parser.FunctionInfo
		for _, fn := range mainPkg.Functions {
			if fn.Name == "GetName" && fn.Receiver != nil {
				getNameMethod = fn
				break
			}
		}

		require.NotNil(t, getNameMethod)
		assert.NotNil(t, getNameMethod.Receiver)
		// Receiver name includes pointer notation
		assert.Contains(t, getNameMethod.Receiver.Name, "User")
		assert.True(t, getNameMethod.IsExported)
	})

	t.Run("Should extract interfaces correctly", func(t *testing.T) {
		ctx := context.Background()
		result, err := service.ParseProject(ctx, testDir, nil)

		require.NoError(t, err)
		// Find UserService interface in result.Interfaces
		var userService *parser.InterfaceInfo
		for _, iface := range result.Interfaces {
			if iface.Name == "UserService" {
				userService = iface
				break
			}
		}
		require.NotNil(t, userService)
		assert.True(t, userService.IsExported)
		assert.Len(t, userService.Methods, 2)

		// Check GetUser method
		getUserMethod := userService.Methods[0]
		assert.Equal(t, "GetUser", getUserMethod.Name)
	})

	t.Run("Should handle context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		result, err := service.ParseProject(ctx, testDir, nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("Should handle non-existent directory", func(t *testing.T) {
		ctx := context.Background()

		result, err := service.ParseProject(ctx, "/non/existent/directory", nil)

		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should handle invalid Go syntax", func(t *testing.T) {
		invalidDir := t.TempDir()
		invalidFile := filepath.Join(invalidDir, "invalid.go")
		invalidContent := `package main

func broken( {
	// Missing closing parenthesis
}
`
		err := os.WriteFile(filepath.Join(invalidDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(invalidFile, []byte(invalidContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := service.ParseProject(ctx, invalidDir, nil)

		// With the new error handling, syntax errors now cause failures
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "package loading failed")
	})

	t.Run("Should extract import aliases correctly", func(t *testing.T) {
		aliasDir := t.TempDir()
		aliasFile := filepath.Join(aliasDir, "alias.go")
		aliasContent := `package main

import (
	f "fmt"
	_ "strings"
	. "time"
)

func main() {
	f.Println("Hello")
	_ = Now() // Use time.Now to avoid unused import error
}
`
		err := os.WriteFile(filepath.Join(aliasDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(aliasFile, []byte(aliasContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := service.ParseProject(ctx, aliasDir, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.GreaterOrEqual(t, len(result.Packages), 1)

		mainPkg := result.Packages[0]
		require.GreaterOrEqual(t, len(mainPkg.Files), 1)

		file := mainPkg.Files[0]
		assert.Len(t, file.Imports, 3)

		// Check aliases
		for _, imp := range file.Imports {
			switch imp.Path {
			case "fmt":
				assert.Equal(t, "f", imp.Name)
			case "strings":
				assert.Equal(t, "_", imp.Name)
			case "time":
				assert.Equal(t, ".", imp.Name)
			}
		}
	})
}

func TestService_ParseProject_AdvancedFeatures(t *testing.T) {
	service := parser.NewService(nil)

	t.Run("Should detect interface implementations", func(t *testing.T) {
		implDir := t.TempDir()
		implFile := filepath.Join(implDir, "impl.go")
		implContent := `package main

import "io"

type Writer interface {
	Write([]byte) (int, error)
}

type FileWriter struct {
	path string
}

func (f *FileWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

type Buffer struct{}

func (b Buffer) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// Also implements io.Writer
var _ io.Writer = &FileWriter{}
var _ io.Writer = Buffer{}
`
		goModContent := `module testimpl

go 1.21
`
		err := os.WriteFile(filepath.Join(implDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(implFile, []byte(implContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := service.ParseProject(ctx, implDir, nil)

		require.NoError(t, err)
		require.NotNil(t, result)

		// Check interfaces
		assert.GreaterOrEqual(t, len(result.Interfaces), 1)

		// Find Writer interface
		var writerIface *parser.InterfaceInfo
		for _, iface := range result.Interfaces {
			if iface.Name == "Writer" {
				writerIface = iface
				break
			}
		}

		require.NotNil(t, writerIface)
		assert.GreaterOrEqual(t, len(writerIface.Implementations), 2)

		// Check implementations
		implNames := make(map[string]bool)
		for _, impl := range writerIface.Implementations {
			implNames[impl.Type.Name] = true
		}
		assert.True(t, implNames["FileWriter"])
		assert.True(t, implNames["Buffer"])
	})

	t.Run("Should parse function signatures with complex types", func(t *testing.T) {
		sigDir := t.TempDir()
		sigFile := filepath.Join(sigDir, "signatures.go")
		sigContent := `package main

import "context"

// Complex function signatures
func ProcessData(ctx context.Context, data []byte, opts ...Option) (result *Result, err error) {
	return nil, nil
}

func HandleRequest(fn func(string) error) func(int) bool {
	return func(x int) bool {
		return fn("test") == nil
	}
}

type Option func(*Config)
type Result struct{}
type Config struct{}
`
		goModContent := `module testsig

go 1.21
`
		err := os.WriteFile(filepath.Join(sigDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(sigFile, []byte(sigContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := service.ParseProject(ctx, sigDir, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.GreaterOrEqual(t, len(result.Packages), 1)

		mainPkg := result.Packages[0]

		// Find ProcessData function
		var processFunc *parser.FunctionInfo
		for _, fn := range mainPkg.Functions {
			if fn.Name == "ProcessData" {
				processFunc = fn
				break
			}
		}

		require.NotNil(t, processFunc)
		assert.NotNil(t, processFunc.Signature)
		// Verify signature has parameters and results
		assert.NotNil(t, processFunc.Signature.Params())
		assert.NotNil(t, processFunc.Signature.Results())
	})

	t.Run("Should handle embedded interfaces", func(t *testing.T) {
		embedDir := t.TempDir()
		embedFile := filepath.Join(embedDir, "embed.go")
		embedContent := `package main

import "io"

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

type Closer interface {
	Close() error
}

type ReadWriteCloser interface {
	ReadWriter
	Closer
}

// Embedding external interface
type MyReadCloser interface {
	io.Reader
	Closer
}
`
		goModContent := `module testembed

go 1.21
`
		err := os.WriteFile(filepath.Join(embedDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(embedFile, []byte(embedContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := service.ParseProject(ctx, embedDir, nil)

		require.NoError(t, err)
		require.NotNil(t, result)

		// Find ReadWriter interface
		var readWriter *parser.InterfaceInfo
		for _, iface := range result.Interfaces {
			if iface.Name == "ReadWriter" {
				readWriter = iface
				break
			}
		}

		require.NotNil(t, readWriter)
		assert.Len(t, readWriter.Embeds, 2)

		// Find ReadWriteCloser interface
		var rwc *parser.InterfaceInfo
		for _, iface := range result.Interfaces {
			if iface.Name == "ReadWriteCloser" {
				rwc = iface
				break
			}
		}

		require.NotNil(t, rwc)
		assert.GreaterOrEqual(t, len(rwc.Embeds), 2)
	})
}

func TestService_TypeInformationAccuracy(t *testing.T) {
	t.Run("Should extract accurate type information with Go type system", func(t *testing.T) {
		testDir := t.TempDir()
		testFile := filepath.Join(testDir, "types.go")
		testContent := `package main

import (
	"fmt"
	"time"
)

// User represents a user with detailed type information
type User struct {
	ID        string    ` + "`json:\"id\"`" + `
	Name      string    ` + "`json:\"name\"`" + `
	Age       int       ` + "`json:\"age\"`" + `
	Email     *string   ` + "`json:\"email,omitempty\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\"`" + `
	Metadata  map[string]interface{} ` + "`json:\"metadata\"`" + `
}

// UserService provides user operations
type UserService interface {
	GetUser(id string) (*User, error)
	CreateUser(user *User) (*User, error)
	UpdateUser(id string, updates map[string]interface{}) (*User, error)
	DeleteUser(id string) error
	ListUsers(limit int, offset int) ([]*User, error)
}

// HTTPUserService implements UserService via HTTP
type HTTPUserService struct {
	client   HttpClient
	baseURL  string
	timeout  time.Duration
	retries  int
}

type HttpClient interface {
	Do(req *HttpRequest) (*HttpResponse, error)
}

type HttpRequest struct {
	Method string
	URL    string
	Body   []byte
}

type HttpResponse struct {
	StatusCode int
	Body       []byte
}

// GetUser retrieves a user by ID
func (s *HTTPUserService) GetUser(id string) (*User, error) {
	req := &HttpRequest{
		Method: "GET",
		URL:    fmt.Sprintf("%s/users/%s", s.baseURL, id),
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	return parseUser(resp.Body), nil
}

func parseUser(data []byte) *User {
	return &User{}
}

// Helper function with complex signature
func ProcessUsers(users []*User, filter func(*User) bool, transformer func(*User) *User) []*User {
	var result []*User
	for _, user := range users {
		if filter(user) {
			result = append(result, transformer(user))
		}
	}
	return result
}

const (
	MaxUsersPerPage = 100
	DefaultTimeout  = 30
)

var (
	defaultClient = &HttpRequest{}
	userCounter   int64
)
`
		goModContent := `module testtypes

go 1.21
`
		err := os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err)

		service := parser.NewService(nil)
		ctx := context.Background()
		result, err := service.ParseProject(ctx, testDir, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.GreaterOrEqual(t, len(result.Packages), 1)

		// Find main package
		var mainPkg *parser.PackageInfo
		for _, pkg := range result.Packages {
			if pkg.Name == "main" {
				mainPkg = pkg
				break
			}
		}
		require.NotNil(t, mainPkg)

		// Validate User struct type information
		var userType *parser.TypeInfo
		for _, t := range mainPkg.Types {
			if t.Name == "User" {
				userType = t
				break
			}
		}
		require.NotNil(t, userType)
		assert.True(t, userType.IsExported)
		assert.NotNil(t, userType.Type)
		assert.NotNil(t, userType.Underlying)

		// Validate struct fields with accurate type information
		require.Len(t, userType.Fields, 6)

		// Check ID field
		idField := userType.Fields[0]
		assert.Equal(t, "ID", idField.Name)
		assert.True(t, idField.IsExported)
		assert.Equal(t, "json:\"id\"", idField.Tag)
		assert.Equal(t, "string", idField.Type.String())

		// Check Email field (pointer type)
		emailField := userType.Fields[3]
		assert.Equal(t, "Email", emailField.Name)
		assert.True(t, emailField.IsExported)
		assert.Equal(t, "json:\"email,omitempty\"", emailField.Tag)
		assert.Equal(t, "*string", emailField.Type.String())

		// Check CreatedAt field (external type)
		createdAtField := userType.Fields[4]
		assert.Equal(t, "CreatedAt", createdAtField.Name)
		assert.True(t, createdAtField.IsExported)
		assert.Contains(t, createdAtField.Type.String(), "time.Time")

		// Check Metadata field (complex type)
		metadataField := userType.Fields[5]
		assert.Equal(t, "Metadata", metadataField.Name)
		assert.True(t, metadataField.IsExported)
		assert.Contains(t, metadataField.Type.String(), "map[string]interface{}")

		// Validate function signatures with accurate type information
		var getUserFunc *parser.FunctionInfo
		for _, fn := range mainPkg.Functions {
			if fn.Name == "GetUser" && fn.Receiver != nil {
				getUserFunc = fn
				break
			}
		}
		require.NotNil(t, getUserFunc)
		assert.NotNil(t, getUserFunc.Signature)
		assert.NotNil(t, getUserFunc.Receiver)
		assert.Contains(t, getUserFunc.Receiver.Name, "HTTPUserService")

		// Validate complex function signature
		var processFunc *parser.FunctionInfo
		for _, fn := range mainPkg.Functions {
			if fn.Name == "ProcessUsers" {
				processFunc = fn
				break
			}
		}
		require.NotNil(t, processFunc)
		assert.NotNil(t, processFunc.Signature)

		// Verify signature has the correct number of parameters and results
		sig := processFunc.Signature
		assert.Equal(t, 3, sig.Params().Len())  // users, filter, transformer
		assert.Equal(t, 1, sig.Results().Len()) // []*User

		// Validate interface type information
		var userServiceInterface *parser.InterfaceInfo
		for _, iface := range result.Interfaces {
			if iface.Name == "UserService" {
				userServiceInterface = iface
				break
			}
		}
		require.NotNil(t, userServiceInterface)
		assert.True(t, userServiceInterface.IsExported)
		assert.Len(t, userServiceInterface.Methods, 5)

		// Check that we have the expected methods (ordering may vary)
		methodNames := make(map[string]*parser.MethodInfo)
		for _, method := range userServiceInterface.Methods {
			methodNames[method.Name] = method
		}

		// Verify GetUser method exists and has correct signature
		getUserMethod, exists := methodNames["GetUser"]
		require.True(t, exists)
		assert.NotNil(t, getUserMethod.Signature)
		assert.Equal(t, 1, getUserMethod.Signature.Params().Len())  // id string
		assert.Equal(t, 2, getUserMethod.Signature.Results().Len()) // (*User, error)

		// Validate interface implementation detection (may not be detected if no explicit implementation)
		// Note: This complex example may not trigger automatic interface implementation detection
		// as it's based on structural typing which requires the actual method implementations to be analyzed
		if len(userServiceInterface.Implementations) > 0 {
			impl := userServiceInterface.Implementations[0]
			assert.Equal(t, "HTTPUserService", impl.Type.Name)
			assert.True(t, impl.IsComplete)
		}

		// Validate embedded interface in HttpClient
		var httpClientInterface *parser.InterfaceInfo
		for _, iface := range result.Interfaces {
			if iface.Name == "HttpClient" {
				httpClientInterface = iface
				break
			}
		}
		require.NotNil(t, httpClientInterface)
		assert.True(t, httpClientInterface.IsExported)
		assert.Len(t, httpClientInterface.Methods, 1)

		// Check Do method signature with complex types
		doMethod := httpClientInterface.Methods[0]
		assert.Equal(t, "Do", doMethod.Name)
		assert.NotNil(t, doMethod.Signature)
	})
}

func TestService_ConstantsAndVariablesParsing(t *testing.T) {
	t.Run("Should parse constants and variables correctly", func(t *testing.T) {
		testDir := t.TempDir()
		testFile := filepath.Join(testDir, "constants_vars.go")
		testContent := `package main

import "time"

// Constants
const (
	MaxUsersPerPage = 100
	DefaultTimeout  = 30 * time.Second
	AppName         = "MyApp"
	IsDebug         = true
)

const SingleConstant = "single value"

// Variables
var (
	defaultUser = &User{Name: "Default", Age: 0}
	userCache   map[string]*User
	counter     int64
)

var singleVar = "single variable"

type User struct {
	Name string
	Age  int
}
`
		goModContent := `module testconstvars

go 1.21
`
		err := os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err)

		service := parser.NewService(nil)
		ctx := context.Background()
		result, err := service.ParseProject(ctx, testDir, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.GreaterOrEqual(t, len(result.Packages), 1)

		// Find main package
		var mainPkg *parser.PackageInfo
		for _, pkg := range result.Packages {
			if pkg.Name == "main" {
				mainPkg = pkg
				break
			}
		}
		require.NotNil(t, mainPkg)

		// Validate constants
		assert.GreaterOrEqual(t, len(mainPkg.Constants), 4)

		// Find MaxUsersPerPage constant
		var maxUsersConst *parser.ConstantInfo
		for _, c := range mainPkg.Constants {
			if c.Name == "MaxUsersPerPage" {
				maxUsersConst = c
				break
			}
		}
		require.NotNil(t, maxUsersConst)
		assert.True(t, maxUsersConst.IsExported)
		assert.NotNil(t, maxUsersConst.Type)
		assert.Equal(t, "100", maxUsersConst.Value)

		// Find AppName constant
		var appNameConst *parser.ConstantInfo
		for _, c := range mainPkg.Constants {
			if c.Name == "AppName" {
				appNameConst = c
				break
			}
		}
		require.NotNil(t, appNameConst)
		assert.True(t, appNameConst.IsExported)
		assert.Equal(t, "\"MyApp\"", appNameConst.Value)

		// Find SingleConstant
		var singleConst *parser.ConstantInfo
		for _, c := range mainPkg.Constants {
			if c.Name == "SingleConstant" {
				singleConst = c
				break
			}
		}
		require.NotNil(t, singleConst)
		assert.True(t, singleConst.IsExported)
		assert.Equal(t, "\"single value\"", singleConst.Value)

		// Validate variables
		assert.GreaterOrEqual(t, len(mainPkg.Variables), 3)

		// Find defaultUser variable
		var defaultUserVar *parser.VariableInfo
		for _, v := range mainPkg.Variables {
			if v.Name == "defaultUser" {
				defaultUserVar = v
				break
			}
		}
		require.NotNil(t, defaultUserVar)
		assert.False(t, defaultUserVar.IsExported)
		assert.NotNil(t, defaultUserVar.Type)
		assert.Contains(t, defaultUserVar.Value, "&User{")

		// Find userCache variable
		var userCacheVar *parser.VariableInfo
		for _, v := range mainPkg.Variables {
			if v.Name == "userCache" {
				userCacheVar = v
				break
			}
		}
		require.NotNil(t, userCacheVar)
		assert.False(t, userCacheVar.IsExported)
		assert.NotNil(t, userCacheVar.Type)
		assert.Contains(t, userCacheVar.Type.String(), "map[string]*")

		// Find singleVar variable
		var singleVarVar *parser.VariableInfo
		for _, v := range mainPkg.Variables {
			if v.Name == "singleVar" {
				singleVarVar = v
				break
			}
		}
		require.NotNil(t, singleVarVar)
		assert.False(t, singleVarVar.IsExported)
		assert.Equal(t, "\"single variable\"", singleVarVar.Value)

		// Validate line numbers are set
		assert.Greater(t, maxUsersConst.LineStart, 0)
		assert.Greater(t, defaultUserVar.LineStart, 0)
	})
}

func TestService_BackwardCompatibilityAPIs(t *testing.T) {
	service := parser.NewService(nil)

	t.Run("Should parse single file with ParseFile", func(t *testing.T) {
		testDir := t.TempDir()
		testFile := filepath.Join(testDir, "single.go")
		testContent := `package main

import "fmt"

type Config struct {
	Name string
	Port int
}

func main() {
	fmt.Println("Hello")
}

const DefaultPort = 8080
var globalConfig = &Config{}
`
		goModContent := `module testsingle

go 1.21
`
		err := os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := service.ParseFile(ctx, testFile, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, testFile, result.FilePath)
		assert.NotNil(t, result.Package)
		assert.NotNil(t, result.FileInfo)
		assert.Greater(t, result.ParseTime, int64(0))

		// Verify package contains the expected content
		assert.Equal(t, "main", result.Package.Name)
		assert.GreaterOrEqual(t, len(result.Package.Types), 1)
		assert.GreaterOrEqual(t, len(result.Package.Functions), 1)
		assert.GreaterOrEqual(t, len(result.Package.Constants), 1)
		assert.GreaterOrEqual(t, len(result.Package.Variables), 1)

		// Verify file info
		assert.Equal(t, testFile, result.FileInfo.Path)
		assert.Equal(t, "main", result.FileInfo.Package)
		assert.GreaterOrEqual(t, len(result.FileInfo.Imports), 1)
	})

	t.Run("Should parse directory with ParseDirectory", func(t *testing.T) {
		testDir := t.TempDir()

		// Create multiple Go files in the directory
		file1 := filepath.Join(testDir, "main.go")
		file1Content := `package main

import "fmt"

func main() {
	fmt.Println("Hello from main")
}
`

		file2 := filepath.Join(testDir, "utils.go")
		file2Content := `package main

func Helper() string {
	return "helper"
}
`

		goModContent := `module testdir

go 1.21
`
		err := os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(file1, []byte(file1Content), 0644)
		require.NoError(t, err)
		err = os.WriteFile(file2, []byte(file2Content), 0644)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := service.ParseDirectory(ctx, testDir, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, testDir, result.DirectoryPath)
		assert.GreaterOrEqual(t, len(result.Packages), 1)
		assert.Greater(t, result.ParseTime, int64(0))

		// Find main package
		var mainPkg *parser.PackageInfo
		for _, pkg := range result.Packages {
			if pkg.Name == "main" {
				mainPkg = pkg
				break
			}
		}
		require.NotNil(t, mainPkg)

		// Verify package contains functions from both files
		assert.GreaterOrEqual(t, len(mainPkg.Functions), 2) // main and Helper
		assert.GreaterOrEqual(t, len(mainPkg.Files), 2)     // main.go and utils.go
	})

	t.Run("Should handle file validation errors", func(t *testing.T) {
		ctx := context.Background()

		// Test non-existent file
		result, err := service.ParseFile(ctx, "/non/existent/file.go", nil)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "file does not exist")

		// Test directory instead of file
		tempDir := t.TempDir()
		result, err = service.ParseFile(ctx, tempDir, nil)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "path is a directory")

		// Test non-Go file
		nonGoFile := filepath.Join(tempDir, "test.txt")
		err = os.WriteFile(nonGoFile, []byte("not go"), 0644)
		require.NoError(t, err)
		result, err = service.ParseFile(ctx, nonGoFile, nil)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not a Go source file")
	})
}

func TestService_SSAPerformanceMonitoring(t *testing.T) {
	service := parser.NewService(nil)

	t.Run("Should collect performance metrics when enabled", func(t *testing.T) {
		testDir := t.TempDir()
		testFile := filepath.Join(testDir, "perf.go")
		testContent := `package main

import "fmt"

type Data struct {
	Value int
	Name  string
}

func ProcessData(d *Data) *Data {
	return &Data{Value: d.Value * 2, Name: d.Name}
}

func main() {
	d := &Data{Value: 42, Name: "test"}
	result := ProcessData(d)
	fmt.Printf("Result: %+v\n", result)
}
`
		goModContent := `module testperf

go 1.21
`
		err := os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err)

		// Enable performance monitoring
		config := &parser.Config{
			EnableSSA:              true,
			EnablePerformanceStats: true,
			IncludeTests:           false,
			IncludeVendor:          false,
		}

		ctx := context.Background()
		result, err := service.ParseProject(ctx, testDir, config)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.PerformanceStats)

		// Verify performance metrics are collected
		perfStats := result.PerformanceStats
		assert.Greater(t, perfStats.BuildDuration, time.Duration(0))
		assert.Greater(t, perfStats.PreparationTime, time.Duration(0))
		assert.Greater(t, perfStats.ConstructionTime, time.Duration(0))
		assert.Greater(t, perfStats.PackagesProcessed, 0)
		assert.Greater(t, perfStats.FunctionsAnalyzed, 0)
		assert.GreaterOrEqual(t, perfStats.MemoryUsageMB, int64(0))

		// Verify phase breakdown is populated
		assert.NotNil(t, perfStats.PhaseBreakdown)
		assert.Greater(t, len(perfStats.PhaseBreakdown), 0)
		assert.Contains(t, perfStats.PhaseBreakdown, "preparation")
		assert.Contains(t, perfStats.PhaseBreakdown, "construction")

		// Verify memory profile is collected
		assert.NotNil(t, perfStats.MemoryProfile)
		assert.GreaterOrEqual(t, perfStats.MemoryProfile.InitialMemoryMB, int64(0))
		assert.GreaterOrEqual(t, perfStats.MemoryProfile.FinalMemoryMB, int64(0))
		assert.GreaterOrEqual(t, perfStats.MemoryProfile.PeakMemoryMB, perfStats.MemoryProfile.InitialMemoryMB)
	})

	t.Run("Should not collect performance metrics when disabled", func(t *testing.T) {
		testDir := t.TempDir()
		testFile := filepath.Join(testDir, "noperf.go")
		testContent := `package main

func main() {
	println("Hello")
}
`
		goModContent := `module testnoperf

go 1.21
`
		err := os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err)

		// Disable performance monitoring
		config := &parser.Config{
			EnableSSA:              true,
			EnablePerformanceStats: false,
			IncludeTests:           false,
			IncludeVendor:          false,
		}

		ctx := context.Background()
		result, err := service.ParseProject(ctx, testDir, config)

		require.NoError(t, err)
		require.NotNil(t, result)
		// Performance stats should be nil when disabled
		assert.Nil(t, result.PerformanceStats)
	})

	t.Run("Should handle SSA disabled with performance monitoring", func(t *testing.T) {
		testDir := t.TempDir()
		testFile := filepath.Join(testDir, "noSSA.go")
		testContent := `package main

func main() {
	println("No SSA")
}
`
		goModContent := `module testnoSSA

go 1.21
`
		err := os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err)

		// Enable performance monitoring but disable SSA
		config := &parser.Config{
			EnableSSA:              false,
			EnablePerformanceStats: true,
			IncludeTests:           false,
			IncludeVendor:          false,
		}

		ctx := context.Background()
		result, err := service.ParseProject(ctx, testDir, config)

		require.NoError(t, err)
		require.NotNil(t, result)
		// Performance stats should be nil when SSA is disabled
		assert.Nil(t, result.PerformanceStats)
		assert.Nil(t, result.SSAProgram)
	})

	t.Run("Should measure realistic performance metrics", func(t *testing.T) {
		testDir := t.TempDir()
		testFile := filepath.Join(testDir, "complex.go")
		// More complex code to generate meaningful SSA metrics
		testContent := `package main

import (
	"fmt"
	"sort"
	"strings"
)

type Person struct {
	Name string
	Age  int
}

type PersonService interface {
	GetPerson(name string) *Person
	SavePerson(p *Person) error
}

type InMemoryPersonService struct {
	data map[string]*Person
}

func NewInMemoryPersonService() PersonService {
	return &InMemoryPersonService{
		data: make(map[string]*Person),
	}
}

func (s *InMemoryPersonService) GetPerson(name string) *Person {
	return s.data[name]
}

func (s *InMemoryPersonService) SavePerson(p *Person) error {
	s.data[p.Name] = p
	return nil
}

func ProcessPeople(people []*Person) []*Person {
	filtered := make([]*Person, 0)
	for _, p := range people {
		if p.Age >= 18 {
			filtered = append(filtered, p)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return strings.Compare(filtered[i].Name, filtered[j].Name) < 0
	})
	return filtered
}

func main() {
	service := NewInMemoryPersonService()
	people := []*Person{
		{Name: "Alice", Age: 25},
		{Name: "Bob", Age: 17},
		{Name: "Carol", Age: 30},
	}
	for _, p := range people {
		service.SavePerson(p)
	}
	adults := ProcessPeople(people)
	fmt.Printf("Adults: %+v\n", adults)
}
`
		goModContent := `module testcomplex

go 1.21
`
		err := os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goModContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err)

		// Enable performance monitoring
		config := &parser.Config{
			EnableSSA:              true,
			EnablePerformanceStats: true,
			IncludeTests:           false,
			IncludeVendor:          false,
		}

		ctx := context.Background()
		result, err := service.ParseProject(ctx, testDir, config)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.PerformanceStats)

		perfStats := result.PerformanceStats
		// Should have analyzed multiple functions
		assert.GreaterOrEqual(
			t,
			perfStats.FunctionsAnalyzed,
			5,
		) // main, NewInMemoryPersonService, GetPerson, SavePerson, ProcessPeople
		assert.Equal(t, 1, perfStats.PackagesProcessed) // main package only

		// Verify timing relationships
		assert.LessOrEqual(t, perfStats.PreparationTime, perfStats.BuildDuration)
		assert.LessOrEqual(t, perfStats.ConstructionTime, perfStats.BuildDuration)
		assert.GreaterOrEqual(t, perfStats.BuildDuration, perfStats.PreparationTime+perfStats.ConstructionTime)

		// Verify memory usage tracking
		assert.GreaterOrEqual(t, perfStats.MemoryProfile.PeakMemoryMB, perfStats.MemoryProfile.InitialMemoryMB)
		assert.GreaterOrEqual(t, perfStats.MemoryProfile.FinalMemoryMB, int64(0))
	})
}
