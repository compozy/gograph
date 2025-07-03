package mcp

import (
	"context"
	"fmt"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/parser"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// serviceAdapter implements ServiceAdapter interface
type serviceAdapter struct {
	driver          neo4j.DriverWithContext
	graphService    graph.Service
	parserService   parser.Parser
	analyzerService analyzer.Analyzer
}

// NewServiceAdapter creates a new service adapter
func NewServiceAdapter(
	driver neo4j.DriverWithContext,
	graphService graph.Service,
	parserService parser.Parser,
	analyzerService analyzer.Analyzer,
) ServiceAdapter {
	return &serviceAdapter{
		driver:          driver,
		graphService:    graphService,
		parserService:   parserService,
		analyzerService: analyzerService,
	}
}

// ParseProject parses a Go project
func (s *serviceAdapter) ParseProject(ctx context.Context, projectPath string) (*parser.ParseResult, error) {
	config := &parser.Config{
		IgnoreDirs:     []string{".git", "vendor", "node_modules"},
		IgnoreFiles:    []string{},
		IncludeTests:   false,
		IncludeVendor:  false,
		MaxConcurrency: 4,
	}
	return s.parserService.ParseProject(ctx, projectPath, config)
}

// AnalyzeProject analyzes parsed project data
func (s *serviceAdapter) AnalyzeProject(
	ctx context.Context,
	projectID core.ID,
	files []*parser.FileInfo,
) (*analyzer.AnalysisReport, error) {
	input := &analyzer.AnalysisInput{
		ProjectID: string(projectID),
		Files:     files,
	}
	return s.analyzerService.AnalyzeProject(ctx, input)
}

// InitializeProject initializes a project in the graph
func (s *serviceAdapter) InitializeProject(ctx context.Context, project *core.Project) error {
	return s.graphService.InitializeProject(ctx, project)
}

// ImportAnalysisResult imports analysis results into the graph
func (s *serviceAdapter) ImportAnalysisResult(
	ctx context.Context,
	result *core.AnalysisResult,
) (*graph.ProjectGraph, error) {
	// First import the analysis
	err := s.graphService.ImportAnalysis(ctx, result.ProjectID, result)
	if err != nil {
		return nil, fmt.Errorf("failed to import analysis: %w", err)
	}

	// Then get the project graph
	return s.graphService.GetProjectGraph(ctx, result.ProjectID)
}

// BuildAnalysisResult builds a complete analysis result with relationships using the graph builder
func (s *serviceAdapter) BuildAnalysisResult(
	ctx context.Context,
	projectID core.ID,
	parseResult *parser.ParseResult,
	analysisReport *analyzer.AnalysisReport,
) (*core.AnalysisResult, error) {
	// Use the graph builder to create proper nodes and relationships
	builder := graph.NewBuilder(graph.DefaultBuilderConfig())
	return builder.BuildFromAnalysis(ctx, projectID, parseResult, analysisReport)
}

// GetProjectStatistics gets project statistics
func (s *serviceAdapter) GetProjectStatistics(
	ctx context.Context,
	projectID core.ID,
) (*graph.ProjectStatistics, error) {
	return s.graphService.GetProjectStatistics(ctx, projectID)
}

// ExecuteQuery executes a custom Cypher query
func (s *serviceAdapter) ExecuteQuery(
	ctx context.Context,
	query string,
	params map[string]any,
) ([]map[string]any, error) {
	return s.graphService.ExecuteQuery(ctx, query, params)
}

// ListProjects lists all projects in the database
func (s *serviceAdapter) ListProjects(ctx context.Context) ([]core.Project, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	query := `
		MATCH (n)
		WHERE n.project_id IS NOT NULL
		WITH DISTINCT n.project_id as project_id
		RETURN project_id
		ORDER BY project_id
	`

	result, err := session.Run(ctx, query, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	var projects []core.Project
	for result.Next(ctx) {
		record := result.Record()
		if projectIDValue, exists := record.Get("project_id"); exists {
			if projectIDStr, ok := projectIDValue.(string); ok {
				projects = append(projects, core.Project{
					ID:   core.ID(projectIDStr),
					Name: projectIDStr, // Use ID as name if no separate name stored
				})
			}
		}
	}

	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("failed to process project list results: %w", err)
	}

	return projects, nil
}

// ValidateProject checks if a project exists in the database
func (s *serviceAdapter) ValidateProject(ctx context.Context, projectID core.ID) (bool, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	query := `
		MATCH (n {project_id: $project_id})
		RETURN count(n) > 0 as exists
		LIMIT 1
	`

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{
			"project_id": string(projectID),
		})
		if err != nil {
			return false, err
		}

		if result.Next(ctx) {
			record := result.Record()
			if existsValue, exists := record.Get("exists"); exists {
				if existsBool, ok := existsValue.(bool); ok {
					return existsBool, nil
				}
			}
		}

		return false, result.Err()
	})

	if err != nil {
		return false, fmt.Errorf("failed to validate project: %w", err)
	}

	if boolResult, ok := result.(bool); ok {
		return boolResult, nil
	}

	return false, fmt.Errorf("unexpected result type from validate query")
}

// ClearProject removes all data for a specific project
func (s *serviceAdapter) ClearProject(ctx context.Context, projectID core.ID) error {
	return s.graphService.ClearProject(ctx, projectID)
}
