package graph

import (
	"context"
	"time"

	"github.com/compozy/gograph/engine/core"
)

// Repository defines the interface for graph database operations
type Repository interface {
	// Connection management
	Connect(ctx context.Context, uri, username, password string) error
	Close() error

	// Node operations
	CreateNode(ctx context.Context, node *core.Node) error
	CreateNodes(ctx context.Context, nodes []core.Node) error
	GetNode(ctx context.Context, id core.ID) (*core.Node, error)
	UpdateNode(ctx context.Context, node *core.Node) error
	DeleteNode(ctx context.Context, id core.ID) error

	// Relationship operations
	CreateRelationship(ctx context.Context, rel *core.Relationship) error
	CreateRelationships(ctx context.Context, rels []core.Relationship) error
	GetRelationship(ctx context.Context, id core.ID) (*core.Relationship, error)
	DeleteRelationship(ctx context.Context, id core.ID) error

	// Query operations
	ExecuteQuery(ctx context.Context, query string, params map[string]any) ([]map[string]any, error)

	// Bulk operations
	ImportAnalysisResult(ctx context.Context, result *core.AnalysisResult) error
	StoreAnalysis(ctx context.Context, result *core.AnalysisResult) error
	ClearProject(ctx context.Context, projectID core.ID) error

	// Search operations
	FindNodesByType(ctx context.Context, nodeType core.NodeType) ([]core.Node, error)
	FindNodesByName(ctx context.Context, name string) ([]core.Node, error)
	FindRelationshipsByType(ctx context.Context, relType core.RelationType) ([]core.Relationship, error)
}

// Service defines the interface for graph service operations
type Service interface {
	// Project operations
	InitializeProject(ctx context.Context, project *core.Project) error
	ImportAnalysis(ctx context.Context, projectID core.ID, result *core.AnalysisResult) error

	// Query operations
	GetProjectGraph(ctx context.Context, projectID core.ID) (*ProjectGraph, error)
	GetNodeWithRelationships(ctx context.Context, nodeID core.ID) (*NodeWithRelations, error)
	FindPath(ctx context.Context, fromID, toID core.ID) ([]PathSegment, error)

	// Analysis operations
	GetDependencyGraph(ctx context.Context, packageName string) (*DependencyGraph, error)
	GetCallGraph(ctx context.Context, functionName string) (*CallGraph, error)
	GetProjectStatistics(ctx context.Context, projectID core.ID) (*ProjectStatistics, error)
}

// ProjectGraph represents the entire project graph
type ProjectGraph struct {
	Nodes         []core.Node         `json:"nodes"`
	Relationships []core.Relationship `json:"relationships"`
}

// NodeWithRelations represents a node with its relationships
type NodeWithRelations struct {
	Node              core.Node           `json:"node"`
	IncomingRelations []core.Relationship `json:"incoming_relations"`
	OutgoingRelations []core.Relationship `json:"outgoing_relations"`
}

// PathSegment represents a segment in a path between nodes
type PathSegment struct {
	FromNode     core.Node         `json:"from_node"`
	Relationship core.Relationship `json:"relationship"`
	ToNode       core.Node         `json:"to_node"`
}

// DependencyGraph represents a dependency graph
type DependencyGraph struct {
	RootPackage  string           `json:"root_package"`
	Dependencies []DependencyNode `json:"dependencies"`
}

// DependencyNode represents a node in the dependency graph
type DependencyNode struct {
	Package      string   `json:"package"`
	Dependencies []string `json:"dependencies"`
	Level        int      `json:"level"`
}

// CallGraph represents a function call graph
type CallGraph struct {
	RootFunction string     `json:"root_function"`
	Calls        []CallNode `json:"calls"`
}

// CallNode represents a node in the call graph
type CallNode struct {
	Function string   `json:"function"`
	Calls    []string `json:"calls"`
	CalledBy []string `json:"called_by"`
	Level    int      `json:"level"`
}

// ProjectStatistics represents project statistics
type ProjectStatistics struct {
	TotalNodes          int                       `json:"total_nodes"`
	TotalRelationships  int                       `json:"total_relationships"`
	NodesByType         map[core.NodeType]int     `json:"nodes_by_type"`
	RelationshipsByType map[core.RelationType]int `json:"relationships_by_type"`
	TopPackages         []PackageStats            `json:"top_packages"`
	TopFunctions        []FunctionStats           `json:"top_functions"`
}

// PackageStats represents statistics for a package
type PackageStats struct {
	Name          string `json:"name"`
	FileCount     int    `json:"file_count"`
	FunctionCount int    `json:"function_count"`
	Dependencies  int    `json:"dependencies"`
}

// FunctionStats represents statistics for a function
type FunctionStats struct {
	Name      string `json:"name"`
	CallCount int    `json:"call_count"`
	CalledBy  int    `json:"called_by"`
}

// AnalysisSummary represents a summary of analysis results
type AnalysisSummary struct {
	ProjectID  core.ID        `json:"project_id"`
	NodeCounts map[string]int `json:"node_counts"`
	TotalNodes int            `json:"total_nodes"`
	Timestamp  time.Time      `json:"timestamp"`
}

// CallChain represents a chain of function calls
type CallChain struct {
	StartFunction string      `json:"start_function"`
	Chain         []ChainNode `json:"chain"`
	MaxDepth      int         `json:"max_depth"`
}

// ChainNode represents a node in a call chain
type ChainNode struct {
	Function string `json:"function"`
	Depth    int    `json:"depth"`
	Calls    int    `json:"calls"`
}

// CircularDependency represents a circular dependency in the code
type CircularDependency struct {
	Type     string   `json:"type"` // "package" or "function"
	Elements []string `json:"elements"`
}

// ProjectStats represents simplified project statistics
type ProjectStats struct {
	ProjectID            core.ID `json:"project_id"`
	PackageCount         int     `json:"package_count"`
	FileCount            int     `json:"file_count"`
	FunctionCount        int     `json:"function_count"`
	InterfaceCount       int     `json:"interface_count"`
	StructCount          int     `json:"struct_count"`
	TotalRelationships   int     `json:"total_relationships"`
	CircularDependencies int     `json:"circular_dependencies"`
	UnusedFunctions      int     `json:"unused_functions"`
}
