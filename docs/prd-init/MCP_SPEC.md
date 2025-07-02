# MCP Server Implementation Specification for gograph

## Overview

This document specifies the implementation of a Model Context Protocol (MCP) server for gograph, designed to expose Go codebase analysis capabilities to LLM applications. The server will help LLMs avoid hallucinations by providing concrete, verified information about code structure, dependencies, and relationships.

## Architecture

### Core Components

1. **MCP Server** (`engine/mcp/server.go`)

   - Implements the MCP protocol using [mcp-go](https://mcp-go.dev/)
   - Manages tool registration and execution
   - Handles resource exposure
   - Provides streaming support for long operations

2. **Tool Definitions** (`engine/mcp/tools.go`)

   - Code analysis tools
   - Code navigation tools
   - Query execution tools
   - Validation and verification tools

3. **Resource Providers** (`engine/mcp/resources.go`)

   - Project metadata resources
   - Analysis result resources
   - Configuration resources

4. **Server Configuration** (`pkg/mcp/config.go`)

   - Port and authentication settings
   - Security policies
   - Performance tuning options

5. **CLI Integration** (`cmd/gograph/commands/serve_mcp.go`)
   - `gograph serve-mcp` command
   - Server lifecycle management

## Tool Categories

### 1. Code Analysis Tools

#### 1.1 `analyze_project`

- **Purpose**: Analyze a Go project and store results in Neo4j
- **Parameters**:
  - `project_path`: Path to Go project
  - `project_id`: Unique identifier for the project
  - `ignore_patterns`: Optional patterns to exclude
- **Returns**: Analysis summary with node/relationship counts
- **Anti-hallucination**: Creates authoritative graph representation

#### 1.2 `query_dependencies`

- **Purpose**: Find all dependencies for a package or function
- **Parameters**:
  - `project_id`: Project identifier
  - `element_type`: "package" | "function" | "file"
  - `element_name`: Name of the element
  - `depth`: Maximum dependency depth (default: 3)
- **Returns**: Dependency tree with import paths and usage locations
- **Anti-hallucination**: Traces actual import statements

#### 1.3 `find_implementations`

- **Purpose**: Find all implementations of an interface
- **Parameters**:
  - `project_id`: Project identifier
  - `interface_name`: Name of the interface
  - `package_filter`: Optional package scope
- **Returns**: List of implementing structs with methods
- **Anti-hallucination**: Verifies method signatures match exactly

#### 1.4 `trace_call_chain`

- **Purpose**: Trace function call relationships
- **Parameters**:
  - `project_id`: Project identifier
  - `function_name`: Starting function
  - `direction`: "callers" | "callees" | "both"
  - `max_depth`: Maximum call depth
- **Returns**: Call graph with line numbers
- **Anti-hallucination**: Based on AST analysis, not guessing

#### 1.5 `detect_circular_deps`

- **Purpose**: Detect circular dependencies
- **Parameters**:
  - `project_id`: Project identifier
  - `scope`: "packages" | "files" | "all"
- **Returns**: List of circular dependency cycles
- **Anti-hallucination**: Algorithmic detection, not pattern matching

### 2. Code Navigation Tools

#### 2.1 `get_function_info`

- **Purpose**: Get detailed information about a function
- **Parameters**:
  - `project_id`: Project identifier
  - `function_name`: Function name
  - `package_name`: Optional package filter
- **Returns**: Function signature, body, calls, location
- **Anti-hallucination**: Returns exact AST-parsed information

#### 2.2 `list_packages`

- **Purpose**: List all packages in the project
- **Parameters**:
  - `project_id`: Project identifier
  - `include_external`: Include external dependencies
- **Returns**: Package list with file counts and main types
- **Anti-hallucination**: Enumeration of actual packages

#### 2.3 `get_package_structure`

- **Purpose**: Get detailed structure of a package
- **Parameters**:
  - `project_id`: Project identifier
  - `package_name`: Package to analyze
- **Returns**: Files, types, functions, and dependencies
- **Anti-hallucination**: Complete package inventory

### 3. Query Tools

#### 3.1 `execute_cypher`

- **Purpose**: Execute custom Cypher queries
- **Parameters**:
  - `project_id`: Project identifier
  - `query`: Cypher query string
  - `parameters`: Query parameters
  - `timeout`: Execution timeout
- **Returns**: Query results in structured format
- **Anti-hallucination**: Direct database queries

#### 3.2 `natural_language_query`

- **Purpose**: Convert natural language to Cypher and execute
- **Parameters**:
  - `project_id`: Project identifier
  - `question`: Natural language question
  - `context`: Additional context
- **Returns**: Query results with explanation
- **Anti-hallucination**: Shows generated Cypher for verification

### 4. Verification Tools

#### 4.1 `verify_code_exists`

- **Purpose**: Verify if a code element exists
- **Parameters**:
  - `project_id`: Project identifier
  - `element_type`: Type of element
  - `element_name`: Name to verify
  - `package_scope`: Optional package filter
- **Returns**: Exists boolean, exact location, signature
- **Anti-hallucination**: Primary defense against inventing code

#### 4.2 `get_code_context`

- **Purpose**: Get surrounding code context
- **Parameters**:
  - `project_id`: Project identifier
  - `file_path`: File path
  - `line_number`: Target line
  - `context_lines`: Lines before/after
- **Returns**: Code snippet with line numbers
- **Anti-hallucination**: Precise line-targeted retrieval

#### 4.3 `validate_import_path`

- **Purpose**: Verify import path validity
- **Parameters**:
  - `project_id`: Project identifier
  - `import_path`: Import to validate
- **Returns**: Valid boolean, usage locations
- **Anti-hallucination**: Prevents invalid imports

### 5. Pattern Detection Tools

#### 5.1 `detect_code_patterns`

- **Purpose**: Identify common Go patterns
- **Parameters**:
  - `project_id`: Project identifier
  - `pattern_types`: Array of patterns to detect
  - `package_filter`: Optional scope
- **Returns**: Detected patterns with examples
- **Anti-hallucination**: Helps follow established patterns

#### 5.2 `get_naming_conventions`

- **Purpose**: Analyze project naming conventions
- **Parameters**:
  - `project_id`: Project identifier
  - `scope`: "functions" | "types" | "variables"
- **Returns**: Convention patterns with statistics
- **Anti-hallucination**: Ensures consistent naming

### 6. Test Integration Tools

#### 6.1 `find_tests_for_code`

- **Purpose**: Find tests covering specific code
- **Parameters**:
  - `project_id`: Project identifier
  - `target_type`: "function" | "file" | "package"
  - `target_name`: Name of target
- **Returns**: Test files and functions
- **Anti-hallucination**: Links code to actual tests

#### 6.2 `check_test_coverage`

- **Purpose**: Check test coverage for code elements
- **Parameters**:
  - `project_id`: Project identifier
  - `element_path`: Path to check
- **Returns**: Coverage percentage and gaps
- **Anti-hallucination**: Based on actual test analysis

## Resources

### 1. Project Metadata Resource

- **URI**: `/projects/{project_id}/metadata`
- **Content**: Project statistics, main packages, dependency summary
- **Updates**: On each analysis run

### 2. Query Templates Resource

- **URI**: `/templates/queries`
- **Content**: Pre-defined Cypher query templates
- **Categories**: Overview, dependencies, functions, types

### 3. Code Patterns Resource

- **URI**: `/projects/{project_id}/patterns`
- **Content**: Detected patterns and conventions
- **Updates**: On pattern analysis

### 4. Project Invariants Resource

- **URI**: `/projects/{project_id}/invariants`
- **Content**: Project-specific rules and constraints
- **Purpose**: Prevent rule violations

## Configuration

```yaml
mcp:
  server:
    port: 8080
    host: "localhost"
    max_connections: 100

  auth:
    enabled: false # Can be enabled for production
    method: "token"

  performance:
    max_context_size: 1000000
    cache_ttl: 3600
    enable_streaming: true
    batch_size: 100

  security:
    allowed_paths: ["."]
    forbidden_paths: [".git", "vendor", "node_modules"]
    rate_limit: 100 # requests per minute
    max_query_time: 30 # seconds

  features:
    enable_incremental: true
    enable_validation: true
    enable_patterns: true
    enable_caching: true
```

## Implementation Guidelines

### 1. Error Handling

- All tools must return structured errors with codes
- Provide helpful error messages with recovery suggestions
- Implement graceful degradation for partial failures

### 2. Performance Optimization

- Use streaming for large result sets
- Implement caching for frequently accessed data
- Batch operations where possible
- Respect timeout configurations

### 3. Security Considerations

- Validate all input parameters
- Respect file access restrictions
- Implement rate limiting
- Log all operations for audit

### 4. Testing Requirements

- Unit tests for each tool
- Integration tests with Neo4j
- Performance benchmarks
- Security vulnerability tests

## CLI Usage

```bash
# Start MCP server with default config
gograph serve-mcp

# Start with custom config
gograph serve-mcp --config mcp.yaml

# Start with specific port
gograph serve-mcp --port 9090

# Start with authentication
gograph serve-mcp --auth-token YOUR_TOKEN

# Start in debug mode
gograph serve-mcp --debug
```

## Integration Examples

### With Claude Desktop

```json
{
  "mcp": {
    "servers": {
      "gograph": {
        "command": "gograph",
        "args": ["serve-mcp", "--port", "8080"],
        "cwd": "/path/to/project"
      }
    }
  }
}
```

### With Custom LLM Applications

```go
client := mcp.NewClient("http://localhost:8080")
result, err := client.CallTool("analyze_project", map[string]any{
    "project_path": "/path/to/go/project",
    "project_id": "my-project",
})
```

## Success Metrics

1. **Hallucination Reduction**: 50%+ reduction in incorrect code assumptions
2. **Query Accuracy**: 95%+ accurate query results
3. **Performance**: Sub-second response for most operations
4. **Reliability**: 99.9% uptime with graceful error handling

## Future Enhancements

1. **Multi-language Support**: Extend beyond Go
2. **Real-time Updates**: File watcher integration
3. **Collaborative Features**: Multi-user support
4. **AI-Powered Insights**: Code smell detection, refactoring suggestions
5. **Cloud Deployment**: Hosted MCP service option
