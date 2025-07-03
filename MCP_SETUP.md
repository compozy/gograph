# GoGraph MCP Server Setup for Cursor

This guide explains how to set up the GoGraph MCP (Model Context Protocol) server to work with Cursor IDE.

## Prerequisites

1. **Neo4j Database**: Ensure Neo4j is running on `bolt://localhost:7687`
2. **Built GoGraph Binary**: Run `make build` to create the `bin/gograph` executable

## Setup Instructions

### 1. Build GoGraph

```bash
make build
```

### 2. Configure Cursor MCP Settings

1. Open Cursor IDE
2. Go to **Settings** (Cmd/Ctrl + ,)
3. Search for "mcp" or navigate to **Extensions** → **Model Context Protocol**
4. Add the following configuration to your MCP settings:

```json
{
  "mcpServers": {
    "gograph": {
      "command": "/Users/pedronauck/Dev/ai/gograph/bin/gograph",
      "args": ["serve-mcp"],
      "env": {
        "NEO4J_URI": "bolt://localhost:7687",
        "NEO4J_USERNAME": "neo4j",
        "NEO4J_PASSWORD": "password"
      }
    }
  }
}
```

**Important**: Update the paths and credentials:
- Replace `/Users/pedronauck/Dev/ai/gograph/bin/gograph` with the absolute path to your built binary
- Update Neo4j credentials if different from defaults

### 3. Alternative Configuration File

You can also create a separate MCP configuration file:

1. Create `~/.cursor/mcp_settings.json` with the configuration above
2. Or use the provided `cursor-mcp-config.json` as a template

### 4. Restart Cursor

After adding the configuration, restart Cursor IDE for the changes to take effect.

## Available Tools

Once configured, Cursor will have access to 17 GoGraph tools:

### 📊 Analysis Tools
- `analyze_project` - Analyze a Go project and build its dependency graph
- `query_dependencies` - Query dependencies of a package or file

### 🧭 Navigation Tools  
- `find_implementations` - Find all implementations of an interface
- `trace_call_chain` - Trace call chains between functions
- `detect_circular_deps` - Detect circular dependencies in the project
- `get_function_info` - Get detailed information about a function
- `list_packages` - List all packages in the project
- `get_package_structure` - Get detailed structure of a package

### 🔍 Query Tools
- `execute_cypher` - Execute a Cypher query against the graph database
- `natural_language_query` - Convert natural language to Cypher and execute

### ✅ Verification Tools
- `verify_code_exists` - Verify if a code element exists in the project
- `get_code_context` - Get context around a code element for LLM understanding
- `validate_import_path` - Validate and resolve an import path

### 🎯 Pattern Tools
- `detect_code_patterns` - Detect common code patterns and anti-patterns
- `get_naming_conventions` - Analyze naming conventions used in the project

### 🧪 Test Tools
- `find_tests_for_code` - Find tests for a specific code element
- `check_test_coverage` - Check test coverage for packages or files

## Usage Examples

Once configured, you can use natural language in Cursor to interact with your Go codebase:

- *"Analyze this project and show me the main packages"*
- *"Find all implementations of the ServiceAdapter interface"*
- *"Show me the call chain from main() to database operations"*
- *"What functions call the HandleRequest method?"*
- *"Detect any circular dependencies in this codebase"*
- *"Find tests for the ParseProject function"*

## Troubleshooting

### MCP Server Not Starting
1. Verify Neo4j is running: `docker ps` or check your local Neo4j instance
2. Check the binary path is correct and executable: `ls -la /path/to/bin/gograph`
3. Test the server manually: `./bin/gograph serve-mcp --help`

### Connection Issues
1. Verify Neo4j credentials in the configuration
2. Check Neo4j is accessible: `nc -zv localhost 7687`
3. Review Cursor's MCP logs in the developer console

### Tools Not Available
1. Restart Cursor after configuration changes
2. Check MCP configuration syntax is valid JSON
3. Verify the `command` path points to the built binary

## Manual Testing

Test the MCP server manually:

```bash
# Test tools list
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | ./bin/gograph serve-mcp

# Test project analysis (replace with your project path)
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"analyze_project","arguments":{"project_path":".","project_id":"test-project"}}}' | ./bin/gograph serve-mcp
```

## Next Steps

Once set up, GoGraph will enhance Cursor's understanding of your Go codebase by providing:
- Accurate code navigation and search
- Dependency analysis and visualization  
- Interface implementation discovery
- Call chain tracing
- Anti-pattern detection
- Test coverage analysis

The integration enables Cursor to provide more accurate suggestions and better understand your codebase structure.