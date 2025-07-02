# MCP (Model Context Protocol) Integration Guide

## Overview

GoGraph provides an MCP server that exposes code analysis capabilities to LLM applications. This allows AI assistants to analyze Go codebases, query dependencies, detect patterns, and provide concrete code verification to prevent hallucinations.

## Installation and Setup

### Prerequisites

- GoGraph installed and configured
- Neo4j database running
- Go project to analyze

### Basic Setup

1. **Analyze your project first:**

   ```bash
   gograph analyze /path/to/your/go/project
   ```

2. **Start the MCP server:**

   ```bash
   # Default stdio transport
   gograph serve-mcp

   # HTTP transport (when available)
   gograph serve-mcp --http --port 8080
   ```

3. **Configure your LLM application** to connect to the MCP server.

## Configuration

### Configuration File

Create a `.gograph/mcp.yaml` file in your project:

```yaml
server:
  port: 8080
  host: localhost
  max_connections: 100

auth:
  enabled: false
  method: token
  token: "" # Set if auth is enabled

performance:
  max_context_size: 1000000
  cache_ttl: 3600
  enable_streaming: true
  batch_size: 100
  request_timeout: 30s

security:
  allowed_paths:
    - "."
    - "~/projects"
  forbidden_paths:
    - ".git"
    - "vendor"
    - "node_modules"
  rate_limit: 100
  max_query_time: 30

features:
  enable_incremental: true
  enable_validation: true
  enable_patterns: true
  enable_caching: true
```

### Environment Variables

You can also configure the MCP server using environment variables:

```bash
export GOGRAPH_MCP_PORT=8080
export GOGRAPH_MCP_AUTH_ENABLED=true
export GOGRAPH_MCP_AUTH_TOKEN=your-secret-token
```

## Available Tools

The MCP server exposes the following tools to LLM applications:

### Code Analysis Tools

#### analyze_project

Analyzes a Go project and stores the results in the graph database.

**Parameters:**

- `project_path` (string): Path to the Go project
- `project_id` (string): Unique identifier for the project

**Example:**

```json
{
  "tool": "analyze_project",
  "parameters": {
    "project_path": "/home/user/myproject",
    "project_id": "myproject"
  }
}
```

#### query_dependencies

Queries project dependencies and import relationships.

**Parameters:**

- `project_id` (string): Project identifier
- `package_name` (string, optional): Specific package to query
- `direction` (string): "imports" or "imported_by"
- `max_depth` (integer): Maximum dependency depth

### Navigation Tools

#### get_function_info

Retrieves detailed information about a specific function.

**Parameters:**

- `project_id` (string): Project identifier
- `function_name` (string): Function name
- `package_name` (string, optional): Package containing the function

#### list_packages

Lists all packages in a project.

**Parameters:**

- `project_id` (string): Project identifier

### Query Tools

#### execute_cypher

Executes a Cypher query against the graph database.

**Parameters:**

- `query` (string): Cypher query
- `parameters` (object): Query parameters

**Example:**

```json
{
  "tool": "execute_cypher",
  "parameters": {
    "query": "MATCH (f:Function)-[:CALLS]->(g:Function) WHERE f.name = $name RETURN g",
    "parameters": {
      "name": "main"
    }
  }
}
```

#### natural_language_query

Translates natural language to Cypher and executes the query.

**Parameters:**

- `question` (string): Natural language question
- `project_id` (string): Project identifier

### Pattern Detection Tools

#### detect_code_patterns

Detects common design patterns in the codebase.

**Parameters:**

- `project_id` (string): Project identifier
- `pattern_types` (array): ["singleton", "factory", "observer", "builder", "repository"]

### Verification Tools

#### verify_function_exists

Verifies if a function exists to prevent hallucinations.

**Parameters:**

- `project_id` (string): Project identifier
- `function_name` (string): Function to verify
- `package_name` (string, optional): Expected package

#### verify_import_relationship

Verifies if an import relationship exists between packages.

**Parameters:**

- `project_id` (string): Project identifier
- `from_package` (string): Importing package
- `to_package` (string): Imported package

## Available Resources

The MCP server provides the following resources:

### project_metadata

Returns metadata about analyzed projects.

```json
{
  "uri": "/projects/{project_id}/metadata",
  "data": {
    "project_id": "myproject",
    "analyzed_at": "2024-01-15T10:30:00Z",
    "total_files": 150,
    "total_packages": 25,
    "total_functions": 500
  }
}
```

### query_templates

Provides common Cypher query templates.

```json
{
  "uri": "/templates/queries",
  "data": {
    "find_circular_dependencies": "MATCH path=(p1:Package)-[:IMPORTS*]->(p1) RETURN path",
    "get_function_calls": "MATCH (f:Function)-[:CALLS]->(g:Function) WHERE f.name = $name RETURN g"
  }
}
```

## Integration Examples

### Claude Desktop Integration

Add to your Claude Desktop configuration:

```json
{
  "mcp_servers": {
    "gograph": {
      "command": "gograph",
      "args": ["serve-mcp"],
      "env": {
        "GOGRAPH_CONFIG": "/path/to/.gograph.yaml"
      }
    }
  }
}
```

### Custom LLM Integration

```python
import json
import subprocess

# Start MCP server
proc = subprocess.Popen(
    ["gograph", "serve-mcp"],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
    text=True
)

# Send tool request
request = {
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
        "name": "analyze_project",
        "arguments": {
            "project_path": "/path/to/project",
            "project_id": "myproject"
        }
    },
    "id": 1
}

proc.stdin.write(json.dumps(request) + "\n")
proc.stdin.flush()

# Read response
response = json.loads(proc.stdout.readline())
```

## Best Practices

1. **Pre-analyze Projects**: Always analyze your project with `gograph analyze` before starting the MCP server.

2. **Use Caching**: Enable caching for frequently accessed data to improve performance.

3. **Set Rate Limits**: Configure appropriate rate limits to prevent abuse.

4. **Security**:

   - Use authentication in production environments
   - Configure allowed/forbidden paths carefully
   - Set appropriate query timeouts

5. **Incremental Updates**: Use incremental analysis for large codebases:
   ```bash
   gograph analyze --incremental /path/to/project
   ```

## Troubleshooting

### Server Won't Start

1. Check if Neo4j is running:

   ```bash
   gograph query "RETURN 1"
   ```

2. Verify configuration:
   ```bash
   gograph serve-mcp --config /path/to/config.yaml --verbose
   ```

### Connection Issues

1. For stdio transport, ensure your LLM client supports stdio communication
2. For HTTP transport (when available), check firewall settings

### Performance Issues

1. Enable caching in configuration
2. Increase batch_size for bulk operations
3. Use query templates instead of natural language queries for better performance

## Advanced Usage

### Custom Tool Implementation

When the full mcp-go library integration is available, you can add custom tools:

```go
// In engine/mcp/custom_tools.go
func (s *Server) registerCustomTools() {
    s.AddTool(Tool{
        Name: "custom_analysis",
        Description: "Performs custom code analysis",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "metric": map[string]any{
                    "type": "string",
                    "enum": []string{"complexity", "coverage"},
                },
            },
        },
        Handler: s.handleCustomAnalysis,
    })
}
```

### Extending Resources

Add custom resources to provide additional context:

```go
// In engine/mcp/custom_resources.go
func (s *Server) registerCustomResources() {
    s.AddResource(Resource{
        URI: "/metrics/{project_id}/complexity",
        Name: "complexity_metrics",
        Description: "Code complexity metrics",
        Handler: s.handleComplexityMetrics,
    })
}
```

## Limitations

The current implementation has the following limitations:

1. **Stub Implementation**: The current implementation is a stub that allows compilation. Full mcp-go library integration is pending.

2. **HTTP Transport**: Currently only stdio transport is functional. HTTP transport will be available with full mcp-go integration.

3. **Authentication**: Token-based authentication is configured but not enforced in the stub implementation.

4. **Streaming**: Response streaming is not yet implemented.

## Future Enhancements

1. **Full mcp-go Integration**: Complete integration with the official mcp-go library when stable
2. **HTTP/WebSocket Transport**: Support for network-based communication
3. **Enhanced Security**: OAuth2/JWT authentication support
4. **Real-time Updates**: WebSocket support for live code analysis updates
5. **Multi-project Support**: Analyze and query multiple projects simultaneously

## Support

For issues or questions:

1. Check the [GitHub Issues](https://github.com/compozy/gograph/issues)
2. Refer to the [MCP Specification](../MCP_SPEC.md)
3. Review the [Technical Documentation](../TECHNICAL_DOCS.md)
