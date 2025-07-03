package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/parser"
	"github.com/compozy/gograph/engine/query"
	"github.com/compozy/gograph/pkg/config"
	"github.com/compozy/gograph/pkg/logger"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Tool handler implementations - These replace the stub implementations

// getProjectID extracts project ID from input, with fallback to loading from config
func (s *Server) getProjectID(input map[string]any) (string, error) {
	// First, try to get explicit project_id
	if projectID, ok := input["project_id"].(string); ok && projectID != "" {
		return projectID, nil
	}

	// If not provided, try to load from project_path config
	projectPath, ok := input["project_path"].(string)
	if !ok || projectPath == "" {
		// For tools that don't have project_path, try current directory
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("project_id not provided and cannot determine current directory: %w", err)
		}
		projectPath = cwd
	}

	// Try to load from config
	cfg, err := config.LoadProjectConfig(projectPath)
	if err != nil {
		return "", fmt.Errorf("project_id not provided and failed to load config from %s: %w", projectPath, err)
	}

	if cfg.Project.ID == "" {
		return "", fmt.Errorf("project_id not provided and not found in config at %s", projectPath)
	}

	return cfg.Project.ID, nil
}

// HandleAnalyzeProjectInternal analyzes a Go project and stores results in Neo4j
func (s *Server) HandleAnalyzeProjectInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	projectPath, ok := input["project_path"].(string)
	if !ok {
		return nil, fmt.Errorf("project_path is required")
	}

	// Get project ID and validate
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}

	if !s.IsPathAllowed(projectPath) {
		return nil, fmt.Errorf("path %s is not allowed by security policy", projectPath)
	}

	logger.Info("analyzing project", "path", projectPath, "project_id", projectID)

	// Perform analysis
	analysisData, err := s.performAnalysis(ctx, projectPath, projectID)
	if err != nil {
		return nil, err
	}

	// Build response
	return s.buildAnalysisResponse(projectID, analysisData), nil
}

// performAnalysis executes the full analysis pipeline
func (s *Server) performAnalysis(ctx context.Context, projectPath, projectID string) (*analysisData, error) {
	// Create and initialize project
	project := &core.Project{
		ID:       core.ID(projectID),
		Name:     projectID,
		RootPath: projectPath,
	}

	if err := s.serviceAdapter.InitializeProject(ctx, project); err != nil {
		return nil, fmt.Errorf("failed to initialize project: %w", err)
	}

	// Parse project
	parseResult, err := s.serviceAdapter.ParseProject(ctx, projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse project: %w", err)
	}

	// Analyze project
	analysisReport, err := s.serviceAdapter.AnalyzeProject(ctx, project.ID, parseResult)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze project: %w", err)
	}

	// Build and import analysis result
	analysisResult, err := s.serviceAdapter.BuildAnalysisResult(ctx, project.ID, parseResult, analysisReport)
	if err != nil {
		return nil, fmt.Errorf("failed to build analysis result: %w", err)
	}

	projectGraph, err := s.serviceAdapter.ImportAnalysisResult(ctx, analysisResult)
	if err != nil {
		return nil, fmt.Errorf("failed to import project: %w", err)
	}

	// Get statistics
	stats, err := s.serviceAdapter.GetProjectStatistics(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}

	return &analysisData{
		parseResult:  parseResult,
		projectGraph: projectGraph,
		stats:        stats,
	}, nil
}

// analysisData holds the results of project analysis
type analysisData struct {
	parseResult  *parser.ParseResult
	projectGraph *graph.ProjectGraph
	stats        *graph.ProjectStatistics
}

// buildAnalysisResponse creates the tool response from analysis data
func (s *Server) buildAnalysisResponse(projectID string, data *analysisData) *ToolResponse {
	// Count total files from packages
	totalFiles := 0
	for _, pkg := range data.parseResult.Packages {
		totalFiles += len(pkg.Files)
	}

	result := map[string]any{
		"project_id":     projectID,
		"nodes_created":  len(data.projectGraph.Nodes),
		"relationships":  len(data.projectGraph.Relationships),
		"files_analyzed": totalFiles,
		"statistics":     ConvertStatistics(data.stats),
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Successfully analyzed project %s", projectID),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/metadata", projectID),
					"data": result,
				},
			},
		},
	}
}

// HandleExecuteCypherInternal executes a custom Cypher query
func (s *Server) HandleExecuteCypherInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}

	query, ok := input["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query is required")
	}

	// Get parameters if provided
	parameters := make(map[string]any)
	if params, ok := input["parameters"].(map[string]any); ok {
		parameters = params
	}
	// Add project_id to parameters to scope the query
	parameters["project_id"] = projectID

	logger.Info("executing cypher query", "project_id", projectID, "query", query)

	// Execute query
	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	result := map[string]any{
		"query":        query,
		"parameters":   parameters,
		"results":      results,
		"result_count": len(results),
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Query executed successfully, returned %d results", len(results)),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/query-results", projectID),
					"data": result,
				},
			},
		},
	}, nil
}

// HandleGetFunctionInfoInternal gets detailed information about a function
//
//nolint:funlen,gocyclo // MCP tool handlers can be longer and have complex logic
func (s *Server) HandleGetFunctionInfoInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	functionName, ok := input["function_name"].(string)
	if !ok {
		return nil, fmt.Errorf("function_name is required")
	}

	packageName := ""
	if p, ok := input["package"].(string); ok {
		packageName = p
	}
	includeCalls := false
	if ic, ok := input["include_calls"].(bool); ok {
		includeCalls = ic
	}
	includeCallers := false
	if icr, ok := input["include_callers"].(bool); ok {
		includeCallers = icr
	}

	logger.Info("getting function info",
		"project_id", projectID,
		"function", functionName,
		"package", packageName)

	// Build query to find function - try exact match first, then fuzzy
	query := `
		MATCH (f:Function {project_id: $project_id})
		WHERE f.name = $function_name OR toLower(f.name) CONTAINS toLower($function_name)
	`
	params := map[string]any{
		"project_id":    projectID,
		"function_name": functionName,
	}

	if packageName != "" {
		query += " AND (f.package = $package OR toLower(f.package) CONTAINS toLower($package))"
		params["package"] = packageName
	}

	query += `
		WITH f
		OPTIONAL MATCH (file:File {project_id: $project_id})-[:DEFINES]->(f)
		RETURN f, COALESCE(file.path, f.file_path) as file_path
		ORDER BY CASE WHEN f.name = $function_name THEN 0 ELSE 1 END
		LIMIT 1
	`

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to query function: %w", err)
	}

	if len(results) == 0 {
		return &ToolResponse{
			Content: []any{
				map[string]any{
					"type": "text",
					"text": fmt.Sprintf("Function %s not found", functionName),
				},
			},
		}, nil
	}

	functionData := results[0]
	var functionNode map[string]any

	// Handle different possible types for the function node
	switch f := functionData["f"].(type) {
	case map[string]any:
		functionNode = f
	case neo4j.Node:
		// Convert Neo4j Node to map[string]any
		functionNode = f.Props
	default:
		return nil, fmt.Errorf("invalid function data format: expected map[string]any or neo4j.Node, got %T",
			functionData["f"])
	}
	filePath := ""
	if fp, ok := functionData["file_path"].(string); ok && fp != "" {
		filePath = fp
	} else if fp, ok := functionNode["file_path"].(string); ok && fp != "" {
		filePath = fp
	}

	// Get actual values from functionNode
	packageActual := packageName
	if p, ok := functionNode["package"].(string); ok && p != "" {
		packageActual = p
	}

	signature := ""
	if s, ok := functionNode["signature"].(string); ok {
		signature = s
	}

	result := map[string]any{
		"function_name": functionName,
		"package":       packageActual,
		"signature":     signature,
		"file_path":     filePath,
		"line_start":    functionNode["line_start"],
		"line_end":      functionNode["line_end"],
		"is_exported":   functionNode["is_exported"],
	}

	// Include function calls and callers if requested
	s.AddFunctionRelationships(ctx, result, params, includeCalls, includeCallers)

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Function information for %s", functionName),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/functions/%s", projectID, functionName),
					"data": result,
				},
			},
		},
	}, nil
}

// AddFunctionRelationships adds calls and callers to function result
func (s *Server) AddFunctionRelationships(
	ctx context.Context,
	result map[string]any,
	params map[string]any,
	includeCalls, includeCallers bool,
) {
	if includeCalls {
		callsQuery := `
			MATCH (f:Function {project_id: $project_id})-[:CALLS]->(called:Function)
			WHERE f.name = $function_name OR toLower(f.name) CONTAINS toLower($function_name)
			RETURN called.name as name, 
			       called.package as package, 
			       called.signature as signature,
			       called.file_path as file_path,
			       called.line_start as line_start,
			       called.is_exported as is_exported
			ORDER BY called.package, called.name
		`
		callsResults, err := s.serviceAdapter.ExecuteQuery(ctx, callsQuery, params)
		if err != nil {
			logger.Warn("failed to fetch function calls", "error", err)
		} else {
			result["calls"] = callsResults
			result["calls_count"] = len(callsResults)
		}
	}

	if includeCallers {
		callersQuery := `
			MATCH (caller:Function)-[:CALLS]->(f:Function {project_id: $project_id})
			WHERE f.name = $function_name OR toLower(f.name) CONTAINS toLower($function_name)
			RETURN caller.name as name, 
			       caller.package as package, 
			       caller.signature as signature,
			       caller.file_path as file_path,
			       caller.line_start as line_start,
			       caller.is_exported as is_exported
			ORDER BY caller.package, caller.name
		`
		callersResults, err := s.serviceAdapter.ExecuteQuery(ctx, callersQuery, params)
		if err != nil {
			logger.Warn("failed to fetch function callers", "error", err)
		} else {
			result["callers"] = callersResults
			result["callers_count"] = len(callersResults)
		}
	}
}

// handleQueryDependencies finds dependencies for a package or function
//
//nolint:funlen // MCP tool handlers can be longer for comprehensive functionality
func (s *Server) HandleQueryDependenciesInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	path, ok := input["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path is required")
	}

	direction := ""
	if d, ok := input["direction"].(string); ok {
		direction = d
	}
	recursive := false
	if r, ok := input["recursive"].(bool); ok {
		recursive = r
	}

	logger.Info("querying dependencies",
		"project_id", projectID,
		"path", path,
		"direction", direction,
		"recursive", recursive)

	// Build dependency query based on direction
	var query string
	params := map[string]any{
		"project_id": projectID,
		"path":       path,
	}

	switch direction {
	case "incoming":
		if recursive {
			query = `
				MATCH path = (start)-[:DEPENDS_ON*]->(target)
				WHERE target.project_id = $project_id 
				AND (target.path = $path OR target.name = $path)
				RETURN start, relationships(path) as deps
			`
		} else {
			query = `
				MATCH (start)-[:DEPENDS_ON]->(target)
				WHERE target.project_id = $project_id 
				AND (target.path = $path OR target.name = $path)
				RETURN start, target
			`
		}
	case "outgoing":
		if recursive {
			query = `
				MATCH path = (start)-[:DEPENDS_ON*]->(target)
				WHERE start.project_id = $project_id 
				AND (start.path = $path OR start.name = $path)
				RETURN target, relationships(path) as deps
			`
		} else {
			query = `
				MATCH (start)-[:DEPENDS_ON]->(target)
				WHERE start.project_id = $project_id 
				AND (start.path = $path OR start.name = $path)
				RETURN start, target
			`
		}
	default:
		// Both directions
		if recursive {
			query = `
				MATCH path = (start)-[:DEPENDS_ON*]-(target)
				WHERE (start.project_id = $project_id OR target.project_id = $project_id)
				AND (start.path = $path OR start.name = $path OR target.path = $path OR target.name = $path)
				RETURN start, target, relationships(path) as deps
			`
		} else {
			query = `
				MATCH (start)-[:DEPENDS_ON]-(target)
				WHERE (start.project_id = $project_id OR target.project_id = $project_id)
				AND (start.path = $path OR start.name = $path OR target.path = $path OR target.name = $path)
				RETURN start, target
			`
		}
	}

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to query dependencies: %w", err)
	}

	result := map[string]any{
		"path":         path,
		"direction":    direction,
		"recursive":    recursive,
		"dependencies": results,
		"count":        len(results),
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Found %d dependencies for %s", len(results), path),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/dependencies", projectID),
					"data": result,
				},
			},
		},
	}, nil
}

// Helper methods

func (s *Server) IsPathAllowed(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Check forbidden paths
	for _, forbidden := range s.config.Security.ForbiddenPaths {
		if strings.Contains(absPath, forbidden) {
			return false
		}
	}

	// Check allowed paths
	for _, allowed := range s.config.Security.AllowedPaths {
		allowedAbs, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, allowedAbs) {
			return true
		}
	}

	return false
}

// handleFindImplementations finds all implementations of an interface
//
//nolint:funlen // MCP tool handlers can be longer for comprehensive functionality
func (s *Server) HandleFindImplementationsInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	interfaceName, ok := input["interface_name"].(string)
	if !ok {
		return nil, fmt.Errorf("interface_name is required")
	}
	packageName := ""
	if p, ok := input["package"].(string); ok {
		packageName = p
	}

	logger.Info("finding implementations",
		"project_id", projectID,
		"interface", interfaceName,
		"package", packageName)

	// Query to find interface implementations
	query := `
		MATCH (iface:Interface {project_id: $project_id, name: $interface_name})
		MATCH (impl:Struct)-[:IMPLEMENTS]->(iface)
		OPTIONAL MATCH (impl_file:File)-[:DEFINES]->(impl)
		RETURN impl, impl_file.path as file_path
	`
	params := map[string]any{
		"project_id":     projectID,
		"interface_name": interfaceName,
	}

	if packageName != "" {
		query = strings.Replace(query, "Interface {", "Interface {package: $package, ", 1)
		params["package"] = packageName
	}

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to find implementations: %w", err)
	}

	implementations := make([]map[string]any, len(results))
	for i, result := range results {
		impl, ok := result["impl"].(map[string]any)
		if !ok {
			continue // Skip invalid entries
		}
		filePath := ""
		if fp, ok := result["file_path"].(string); ok {
			filePath = fp
		}

		implementations[i] = map[string]any{
			"name":        impl["name"],
			"package":     impl["package"],
			"file_path":   filePath,
			"line_start":  impl["line_start"],
			"line_end":    impl["line_end"],
			"is_exported": impl["is_exported"],
		}
	}

	result := map[string]any{
		"interface_name":  interfaceName,
		"package":         packageName,
		"implementations": implementations,
		"count":           len(implementations),
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Found %d implementations of %s", len(implementations), interfaceName),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/interfaces/%s/implementations", projectID, interfaceName),
					"data": result,
				},
			},
		},
	}, nil
}

// handleTraceCallChain traces call chains between functions
//
//nolint:funlen // MCP tool handlers can be longer for comprehensive functionality
func (s *Server) HandleTraceCallChainInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	fromFunction, ok := input["from_function"].(string)
	if !ok {
		return nil, fmt.Errorf("from_function is required")
	}
	toFunction := ""
	if t, ok := input["to_function"].(string); ok {
		toFunction = t
	}
	maxDepth := 5 // Default depth
	if m, ok := input["max_depth"].(float64); ok {
		maxDepth = int(m)
	}

	logger.Info("tracing call chain",
		"project_id", projectID,
		"from", fromFunction,
		"to", toFunction,
		"max_depth", maxDepth)

	var query string
	params := map[string]any{
		"project_id":    projectID,
		"from_function": fromFunction,
		"max_depth":     maxDepth,
	}

	if toFunction != "" {
		// Find path between specific functions - try exact match first, then fuzzy
		query = `
			MATCH (start:Function {project_id: $project_id}), (end:Function {project_id: $project_id})
			WHERE (start.name = $from_function OR toLower(start.name) CONTAINS toLower($from_function))
			  AND (end.name = $to_function OR toLower(end.name) CONTAINS toLower($to_function))
			WITH start, end
			MATCH path = (start)-[:CALLS*1..` + fmt.Sprintf("%d", maxDepth) + `]->(end)
			RETURN [node in nodes(path) | {
				name: node.name, 
				package: node.package,
				file_path: node.file_path,
				signature: node.signature
			}] as call_chain,
			length(path) as depth,
			start.name as actual_start,
			end.name as actual_end
			ORDER BY depth
			LIMIT 10
		`
		params["to_function"] = toFunction
	} else {
		// Find all calls from the function up to max depth
		query = `
			MATCH (start:Function {project_id: $project_id})
			WHERE start.name = $from_function OR toLower(start.name) CONTAINS toLower($from_function)
			WITH start
			MATCH path = (start)-[:CALLS*1..` + fmt.Sprintf("%d", maxDepth) + `]->(called:Function)
			WHERE called.project_id = $project_id
			RETURN [node in nodes(path) | {
				name: node.name, 
				package: node.package,
				file_path: node.file_path,
				signature: node.signature
			}] as call_chain,
			length(path) as depth,
			start.name as actual_start
			ORDER BY depth
			LIMIT 50
		`
	}

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to trace call chain: %w", err)
	}

	result := map[string]any{
		"from_function": fromFunction,
		"to_function":   toFunction,
		"max_depth":     maxDepth,
		"call_chains":   results,
		"count":         len(results),
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Found %d call chains from %s", len(results), fromFunction),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/call-chains", projectID),
					"data": result,
				},
			},
		},
	}, nil
}

// handleDetectCircularDeps detects circular dependencies
func (s *Server) HandleDetectCircularDepsInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	scope := "packages"
	if s, ok := input["scope"].(string); ok {
		scope = s
	}

	logger.Info("detecting circular dependencies",
		"project_id", projectID, "scope", scope)

	var query string
	params := map[string]any{"project_id": projectID}

	switch scope {
	case "packages":
		query = `
			MATCH (p1:Package {project_id: $project_id})-[:DEPENDS_ON*2..]->(p1)
			RETURN collect(DISTINCT p1.name) as cycle_packages
		`
	case "functions":
		query = `
			MATCH (f1:Function {project_id: $project_id})-[:CALLS*2..]->(f1)
			RETURN collect(DISTINCT f1.name) as cycle_functions
		`
	default:
		query = `
			MATCH (n {project_id: $project_id})-[:DEPENDS_ON|CALLS*2..]->(n)
			RETURN collect(DISTINCT n.name) as cycles, labels(n)[0] as type
		`
	}

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to detect circular dependencies: %w", err)
	}

	result := map[string]any{
		"scope":   scope,
		"circles": results,
		"count":   len(results),
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Detected %d circular dependencies in %s", len(results), scope),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/circular-deps", projectID),
					"data": result,
				},
			},
		},
	}, nil
}

// handleListPackages lists all packages in the project
func (s *Server) HandleListPackagesInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	pattern := ""
	if p, ok := input["pattern"].(string); ok {
		pattern = p
	}
	includeExternal := false
	if ie, ok := input["include_external"].(bool); ok {
		includeExternal = ie
	}

	logger.Info("listing packages",
		"project_id", projectID,
		"pattern", pattern,
		"include_external", includeExternal)

	query := `
		MATCH (p:Package {project_id: $project_id})
		OPTIONAL MATCH (p)<-[:BELONGS_TO]-(f:File)
		OPTIONAL MATCH (p)<-[:BELONGS_TO]-(fn:Function)
		RETURN p.name as name, p.path as path, count(DISTINCT f) as file_count, count(DISTINCT fn) as function_count
		ORDER BY p.name
	`
	params := map[string]any{"project_id": projectID}

	if pattern != "" {
		query = `
		MATCH (p:Package {project_id: $project_id})
		WHERE p.name CONTAINS $pattern
		OPTIONAL MATCH (p)<-[:BELONGS_TO]-(f:File)
		OPTIONAL MATCH (p)<-[:BELONGS_TO]-(fn:Function)
		RETURN p.name as name, p.path as path, count(DISTINCT f) as file_count, count(DISTINCT fn) as function_count
		ORDER BY p.name
	`
		params["pattern"] = pattern
	}

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	result := map[string]any{
		"packages":         results,
		"count":            len(results),
		"pattern":          pattern,
		"include_external": includeExternal,
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Found %d packages", len(results)),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/packages", projectID),
					"data": result,
				},
			},
		},
	}, nil
}

// handleGetPackageStructure gets detailed structure of a package
//
//nolint:funlen // MCP tool handlers can be longer for comprehensive functionality
func (s *Server) HandleGetPackageStructureInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	packageName, ok := input["package"].(string)
	if !ok {
		return nil, fmt.Errorf("package is required")
	}
	includePrivate := false
	if ip, ok := input["include_private"].(bool); ok {
		includePrivate = ip
	}

	logger.Info("getting package structure",
		"project_id", projectID,
		"package", packageName,
		"include_private", includePrivate)

	// Get package structure
	query := `
		MATCH (pkg:Package {project_id: $project_id, name: $package})
		OPTIONAL MATCH (pkg)-[:CONTAINS]->(f:File)
		OPTIONAL MATCH (f)-[:DEFINES]->(fn:Function)
		OPTIONAL MATCH (f)-[:DEFINES]->(s:Struct)
		OPTIONAL MATCH (f)-[:DEFINES]->(i:Interface)
		RETURN pkg,
		       collect(DISTINCT {name: f.name, path: f.path}) as files,
		       collect(DISTINCT {name: fn.name, signature: fn.signature, is_exported: fn.is_exported}) as functions,
		       collect(DISTINCT {name: s.name, is_exported: s.is_exported}) as structs,
		       collect(DISTINCT {name: i.name, is_exported: i.is_exported}) as interfaces
	`
	params := map[string]any{
		"project_id": projectID,
		"package":    packageName,
	}

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get package structure: %w", err)
	}

	if len(results) == 0 {
		return &ToolResponse{
			Content: []any{
				map[string]any{
					"type": "text",
					"text": fmt.Sprintf("Package %s not found", packageName),
				},
			},
		}, nil
	}

	packageData := results[0]
	files, ok := packageData["files"].([]any)
	if !ok {
		files = []any{}
	}
	functions, ok := packageData["functions"].([]any)
	if !ok {
		functions = []any{}
	}
	structs, ok := packageData["structs"].([]any)
	if !ok {
		structs = []any{}
	}
	interfaces, ok := packageData["interfaces"].([]any)
	if !ok {
		interfaces = []any{}
	}

	// Filter out private elements if not requested
	if !includePrivate {
		functions = FilterExported(functions)
		structs = FilterExported(structs)
		interfaces = FilterExported(interfaces)
	}

	result := map[string]any{
		"package":         packageName,
		"files":           files,
		"functions":       functions,
		"types":           structs,
		"interfaces":      interfaces,
		"include_private": includePrivate,
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Package structure for %s", packageName),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/packages/%s/structure", projectID, packageName),
					"data": result,
				},
			},
		},
	}, nil
}

// handleNaturalLanguageQuery converts natural language to Cypher and executes
func (s *Server) HandleNaturalLanguageQueryInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Extract and validate inputs
	projectID, query, context, err := s.extractNLQueryInputs(input)
	if err != nil {
		return nil, err
	}

	logger.Info("natural language query",
		"project_id", projectID,
		"query", query,
		"context", context)

	// Get schema information for better translation
	schema := s.getSchemaForNLQuery(ctx, projectID)

	// Translate natural language to Cypher
	cypherQuery, params, translationError := s.translateNLToCypher(ctx, query, projectID, schema)

	// Validate the generated query
	isValid, validationSuggestions := s.validateCypherQuery(ctx, cypherQuery, projectID)

	// Execute the query
	results, err := s.serviceAdapter.ExecuteQuery(ctx, cypherQuery, params)
	if err != nil {
		return s.createNLQueryErrorResponse(err, cypherQuery, projectID, isValid,
			validationSuggestions, translationError, schema)
	}

	return s.createNLQuerySuccessResponse(query, cypherQuery, context, results, projectID)
}

// extractNLQueryInputs extracts and validates input parameters
func (s *Server) extractNLQueryInputs(input map[string]any) (string, string, string, error) {
	projectID, err := s.getProjectID(input)
	if err != nil {
		return "", "", "", err
	}

	query, ok := input["query"].(string)
	if !ok {
		return "", "", "", fmt.Errorf("query is required")
	}

	context := ""
	if c, ok := input["context"].(string); ok {
		context = c
	}

	return projectID, query, context, nil
}

// getSchemaForNLQuery retrieves database schema for NL query translation
func (s *Server) getSchemaForNLQuery(ctx context.Context, projectID string) map[string]any {
	schemaInfo, err := s.HandleGetDatabaseSchemaInternal(ctx, map[string]any{
		"project_id":       projectID,
		"include_examples": true,
	})

	if err != nil || len(schemaInfo.Content) < 2 {
		return nil
	}

	resource, ok := schemaInfo.Content[1].(map[string]any)
	if !ok {
		return nil
	}

	resourceData, ok := resource["resource"].(map[string]any)
	if !ok {
		return nil
	}

	data, ok := resourceData["data"].(map[string]any)
	if !ok {
		return nil
	}

	return data
}

// translateNLToCypher translates natural language to Cypher query
func (s *Server) translateNLToCypher(ctx context.Context, query, projectID string,
	schema map[string]any) (string, map[string]any, error) {
	if s.llmService != nil && schema != nil {
		schemaString := s.formatSchemaForLLM(schema)
		result, err := s.llmService.Translate(ctx, query, schemaString)
		if err != nil {
			logger.Warn("Failed to translate query with LLM, using fallback", "error", err)
			cypherQuery, params := GenerateFallbackCypher(query, projectID)
			return cypherQuery, params, err
		}
		return result.Query, map[string]any{"project_id": projectID}, nil
	}

	cypherQuery, params := GenerateFallbackCypher(query, projectID)
	return cypherQuery, params, nil
}

// validateCypherQuery validates a Cypher query
func (s *Server) validateCypherQuery(ctx context.Context, cypherQuery, projectID string) (bool, []string) {
	validation, err := s.HandleValidateCypherQueryInternal(ctx, map[string]any{
		"query":      cypherQuery,
		"project_id": projectID,
	})

	if err != nil || len(validation.Content) < 2 {
		return false, nil
	}

	resource, ok := validation.Content[1].(map[string]any)
	if !ok {
		return false, nil
	}

	resourceData, ok := resource["resource"].(map[string]any)
	if !ok {
		return false, nil
	}

	data, ok := resourceData["data"].(map[string]any)
	if !ok {
		return false, nil
	}

	var isValid bool
	if valid, ok := data["is_valid"].(bool); ok {
		isValid = valid
	}

	var suggestions []string
	if sugg, ok := data["suggestions"].([]string); ok {
		suggestions = sugg
	}

	return isValid, suggestions
}

// createNLQueryErrorResponse creates error response for NL query
func (s *Server) createNLQueryErrorResponse(err error, cypherQuery, projectID string,
	isValid bool, suggestions []string, translationError error, schema map[string]any) (*ToolResponse, error) {
	errorInfo := map[string]any{
		"original_error":       err.Error(),
		"generated_query":      cypherQuery,
		"validation_performed": true,
		"query_was_valid":      isValid,
		"suggestions":          suggestions,
		"translation_error":    translationError,
		"schema_available":     schema != nil,
	}

	if schema != nil {
		errorInfo["available_node_types"] = schema["node_types"]
		errorInfo["available_relationships"] = schema["relationship_types"]
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Query execution failed for project %s", projectID),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/nl-query-error", projectID),
					"data": errorInfo,
				},
			},
		},
	}, nil
}

// createNLQuerySuccessResponse creates success response for NL query
func (s *Server) createNLQuerySuccessResponse(query, cypherQuery, context string,
	results []map[string]any, projectID string) (*ToolResponse, error) {
	result := map[string]any{
		"natural_query": query,
		"cypher_query":  cypherQuery,
		"context":       context,
		"results":       results,
		"result_count":  len(results),
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Natural language query executed, returned %d results", len(results)),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/nl-query-results", projectID),
					"data": result,
				},
			},
		},
	}, nil
}

// handleVerifyCodeExists verifies if a code element exists
//
//nolint:gocyclo,funlen // MCP tool handlers need multiple branches for different element types
func (s *Server) HandleVerifyCodeExistsInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	elementType, ok := input["element_type"].(string)
	if !ok {
		return nil, fmt.Errorf("element_type is required")
	}
	name, ok := input["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name is required")
	}
	packageName := ""
	if p, ok := input["package"].(string); ok {
		packageName = p
	}

	logger.Info("verifying code exists",
		"project_id", projectID,
		"type", elementType,
		"name", name,
		"package", packageName)

	// Build query based on element type
	var query string
	params := map[string]any{
		"project_id": projectID,
		"name":       name,
	}

	switch strings.ToLower(elementType) {
	case "function":
		query = `MATCH (f:Function {project_id: $project_id, name: $name})`
		if packageName != "" {
			query += ` WHERE f.package = $package`
			params["package"] = packageName
		}
		query += ` RETURN f, exists((f)<-[:CONTAINS]-(:File)) as has_file`
	case "struct", "type":
		query = `MATCH (s:Struct {project_id: $project_id, name: $name})`
		if packageName != "" {
			query += ` WHERE s.package = $package`
			params["package"] = packageName
		}
		query += ` RETURN s, exists((s)<-[:CONTAINS]-(:File)) as has_file`
	case "interface":
		query = `MATCH (i:Interface {project_id: $project_id, name: $name})`
		if packageName != "" {
			query += ` WHERE i.package = $package`
			params["package"] = packageName
		}
		query += ` RETURN i, exists((i)<-[:CONTAINS]-(:File)) as has_file`
	case "package":
		query = `MATCH (p:Package {project_id: $project_id, name: $name}) RETURN p, true as has_file`
	default:
		return nil, fmt.Errorf("unsupported element type: %s", elementType)
	}

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to verify code existence: %w", err)
	}

	exists := len(results) > 0
	var elementData map[string]any
	if exists {
		elementData = results[0]
	}

	result := map[string]any{
		"exists":       exists,
		"element_type": elementType,
		"name":         name,
		"package":      packageName,
	}

	if exists && elementData != nil {
		// Add detailed information about the found element
		for key, value := range elementData {
			if key != "has_file" {
				if nodeData, ok := value.(map[string]any); ok {
					result["details"] = nodeData
				}
			}
		}
	}

	message := fmt.Sprintf("Element %s not found", name)
	if exists {
		message = fmt.Sprintf("Element %s exists", name)
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": message,
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/verify/%s", projectID, name),
					"data": result,
				},
			},
		},
	}, nil
}

// Helper methods and conversion functions

func FilterExported(items []any) []any {
	var filtered []any
	for _, item := range items {
		if itemMap, ok := item.(map[string]any); ok {
			if exported, ok := itemMap["is_exported"].(bool); ok && exported {
				filtered = append(filtered, item)
			}
		}
	}
	return filtered
}

// GenerateFallbackCypher generates a parameterized Cypher query from natural language
// Returns the query string and a map of parameters to prevent injection attacks
func GenerateFallbackCypher(naturalQuery, projectID string) (string, map[string]any) {
	// Enhanced fallback query generation with better keyword parsing
	query := strings.ToLower(naturalQuery)

	// Extract search terms
	searchTerms := extractSearchTerms(query)

	// Initialize parameters with project ID
	params := map[string]any{"project_id": projectID}

	// Determine query type and generate appropriate query
	switch {
	case strings.Contains(query, "test") && strings.Contains(query, "file"):
		return generateTestFileQuery(searchTerms, params)
	case strings.Contains(query, "function"):
		return generateFunctionQuery(searchTerms, params)
	case strings.Contains(query, "package"):
		return generatePackageQuery(searchTerms, params)
	case strings.Contains(query, "struct") || strings.Contains(query, "type"):
		return generateStructQuery(searchTerms, params)
	case strings.Contains(query, "interface"):
		return generateInterfaceQuery(searchTerms, params)
	default:
		return generateDefaultQuery(params)
	}
}

// extractSearchTerms extracts meaningful words from a query
func extractSearchTerms(query string) []string {
	words := strings.Fields(query)
	var searchTerms []string

	// Common stop words to ignore
	stopWords := map[string]bool{
		"show": true, "me": true, "the": true, "a": true, "an": true,
		"for": true, "in": true, "existing": true, "find": true, "get": true,
		"list": true, "all": true, "with": true, "from": true, "to": true,
	}

	// Extract search terms
	for _, word := range words {
		if !stopWords[word] && len(word) > 2 {
			searchTerms = append(searchTerms, word)
		}
	}

	return searchTerms
}

// generateTestFileQuery generates a query for finding test files
func generateTestFileQuery(searchTerms []string, params map[string]any) (string, map[string]any) {
	conditions := []string{"(f.path CONTAINS '_test.go' OR f.path CONTAINS '/test/')"}

	// Add search term filters with parameterized queries
	termIndex := 0
	for _, term := range searchTerms {
		if term != "test" && term != "file" && term != "files" {
			paramName := fmt.Sprintf("term%d", termIndex)
			conditions = append(
				conditions,
				fmt.Sprintf("(toLower(f.path) CONTAINS $%s OR toLower(f.name) CONTAINS $%s)", paramName, paramName),
			)
			params[paramName] = term
			termIndex++
		}
	}

	queryString := fmt.Sprintf(
		"MATCH (f:File {project_id: $project_id}) WHERE %s RETURN f.path, f.name LIMIT 20",
		strings.Join(conditions, " AND "),
	)
	return queryString, params
}

// generateFunctionQuery generates a query for finding functions
func generateFunctionQuery(searchTerms []string, params map[string]any) (string, map[string]any) {
	conditions := buildSearchConditions(searchTerms, []string{"function", "functions"}, "f", params)
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	queryString := fmt.Sprintf(
		"MATCH (f:Function {project_id: $project_id})%s "+
			"RETURN f.name, f.package, f.signature, f.file_path, f.line_start, f.is_exported LIMIT 20",
		whereClause,
	)
	return queryString, params
}

// generatePackageQuery generates a query for finding packages
func generatePackageQuery(searchTerms []string, params map[string]any) (string, map[string]any) {
	conditions := buildSearchConditions(searchTerms, []string{"package", "packages"}, "p", params)
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	queryString := fmt.Sprintf(
		"MATCH (p:Package {project_id: $project_id})%s RETURN p.name, p.path LIMIT 20",
		whereClause,
	)
	return queryString, params
}

// generateStructQuery generates a query for finding structs
func generateStructQuery(searchTerms []string, params map[string]any) (string, map[string]any) {
	conditions := buildSearchConditions(searchTerms, []string{"struct", "type", "structs", "types"}, "s", params)
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	queryString := fmt.Sprintf(
		"MATCH (s:Struct {project_id: $project_id})%s RETURN s.name, s.package LIMIT 20",
		whereClause,
	)
	return queryString, params
}

// generateInterfaceQuery generates a query for finding interfaces
func generateInterfaceQuery(searchTerms []string, params map[string]any) (string, map[string]any) {
	conditions := buildSearchConditions(searchTerms, []string{"interface", "interfaces"}, "i", params)
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	queryString := fmt.Sprintf(
		"MATCH (i:Interface {project_id: $project_id})%s RETURN i.name, i.package LIMIT 20",
		whereClause,
	)
	return queryString, params
}

// generateDefaultQuery generates a default overview query
func generateDefaultQuery(params map[string]any) (string, map[string]any) {
	defaultQuery := "MATCH (f:Function {project_id: $project_id}) " +
		"RETURN f.name, f.package, f.signature, f.file_path, f.line_start, f.is_exported " +
		"ORDER BY f.package, f.name LIMIT 50"
	return defaultQuery, params
}

// buildSearchConditions builds search conditions for queries
func buildSearchConditions(searchTerms []string, excludeTerms []string, alias string, params map[string]any) []string {
	conditions := []string{}
	termIndex := 0

	// Convert excludeTerms to map for efficient lookup
	excludeMap := make(map[string]bool)
	for _, term := range excludeTerms {
		excludeMap[term] = true
	}

	for _, term := range searchTerms {
		if !excludeMap[term] {
			paramName := fmt.Sprintf("term%d", termIndex)
			nameField := fmt.Sprintf("%s.name", alias)
			var secondField string
			if alias == "p" {
				secondField = fmt.Sprintf("%s.path", alias)
			} else {
				secondField = fmt.Sprintf("%s.package", alias)
			}
			conditions = append(
				conditions,
				fmt.Sprintf(
					"(toLower(%s) CONTAINS $%s OR toLower(%s) CONTAINS $%s)",
					nameField,
					paramName,
					secondField,
					paramName,
				),
			)
			params[paramName] = term
			termIndex++
		}
	}

	return conditions
}

// Remaining stub handlers - These will be implemented in Phase 2

// handleGetCodeContext gets context around a code element
func (s *Server) HandleGetCodeContextInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	projectID, elementType, name, contextLines, err := s.ParseCodeContextInput(input)
	if err != nil {
		return nil, err
	}

	// Get the element's location from the graph
	query, err := s.BuildElementLocationQuery(elementType)
	if err != nil {
		return nil, err
	}

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, map[string]any{
		"project_id": projectID,
		"name":       name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query element location: %w", err)
	}

	return s.ExtractCodeContextFromResults(results, projectID, elementType, name, contextLines)
}

// BuildElementLocationQuery builds a query to find an element's location
func (s *Server) BuildElementLocationQuery(elementType string) (string, error) {
	switch elementType {
	case "function":
		return `
			MATCH (f:Function {project_id: $project_id})
			WHERE f.name = $name OR toLower(f.name) CONTAINS toLower($name)
			OPTIONAL MATCH (file:File)-[:DEFINES]->(f)
			RETURN f.line_start as line_start, f.line_end as line_end, 
			       COALESCE(file.path, f.file_path) as file_path,
			       f.signature as signature,
			       f.package as package
			ORDER BY CASE WHEN f.name = $name THEN 0 ELSE 1 END
			LIMIT 1
		`, nil
	case "struct":
		return `
			MATCH (s:Struct {project_id: $project_id})
			WHERE s.name = $name OR toLower(s.name) CONTAINS toLower($name)
			OPTIONAL MATCH (file:File)-[:DEFINES]->(s)
			RETURN s.line_start as line_start, s.line_end as line_end, 
			       COALESCE(file.path, s.file_path) as file_path,
			       s.package as package
			ORDER BY CASE WHEN s.name = $name THEN 0 ELSE 1 END
			LIMIT 1
		`, nil
	case "interface":
		return `
			MATCH (i:Interface {project_id: $project_id})
			WHERE i.name = $name OR toLower(i.name) CONTAINS toLower($name)
			OPTIONAL MATCH (file:File)-[:DEFINES]->(i)
			RETURN i.line_start as line_start, i.line_end as line_end, 
			       COALESCE(file.path, i.file_path) as file_path,
			       i.package as package
			ORDER BY CASE WHEN i.name = $name THEN 0 ELSE 1 END
			LIMIT 1
		`, nil
	default:
		return "", fmt.Errorf("unsupported element type: %s", elementType)
	}
}

// codeContextParams holds parameters for code context requests
// ParseCodeContextInput parses and validates input for code context requests
func (s *Server) ParseCodeContextInput(
	input map[string]any,
) (projectID, elementType, name string, contextLines int, err error) {
	// Get project ID using helper
	projectID, err = s.getProjectID(input)
	if err != nil {
		return "", "", "", 0, err
	}
	elementType, ok := input["element_type"].(string)
	if !ok {
		return "", "", "", 0, fmt.Errorf("element_type is required")
	}
	name, ok = input["name"].(string)
	if !ok {
		return "", "", "", 0, fmt.Errorf("name is required")
	}
	contextLines = 5 // Default
	if cl, ok := input["context_lines"].(float64); ok {
		contextLines = int(cl)
	}

	logger.Info("getting code context",
		"project_id", projectID,
		"type", elementType,
		"name", name,
		"context_lines", contextLines)

	return projectID, elementType, name, contextLines, nil
}

// ExtractCodeContextFromResults processes query results and extracts code context
//
//nolint:funlen // MCP handler functions can be comprehensive for full source code extraction
func (s *Server) ExtractCodeContextFromResults(
	results []map[string]any,
	projectID, elementType, name string,
	contextLines int,
) (*ToolResponse, error) {
	if len(results) == 0 {
		return &ToolResponse{
			Content: []any{
				map[string]any{
					"type": "text",
					"text": fmt.Sprintf("%s '%s' not found in project", elementType, name),
				},
			},
		}, nil
	}

	elementData := results[0]
	filePath, ok := elementData["file_path"].(string)
	if !ok {
		return &ToolResponse{
			Content: []any{
				map[string]any{
					"type": "text",
					"text": fmt.Sprintf("Invalid file_path data for %s '%s'", elementType, name),
				},
			},
		}, nil
	}
	lineStart, ok := elementData["line_start"].(int64)
	if !ok {
		lineStart = 1 // Default to line 1 if not available
	}
	lineEnd := lineStart
	if le, ok := elementData["line_end"].(int64); ok {
		lineEnd = le
	}

	if filePath == "" {
		return &ToolResponse{
			Content: []any{
				map[string]any{
					"type": "text",
					"text": fmt.Sprintf("File path not available for %s '%s'", elementType, name),
				},
			},
		}, nil
	}

	// Read the entire function/struct/interface if context_lines is 0
	startLine := int(lineStart)
	endLine := int(lineEnd)
	if contextLines > 0 {
		// Include context around the element
		startLine = max(1, int(lineStart)-contextLines)
		endLine = int(lineEnd) + contextLines
	}

	// Read the file and extract the source code
	sourceCode, err := s.ReadFileLines(filePath, startLine, endLine)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Build metadata
	metadata := map[string]any{
		"element_type": elementType,
		"name":         name,
		"file_path":    filePath,
		"line_start":   lineStart,
		"line_end":     lineEnd,
	}

	// Add additional metadata if available
	if sig, ok := elementData["signature"].(string); ok && sig != "" {
		metadata["signature"] = sig
	}
	if pkg, ok := elementData["package"].(string); ok && pkg != "" {
		metadata["package"] = pkg
	}

	result := map[string]any{
		"code":     sourceCode,
		"metadata": metadata,
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Source code for %s '%s' from %s", elementType, name, filePath),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/source/%s/%s", projectID, elementType, name),
					"data": result,
				},
			},
		},
	}, nil
}

// handleValidateImportPath validates an import path
func (s *Server) HandleValidateImportPathInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	importPath, ok := input["import_path"].(string)
	if !ok {
		return nil, fmt.Errorf("import_path is required")
	}
	fromPackage := ""
	if fp, ok := input["from_package"].(string); ok {
		fromPackage = fp
	}

	logger.Info("validating import path",
		"project_id", projectID,
		"import_path", importPath,
		"from_package", fromPackage)

	// Check if the import path exists in the project's packages
	query := `
		MATCH (p:Package {project_id: $project_id})
		WHERE p.import_path = $import_path OR p.name = $import_path
		RETURN p.name as package_name, p.import_path as resolved_path, p.path as file_path
	`

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, map[string]any{
		"project_id":  projectID,
		"import_path": importPath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to validate import path: %w", err)
	}

	isValid := len(results) > 0
	var resolvedPath string
	var packageName string

	if isValid {
		packageData := results[0]
		var ok bool
		packageName, ok = packageData["package_name"].(string)
		if !ok {
			packageName = importPath // Fallback to import path
		}
		resolvedPath, ok = packageData["resolved_path"].(string)
		if !ok {
			resolvedPath = importPath // Fallback to import path
		}
		if resolvedPath == "" {
			resolvedPath = importPath
		}
	} else {
		// Check if it's a standard library package
		isValid = s.IsStandardLibraryPackage(importPath)
		resolvedPath = importPath
		packageName = importPath
	}

	result := map[string]any{
		"valid":        isValid,
		"import_path":  importPath,
		"resolved_to":  resolvedPath,
		"package_name": packageName,
		"from_package": fromPackage,
		"is_standard":  s.IsStandardLibraryPackage(importPath),
		"is_external":  !isValid && !s.IsStandardLibraryPackage(importPath),
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Import path %s is valid", importPath),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/imports/%s", projectID, importPath),
					"data": result,
				},
			},
		},
	}, nil
}

// handleDetectCodePatterns detects code patterns
func (s *Server) HandleDetectCodePatternsInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	// Extract patterns filter (optional)
	_ = input["patterns"] // TODO: Use specific patterns filter in future
	scope := ""
	if s, ok := input["scope"].(string); ok {
		scope = s
	}

	logger.Info("detecting code patterns",
		"project_id", projectID, "scope", scope)
	// Detect common Go patterns in the codebase
	patternsFound := s.DetectCommonPatterns(ctx, projectID, scope)

	result := map[string]any{
		"patterns_found": patternsFound,
		"scope":          scope,
		"count":          len(patternsFound),
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": "Pattern detection complete",
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/patterns", projectID),
					"data": result,
				},
			},
		},
	}, nil
}

// handleGetNamingConventions analyzes naming conventions
func (s *Server) HandleGetNamingConventionsInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	scope := ""
	if s, ok := input["scope"].(string); ok {
		scope = s
	}
	includeSuggestions := false
	if is, ok := input["include_suggestions"].(bool); ok {
		includeSuggestions = is
	}

	logger.Info("getting naming conventions",
		"project_id", projectID,
		"scope", scope,
		"include_suggestions", includeSuggestions)

	// Analyze naming conventions in the project
	conventions := s.AnalyzeNamingConventions(ctx, projectID, scope)

	result := map[string]any{
		"conventions": conventions,
		"scope":       scope,
		"analysis":    "Based on actual code patterns in the project",
	}

	if includeSuggestions {
		result["suggestions"] = []map[string]any{}
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": "Naming conventions analysis complete",
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/naming", projectID),
					"data": result,
				},
			},
		},
	}, nil
}

// handleFindTestsForCode finds tests for code elements
func (s *Server) HandleFindTestsForCodeInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	elementType, ok := input["element_type"].(string)
	if !ok {
		return nil, fmt.Errorf("element_type is required")
	}
	name, ok := input["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name is required")
	}
	packageName := ""
	if p, ok := input["package"].(string); ok {
		packageName = p
	}

	logger.Info("finding tests for code",
		"project_id", projectID,
		"type", elementType,
		"name", name,
		"package", packageName)

	// Find test files and functions that might test this element
	testsFound := s.FindTestsForElement(ctx, projectID, elementType, name, packageName)

	result := map[string]any{
		"element":      name,
		"tests_found":  testsFound,
		"element_type": elementType,
		"package":      packageName,
		"test_count":   len(testsFound),
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Test search complete for %s", name),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/tests/%s", projectID, name),
					"data": result,
				},
			},
		},
	}, nil
}

// handleCheckTestCoverage checks test coverage
func (s *Server) HandleCheckTestCoverageInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	path := ""
	if p, ok := input["path"].(string); ok {
		path = p
	}
	detailed := false
	if d, ok := input["detailed"].(bool); ok {
		detailed = d
	}

	logger.Info("checking test coverage",
		"project_id", projectID,
		"path", path,
		"detailed", detailed)

	// Analyze test coverage for the given path
	coverage := s.AnalyzeTestCoverage(ctx, projectID, path)

	result := map[string]any{
		"path":            path,
		"coverage":        coverage.Percentage,
		"covered_lines":   coverage.CoveredLines,
		"total_lines":     coverage.TotalLines,
		"test_files":      coverage.TestFiles,
		"analysis_method": "Static analysis based on test function naming patterns",
	}

	if detailed {
		result["details"] = map[string]any{
			"lines_covered":     0,
			"lines_total":       0,
			"functions_covered": 0,
			"functions_total":   0,
		}
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": "Test coverage analysis complete",
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/coverage", projectID),
					"data": result,
				},
			},
		},
	}, nil
}

// Resource handlers (stubs for now)

// handleProjectMetadataResource provides project metadata
func (s *Server) HandleProjectMetadataResource(ctx context.Context, params map[string]string) ([]byte, error) {
	projectID := params["project_id"]

	// Get project statistics
	stats, err := s.serviceAdapter.GetProjectStatistics(ctx, core.ID(projectID))
	if err != nil {
		return nil, fmt.Errorf("failed to get project statistics: %w", err)
	}

	metadata := map[string]any{
		"project_id": projectID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"statistics": ConvertStatistics(stats),
		"version":    "1.0.0",
	}

	return json.Marshal(metadata)
}

// handleQueryTemplatesResource provides query templates
func (s *Server) HandleQueryTemplatesResource(_ context.Context, _ map[string]string) ([]byte, error) {
	// Return available query templates from the CommonTemplates map
	templateList := make([]map[string]any, 0)
	for name, template := range query.CommonTemplates {
		templateList = append(templateList, map[string]any{
			"name":        name,
			"description": template.Description,
			"category":    template.Category,
			"parameters":  template.Parameters,
		})
	}

	data := map[string]any{
		"templates": templateList,
		"categories": []string{
			"dependency_analysis",
			"code_structure",
			"quality_metrics",
			"test_coverage",
		},
	}

	return json.Marshal(data)
}

// handleCodePatternsResource provides code patterns
func (s *Server) HandleCodePatternsResource(_ context.Context, _ map[string]string) ([]byte, error) {
	patterns := map[string]any{
		"patterns": []map[string]string{
			{
				"id":          "singleton",
				"name":        "Singleton Pattern",
				"description": "Ensures a class has only one instance",
				"category":    "creational",
			},
			{
				"id":          "factory",
				"name":        "Factory Pattern",
				"description": "Creates objects without specifying exact classes",
				"category":    "creational",
			},
			{
				"id":          "circular_dependency",
				"name":        "Circular Dependency",
				"description": "Mutual dependencies between packages",
				"category":    "anti-pattern",
			},
		},
		"categories": []string{
			"creational",
			"structural",
			"behavioral",
			"anti-pattern",
		},
	}

	return json.Marshal(patterns)
}

// handleProjectInvariantsResource provides project invariants
func (s *Server) HandleProjectInvariantsResource(_ context.Context, params map[string]string) ([]byte, error) {
	projectID := params["project_id"]

	invariants := map[string]any{
		"project_id": projectID,
		"rules": []map[string]any{
			{
				"id":          "no_circular_deps",
				"description": "No circular dependencies allowed",
				"severity":    "error",
				"enabled":     true,
			},
			{
				"id":          "max_package_depth",
				"description": "Maximum package nesting depth",
				"severity":    "warning",
				"value":       5,
				"enabled":     true,
			},
		},
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}

	return json.Marshal(invariants)
}

func ConvertStatistics(stats *graph.ProjectStatistics) map[string]any {
	if stats == nil {
		return map[string]any{}
	}

	return map[string]any{
		"total_nodes":           stats.TotalNodes,
		"total_relationships":   stats.TotalRelationships,
		"nodes_by_type":         stats.NodesByType,
		"relationships_by_type": stats.RelationshipsByType,
		"top_packages":          stats.TopPackages,
		"top_functions":         stats.TopFunctions,
	}
}

// ReadFileLines reads lines from a file between startLine and endLine (inclusive)
func (s *Server) ReadFileLines(filePath string, startLine, endLine int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	var result strings.Builder
	scanner := bufio.NewScanner(file)
	lineNum := 1

	for scanner.Scan() {
		if lineNum >= startLine && lineNum <= endLine {
			result.WriteString(fmt.Sprintf("%4d: %s\n", lineNum, scanner.Text()))
		}
		if lineNum > endLine {
			break
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	return result.String(), nil
}

// ReadFileContext reads lines around a specific line number in a file
func (s *Server) ReadFileContext(filePath string, targetLine, contextLines int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	lineNum := 1

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	// Calculate the range
	startLine := targetLine - contextLines
	endLine := targetLine + contextLines

	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Build the context with line numbers
	var result strings.Builder
	for i := startLine - 1; i < endLine; i++ {
		lineMarker := "  "
		if i+1 == targetLine {
			lineMarker = ">>" // Mark the target line
		}
		result.WriteString(fmt.Sprintf("%s %4d: %s\n", lineMarker, i+1, lines[i]))
	}

	return result.String(), nil
}

// IsStandardLibraryPackage checks if an import path is a Go standard library package
func (s *Server) IsStandardLibraryPackage(importPath string) bool {
	// Common Go standard library packages
	standardPackages := map[string]bool{
		"fmt":     true,
		"os":      true,
		"io":      true,
		"net":     true,
		"http":    true,
		"strings": true,
		"strconv": true,
		"time":    true,
		"context": true,
		"errors":  true,
		"sync":    true,
		"json":    true,
		"log":     true,
		"path":    true,
		"sort":    true,
		"bytes":   true,
		"bufio":   true,
		"regexp":  true,
		"reflect": true,
	}

	// Check if it's a direct standard package
	if standardPackages[importPath] {
		return true
	}

	// Check if it's a subpackage of a standard package
	standardPrefixes := []string{
		"net/",
		"crypto/",
		"encoding/",
		"go/",
		"text/",
		"html/",
		"image/",
		"archive/",
		"compress/",
		"container/",
		"database/",
		"debug/",
		"path/",
		"mime/",
		"math/",
		"os/",
		"runtime/",
		"testing/",
	}

	for _, prefix := range standardPrefixes {
		if strings.HasPrefix(importPath, prefix) {
			return true
		}
	}

	return false
}

// DetectCommonPatterns detects common Go programming patterns
func (s *Server) DetectCommonPatterns(ctx context.Context, projectID, _ string) []map[string]any {
	var patterns []map[string]any

	// Pattern 1: Interface implementations
	interfacePattern := s.DetectInterfacePattern(ctx, projectID)
	if interfacePattern != nil {
		patterns = append(patterns, interfacePattern)
	}

	// Pattern 2: Factory functions
	factoryPattern := s.DetectFactoryPattern(ctx, projectID)
	if factoryPattern != nil {
		patterns = append(patterns, factoryPattern)
	}

	// Pattern 3: Error handling patterns
	errorPattern := s.DetectErrorPattern(ctx, projectID)
	if errorPattern != nil {
		patterns = append(patterns, errorPattern)
	}

	return patterns
}

// DetectInterfacePattern detects interface implementation patterns
func (s *Server) DetectInterfacePattern(ctx context.Context, projectID string) map[string]any {
	query := `
		MATCH (i:Interface {project_id: $project_id})
		OPTIONAL MATCH (s:Struct {project_id: $project_id})-[:IMPLEMENTS]->(i)
		RETURN i.name as interface_name, count(s) as implementations
		ORDER BY implementations DESC
		LIMIT 5
	`

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, map[string]any{
		"project_id": projectID,
	})
	if err != nil || len(results) == 0 {
		return nil
	}

	return map[string]any{
		"type":        "interface_implementation",
		"name":        "Interface Implementation Pattern",
		"description": "Interfaces with multiple implementations detected",
		"examples":    results,
		"confidence":  0.8,
	}
}

// DetectFactoryPattern detects factory function patterns
func (s *Server) DetectFactoryPattern(ctx context.Context, projectID string) map[string]any {
	query := `
		MATCH (f:Function {project_id: $project_id})
		WHERE f.name STARTS WITH 'New' AND size(f.returns) > 0
		RETURN f.name as function_name, f.package as package_name, f.returns as returns
		LIMIT 10
	`

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, map[string]any{
		"project_id": projectID,
	})
	if err != nil || len(results) == 0 {
		return nil
	}

	return map[string]any{
		"type":        "factory_function",
		"name":        "Factory Function Pattern",
		"description": "Constructor functions following 'New*' naming convention",
		"examples":    results,
		"confidence":  0.9,
	}
}

// DetectErrorPattern detects error handling patterns
func (s *Server) DetectErrorPattern(ctx context.Context, projectID string) map[string]any {
	query := `
		MATCH (f:Function {project_id: $project_id})
		WHERE any(ret IN f.returns WHERE ret = 'error')
		RETURN count(f) as error_returning_functions
	`

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, map[string]any{
		"project_id": projectID,
	})
	if err != nil || len(results) == 0 {
		return nil
	}

	errorCount, ok := results[0]["error_returning_functions"].(int64)
	if !ok {
		errorCount = 0
	}
	if errorCount == 0 {
		return nil
	}

	return map[string]any{
		"type":        "error_handling",
		"name":        "Error Handling Pattern",
		"description": fmt.Sprintf("Found %d functions returning errors", errorCount),
		"examples":    results,
		"confidence":  0.95,
	}
}

// AnalyzeNamingConventions analyzes naming patterns in the codebase
func (s *Server) AnalyzeNamingConventions(ctx context.Context, projectID, _ string) map[string]any {
	conventions := make(map[string]any)

	// Analyze function naming
	functionQuery := `
		MATCH (f:Function {project_id: $project_id})
		WHERE f.is_exported = true
		RETURN f.name as name
		LIMIT 20
	`

	functionResults, err := s.serviceAdapter.ExecuteQuery(ctx, functionQuery, map[string]any{
		"project_id": projectID,
	})
	if err == nil && len(functionResults) > 0 {
		functionPatterns := s.AnalyzeFunctionNaming(functionResults)
		conventions["functions"] = functionPatterns
	}

	// Analyze type naming
	typeQuery := `
		MATCH (s:Struct {project_id: $project_id})
		WHERE s.is_exported = true
		RETURN s.name as name
		LIMIT 20
	`

	typeResults, err := s.serviceAdapter.ExecuteQuery(ctx, typeQuery, map[string]any{
		"project_id": projectID,
	})
	if err == nil && len(typeResults) > 0 {
		typePatterns := s.AnalyzeTypeNaming(typeResults)
		conventions["types"] = typePatterns
	}

	// Analyze interface naming
	interfaceQuery := `
		MATCH (i:Interface {project_id: $project_id})
		WHERE i.is_exported = true
		RETURN i.name as name
		LIMIT 20
	`

	interfaceResults, err := s.serviceAdapter.ExecuteQuery(ctx, interfaceQuery, map[string]any{
		"project_id": projectID,
	})
	if err == nil && len(interfaceResults) > 0 {
		interfacePatterns := s.AnalyzeInterfaceNaming(interfaceResults)
		conventions["interfaces"] = interfacePatterns
	}

	return conventions
}

// AnalyzeFunctionNaming analyzes function naming patterns
func (s *Server) AnalyzeFunctionNaming(results []map[string]any) map[string]any {
	patterns := map[string]int{
		"camelCase":       0,
		"PascalCase":      0,
		"snake_case":      0,
		"starts_with_Get": 0,
		"starts_with_Set": 0,
		"starts_with_New": 0,
	}

	for _, result := range results {
		name, ok := result["name"].(string)
		if !ok || name == "" {
			continue
		}

		// Check patterns
		if s.IsCamelCase(name) {
			patterns["camelCase"]++
		}
		if s.IsPascalCase(name) {
			patterns["PascalCase"]++
		}
		if strings.Contains(name, "_") {
			patterns["snake_case"]++
		}
		if strings.HasPrefix(name, "Get") {
			patterns["starts_with_Get"]++
		}
		if strings.HasPrefix(name, "Set") {
			patterns["starts_with_Set"]++
		}
		if strings.HasPrefix(name, "New") {
			patterns["starts_with_New"]++
		}
	}

	return map[string]any{
		"patterns": patterns,
		"total":    len(results),
	}
}

// AnalyzeTypeNaming analyzes struct/type naming patterns
func (s *Server) AnalyzeTypeNaming(results []map[string]any) map[string]any {
	patterns := map[string]int{
		"PascalCase": 0,
		"camelCase":  0,
		"UPPER_CASE": 0,
	}

	for _, result := range results {
		name, ok := result["name"].(string)
		if !ok || name == "" {
			continue
		}

		switch {
		case s.IsPascalCase(name):
			patterns["PascalCase"]++
		case s.IsCamelCase(name):
			patterns["camelCase"]++
		case strings.ToUpper(name) == name:
			patterns["UPPER_CASE"]++
		}
	}

	return map[string]any{
		"patterns": patterns,
		"total":    len(results),
	}
}

// AnalyzeInterfaceNaming analyzes interface naming patterns
func (s *Server) AnalyzeInterfaceNaming(results []map[string]any) map[string]any {
	patterns := map[string]int{
		"ends_with_er":   0,
		"ends_with_able": 0,
		"PascalCase":     0,
	}

	for _, result := range results {
		name, ok := result["name"].(string)
		if !ok || name == "" {
			continue
		}

		if strings.HasSuffix(name, "er") {
			patterns["ends_with_er"]++
		}
		if strings.HasSuffix(name, "able") {
			patterns["ends_with_able"]++
		}
		if s.IsPascalCase(name) {
			patterns["PascalCase"]++
		}
	}

	return map[string]any{
		"patterns": patterns,
		"total":    len(results),
	}
}

// IsCamelCase checks if a string is in camelCase
func (s *Server) IsCamelCase(str string) bool {
	if str == "" {
		return false
	}
	return str[0] >= 'a' && str[0] <= 'z' && !strings.Contains(str, "_")
}

// IsPascalCase checks if a string is in PascalCase
func (s *Server) IsPascalCase(str string) bool {
	if str == "" {
		return false
	}
	return str[0] >= 'A' && str[0] <= 'Z' && !strings.Contains(str, "_")
}

// FindTestsForElement finds test functions that might test a specific element
func (s *Server) FindTestsForElement(
	ctx context.Context,
	projectID, _, name, _ string,
) []map[string]any {
	// Find test functions that contain the element name
	query := `
		MATCH (f:Function {project_id: $project_id})
		WHERE f.name CONTAINS 'Test' AND (
			f.name CONTAINS $name OR
			f.name =~ ('.*' + $name + '.*') OR
			f.name =~ ('Test.*' + $name + '.*')
		)
		OPTIONAL MATCH (file:File)-[:CONTAINS]->(f)
		RETURN f.name as test_name, f.package as test_package, file.path as file_path
		LIMIT 10
	`

	results, err := s.serviceAdapter.ExecuteQuery(ctx, query, map[string]any{
		"project_id": projectID,
		"name":       name,
	})
	if err != nil {
		return []map[string]any{}
	}

	var tests []map[string]any
	for _, result := range results {
		testName, ok := result["test_name"].(string)
		if !ok {
			continue
		}
		testPackage, ok := result["test_package"].(string)
		if !ok {
			testPackage = ""
		}
		filePath, ok := result["file_path"].(string)
		if !ok {
			filePath = ""
		}

		tests = append(tests, map[string]any{
			"test_name":    testName,
			"test_package": testPackage,
			"file_path":    filePath,
			"match_type":   "name_pattern",
		})
	}

	return tests
}

// TestCoverage represents test coverage information
type TestCoverage struct {
	Percentage   float64          `json:"percentage"`
	CoveredLines int              `json:"covered_lines"`
	TotalLines   int              `json:"total_lines"`
	TestFiles    []map[string]any `json:"test_files"`
}

// AnalyzeTestCoverage analyzes test coverage for a given path
func (s *Server) AnalyzeTestCoverage(ctx context.Context, projectID, path string) TestCoverage {
	// Find functions in the target path
	functionsQuery := `
		MATCH (f:Function {project_id: $project_id})
		OPTIONAL MATCH (file:File)-[:CONTAINS]->(f)
		WHERE file.path CONTAINS $path OR f.package CONTAINS $path
		RETURN count(f) as total_functions
	`

	functionResults, err := s.serviceAdapter.ExecuteQuery(ctx, functionsQuery, map[string]any{
		"project_id": projectID,
		"path":       path,
	})
	if err != nil {
		return TestCoverage{}
	}

	totalFunctions := int64(0)
	if len(functionResults) > 0 {
		if val, ok := functionResults[0]["total_functions"].(int64); ok {
			totalFunctions = val
		}
	}

	// Find test functions that might cover this path
	testQuery := `
		MATCH (f:Function {project_id: $project_id})
		WHERE f.name CONTAINS 'Test'
		OPTIONAL MATCH (file:File)-[:CONTAINS]->(f)
		WHERE file.path CONTAINS 'test' OR file.path CONTAINS '_test'
		RETURN count(f) as test_functions, collect(DISTINCT file.path) as test_files
	`

	testResults, err := s.serviceAdapter.ExecuteQuery(ctx, testQuery, map[string]any{
		"project_id": projectID,
	})
	if err != nil {
		return TestCoverage{}
	}

	testFunctions := int64(0)
	var testFiles []string
	if len(testResults) > 0 {
		if val, ok := testResults[0]["test_functions"].(int64); ok {
			testFunctions = val
		}
		if tf, ok := testResults[0]["test_files"].([]any); ok {
			for _, file := range tf {
				if filePath, ok := file.(string); ok {
					testFiles = append(testFiles, filePath)
				}
			}
		}
	}

	// Simple coverage estimation based on test to function ratio
	coveragePercentage := 0.0
	if totalFunctions > 0 {
		// Very basic heuristic: assume each test covers 1-2 functions
		estimatedCoverage := float64(testFunctions) * 1.5 / float64(totalFunctions)
		if estimatedCoverage > 1.0 {
			estimatedCoverage = 1.0
		}
		coveragePercentage = estimatedCoverage * 100.0
	}

	// Convert test files to structured format
	var testFileStructs []map[string]any
	for _, file := range testFiles {
		testFileStructs = append(testFileStructs, map[string]any{
			"path": file,
			"type": "test_file",
		})
	}

	return TestCoverage{
		Percentage:   coveragePercentage,
		CoveredLines: int(testFunctions), // Approximate
		TotalLines:   int(totalFunctions),
		TestFiles:    testFileStructs,
	}
}

// HandleListProjectsInternal lists all projects in the database
func (s *Server) HandleListProjectsInternal(ctx context.Context, _ map[string]any) (*ToolResponse, error) {
	logger.Info("listing all projects in database")

	projects, err := s.serviceAdapter.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	// Convert projects to response format
	projectData := make([]map[string]any, len(projects))
	for i := range projects {
		projectData[i] = map[string]any{
			"id":   projects[i].ID.String(),
			"name": projects[i].Name,
		}
	}

	result := map[string]any{
		"projects": projectData,
		"count":    len(projects),
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Found %d projects in database", len(projects)),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  "/projects",
					"data": result,
				},
			},
		},
	}, nil
}

// HandleValidateProjectInternal validates if a project exists in the database
func (s *Server) HandleValidateProjectInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	// Get project ID using helper
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}

	logger.Info("validating project existence", "project_id", projectID)

	exists, err := s.serviceAdapter.ValidateProject(ctx, core.ID(projectID))
	if err != nil {
		return nil, fmt.Errorf("failed to validate project: %w", err)
	}

	result := map[string]any{
		"project_id": projectID,
		"exists":     exists,
		"valid":      exists,
	}

	status := "does not exist"
	if exists {
		status = "exists"
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Project %s %s in database", projectID, status),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/validation", projectID),
					"data": result,
				},
			},
		},
	}, nil
}

// HandleGetDatabaseSchemaInternal provides Neo4j schema information for LLM query assistance
func (s *Server) HandleGetDatabaseSchemaInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}
	includeExamples := false
	if ie, ok := input["include_examples"].(bool); ok {
		includeExamples = ie
	}
	filterType := ""
	if ft, ok := input["filter_type"].(string); ok {
		filterType = ft
	}

	logger.Info("getting database schema",
		"project_id", projectID,
		"include_examples", includeExamples,
		"filter_type", filterType)

	// Get node types and their properties
	nodeTypesQuery := `
		MATCH (n {project_id: $project_id})
		WITH labels(n) as labels, keys(n) as props
		UNWIND labels as label
		RETURN label, collect(DISTINCT props) as property_sets
		ORDER BY label
	`
	nodeResults, err := s.serviceAdapter.ExecuteQuery(ctx, nodeTypesQuery, map[string]any{"project_id": projectID})
	if err != nil {
		return nil, fmt.Errorf("failed to get node types: %w", err)
	}

	// Get relationship types
	relationshipTypesQuery := `
		MATCH (a {project_id: $project_id})-[r]->(b {project_id: $project_id})
		RETURN type(r) as relationship_type, count(r) as count, 
		       collect(DISTINCT labels(a)) as source_labels,
		       collect(DISTINCT labels(b)) as target_labels
		ORDER BY relationship_type
	`
	relResults, err := s.serviceAdapter.ExecuteQuery(
		ctx,
		relationshipTypesQuery,
		map[string]any{"project_id": projectID},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get relationship types: %w", err)
	}

	// Build schema information
	schema := map[string]any{
		"node_types":         s.processNodeTypes(nodeResults, filterType),
		"relationship_types": s.processRelationshipTypes(relResults, filterType),
		"project_id":         projectID,
	}

	if includeExamples {
		schema["examples"] = s.generateQueryExamples(projectID)
	}

	schema["common_patterns"] = s.getCommonQueryPatterns()

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Database schema for project %s", projectID),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/schema", projectID),
					"data": schema,
				},
			},
		},
	}, nil
}

// processNodeTypes processes node type results for schema
func (s *Server) processNodeTypes(results []map[string]any, filterType string) []map[string]any {
	var nodeTypes []map[string]any
	propertyMap := make(map[string]map[string]bool)

	for _, result := range results {
		label, ok := result["label"].(string)
		if !ok {
			continue
		}
		if filterType != "" && !strings.Contains(strings.ToLower(label), strings.ToLower(filterType)) {
			continue
		}

		if propertyMap[label] == nil {
			propertyMap[label] = make(map[string]bool)
		}

		// Extract properties from property sets
		if propSets, ok := result["property_sets"].([]any); ok {
			for _, propSet := range propSets {
				if props, ok := propSet.([]any); ok {
					for _, prop := range props {
						if propStr, ok := prop.(string); ok {
							propertyMap[label][propStr] = true
						}
					}
				}
			}
		}
	}

	// Convert to final format
	for label, props := range propertyMap {
		var properties []string
		for prop := range props {
			properties = append(properties, prop)
		}
		nodeTypes = append(nodeTypes, map[string]any{
			"label":      label,
			"properties": properties,
		})
	}

	return nodeTypes
}

// processRelationshipTypes processes relationship type results for schema
func (s *Server) processRelationshipTypes(results []map[string]any, filterType string) []map[string]any {
	var relationshipTypes []map[string]any

	for _, result := range results {
		relType, ok := result["relationship_type"].(string)
		if !ok {
			continue
		}
		if filterType != "" && !strings.Contains(strings.ToLower(relType), strings.ToLower(filterType)) {
			continue
		}

		relationshipTypes = append(relationshipTypes, map[string]any{
			"type":          relType,
			"count":         result["count"],
			"source_labels": result["source_labels"],
			"target_labels": result["target_labels"],
		})
	}

	return relationshipTypes
}

// generateQueryExamples generates example queries for common use cases
func (s *Server) generateQueryExamples(projectID string) map[string]any {
	return map[string]any{
		"find_functions": map[string]any{
			"description": "Find all functions in a package",
			"query": "MATCH (f:Function {project_id: $project_id}) " +
				"WHERE f.package CONTAINS 'parser' " +
				"RETURN f.name, f.package, f.signature LIMIT 10",
			"parameters": map[string]string{"project_id": projectID},
		},
		"function_calls": map[string]any{
			"description": "Find functions that call a specific function",
			"query": "MATCH (caller:Function)-[:CALLS]->(callee:Function {project_id: $project_id}) " +
				"WHERE callee.name = 'Execute' " +
				"RETURN caller.name, caller.package",
			"parameters": map[string]string{"project_id": projectID},
		},
		"package_dependencies": map[string]any{
			"description": "Find package dependencies",
			"query": "MATCH (p:Package {project_id: $project_id})-[:IMPORTS]->(dep:Package) " +
				"RETURN p.name, collect(dep.name) as dependencies",
			"parameters": map[string]string{"project_id": projectID},
		},
		"interface_implementations": map[string]any{
			"description": "Find implementations of an interface",
			"query": "MATCH (impl:Struct)-[:IMPLEMENTS]->(iface:Interface {project_id: $project_id}) " +
				"WHERE iface.name = 'Parser' " +
				"RETURN impl.name, impl.package",
			"parameters": map[string]string{"project_id": projectID},
		},
	}
}

// getCommonQueryPatterns provides common Cypher patterns for LLMs
func (s *Server) getCommonQueryPatterns() map[string]any {
	return map[string]any{
		"node_matching": map[string]any{
			"basic":      "MATCH (n:NodeType {project_id: $project_id}) RETURN n",
			"with_props": "MATCH (n:NodeType {project_id: $project_id, name: 'specific_name'}) RETURN n",
			"filtering":  "MATCH (n:NodeType {project_id: $project_id}) WHERE n.property CONTAINS 'substring' RETURN n",
		},
		"relationship_patterns": map[string]any{
			"basic":       "MATCH (a)-[:RELATIONSHIP_TYPE]->(b) RETURN a, b",
			"with_filter": "MATCH (a:NodeA {project_id: $project_id})-[:REL]->(b:NodeB) WHERE a.name = 'value' RETURN a, b",
			"counting":    "MATCH (a)-[:REL]->(b) RETURN a.name, count(b) as rel_count ORDER BY rel_count DESC",
		},
		"common_mistakes": []string{
			"Always include project_id filter: {project_id: $project_id}",
			"Use CONTAINS for substring matching, not LIKE",
			"Remember to use DISTINCT when counting relationships",
			"Use LIMIT to prevent large result sets",
			"Property names are case-sensitive",
		},
	}
}

// HandleValidateCypherQueryInternal validates Cypher query syntax and suggests corrections
func (s *Server) HandleValidateCypherQueryInternal(ctx context.Context, input map[string]any) (*ToolResponse, error) {
	query, ok := input["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query is required")
	}
	projectID, err := s.getProjectID(input)
	if err != nil {
		return nil, err
	}

	logger.Info("validating cypher query", "project_id", projectID, "query_length", len(query))

	// Try to execute the query with EXPLAIN to check syntax
	explainQuery := "EXPLAIN " + query
	_, explainErr := s.serviceAdapter.ExecuteQuery(ctx, explainQuery, map[string]any{"project_id": projectID})

	validation := map[string]any{
		"query":      query,
		"project_id": projectID,
		"is_valid":   explainErr == nil,
	}

	if explainErr != nil {
		validation["error"] = explainErr.Error()
		validation["suggestions"] = s.analyzeCypherError(query, explainErr.Error())
	} else {
		validation["message"] = "Query syntax is valid"
	}

	return &ToolResponse{
		Content: []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Cypher query validation for project %s", projectID),
			},
			map[string]any{
				"type": "resource",
				"resource": map[string]any{
					"uri":  fmt.Sprintf("/projects/%s/query-validation", projectID),
					"data": validation,
				},
			},
		},
	}, nil
}

// analyzeCypherError analyzes common Cypher errors and provides suggestions
func (s *Server) analyzeCypherError(query, errorMsg string) []string {
	var suggestions []string
	errorLower := strings.ToLower(errorMsg)
	queryLower := strings.ToLower(query)

	if strings.Contains(errorLower, "invalid input") {
		suggestions = append(suggestions, "Check for syntax errors: missing parentheses, brackets, or commas")
	}

	if strings.Contains(errorLower, "undefined property") {
		suggestions = append(suggestions, "Verify property names are correct and case-sensitive")
	}

	if strings.Contains(errorLower, "undefined variable") {
		suggestions = append(suggestions, "Ensure all variables are properly defined in MATCH clauses")
	}

	if !strings.Contains(queryLower, "project_id") {
		suggestions = append(suggestions, "Add project_id filter: {project_id: $project_id}")
	}

	if strings.Contains(queryLower, "like") {
		suggestions = append(suggestions, "Use CONTAINS instead of LIKE for substring matching")
	}

	if strings.Contains(errorLower, "expected") {
		suggestions = append(suggestions, "Check clause order: MATCH, WHERE, RETURN, ORDER BY, LIMIT")
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Check the query syntax and ensure all node/relationship types exist")
	}

	return suggestions
}

// formatSchemaForLLM converts schema map to string format for LLM service
func (s *Server) formatSchemaForLLM(schema map[string]any) string {
	var schemaStr strings.Builder

	schemaStr.WriteString("Neo4j Database Schema:\n\n")

	// Format node types
	if nodeTypes, ok := schema["node_types"].([]map[string]any); ok {
		schemaStr.WriteString("Node Types:\n")
		for _, nodeType := range nodeTypes {
			if label, ok := nodeType["label"].(string); ok {
				schemaStr.WriteString(fmt.Sprintf("- %s", label))
				if properties, ok := nodeType["properties"].([]string); ok && len(properties) > 0 {
					schemaStr.WriteString(fmt.Sprintf(" (properties: %s)", strings.Join(properties, ", ")))
				}
				schemaStr.WriteString("\n")
			}
		}
		schemaStr.WriteString("\n")
	}

	// Format relationship types
	if relTypes, ok := schema["relationship_types"].([]map[string]any); ok {
		schemaStr.WriteString("Relationship Types:\n")
		for _, relType := range relTypes {
			if typeStr, ok := relType["type"].(string); ok {
				schemaStr.WriteString(fmt.Sprintf("- %s", typeStr))
				if count, ok := relType["count"]; ok {
					schemaStr.WriteString(fmt.Sprintf(" (%v occurrences)", count))
				}
				schemaStr.WriteString("\n")
			}
		}
		schemaStr.WriteString("\n")
	}

	// Add common patterns
	if patterns, ok := schema["common_patterns"].(map[string]any); ok {
		schemaStr.WriteString("Common Query Patterns:\n")
		if mistakes, ok := patterns["common_mistakes"].([]string); ok {
			for _, mistake := range mistakes {
				schemaStr.WriteString(fmt.Sprintf("- %s\n", mistake))
			}
		}
	}

	return schemaStr.String()
}
