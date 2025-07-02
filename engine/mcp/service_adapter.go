package mcp

import (
	"context"
	"fmt"
	"time"

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

// GetProjectStatistics gets project statistics
func (s *serviceAdapter) GetProjectStatistics(
	ctx context.Context,
	projectID core.ID,
) (*graph.ProjectStatistics, error) {
	return s.graphService.GetProjectStatistics(ctx, projectID)
}

// ExecuteQuery executes a Cypher query
func (s *serviceAdapter) ExecuteQuery(
	ctx context.Context,
	query string,
	params map[string]any,
) ([]map[string]any, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{})
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

// Helper function to convert parser results to analysis results
//
//nolint:funlen // Will be used when full MCP integration is complete
func convertToAnalysisResult(
	project *core.Project,
	parseResult *parser.ParseResult,
	report *analyzer.AnalysisReport,
) *core.AnalysisResult {
	nodes := make([]core.Node, 0)
	relationships := make([]core.Relationship, 0)

	// Create package nodes
	packages := make(map[string]bool)
	for _, file := range parseResult.Files {
		if !packages[file.Package] {
			packages[file.Package] = true
			nodes = append(nodes, core.Node{
				ID:   core.NewID(),
				Type: core.NodeTypePackage,
				Name: file.Package,
				Path: file.Package,
				Properties: map[string]any{
					"project_id": project.ID,
				},
				CreatedAt: time.Now(),
			})
		}

		// Create file node
		fileNode := core.Node{
			ID:   core.NewID(),
			Type: core.NodeTypeFile,
			Name: file.Path,
			Path: file.Path,
			Properties: map[string]any{
				"project_id": project.ID,
				"package":    file.Package,
			},
			CreatedAt: time.Now(),
		}
		nodes = append(nodes, fileNode)

		// Create function nodes
		for i := range file.Functions {
			fn := &file.Functions[i]
			nodes = append(nodes, core.Node{
				ID:   core.NewID(),
				Type: core.NodeTypeFunction,
				Name: fn.Name,
				Path: file.Path,
				Properties: map[string]any{
					"project_id":  project.ID,
					"package":     file.Package,
					"signature":   fn.Signature,
					"is_exported": fn.IsExported,
					"line_start":  fn.LineStart,
					"line_end":    fn.LineEnd,
				},
				CreatedAt: time.Now(),
			})
		}

		// Create struct nodes
		for _, st := range file.Structs {
			nodes = append(nodes, core.Node{
				ID:   core.NewID(),
				Type: core.NodeTypeStruct,
				Name: st.Name,
				Path: file.Path,
				Properties: map[string]any{
					"project_id":  project.ID,
					"package":     file.Package,
					"is_exported": st.IsExported,
					"line_start":  st.LineStart,
					"line_end":    st.LineEnd,
				},
				CreatedAt: time.Now(),
			})
		}

		// Create interface nodes
		for _, iface := range file.Interfaces {
			nodes = append(nodes, core.Node{
				ID:   core.NewID(),
				Type: core.NodeTypeInterface,
				Name: iface.Name,
				Path: file.Path,
				Properties: map[string]any{
					"project_id":  project.ID,
					"package":     file.Package,
					"is_exported": iface.IsExported,
					"line_start":  iface.LineStart,
					"line_end":    iface.LineEnd,
				},
				CreatedAt: time.Now(),
			})
		}
	}

	// Add relationships from analysis report
	if report != nil && report.DependencyGraph != nil {
		for _, edge := range report.DependencyGraph.Edges {
			relationships = append(relationships, core.Relationship{
				ID:         core.NewID(),
				Type:       core.RelationDependsOn,
				FromNodeID: core.ID(edge.From),
				ToNodeID:   core.ID(edge.To),
				Properties: map[string]any{
					"type": edge.Type,
					"line": edge.Line,
				},
				CreatedAt: time.Now(),
			})
		}
	}

	return &core.AnalysisResult{
		ProjectID:      project.ID,
		Nodes:          nodes,
		Relationships:  relationships,
		TotalFiles:     len(parseResult.Files),
		TotalPackages:  len(packages),
		TotalFunctions: countFunctions(parseResult.Files),
		TotalStructs:   countStructs(parseResult.Files),
		AnalyzedAt:     time.Now(),
		Duration:       time.Duration(parseResult.ParseTime) * time.Millisecond,
	}
}

func countFunctions(files []*parser.FileInfo) int {
	count := 0
	for _, file := range files {
		count += len(file.Functions)
	}
	return count
}

func countStructs(files []*parser.FileInfo) int {
	count := 0
	for _, file := range files {
		count += len(file.Structs)
	}
	return count
}
