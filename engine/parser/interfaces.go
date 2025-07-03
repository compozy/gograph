package parser

import (
	"context"
	"go/types"
	"time"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// Parser defines the interface for advanced Go source code analysis
type Parser interface {
	ParseProject(ctx context.Context, projectPath string, config *Config) (*ParseResult, error)
	// Backward compatibility methods
	ParseFile(ctx context.Context, filePath string, config *Config) (*FileResult, error)
	ParseDirectory(ctx context.Context, dirPath string, config *Config) (*DirectoryResult, error)
}

// ParseResult contains comprehensive analysis results
type ParseResult struct {
	ProjectPath      string                 // Root path of the project
	Packages         []*PackageInfo         // All analyzed packages
	SSAProgram       *ssa.Program           // SSA form of the program
	CallGraph        *CallGraph             // Complete call graph
	Interfaces       []*InterfaceInfo       // All interfaces with implementations
	ParseTime        int64                  // Time taken to parse in milliseconds
	PerformanceStats *SSAPerformanceMetrics // SSA build performance metrics (optional)
}

// FileResult contains results for single file parsing (backward compatibility)
type FileResult struct {
	FilePath  string       // Path to the parsed file
	Package   *PackageInfo // Package information containing the file
	FileInfo  *FileInfo    // Specific file information
	ParseTime int64        // Time taken to parse in milliseconds
}

// DirectoryResult contains results for directory parsing (backward compatibility)
type DirectoryResult struct {
	DirectoryPath string         // Path to the parsed directory
	Packages      []*PackageInfo // All packages found in the directory
	ParseTime     int64          // Time taken to parse in milliseconds
}

// PackageInfo represents analyzed package information
type PackageInfo struct {
	*packages.Package                  // Embedded package info
	Path              string           // Import path
	Name              string           // Package name
	Files             []*FileInfo      // Analyzed files
	Functions         []*FunctionInfo  // All functions and methods
	Types             []*TypeInfo      // All type declarations
	Interfaces        []*InterfaceInfo // Interface declarations
	Constants         []*ConstantInfo  // Constant declarations
	Variables         []*VariableInfo  // Variable declarations
	SSAPackage        *ssa.Package     // SSA representation
}

// FileInfo represents analyzed file information
type FileInfo struct {
	Path         string
	Package      string
	Imports      []*ImportInfo
	Functions    []*FunctionInfo
	Types        []*TypeInfo
	Constants    []*ConstantInfo
	Variables    []*VariableInfo
	Dependencies []string
}

// ImportInfo represents an import with resolved information
type ImportInfo struct {
	Name    string            // Local name (alias or package name)
	Path    string            // Import path
	Package *packages.Package // Resolved package
}

// FunctionInfo represents a function or method with type information
type FunctionInfo struct {
	Name       string
	Receiver   *TypeInfo // For methods
	Signature  *types.Signature
	SSAFunc    *ssa.Function // SSA representation
	Calls      []*FunctionCall
	CalledBy   []*FunctionInfo
	LineStart  int
	LineEnd    int
	IsExported bool
}

// FunctionCall represents a resolved function call
type FunctionCall struct {
	Function *FunctionInfo
	Position string
	Line     int
}

// TypeInfo represents any Go type (struct, interface, alias, etc.)
type TypeInfo struct {
	Name       string
	Type       types.Type
	Underlying types.Type
	Methods    []*FunctionInfo
	Fields     []*FieldInfo     // For structs
	Embeds     []*TypeInfo      // Embedded types
	Implements []*InterfaceInfo // Interfaces implemented
	LineStart  int
	LineEnd    int
	IsExported bool
}

// FieldInfo represents a struct field with full type information
type FieldInfo struct {
	Name       string
	Type       types.Type
	Tag        string
	IsExported bool
	Anonymous  bool // For embedded fields
}

// ConstantInfo represents a constant declaration
type ConstantInfo struct {
	Name       string
	Type       types.Type
	Value      string // String representation of the value
	IsExported bool
	LineStart  int
	LineEnd    int
}

// VariableInfo represents a variable declaration
type VariableInfo struct {
	Name       string
	Type       types.Type
	Value      string // String representation of the initial value (if any)
	IsExported bool
	LineStart  int
	LineEnd    int
}

// InterfaceInfo represents an interface with implementation tracking
type InterfaceInfo struct {
	Name            string
	Package         string // Package path where interface is declared
	Type            *types.Interface
	Methods         []*MethodInfo
	Embeds          []*InterfaceInfo
	Implementations []*Implementation
	LineStart       int
	LineEnd         int
	IsExported      bool
}

// MethodInfo represents an interface method
type MethodInfo struct {
	Name      string
	Signature *types.Signature
}

// Implementation represents a type implementing an interface
type Implementation struct {
	Type           *TypeInfo
	Interface      *InterfaceInfo
	IsComplete     bool
	MethodMatches  map[string]*FunctionInfo // Interface method -> implementation
	MissingMethods []string
}

// CallGraph represents the complete function call graph
type CallGraph struct {
	Root      *CallNode
	Functions map[string]*CallNode // Function key -> node
}

// CallNode represents a node in the call graph
type CallNode struct {
	Function *FunctionInfo
	Calls    []*CallNode
	CalledBy []*CallNode
}

// SSAPerformanceMetrics contains detailed performance metrics for SSA build operations
type SSAPerformanceMetrics struct {
	BuildDuration     time.Duration            // Total SSA build time
	PreparationTime   time.Duration            // Time to prepare SSA packages
	ConstructionTime  time.Duration            // Time for actual SSA construction
	PackagesProcessed int                      // Number of packages processed
	FunctionsAnalyzed int                      // Total functions in SSA program
	CallGraphNodes    int                      // Number of call graph nodes (if enabled)
	MemoryUsageMB     int64                    // Peak memory usage during SSA build in MB
	PhaseBreakdown    map[string]time.Duration // Detailed timing per SSA phase
	MemoryProfile     *SSAMemoryProfile        // Memory usage profile during build
}

// SSAMemoryProfile tracks memory usage during different SSA build phases
type SSAMemoryProfile struct {
	InitialMemoryMB     int64 // Memory usage before SSA build
	PreparationMemoryMB int64 // Memory after package preparation
	BuildMemoryMB       int64 // Memory after SSA construction
	PeakMemoryMB        int64 // Peak memory usage during build
	FinalMemoryMB       int64 // Memory usage after build completion
}

// Config represents parser configuration
type Config struct {
	IgnoreDirs             []string
	IgnoreFiles            []string
	IncludeTests           bool
	IncludeVendor          bool
	BuildTags              []string
	LoadMode               packages.LoadMode
	EnableSSA              bool
	EnableCallGraph        bool
	EnablePerformanceStats bool // Enable detailed performance monitoring for SSA builds
	EnableMemoryMonitoring bool // Enable memory monitoring during SSA builds (can impact performance)
}
