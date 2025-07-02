package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/parser"
	"github.com/compozy/gograph/pkg/logger"
)

// -----
// Service Implementation
// -----

// service implements the Service interface for high-level graph operations
type service struct {
	parser     parser.Parser
	analyzer   analyzer.Analyzer
	builder    Builder
	repository Repository
	config     *ServiceConfig
}

// ServiceConfig holds configuration for the graph service
type ServiceConfig struct {
	ParserConfig     *parser.Config
	AnalyzerConfig   *analyzer.Config
	BatchSize        int    // Number of nodes/relationships to create in batch
	MaxQueryDepth    int    // Maximum depth for path queries
	EnableCaching    bool   // Enable result caching
	ProjectPrefix    string // Prefix for project namespaces
	MaxMemoryUsageMB int    // Maximum memory usage in MB before triggering optimization
	EnableStreaming  bool   // Enable streaming mode for very large codebases
}

// DefaultServiceConfig returns default service configuration
func DefaultServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		ParserConfig:     nil, // Parser will use its own defaults
		AnalyzerConfig:   analyzer.DefaultAnalyzerConfig(),
		BatchSize:        1000,
		MaxQueryDepth:    10,
		EnableCaching:    false,
		ProjectPrefix:    "project",
		MaxMemoryUsageMB: 2048,  // 2GB default limit
		EnableStreaming:  false, // Enable manually for very large codebases
	}
}

// NewService creates a new graph service instance
func NewService(
	parser parser.Parser,
	analyzer analyzer.Analyzer,
	builder Builder,
	repository Repository,
	config *ServiceConfig,
) Service {
	if config == nil {
		config = DefaultServiceConfig()
	}
	return &service{
		parser:     parser,
		analyzer:   analyzer,
		builder:    builder,
		repository: repository,
		config:     config,
	}
}

// InitializeProject creates a new project namespace in the graph
func (s *service) InitializeProject(ctx context.Context, project *core.Project) error {
	if project == nil {
		return fmt.Errorf("project cannot be nil")
	}

	// Clear any existing data for this project
	logger.Debug("initializing project", "name", project.Name, "id", project.ID)
	if err := s.repository.ClearProject(ctx, project.ID); err != nil {
		return fmt.Errorf("failed to clear existing project data: %w", err)
	}

	return nil
}

// ImportAnalysis performs complete analysis pipeline and stores results
func (s *service) ImportAnalysis(
	ctx context.Context,
	projectID core.ID,
	result *core.AnalysisResult,
) error {
	if result == nil {
		return fmt.Errorf("analysis result cannot be nil")
	}

	// Update project ID in result
	result.ProjectID = projectID

	// Store in repository
	logger.Debug("importing analysis results",
		"project_id", projectID,
		"nodes", len(result.Nodes),
		"relationships", len(result.Relationships))

	if err := s.repository.StoreAnalysis(ctx, result); err != nil {
		return fmt.Errorf("failed to store analysis: %w", err)
	}

	logger.Info("analysis imported successfully", "project_id", projectID)
	return nil
}

// GetProjectGraph retrieves the entire graph for a project
func (s *service) GetProjectGraph(ctx context.Context, projectID core.ID) (*ProjectGraph, error) {
	query := `
		MATCH (n)
		WHERE n.project_id = $projectId
		OPTIONAL MATCH (n)-[r]->(m)
		WHERE m.project_id = $projectId
		RETURN n, r, m
	`
	params := map[string]any{"projectId": projectID.String()}

	results, err := s.repository.ExecuteQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get project graph: %w", err)
	}

	graph := &ProjectGraph{
		Nodes:         make([]core.Node, 0),
		Relationships: make([]core.Relationship, 0),
	}

	nodeMap := make(map[string]bool)
	for _, result := range results {
		// Add source node
		if nodeData, ok := result["n"].(map[string]any); ok {
			if id, ok := nodeData["id"].(string); ok && !nodeMap[id] {
				nodeMap[id] = true
				graph.Nodes = append(graph.Nodes, s.mapToNode(&nodeData))
			}
		}

		// Add relationship if exists
		if relData, ok := result["r"].(map[string]any); ok && relData != nil {
			graph.Relationships = append(graph.Relationships, s.mapToRelationship(&relData))
		}

		// Add target node if exists
		if nodeData, ok := result["m"].(map[string]any); ok && nodeData != nil {
			if id, ok := nodeData["id"].(string); ok && !nodeMap[id] {
				nodeMap[id] = true
				graph.Nodes = append(graph.Nodes, s.mapToNode(&nodeData))
			}
		}
	}

	return graph, nil
}

// GetNodeWithRelationships retrieves a node with all its relationships
func (s *service) GetNodeWithRelationships(ctx context.Context, nodeID core.ID) (*NodeWithRelations, error) {
	// Get the node
	node, err := s.repository.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	if node == nil {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	result := &NodeWithRelations{
		Node:              *node,
		IncomingRelations: make([]core.Relationship, 0),
		OutgoingRelations: make([]core.Relationship, 0),
	}

	// Get outgoing relationships
	outQuery := `
		MATCH (n {id: $nodeId})-[r]->(m)
		RETURN r, m.id as to_id
	`
	outResults, err := s.repository.ExecuteQuery(ctx, outQuery, map[string]any{
		"nodeId": nodeID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get outgoing relationships: %w", err)
	}

	for _, res := range outResults {
		if relData, ok := res["r"].(map[string]any); ok {
			rel := s.mapToRelationship(&relData)
			// Ensure the relationship has the correct from/to IDs
			rel.FromNodeID = nodeID
			if toID, ok := res["to_id"].(string); ok {
				rel.ToNodeID = core.ID(toID)
			}
			result.OutgoingRelations = append(result.OutgoingRelations, rel)
		}
	}

	// Get incoming relationships
	inQuery := `
		MATCH (m)-[r]->(n {id: $nodeId})
		RETURN r, m.id as from_id
	`
	inResults, err := s.repository.ExecuteQuery(ctx, inQuery, map[string]any{
		"nodeId": nodeID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get incoming relationships: %w", err)
	}

	for _, res := range inResults {
		if relData, ok := res["r"].(map[string]any); ok {
			rel := s.mapToRelationship(&relData)
			// Ensure the relationship has the correct from/to IDs
			rel.ToNodeID = nodeID
			if fromID, ok := res["from_id"].(string); ok {
				rel.FromNodeID = core.ID(fromID)
			}
			result.IncomingRelations = append(result.IncomingRelations, rel)
		}
	}

	return result, nil
}

// FindPath finds the shortest path between two nodes
func (s *service) FindPath(ctx context.Context, fromID, toID core.ID) ([]PathSegment, error) {
	query := `
		MATCH p = shortestPath((from {id: $fromId})-[*..%d]-(to {id: $toId}))
		RETURN p
	`
	query = fmt.Sprintf(query, s.config.MaxQueryDepth)
	params := map[string]any{
		"fromId": fromID.String(),
		"toId":   toID.String(),
	}

	results, err := s.repository.ExecuteQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to find path: %w", err)
	}

	if len(results) == 0 {
		return nil, nil // No path found
	}

	// Parse the path from results
	// Note: This is simplified - actual Neo4j path parsing would be more complex
	segments := make([]PathSegment, 0)

	// For now, return empty segments - would need proper path parsing
	logger.Debug("path finding not fully implemented", "from", fromID, "to", toID)

	return segments, nil
}

// GetDependencyGraph returns the dependency graph for a specific package
func (s *service) GetDependencyGraph(ctx context.Context, packageName string) (*DependencyGraph, error) {
	// Find all packages that the given package depends on
	query := `
		MATCH (p:Package {name: $packageName})
		OPTIONAL MATCH path = (p)-[:DEPENDS_ON*]->(dep:Package)
		WITH p, dep, length(path) as level
		ORDER BY level
		RETURN dep.name as package, level
	`
	params := map[string]any{"packageName": packageName}

	results, err := s.repository.ExecuteQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency graph: %w", err)
	}

	graph := &DependencyGraph{
		RootPackage:  packageName,
		Dependencies: make([]DependencyNode, 0),
	}

	// Build dependency map by level
	depsByLevel := make(map[int][]string)
	for _, result := range results {
		if pkgName, ok := result["package"].(string); ok && pkgName != "" {
			if level, ok := result["level"].(int64); ok {
				depsByLevel[int(level)] = append(depsByLevel[int(level)], pkgName)
			}
		}
	}

	// Convert to dependency nodes
	for level := 1; level <= len(depsByLevel); level++ {
		if packages, ok := depsByLevel[level]; ok {
			for _, pkg := range packages {
				// Get direct dependencies for this package
				subQuery := `
					MATCH (p:Package {name: $packageName})-[:DEPENDS_ON]->(dep:Package)
					RETURN dep.name as dep
				`
				subResults, err := s.repository.ExecuteQuery(ctx, subQuery, map[string]any{
					"packageName": pkg,
				})
				if err != nil {
					// Log the error but continue processing other packages
					logger.Warn("failed to get dependencies for package",
						"package", pkg,
						"error", err)
					continue
				}

				deps := make([]string, 0)
				for _, subResult := range subResults {
					if dep, ok := subResult["dep"].(string); ok {
						deps = append(deps, dep)
					}
				}

				graph.Dependencies = append(graph.Dependencies, DependencyNode{
					Package:      pkg,
					Dependencies: deps,
					Level:        level,
				})
			}
		}
	}

	return graph, nil
}

// GetCallGraph returns the call graph starting from a specific function
func (s *service) GetCallGraph(ctx context.Context, functionName string) (*CallGraph, error) {
	// Find all functions called by the given function
	results, err := s.getCallGraphData(ctx, functionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get call graph: %w", err)
	}

	graph := &CallGraph{
		RootFunction: functionName,
		Calls:        make([]CallNode, 0),
	}

	// Build function nodes
	functionNodes := s.buildFunctionNodes(functionName, results)
	for _, node := range functionNodes {
		graph.Calls = append(graph.Calls, *node)
	}

	// Populate call relationships
	if err := s.populateCallRelationships(ctx, functionNodes); err != nil {
		return nil, fmt.Errorf("failed to populate call relationships: %w", err)
	}

	return graph, nil
}

// getCallGraphData executes the query to get call graph data
func (s *service) getCallGraphData(ctx context.Context, functionName string) ([]map[string]any, error) {
	query := `
		MATCH (f:Function {name: $functionName})
		OPTIONAL MATCH path = (f)-[:CALLS*..%d]->(called:Function)
		WITH f, called, length(path) as level
		ORDER BY level
		RETURN called.name as function, level
	`
	query = fmt.Sprintf(query, s.config.MaxQueryDepth)
	params := map[string]any{"functionName": functionName}

	return s.repository.ExecuteQuery(ctx, query, params)
}

// buildFunctionNodes creates CallNode instances from query results
func (s *service) buildFunctionNodes(rootFunction string, results []map[string]any) map[string]*CallNode {
	functionNodes := make(map[string]*CallNode)

	// Add root function
	rootNode := &CallNode{
		Function: rootFunction,
		Calls:    make([]string, 0),
		CalledBy: make([]string, 0),
		Level:    0,
	}
	functionNodes[rootFunction] = rootNode

	// Process called functions
	for _, result := range results {
		if funcName, ok := result["function"].(string); ok && funcName != "" {
			if level, ok := result["level"].(int64); ok {
				if _, exists := functionNodes[funcName]; !exists {
					node := &CallNode{
						Function: funcName,
						Calls:    make([]string, 0),
						CalledBy: make([]string, 0),
						Level:    int(level),
					}
					functionNodes[funcName] = node
				}
			}
		}
	}

	return functionNodes
}

// populateCallRelationships fills in the Calls and CalledBy fields for each function node
func (s *service) populateCallRelationships(ctx context.Context, functionNodes map[string]*CallNode) error {
	for funcName, node := range functionNodes {
		// Get functions this function calls
		callsQuery := `
			MATCH (f:Function {name: $functionName})-[:CALLS]->(called:Function)
			RETURN called.name as called
		`
		callsResults, err := s.repository.ExecuteQuery(ctx, callsQuery, map[string]any{
			"functionName": funcName,
		})
		if err != nil {
			return fmt.Errorf("failed to get calls for %s: %w", funcName, err)
		}

		for _, res := range callsResults {
			if called, ok := res["called"].(string); ok {
				node.Calls = append(node.Calls, called)
			}
		}

		// Get functions that call this function
		calledByQuery := `
			MATCH (caller:Function)-[:CALLS]->(f:Function {name: $functionName})
			RETURN caller.name as caller
		`
		calledByResults, err := s.repository.ExecuteQuery(ctx, calledByQuery, map[string]any{
			"functionName": funcName,
		})
		if err != nil {
			return fmt.Errorf("failed to get callers for %s: %w", funcName, err)
		}

		for _, res := range calledByResults {
			if caller, ok := res["caller"].(string); ok {
				node.CalledBy = append(node.CalledBy, caller)
			}
		}
	}

	return nil
}

// GetProjectStatistics returns statistics about the project
func (s *service) GetProjectStatistics(ctx context.Context, projectID core.ID) (*ProjectStatistics, error) {
	stats := &ProjectStatistics{
		NodesByType:         make(map[core.NodeType]int),
		RelationshipsByType: make(map[core.RelationType]int),
		TopPackages:         make([]PackageStats, 0),
		TopFunctions:        make([]FunctionStats, 0),
	}

	// Get node statistics
	if err := s.getNodeStatistics(ctx, projectID, stats); err != nil {
		return nil, fmt.Errorf("failed to get node statistics: %w", err)
	}

	// Get relationship statistics
	if err := s.getRelationshipStatistics(ctx, projectID, stats); err != nil {
		return nil, fmt.Errorf("failed to get relationship statistics: %w", err)
	}

	// Get top packages statistics
	s.getTopPackagesStatistics(ctx, projectID, stats)

	// Get top functions statistics
	s.getTopFunctionsStatistics(ctx, projectID, stats)

	return stats, nil
}

// getNodeStatistics retrieves node counts by type
func (s *service) getNodeStatistics(ctx context.Context, projectID core.ID, stats *ProjectStatistics) error {
	nodeQuery := `
		MATCH (n)
		WHERE n.project_id = $projectId
		WITH labels(n) as nodeLabels
		UNWIND nodeLabels as label
		WITH label, count(*) as count
		WHERE label <> 'Node'
		RETURN label, count
		ORDER BY label
	`
	nodeResults, err := s.repository.ExecuteQuery(ctx, nodeQuery, map[string]any{
		"projectId": projectID.String(),
	})
	if err != nil {
		return err
	}

	for _, result := range nodeResults {
		if label, ok := result["label"].(string); ok {
			if count, ok := result["count"].(int64); ok {
				nodeType := core.NodeType(label)
				stats.NodesByType[nodeType] = int(count)
				stats.TotalNodes += int(count)
			}
		}
	}

	return nil
}

// getRelationshipStatistics retrieves relationship counts by type
func (s *service) getRelationshipStatistics(ctx context.Context, projectID core.ID, stats *ProjectStatistics) error {
	relQuery := `
		MATCH (n)-[r]->(m)
		WHERE n.project_id = $projectId AND m.project_id = $projectId
		RETURN type(r) as type, count(r) as count
		ORDER BY type
	`
	relResults, err := s.repository.ExecuteQuery(ctx, relQuery, map[string]any{
		"projectId": projectID.String(),
	})
	if err != nil {
		return err
	}

	for _, result := range relResults {
		if relType, ok := result["type"].(string); ok {
			if count, ok := result["count"].(int64); ok {
				stats.RelationshipsByType[core.RelationType(relType)] = int(count)
				stats.TotalRelationships += int(count)
			}
		}
	}

	return nil
}

// getTopPackagesStatistics retrieves top packages by file count
func (s *service) getTopPackagesStatistics(ctx context.Context, projectID core.ID, stats *ProjectStatistics) {
	pkgQuery := `
		MATCH (p:Package)-[:CONTAINS]->(f:File)
		WHERE p.project_id = $projectId
		WITH p, count(f) as fileCount
		ORDER BY fileCount DESC
		LIMIT 10
		RETURN p.name as name, fileCount
	`
	pkgResults, err := s.repository.ExecuteQuery(ctx, pkgQuery, map[string]any{
		"projectId": projectID.String(),
	})
	if err != nil {
		// Log the error but continue with other statistics
		logger.Warn("failed to get top packages statistics", "error", err)
		return
	}

	for _, result := range pkgResults {
		if name, ok := result["name"].(string); ok {
			if count, ok := result["fileCount"].(int64); ok {
				stats.TopPackages = append(stats.TopPackages, PackageStats{
					Name:      name,
					FileCount: int(count),
				})
			}
		}
	}
}

// getTopFunctionsStatistics retrieves top functions by call count
func (s *service) getTopFunctionsStatistics(ctx context.Context, projectID core.ID, stats *ProjectStatistics) {
	funcQuery := `
		MATCH (f:Function)<-[c:CALLS]-(caller)
		WHERE f.project_id = $projectId
		WITH f, count(c) as callCount
		ORDER BY callCount DESC
		LIMIT 10
		RETURN f.name as name, callCount
	`
	funcResults, err := s.repository.ExecuteQuery(ctx, funcQuery, map[string]any{
		"projectId": projectID.String(),
	})
	if err != nil {
		// Log the error but continue with other statistics
		logger.Warn("failed to get top functions statistics", "error", err)
		return
	}

	for _, result := range funcResults {
		if name, ok := result["name"].(string); ok {
			if count, ok := result["callCount"].(int64); ok {
				stats.TopFunctions = append(stats.TopFunctions, FunctionStats{
					Name:     name,
					CalledBy: int(count),
				})
			}
		}
	}
}

// Helper methods

func (s *service) mapToNode(data *map[string]any) core.Node {
	node := core.Node{
		Properties: make(map[string]any),
	}

	if data == nil {
		return node
	}

	if id, ok := (*data)["id"].(string); ok {
		node.ID = core.ID(id)
	}
	if name, ok := (*data)["name"].(string); ok {
		node.Name = name
	}
	if nodeType, ok := (*data)["type"].(string); ok {
		node.Type = core.NodeType(nodeType)
	}

	// Copy properties
	for k, v := range *data {
		if k != "id" && k != "name" && k != "type" {
			node.Properties[k] = v
		}
	}

	if createdAt, ok := (*data)["created_at"].(time.Time); ok {
		node.CreatedAt = createdAt
	}

	return node
}

func (s *service) mapToRelationship(data *map[string]any) core.Relationship {
	rel := core.Relationship{
		Properties: make(map[string]any),
	}

	if data == nil {
		return rel
	}

	if id, ok := (*data)["id"].(string); ok {
		rel.ID = core.ID(id)
	}
	if relType, ok := (*data)["type"].(string); ok {
		rel.Type = core.RelationType(relType)
	}
	if fromID, ok := (*data)["from_node_id"].(string); ok {
		rel.FromNodeID = core.ID(fromID)
	}
	if toID, ok := (*data)["to_node_id"].(string); ok {
		rel.ToNodeID = core.ID(toID)
	}

	// Copy properties
	for k, v := range *data {
		if k != "id" && k != "type" && k != "from_node_id" && k != "to_node_id" {
			rel.Properties[k] = v
		}
	}

	if createdAt, ok := (*data)["created_at"].(time.Time); ok {
		rel.CreatedAt = createdAt
	}

	return rel
}
