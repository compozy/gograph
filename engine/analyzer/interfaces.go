package analyzer

import (
	"context"

	"github.com/compozy/gograph/engine/parser"
)

// Analyzer defines the contract for code analysis operations
type Analyzer interface {
	// AnalyzeProject performs comprehensive analysis on parsed project data
	AnalyzeProject(ctx context.Context, input *AnalysisInput) (*AnalysisReport, error)

	// BuildDependencyGraph constructs a dependency graph from file imports
	BuildDependencyGraph(ctx context.Context, files []*parser.FileInfo) (*DependencyGraph, error)

	// DetectInterfaceImplementations finds all structs that implement interfaces
	DetectInterfaceImplementations(
		ctx context.Context,
		files []*parser.FileInfo,
	) ([]*InterfaceImplementation, error)

	// MapCallChains traces function call relationships
	MapCallChains(ctx context.Context, files []*parser.FileInfo) ([]*CallChain, error)

	// DetectCircularDependencies identifies circular import cycles
	DetectCircularDependencies(ctx context.Context, graph *DependencyGraph) ([]*CircularDependency, error)
}

// AnalysisInput contains the input data for analysis
type AnalysisInput struct {
	ProjectID string             // Project identifier
	Files     []*parser.FileInfo // Parsed file information
}

// DependencyGraph represents the project's dependency structure
type DependencyGraph struct {
	Nodes map[string]*DependencyNode // Map of file path to node
	Edges []*DependencyEdge          // All dependency relationships
	Root  string                     // Project root path
}

// DependencyNode represents a file or package in the dependency graph
type DependencyNode struct {
	Path         string   // File or package path
	Type         NodeType // File or Package
	Dependencies []string // Paths of direct dependencies
	Dependents   []string // Paths that depend on this node
}

// NodeType specifies the type of dependency node
type NodeType string

const (
	NodeTypeFile    NodeType = "file"
	NodeTypePackage NodeType = "package"
)

// DependencyEdge represents a dependency relationship
type DependencyEdge struct {
	From string         // Source path
	To   string         // Target path
	Type DependencyType // Import type
	Line int            // Line number of import
}

// DependencyType specifies the type of dependency
type DependencyType string

const (
	DependencyTypeImport    DependencyType = "import"
	DependencyTypeEmbed     DependencyType = "embed"
	DependencyTypeInterface DependencyType = "interface"
	DependencyTypeContains  DependencyType = "contains"
)

// InterfaceImplementation represents a struct implementing an interface
type InterfaceImplementation struct {
	Interface      *InterfaceRef // The interface being implemented with package info
	Implementor    *StructRef    // The struct implementing it with package info
	Methods        []MethodMatch // Matched methods
	IsComplete     bool          // Whether all methods are implemented
	MissingMethods []string      // Names of unimplemented methods
}

// InterfaceRef represents an interface with its package information
type InterfaceRef struct {
	*parser.InterfaceInfo
	Package  string // Package name
	FilePath string // File path where interface is defined
}

// StructRef represents a struct with its package information
type StructRef struct {
	*parser.StructInfo
	Package  string // Package name
	FilePath string // File path where struct is defined
}

// MethodMatch represents a matched interface method implementation
type MethodMatch struct {
	InterfaceMethod string // Interface method name
	StructMethod    string // Implementing method name
	Signature       string // Method signature
}

// CallChain represents a function call relationship
type CallChain struct {
	Caller      *FunctionReference // The calling function
	Callee      *FunctionReference // The called function
	CallSites   []CallSite         // Where the calls occur
	IsRecursive bool               // Whether this is a recursive call
}

// FunctionReference represents a reference to a function
type FunctionReference struct {
	Name      string // Function name
	Package   string // Package name
	Receiver  string // Receiver type if method
	Signature string // Full signature
}

// CallSite represents a specific location where a function is called
type CallSite struct {
	File       string // File path
	Line       int    // Line number
	Column     int    // Column position
	Expression string // Call expression
}

// CircularDependency represents a circular import cycle
type CircularDependency struct {
	Cycle    []string      // Ordered list of paths forming the cycle
	Severity SeverityLevel // Impact severity
	Impact   []string      // Affected components
}

// SeverityLevel indicates the severity of an issue
type SeverityLevel string

const (
	SeverityLow    SeverityLevel = "low"
	SeverityMedium SeverityLevel = "medium"
	SeverityHigh   SeverityLevel = "high"
)

// AnalysisReport contains comprehensive analysis results
type AnalysisReport struct {
	ProjectID                string                     // Project identifier
	Timestamp                int64                      // Analysis timestamp
	DependencyGraph          *DependencyGraph           // Project dependencies
	InterfaceImplementations []*InterfaceImplementation // Interface implementations
	CallChains               []*CallChain               // Function call relationships
	CircularDependencies     []*CircularDependency      // Circular import cycles
	Metrics                  *CodeMetrics               // Code quality metrics
}

// CodeMetrics contains code quality measurements
type CodeMetrics struct {
	TotalFiles           int            // Total number of files
	TotalLines           int            // Total lines of code
	TotalFunctions       int            // Total number of functions
	TotalInterfaces      int            // Total number of interfaces
	TotalStructs         int            // Total number of structs
	CyclomaticComplexity map[string]int // Function complexity scores
	TestCoverage         float64        // Percentage of code covered by tests
	DependencyDepth      int            // Maximum dependency chain length
	CouplingScore        float64        // Inter-package coupling metric
}

// Config holds analyzer configuration
type Config struct {
	MaxDependencyDepth int  // Maximum allowed dependency depth
	IgnoreTestFiles    bool // Skip test file analysis
	IgnoreVendor       bool // Skip vendor directory
	IncludeMetrics     bool // Calculate code metrics
	ParallelWorkers    int  // Number of concurrent workers
}

// DefaultAnalyzerConfig returns default analyzer configuration
func DefaultAnalyzerConfig() *Config {
	return &Config{
		MaxDependencyDepth: 10,
		IgnoreTestFiles:    false,
		IgnoreVendor:       true,
		IncludeMetrics:     true,
		ParallelWorkers:    4,
	}
}
