# GoGraph CLI Reference

This document provides a comprehensive reference for all GoGraph CLI commands.

## Global Flags

These flags can be used with any command:

- `--config string`: Config file path (default: ./gograph.yaml)
- `--help`: Display help information for the command

## Commands

### `gograph init`

Initialize a new GoGraph project configuration.

**Usage:**
```bash
gograph init [flags]
```

**Flags:**
- `--project-id string`: Unique project identifier (required)
- `--project-name string`: Human-readable project name
- `--neo4j-uri string`: Neo4j connection URI (default: bolt://localhost:7687)
- `--neo4j-user string`: Neo4j username (default: neo4j)
- `--neo4j-password string`: Neo4j password (default: password)
- `--force`: Overwrite existing configuration

**Examples:**
```bash
# Initialize with interactive prompts
gograph init

# Initialize with all options
gograph init --project-id myproject --project-name "My Go Project" \
  --neo4j-uri bolt://localhost:7687 --neo4j-user neo4j --neo4j-password secret
```

### `gograph analyze`

Analyze a Go codebase and store the results in Neo4j.

**Usage:**
```bash
gograph analyze [path] [flags]
```

**Flags:**
- `--path string`: Path to analyze (default: current directory)
- `--neo4j-uri string`: Neo4j connection URI (overrides config)
- `--neo4j-user string`: Neo4j username (overrides config)
- `--neo4j-password string`: Neo4j password (overrides config)
- `--project-id string`: Project identifier (overrides config)
- `--concurrency int`: Number of concurrent workers (default: 4)
- `--include-tests`: Include test files in analysis
- `--include-vendor`: Include vendor directory

**Examples:**
```bash
# Analyze current directory
gograph analyze

# Analyze specific directory with options
gograph analyze /path/to/project --include-tests --concurrency 8

# Override project ID
gograph analyze --project-id temporary-analysis
```

### `gograph call-chain`

Trace function call chains to understand execution flow and dependencies.

**Usage:**
```bash
gograph call-chain <function-name> [flags]
```

**Arguments:**
- `function-name`: Name of the function to trace (supports partial matching)

**Flags:**
- `-p, --project string`: Project ID (defaults to current directory config)
- `-t, --to string`: Target function to trace to
- `-d, --depth int`: Maximum search depth (default: 5)
- `-r, --reverse`: Trace in reverse (find functions that call the target)
- `--format string`: Output format: table, json (default: table)
- `--no-progress`: Disable progress indicators

**Examples:**
```bash
# Find all functions called by main
gograph call-chain main

# Find call paths from Handler to SaveUser
gograph call-chain Handler --to SaveUser --depth 10

# Find all functions that call SaveUser (reverse)
gograph call-chain SaveUser --reverse

# Find which functions call ProcessRequest up to main (reverse)
gograph call-chain ProcessRequest --to main --reverse

# Output as JSON
gograph call-chain Execute --format json
```

**Reverse Mode:**
When using `--reverse`:
- Without `--to`: Finds all functions that call the specified function
- With `--to`: Finds paths from the source function to the target function in reverse

### `gograph query`

Execute Cypher queries against the Neo4j database.

**Usage:**
```bash
gograph query "CYPHER_QUERY" [flags]
```

**Flags:**
- `--format string`: Output format: table, json, csv (default: table)
- `--output string`: Output file (default: stdout)
- `--params string`: Query parameters as JSON
- `--limit int`: Maximum number of results (default: 100)
- `-c, --count`: Show result count and timing
- `--no-progress`: Disable progress indicators

**Examples:**
```bash
# Simple query
gograph query "MATCH (p:Package) RETURN p.name"

# Complex query with formatting
gograph query "MATCH (f:Function)<-[:CALLS]-() RETURN f.name, count(*) as calls" --format json

# Query with parameters
gograph query "MATCH (f:Function {name: \$name}) RETURN f" --params '{"name":"main"}'

# Export to file
gograph query "MATCH (n) RETURN n" --format csv --output results.csv
```

### `gograph clear`

Clear project data from the database.

**Usage:**
```bash
gograph clear [project-id] [flags]
```

**Flags:**
- `--all`: Clear all projects (dangerous - use with caution)
- `--force`: Skip confirmation prompt

**Examples:**
```bash
# Clear current project
gograph clear

# Clear specific project
gograph clear my-backend-api

# Clear with confirmation skip
gograph clear --force

# Clear all projects (dangerous)
gograph clear --all --force
```

### `gograph serve-mcp`

Start the Model Context Protocol (MCP) server for LLM integration.

**Usage:**
```bash
gograph serve-mcp [flags]
```

**Flags:**
- `--http`: Use HTTP transport (when available)
- `--port int`: HTTP server port (default: 8080)
- `--config string`: MCP configuration file

**Examples:**
```bash
# Start MCP server with stdio transport
gograph serve-mcp

# Start with HTTP transport (if available)
gograph serve-mcp --http --port 9000
```

### `gograph version`

Display version information.

**Usage:**
```bash
gograph version
```

### `gograph help`

Display help information for any command.

**Usage:**
```bash
gograph help [command]
```

**Examples:**
```bash
# General help
gograph help

# Command-specific help
gograph help analyze
gograph help call-chain
```

## Environment Variables

GoGraph supports configuration through environment variables with the `GOGRAPH_` prefix:

- `GOGRAPH_NEO4J_URI`: Neo4j connection URI
- `GOGRAPH_NEO4J_USERNAME`: Neo4j username
- `GOGRAPH_NEO4J_PASSWORD`: Neo4j password
- `GOGRAPH_NEO4J_DATABASE`: Neo4j database name
- `GOGRAPH_PROJECT_ID`: Default project ID
- `GOGRAPH_PROJECT_NAME`: Default project name

Environment variables take precedence over configuration file values but can be overridden by command-line flags.

## Configuration File

GoGraph uses a YAML configuration file (`gograph.yaml`) with the following structure:

```yaml
project:
  id: my-awesome-project
  name: My Awesome Go Project
  path: .

neo4j:
  uri: bolt://localhost:7687
  username: neo4j
  password: password
  database: neo4j

analysis:
  ignore_dirs:
    - .git
    - vendor
    - node_modules
  ignore_files:
    - "*.pb.go"
    - "*_generated.go"
  include_tests: false
  include_vendor: false
  concurrency: 4
```

## Exit Codes

- `0`: Success
- `1`: General error
- `2`: Configuration error
- `3`: Connection error
- `4`: Analysis error

## Common Workflows

### Initial Setup
```bash
# 1. Initialize configuration
gograph init --project-id myproject

# 2. Analyze your codebase
gograph analyze

# 3. Explore with queries
gograph query "MATCH (n) RETURN n LIMIT 100"
```

### Continuous Analysis
```bash
# Re-analyze after code changes
gograph analyze --include-tests

# Check for issues
gograph call-chain main --depth 10
```

### Impact Analysis
```bash
# Find what depends on a function before modifying it
gograph call-chain SaveUser --reverse

# Check circular dependencies
gograph query "MATCH path=(p1:Package)-[:IMPORTS*]->(p1) RETURN path"
```