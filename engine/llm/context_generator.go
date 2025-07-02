package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
)

// DefaultContextGenerator implements ContextGenerator
type DefaultContextGenerator struct {
	graphService graph.Service
}

// NewDefaultContextGenerator creates a new context generator
func NewDefaultContextGenerator(graphService graph.Service) *DefaultContextGenerator {
	return &DefaultContextGenerator{
		graphService: graphService,
	}
}

// GenerateContext generates context information for LLMs
func (g *DefaultContextGenerator) GenerateContext(
	ctx context.Context,
	projectID string,
	query string,
) (*Context, error) {
	if projectID == "" {
		return nil, core.NewError(fmt.Errorf("project ID cannot be empty"), "EMPTY_PROJECT_ID", nil)
	}
	summary, err := g.GetProjectSummary(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project summary: %w", err)
	}
	// Get project graph for relevant nodes and relationships
	projectGraph, err := g.graphService.GetProjectGraph(ctx, core.ID(projectID))
	if err != nil {
		return nil, core.NewError(
			fmt.Errorf("failed to get project graph: %w", err),
			"GRAPH_SERVICE_ERROR",
			map[string]any{
				"project_id": projectID,
			},
		)
	}
	// Filter relevant nodes based on query if provided
	relevantNodes := g.filterRelevantNodes(projectGraph.Nodes, query)
	relationships := projectGraph.Relationships
	// Generate code examples
	codeExamples := g.generateCodeExamples(relevantNodes)
	// Generate query suggestions
	querySuggestions := g.generateQuerySuggestions(summary)
	return &Context{
		ProjectID:        projectID,
		Summary:          summary,
		RelevantNodes:    relevantNodes,
		Relationships:    relationships,
		CodeExamples:     codeExamples,
		QuerySuggestions: querySuggestions,
	}, nil
}

// GetProjectSummary generates a high-level project summary
func (g *DefaultContextGenerator) GetProjectSummary(ctx context.Context, projectID string) (*ProjectSummary, error) {
	if projectID == "" {
		return nil, core.NewError(fmt.Errorf("project ID cannot be empty"), "EMPTY_PROJECT_ID", nil)
	}
	stats, err := g.graphService.GetProjectStatistics(ctx, core.ID(projectID))
	if err != nil {
		return nil, core.NewError(
			fmt.Errorf("failed to get project statistics: %w", err),
			"GRAPH_SERVICE_ERROR",
			map[string]any{
				"project_id": projectID,
			},
		)
	}
	// Extract main packages from top packages
	mainPackages := make([]string, 0, len(stats.TopPackages))
	for _, pkg := range stats.TopPackages {
		mainPackages = append(mainPackages, pkg.Name)
	}
	// Calculate dependencies map
	dependencies := make(map[string]int)
	for _, pkg := range stats.TopPackages {
		dependencies[pkg.Name] = pkg.Dependencies
	}
	// Calculate metrics
	metrics := map[string]any{
		"avg_functions_per_file": float64(stats.TotalNodes) / float64(len(stats.TopPackages)),
		"node_types":             stats.NodesByType,
		"relationship_types":     stats.RelationshipsByType,
	}
	return &ProjectSummary{
		Name:           projectID, // Using project ID as name for now
		TotalFiles:     g.getNodeCountByType(stats, core.NodeTypeFile),
		TotalPackages:  g.getNodeCountByType(stats, core.NodeTypePackage),
		TotalFunctions: g.getNodeCountByType(stats, core.NodeTypeFunction),
		MainPackages:   mainPackages,
		Dependencies:   dependencies,
		Metrics:        metrics,
	}, nil
}

// filterRelevantNodes filters nodes based on query relevance
func (g *DefaultContextGenerator) filterRelevantNodes(nodes []core.Node, query string) []core.Node {
	if query == "" {
		// Return first 20 nodes if no query specified
		if len(nodes) > 20 {
			return nodes[:20]
		}
		return nodes
	}
	queryLower := strings.ToLower(query)
	relevant := make([]core.Node, 0)
	for _, node := range nodes {
		// Check if node name or properties contain query keywords
		if g.nodeMatchesQuery(&node, queryLower) {
			relevant = append(relevant, node)
		}
		// Limit to 50 relevant nodes
		if len(relevant) >= 50 {
			break
		}
	}
	return relevant
}

// nodeMatchesQuery checks if a node matches the query
func (g *DefaultContextGenerator) nodeMatchesQuery(node *core.Node, queryLower string) bool {
	// Check node name
	if name, exists := node.Properties["name"].(string); exists {
		if strings.Contains(strings.ToLower(name), queryLower) {
			return true
		}
	}
	// Check node path
	if path, exists := node.Properties["path"].(string); exists {
		if strings.Contains(strings.ToLower(path), queryLower) {
			return true
		}
	}
	// Check node signature (for functions)
	if signature, exists := node.Properties["signature"].(string); exists {
		if strings.Contains(strings.ToLower(signature), queryLower) {
			return true
		}
	}
	return false
}

// generateCodeExamples creates code examples from nodes
func (g *DefaultContextGenerator) generateCodeExamples(nodes []core.Node) []CodeExample {
	examples := make([]CodeExample, 0)
	for _, node := range nodes {
		if node.Type == core.NodeTypeFunction {
			example := g.createFunctionExample(&node)
			if example != nil {
				examples = append(examples, *example)
			}
		}
		// Limit to 10 examples
		if len(examples) >= 10 {
			break
		}
	}
	return examples
}

// createFunctionExample creates a code example for a function node
func (g *DefaultContextGenerator) createFunctionExample(node *core.Node) *CodeExample {
	name, nameExists := node.Properties["name"].(string)
	signature, sigExists := node.Properties["signature"].(string)
	filePath, pathExists := node.Properties["file_path"].(string)
	if !nameExists || !sigExists {
		return nil
	}
	// Extract line numbers if available
	lineStart := 0
	lineEnd := 0
	if start, exists := node.Properties["line_start"].(int); exists {
		lineStart = start
	}
	if end, exists := node.Properties["line_end"].(int); exists {
		lineEnd = end
	}
	if !pathExists {
		filePath = "unknown"
	}
	return &CodeExample{
		FilePath:    filePath,
		Function:    name,
		Code:        signature, // Using signature as code for now
		Description: fmt.Sprintf("Function %s definition", name),
		LineStart:   lineStart,
		LineEnd:     lineEnd,
	}
}

// generateQuerySuggestions creates useful query suggestions
func (g *DefaultContextGenerator) generateQuerySuggestions(summary *ProjectSummary) []string {
	suggestions := []string{
		"Find all functions in the main package",
		"Show dependencies between packages",
		"List all interfaces and their implementations",
		"Find functions that are never called",
		"Show the most complex functions",
		"Find circular dependencies",
		"List all exported functions",
		"Show package import relationships",
	}
	// Add package-specific suggestions
	for _, pkg := range summary.MainPackages {
		suggestions = append(suggestions, fmt.Sprintf("Analyze the %s package", pkg))
	}
	return suggestions
}

// getNodeCountByType gets the count of nodes by type from statistics
func (g *DefaultContextGenerator) getNodeCountByType(stats *graph.ProjectStatistics, nodeType core.NodeType) int {
	if count, exists := stats.NodesByType[nodeType]; exists {
		return count
	}
	return 0
}
