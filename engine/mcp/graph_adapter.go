package mcp

import (
	"context"
	"fmt"

	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// GraphAdapter provides graph operations needed by MCP server
type GraphAdapter struct {
	driver  neo4j.DriverWithContext
	service graph.Service
}

// NewGraphAdapter creates a new graph adapter
func NewGraphAdapter(driver neo4j.DriverWithContext, service graph.Service) *GraphAdapter {
	return &GraphAdapter{
		driver:  driver,
		service: service,
	}
}

// ExecuteQuery executes a Cypher query directly
func (a *GraphAdapter) ExecuteQuery(
	ctx context.Context,
	query string,
	params map[string]any,
) ([]map[string]any, error) {
	session := a.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var results []map[string]any
	for result.Next(ctx) {
		record := result.Record()
		row := make(map[string]any)
		for i, key := range record.Keys {
			row[key] = record.Values[i]
		}
		results = append(results, row)
	}

	if err = result.Err(); err != nil {
		return nil, fmt.Errorf("error processing results: %w", err)
	}

	return results, nil
}

// GetProjectStatistics delegates to the underlying service
func (a *GraphAdapter) GetProjectStatistics(ctx context.Context, projectID core.ID) (*graph.ProjectStatistics, error) {
	return a.service.GetProjectStatistics(ctx, projectID)
}
