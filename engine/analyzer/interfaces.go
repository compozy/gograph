package analyzer

import (
	"context"

	"github.com/compozy/gograph/engine/parser"
)

// Analyzer defines the contract for code analysis operations
type Analyzer interface {
	// AnalyzeProject performs comprehensive analysis on parsed project data
	AnalyzeProject(ctx context.Context, input *AnalysisInput) (*AnalysisReport, error)

	// BuildDependencyGraph constructs a dependency graph from parsed packages
	BuildDependencyGraph(ctx context.Context, packages []*parser.PackageInfo) (*DependencyGraph, error)

	// MapCallChains traces function call relationships using proper type information
	MapCallChains(ctx context.Context, packages []*parser.PackageInfo) ([]*CallChain, error)

	// DetectCircularDependencies identifies circular import cycles
	DetectCircularDependencies(ctx context.Context, graph *DependencyGraph) ([]*CircularDependency, error)
}

// AnalysisInput contains the input data for analysis
type AnalysisInput struct {
	ProjectID   string              // Project identifier
	ParseResult *parser.ParseResult // Complete parse result with type information
}

// DependencyGraph represents the project's dependency structure
type DependencyGraph struct {
	Nodes map[string]*DependencyNode // Map of package path to node
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
	ProjectID                string                   // Project identifier
	Timestamp                int64                    // Analysis timestamp
	DependencyGraph          *DependencyGraph         // Project dependencies
	InterfaceImplementations []*parser.Implementation // Interface implementations from parser
	CallChains               []*CallChain             // Function call relationships
	CircularDependencies     []*CircularDependency    // Circular import cycles
	Metrics                  *CodeMetrics             // Code quality metrics
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
