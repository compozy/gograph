package mcp

import (
	"context"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/parser"
)

// ServiceAdapter provides a unified interface for MCP operations
type ServiceAdapter interface {
	// Parse operations
	ParseProject(ctx context.Context, projectPath string) (*parser.ParseResult, error)

	// Analyze operations
	AnalyzeProject(ctx context.Context, projectID core.ID, files []*parser.FileInfo) (*analyzer.AnalysisReport, error)

	// Graph operations
	InitializeProject(ctx context.Context, project *core.Project) error
	ImportAnalysisResult(ctx context.Context, result *core.AnalysisResult) (*graph.ProjectGraph, error)
	GetProjectStatistics(ctx context.Context, projectID core.ID) (*graph.ProjectStatistics, error)
	ExecuteQuery(ctx context.Context, query string, params map[string]any) ([]map[string]any, error)
}

// MCPServer defines the interface for the MCP server
//
//nolint:revive // Keeping original MCP prefix for clarity in this context
type MCPServer interface {
	AddTool(tool any)
	AddResource(resource any)
	Connect(ctx context.Context, transport any) error
}

// ToolResponse represents a response from a tool
type ToolResponse struct {
	Content []any `json:"content"`
}

// ResourceResponse represents a response from a resource
type ResourceResponse struct {
	Content []any `json:"content"`
}
