package mcp

import (
	"context"

	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
)

// GraphAdapter provides graph operations needed by MCP server
type GraphAdapter struct {
	repository graph.Repository
	service    graph.Service
}

// NewGraphAdapter creates a new graph adapter
func NewGraphAdapter(repository graph.Repository, service graph.Service) *GraphAdapter {
	return &GraphAdapter{
		repository: repository,
		service:    service,
	}
}

// ExecuteQuery executes a Cypher query directly
func (a *GraphAdapter) ExecuteQuery(
	ctx context.Context,
	query string,
	params map[string]any,
) ([]map[string]any, error) {
	return a.repository.ExecuteQuery(ctx, query, params)
}

// GetProjectStatistics delegates to the underlying service
func (a *GraphAdapter) GetProjectStatistics(ctx context.Context, projectID core.ID) (*graph.ProjectStatistics, error) {
	return a.service.GetProjectStatistics(ctx, projectID)
}
