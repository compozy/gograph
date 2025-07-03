package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/gograph/engine/llm"
	"github.com/compozy/gograph/engine/query"
	"github.com/compozy/gograph/pkg/logger"
	mcpconfig "github.com/compozy/gograph/pkg/mcp"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server represents the MCP server
type Server struct {
	config           *mcpconfig.Config
	serviceAdapter   ServiceAdapter
	llmService       llm.CypherTranslator
	contextGenerator llm.ContextGenerator
	queryBuilder     *query.HighLevelBuilder
	mu               sync.RWMutex //nolint:unused // Will be used when caching is implemented
	cache            map[string]cacheEntry
	rateLimiter      *rateLimiter
	mcpServer        *server.MCPServer
}

// cacheEntry represents a cached result
//
//nolint:unused // Will be used when caching is implemented
type cacheEntry struct {
	result    any
	timestamp time.Time
}

// rateLimiter implements a simple rate limiting mechanism
//
//nolint:unused // Will be used when rate limiting is implemented
type rateLimiter struct {
	mu        sync.Mutex
	requests  map[string][]time.Time
	maxPerMin int
}

// NewServer creates a new MCP server instance
func NewServer(
	config *mcpconfig.Config,
	serviceAdapter ServiceAdapter,
	llmService llm.CypherTranslator,
	contextGenerator llm.ContextGenerator,
	queryBuilder *query.HighLevelBuilder,
) *Server {
	s := &Server{
		config:           config,
		serviceAdapter:   serviceAdapter,
		llmService:       llmService,
		contextGenerator: contextGenerator,
		queryBuilder:     queryBuilder,
		cache:            make(map[string]cacheEntry),
		rateLimiter: &rateLimiter{
			requests:  make(map[string][]time.Time),
			maxPerMin: 100, // Default rate limit
		},
	}

	// Create MCP server instance
	s.mcpServer = server.NewMCPServer(
		"gograph",
		"1.0.0",
		server.WithToolCapabilities(false), // Static tool set
		server.WithResourceCapabilities(true, true),
	)

	// Register all tools
	s.registerTools()

	// Register resources
	s.registerResources()

	return s
}

// Start starts the MCP server
func (s *Server) Start(_ context.Context) error {
	logger.Info("Starting MCP server on stdio")

	// Use stdio transport for CLI integration
	return server.ServeStdio(s.mcpServer)
}

// registerTools registers all MCP tools
func (s *Server) registerTools() {
	s.registerAnalysisTools()
	s.registerNavigationTools()
	s.registerQueryTools()
	s.registerVerificationTools()
	s.registerPatternTools()
	s.registerTestTools()
	s.registerProjectManagementTools()
}

// registerAnalysisTools registers code analysis tools
func (s *Server) registerAnalysisTools() {
	// analyze_project tool
	analyzeProjectTool := mcp.NewTool(
		"analyze_project",
		mcp.WithDescription("Analyze a Go project and build its dependency graph"),
		mcp.WithString("project_path", mcp.Required(), mcp.Description("Path to the Go project to analyze")),
		mcp.WithString(
			"project_id",
			mcp.Description(
				"Unique identifier for the project (optional - will be derived from project_path config if not provided)",
			),
		),
		mcp.WithString("exclude_patterns", mcp.Description("Comma-separated list of path patterns to exclude")),
	)
	s.mcpServer.AddTool(analyzeProjectTool, s.handleAnalyzeProject)

	// query_dependencies tool
	queryDependenciesTool := mcp.NewTool(
		"query_dependencies",
		mcp.WithDescription("Query dependencies of a package or file"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("path", mcp.Required(), mcp.Description("Package or file path")),
		mcp.WithString("direction", mcp.Description("'imports' or 'imported_by' (default: imports)")),
		mcp.WithBoolean("recursive", mcp.Description("Include transitive dependencies")),
	)
	s.mcpServer.AddTool(queryDependenciesTool, s.handleQueryDependencies)
}

// registerNavigationTools registers code navigation tools
func (s *Server) registerNavigationTools() {
	// find_implementations tool
	findImplementationsTool := mcp.NewTool(
		"find_implementations",
		mcp.WithDescription("Find all implementations of an interface"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("interface_name", mcp.Required(), mcp.Description("Fully qualified interface name")),
		mcp.WithString("package", mcp.Description("Package containing the interface")),
	)
	s.mcpServer.AddTool(findImplementationsTool, s.handleFindImplementations)

	// trace_call_chain tool
	traceCallChainTool := mcp.NewTool(
		"trace_call_chain",
		mcp.WithDescription("Trace call chains between functions"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("from_function", mcp.Required(), mcp.Description("Starting function")),
		mcp.WithString("to_function", mcp.Description("Target function (optional)")),
		mcp.WithNumber("max_depth", mcp.Description("Maximum search depth")),
	)
	s.mcpServer.AddTool(traceCallChainTool, s.handleTraceCallChain)

	// detect_circular_deps tool
	detectCircularDepsTool := mcp.NewTool(
		"detect_circular_deps",
		mcp.WithDescription("Detect circular dependencies in the project"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("scope", mcp.Description("'package' or 'file' level (default: package)")),
	)
	s.mcpServer.AddTool(detectCircularDepsTool, s.handleDetectCircularDeps)

	// get_function_info tool
	getFunctionInfoTool := mcp.NewTool(
		"get_function_info",
		mcp.WithDescription("Get detailed information about a function"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("function_name", mcp.Required(), mcp.Description("Function name")),
		mcp.WithString("package", mcp.Description("Package containing the function")),
		mcp.WithBoolean("include_calls", mcp.Description("Include functions this function calls")),
		mcp.WithBoolean("include_callers", mcp.Description("Include functions that call this function")),
	)
	s.mcpServer.AddTool(getFunctionInfoTool, s.handleGetFunctionInfo)

	// list_packages tool
	listPackagesTool := mcp.NewTool(
		"list_packages",
		mcp.WithDescription("List all packages in the project"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("pattern", mcp.Description("Filter packages by pattern")),
		mcp.WithBoolean("include_external", mcp.Description("Include external dependencies")),
	)
	s.mcpServer.AddTool(listPackagesTool, s.handleListPackages)

	// get_package_structure tool
	getPackageStructureTool := mcp.NewTool(
		"get_package_structure",
		mcp.WithDescription("Get detailed structure of a package"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("package", mcp.Required(), mcp.Description("Package path")),
		mcp.WithBoolean("include_private", mcp.Description("Include unexported types and functions")),
	)
	s.mcpServer.AddTool(getPackageStructureTool, s.handleGetPackageStructure)
}

// registerQueryTools registers query and execution tools
func (s *Server) registerQueryTools() {
	// execute_cypher tool
	executeCypherTool := mcp.NewTool(
		"execute_cypher",
		mcp.WithDescription("Execute a Cypher query against the graph database"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("query", mcp.Required(), mcp.Description("Cypher query to execute")),
		mcp.WithObject("parameters", mcp.Description("Query parameters")),
	)
	s.mcpServer.AddTool(executeCypherTool, s.handleExecuteCypher)

	// natural_language_query tool
	naturalLanguageQueryTool := mcp.NewTool(
		"natural_language_query",
		mcp.WithDescription("Convert natural language to Cypher and execute"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("query", mcp.Required(), mcp.Description("Natural language query")),
		mcp.WithString("context", mcp.Description("Additional context for the query")),
	)
	s.mcpServer.AddTool(naturalLanguageQueryTool, s.handleNaturalLanguageQuery)

	// get_database_schema tool
	getDatabaseSchemaTool := mcp.NewTool(
		"get_database_schema",
		mcp.WithDescription(
			"Get Neo4j database schema with node types, relationships, and properties for LLM query assistance",
		),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithBoolean("include_examples", mcp.Description("Include example queries for each node/relationship type")),
		mcp.WithString("filter_type", mcp.Description("Filter by specific node/relationship type")),
	)
	s.mcpServer.AddTool(getDatabaseSchemaTool, s.handleGetDatabaseSchema)

	// validate_cypher_query tool
	validateCypherQueryTool := mcp.NewTool(
		"validate_cypher_query",
		mcp.WithDescription("Validate Cypher query syntax and suggest corrections for common mistakes"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Cypher query to validate")),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
	)
	s.mcpServer.AddTool(validateCypherQueryTool, s.handleValidateCypherQuery)
}

// registerVerificationTools registers code verification tools
func (s *Server) registerVerificationTools() {
	// verify_code_exists tool
	verifyCodeExistsTool := mcp.NewTool(
		"verify_code_exists",
		mcp.WithDescription("Verify if a code element exists in the project"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("element_type", mcp.Required(), mcp.Description("'function', 'type', 'interface', 'package'")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name of the element")),
		mcp.WithString("package", mcp.Description("Package containing the element")),
	)
	s.mcpServer.AddTool(verifyCodeExistsTool, s.handleVerifyCodeExists)

	// get_code_context tool
	getCodeContextTool := mcp.NewTool(
		"get_code_context",
		mcp.WithDescription("Get context around a code element for LLM understanding"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("element_type", mcp.Required(), mcp.Description("Type of element")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Element name")),
		mcp.WithNumber("context_lines", mcp.Description("Number of context lines")),
	)
	s.mcpServer.AddTool(getCodeContextTool, s.handleGetCodeContext)

	// validate_import_path tool
	validateImportPathTool := mcp.NewTool(
		"validate_import_path",
		mcp.WithDescription("Validate and resolve an import path"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("import_path", mcp.Required(), mcp.Description("Import path to validate")),
		mcp.WithString("from_package", mcp.Description("Package context for relative imports")),
	)
	s.mcpServer.AddTool(validateImportPathTool, s.handleValidateImportPath)
}

// registerPatternTools registers pattern detection tools
func (s *Server) registerPatternTools() {
	// detect_code_patterns tool
	detectCodePatternsTool := mcp.NewTool(
		"detect_code_patterns",
		mcp.WithDescription("Detect common code patterns and anti-patterns"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithArray("patterns", mcp.Description("Specific patterns to look for")),
		mcp.WithString("scope", mcp.Description("'project', 'package', or specific path")),
	)
	s.mcpServer.AddTool(detectCodePatternsTool, s.handleDetectCodePatterns)

	// get_naming_conventions tool
	getNamingConventionsTool := mcp.NewTool(
		"get_naming_conventions",
		mcp.WithDescription("Analyze naming conventions used in the project"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("scope", mcp.Description("Scope of analysis")),
		mcp.WithBoolean("include_suggestions", mcp.Description("Include improvement suggestions")),
	)
	s.mcpServer.AddTool(getNamingConventionsTool, s.handleGetNamingConventions)
}

// registerTestTools registers test integration tools
func (s *Server) registerTestTools() {
	// find_tests_for_code tool
	findTestsForCodeTool := mcp.NewTool(
		"find_tests_for_code",
		mcp.WithDescription("Find tests for a specific code element"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("element_type", mcp.Required(), mcp.Description("Type of element")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Element name")),
		mcp.WithString("package", mcp.Description("Package containing the element")),
	)
	s.mcpServer.AddTool(findTestsForCodeTool, s.handleFindTestsForCode)

	// check_test_coverage tool
	checkTestCoverageTool := mcp.NewTool(
		"check_test_coverage",
		mcp.WithDescription("Check test coverage for packages or files"),
		mcp.WithString(
			"project_id",
			mcp.Description("Project identifier (optional - will be derived from config if not provided)"),
		),
		mcp.WithString("path", mcp.Description("Package or file path")),
		mcp.WithBoolean("detailed", mcp.Description("Include detailed coverage info")),
	)
	s.mcpServer.AddTool(checkTestCoverageTool, s.handleCheckTestCoverage)
}

// registerProjectManagementTools registers project management tools
func (s *Server) registerProjectManagementTools() {
	// list_projects tool
	listProjectsTool := mcp.NewTool("list_projects",
		mcp.WithDescription("List all projects in the database"),
	)
	s.mcpServer.AddTool(listProjectsTool, s.handleListProjects)

	// validate_project tool
	validateProjectTool := mcp.NewTool("validate_project",
		mcp.WithDescription("Validate if a project exists in the database"),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("Project identifier to validate")),
	)
	s.mcpServer.AddTool(validateProjectTool, s.handleValidateProject)
}

// registerResources registers all MCP resources
func (s *Server) registerResources() {
	// Project metadata resource
	s.mcpServer.AddResourceTemplate(mcp.NewResourceTemplate(
		"project://metadata/{project_id}",
		"project_metadata",
		mcp.WithTemplateDescription("Project metadata and statistics"),
		mcp.WithTemplateMIMEType("application/json"),
	), wrapResourceHandler(s.HandleProjectMetadataResource))

	// Query templates resource
	s.mcpServer.AddResourceTemplate(mcp.NewResourceTemplate(
		"templates://queries",
		"query_templates",
		mcp.WithTemplateDescription("Available Cypher query templates"),
		mcp.WithTemplateMIMEType("application/json"),
	), wrapResourceHandler(s.HandleQueryTemplatesResource))

	// Code patterns resource
	s.mcpServer.AddResourceTemplate(mcp.NewResourceTemplate(
		"patterns://catalog",
		"code_patterns",
		mcp.WithTemplateDescription("Catalog of detectable code patterns"),
		mcp.WithTemplateMIMEType("application/json"),
	), wrapResourceHandler(s.HandleCodePatternsResource))

	// Project invariants resource
	s.mcpServer.AddResourceTemplate(mcp.NewResourceTemplate(
		"invariants://project/{project_id}",
		"project_invariants",
		mcp.WithTemplateDescription("Project architectural invariants and rules"),
		mcp.WithTemplateMIMEType("application/json"),
	), wrapResourceHandler(s.HandleProjectInvariantsResource))
}

// Tool handler implementations

func (s *Server) handleAnalyzeProject(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectPath, err := req.RequireString("project_path")
	if err != nil {
		return nil, fmt.Errorf("project_path is required: %w", err)
	}
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")

	excludePatterns := getString(req, "exclude_patterns")

	// Call the implementation
	response, err := s.HandleAnalyzeProjectInternal(ctx, map[string]any{
		"project_path":     projectPath,
		"project_id":       projectID,
		"exclude_patterns": excludePatterns,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleQueryDependencies(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	path, err := req.RequireString("path")
	if err != nil {
		return nil, err
	}
	direction := getString(req, "direction")
	if direction == "" {
		direction = "imports"
	}
	recursive := getBool(req, "recursive")

	response, err := s.HandleQueryDependenciesInternal(ctx, map[string]any{
		"project_id": projectID,
		"path":       path,
		"direction":  direction,
		"recursive":  recursive,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleFindImplementations(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	interfaceName, err := req.RequireString("interface_name")
	if err != nil {
		return nil, err
	}
	packageName := getString(req, "package")

	response, err := s.HandleFindImplementationsInternal(ctx, map[string]any{
		"project_id":     projectID,
		"interface_name": interfaceName,
		"package":        packageName,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleTraceCallChain(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	fromFunction, err := req.RequireString("from_function")
	if err != nil {
		return nil, err
	}
	toFunction := getString(req, "to_function")
	maxDepth := getFloat(req, "max_depth")

	response, err := s.HandleTraceCallChainInternal(ctx, map[string]any{
		"project_id":    projectID,
		"from_function": fromFunction,
		"to_function":   toFunction,
		"max_depth":     int(maxDepth),
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleDetectCircularDeps(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	scope := getString(req, "scope")
	if scope == "" {
		scope = "package"
	}

	response, err := s.HandleDetectCircularDepsInternal(ctx, map[string]any{
		"project_id": projectID,
		"scope":      scope,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleGetFunctionInfo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	functionName, err := req.RequireString("function_name")
	if err != nil {
		return nil, err
	}
	packageName := getString(req, "package")
	includeCalls := getBool(req, "include_calls")
	includeCallers := getBool(req, "include_callers")

	response, err := s.HandleGetFunctionInfoInternal(ctx, map[string]any{
		"project_id":      projectID,
		"function_name":   functionName,
		"package":         packageName,
		"include_calls":   includeCalls,
		"include_callers": includeCallers,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleListPackages(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	pattern := getString(req, "pattern")
	includeExternal := getBool(req, "include_external")

	response, err := s.HandleListPackagesInternal(ctx, map[string]any{
		"project_id":       projectID,
		"pattern":          pattern,
		"include_external": includeExternal,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleGetPackageStructure(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	packageName, err := req.RequireString("package")
	if err != nil {
		return nil, err
	}
	includePrivate := getBool(req, "include_private")

	response, err := s.HandleGetPackageStructureInternal(ctx, map[string]any{
		"project_id":      projectID,
		"package":         packageName,
		"include_private": includePrivate,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleExecuteCypher(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	query, err := req.RequireString("query")
	if err != nil {
		return nil, err
	}
	parameters := req.GetArguments()["parameters"]

	response, err := s.HandleExecuteCypherInternal(ctx, map[string]any{
		"project_id": projectID,
		"query":      query,
		"parameters": parameters,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleNaturalLanguageQuery(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	query, err := req.RequireString("query")
	if err != nil {
		return nil, err
	}
	context := getString(req, "context")

	response, err := s.HandleNaturalLanguageQueryInternal(ctx, map[string]any{
		"project_id": projectID,
		"query":      query,
		"context":    context,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleVerifyCodeExists(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	elementType, err := req.RequireString("element_type")
	if err != nil {
		return nil, err
	}
	name, err := req.RequireString("name")
	if err != nil {
		return nil, err
	}
	packageName := getString(req, "package")

	response, err := s.HandleVerifyCodeExistsInternal(ctx, map[string]any{
		"project_id":   projectID,
		"element_type": elementType,
		"name":         name,
		"package":      packageName,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleGetCodeContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	elementType, err := req.RequireString("element_type")
	if err != nil {
		return nil, err
	}
	name, err := req.RequireString("name")
	if err != nil {
		return nil, err
	}
	contextLines := getFloat(req, "context_lines")

	response, err := s.HandleGetCodeContextInternal(ctx, map[string]any{
		"project_id":    projectID,
		"element_type":  elementType,
		"name":          name,
		"context_lines": int(contextLines),
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleValidateImportPath(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	importPath, err := req.RequireString("import_path")
	if err != nil {
		return nil, err
	}
	fromPackage := getString(req, "from_package")

	response, err := s.HandleValidateImportPathInternal(ctx, map[string]any{
		"project_id":   projectID,
		"import_path":  importPath,
		"from_package": fromPackage,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleDetectCodePatterns(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	patterns := req.GetArguments()["patterns"]
	scope := getString(req, "scope")
	if scope == "" {
		scope = "project"
	}

	response, err := s.HandleDetectCodePatternsInternal(ctx, map[string]any{
		"project_id": projectID,
		"patterns":   patterns,
		"scope":      scope,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleGetNamingConventions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	scope := getString(req, "scope")
	includeSuggestions := getBool(req, "include_suggestions")

	response, err := s.HandleGetNamingConventionsInternal(ctx, map[string]any{
		"project_id":          projectID,
		"scope":               scope,
		"include_suggestions": includeSuggestions,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleFindTestsForCode(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	elementType, err := req.RequireString("element_type")
	if err != nil {
		return nil, err
	}
	name, err := req.RequireString("name")
	if err != nil {
		return nil, err
	}
	packageName := getString(req, "package")

	response, err := s.HandleFindTestsForCodeInternal(ctx, map[string]any{
		"project_id":   projectID,
		"element_type": elementType,
		"name":         name,
		"package":      packageName,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleCheckTestCoverage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")
	path := getString(req, "path")
	detailed := getBool(req, "detailed")

	response, err := s.HandleCheckTestCoverageInternal(ctx, map[string]any{
		"project_id": projectID,
		"path":       path,
		"detailed":   detailed,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleListProjects(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	response, err := s.HandleListProjectsInternal(ctx, map[string]any{})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleValidateProject(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// project_id is now optional - will be derived from config if not provided
	projectID := getString(req, "project_id")

	response, err := s.HandleValidateProjectInternal(ctx, map[string]any{
		"project_id": projectID,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

// Resource handler implementations

// checkRateLimit checks if the client has exceeded the rate limit
//
//nolint:unused // Will be used when rate limiting is implemented
func (s *Server) checkRateLimit(clientID string) error {
	s.rateLimiter.mu.Lock()
	defer s.rateLimiter.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Minute)

	// Clean old requests
	var validRequests []time.Time
	for _, t := range s.rateLimiter.requests[clientID] {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}

	if len(validRequests) >= s.rateLimiter.maxPerMin {
		return fmt.Errorf("rate limit exceeded: max %d requests per minute", s.rateLimiter.maxPerMin)
	}

	validRequests = append(validRequests, now)
	s.rateLimiter.requests[clientID] = validRequests
	return nil
}

// getCacheKey generates a cache key for the given operation and parameters
//
//nolint:unused // Will be used when caching is implemented
func (s *Server) getCacheKey(operation string, params map[string]any) string {
	// Simple cache key generation - could be improved with better hashing
	return fmt.Sprintf("%s:%v", operation, params)
}

// getFromCache retrieves a cached result if available and not expired
//
//nolint:unused // Will be used when caching is implemented
func (s *Server) getFromCache(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.cache[key]
	if !exists {
		return nil, false
	}

	// Check if cache entry is expired (default: 5 minutes)
	if time.Since(entry.timestamp) > 5*time.Minute {
		return nil, false
	}

	return entry.result, true
}

// setCache stores a result in the cache
//
//nolint:unused // Will be used when caching is implemented
func (s *Server) setCache(key string, result any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[key] = cacheEntry{
		result:    result,
		timestamp: time.Now(),
	}

	// Simple cache eviction - remove old entries if cache is too large
	if len(s.cache) > 1000 {
		// Remove oldest entries
		var oldestKey string
		var oldestTime time.Time
		for k, v := range s.cache {
			if oldestKey == "" || v.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.timestamp
			}
		}
		delete(s.cache, oldestKey)
	}
}

func (s *Server) handleGetDatabaseSchema(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := getString(req, "project_id")
	includeExamples := getBool(req, "include_examples")
	filterType := getString(req, "filter_type")

	response, err := s.HandleGetDatabaseSchemaInternal(ctx, map[string]any{
		"project_id":       projectID,
		"include_examples": includeExamples,
		"filter_type":      filterType,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}

func (s *Server) handleValidateCypherQuery(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return nil, err
	}
	projectID := getString(req, "project_id")

	response, err := s.HandleValidateCypherQueryInternal(ctx, map[string]any{
		"query":      query,
		"project_id": projectID,
	})
	if err != nil {
		return nil, err
	}

	return newToolResultFromResponse(response)
}
