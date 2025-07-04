# gograph - Go Codebase Graph Analyzer

[![CI](https://github.com/compozy/gograph/workflows/CI/badge.svg)](https://github.com/compozy/gograph/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/compozy/gograph)](https://goreportcard.com/report/github.com/compozy/gograph)
[![codecov](https://codecov.io/gh/compozy/gograph/branch/main/graph/badge.svg)](https://codecov.io/gh/compozy/gograph)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A powerful CLI tool that analyzes Go codebases and generates graph visualizations using Neo4j, designed to help LLMs better understand project structures and avoid hallucination through concrete dependency mapping.

## 🚀 Features

- **🔍 AST-based Analysis**: Deep parsing of Go source code using Go's native AST parser
- **📊 Neo4j Integration**: Stores codebase structure in a graph database for powerful querying
- **🏗️ Per-Project Isolation**: Each project gets its own database namespace
- **🤖 MCP Server**: Model Context Protocol integration for LLM applications
- **⚡ Concurrent Processing**: Efficient parallel parsing of large codebases
- **🔧 CLI Tool**: Easy-to-use command-line interface with Cobra
- **📝 Configurable**: YAML-based configuration for project-specific settings
- **🏛️ Clean Architecture**: Extensible design following Domain-Driven Design principles

## 🛡️ Project Isolation

gograph provides **complete project isolation** to enable multiple Go projects to coexist safely in the same Neo4j database:

### Key Benefits

- **🏗️ Multi-Project Support**: Analyze multiple projects without data conflicts
- **🔒 Data Isolation**: Each project's data is completely isolated from others
- **🏷️ Unique Identifiers**: User-defined project IDs ensure clear project boundaries
- **🚀 Performance Optimized**: Database indexes on project_id for fast queries
- **🧹 Safe Cleanup**: Clear project data without affecting other projects

### How It Works

1. **Project Initialization**: Each project requires a unique `project_id` during setup
2. **Data Tagging**: All nodes and relationships are tagged with the project_id
3. **Query Filtering**: All operations automatically filter by project_id
4. **Index Optimization**: Database indexes ensure fast project-scoped queries

### Usage Example

```bash
# Project A
cd /path/to/project-a
gograph init --project-id backend-api --project-name "Backend API"
gograph analyze

# Project B
cd /path/to/project-b
gograph init --project-id frontend-app --project-name "Frontend App"
gograph analyze

# Both projects coexist safely in the same database
# Queries automatically filter by project_id
```

## 📋 Table of Contents

- [Project Isolation](#️-project-isolation)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Complete Workflow](#-complete-codebase-analysis-workflow)
- [Usage](#usage)
- [Configuration](#configuration)
- [Graph Schema](#graph-schema)
- [MCP Integration](#mcp-integration)
- [Architecture](#️-architecture)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## 🛠 Installation

### Prerequisites

- Go 1.24 or higher
- Neo4j 5.x or higher
- Make (for build automation)

### Install from Source

```bash
# Clone the repository
git clone https://github.com/compozy/gograph.git
cd gograph

# Install dependencies and build
make deps
make build

# Install to GOPATH (optional)
make install
```

### Using Go Install

```bash
go install github.com/compozy/gograph/cmd/gograph@latest
```

### Using Docker

```bash
# Pull the image
docker pull compozy/gograph:latest

# Run with volume mount
docker run -v $(pwd):/workspace compozy/gograph:latest analyze /workspace
```

## 🚀 Quick Start

1. **Start Neo4j**:

   ```bash
   # Using Docker
   make run-neo4j

   # Or using Docker directly
   docker run -d \
     --name gograph-neo4j \
     -p 7474:7474 -p 7687:7687 \
     -e NEO4J_AUTH=neo4j/password \
     neo4j:5-community
   ```

2. **Initialize project configuration**:

   ```bash
   gograph init --project-id my-awesome-project
   ```

3. **Analyze your Go project**:

   ```bash
   gograph analyze
   ```

4. **Query the graph**:

   ```bash
   gograph query "MATCH (n:Function) RETURN n.name LIMIT 10"
   ```

5. **View the complete graph in Neo4j Browser**:
   ```bash
   # Open Neo4j Browser at http://localhost:7474
   # Login with: neo4j/password
   # Run this query to see the entire codebase graph:
   MATCH (n) RETURN n LIMIT 500
   ```

## 🔍 Complete Codebase Analysis Workflow

Here's a step-by-step guide to analyze your entire codebase and visualize it in Neo4j:

### 1. Setup and Analysis

```bash
# Start Neo4j database
make run-neo4j

# Navigate to your Go project directory
cd /path/to/your/go/project

# Initialize gograph configuration with required project ID
gograph init --project-id my-awesome-project --project-name "My Awesome Project"

# Analyze the entire codebase
# Note: gograph automatically finds and uses the project ID from gograph.yaml
# in the project directory or any parent directory
gograph analyze . \
  --include-tests \
  --include-vendor \
  --concurrency 8
```

### 2. Access Neo4j Browser

1. Open your web browser and go to: **http://localhost:7474**
2. Login with:
   - **Username**: `neo4j`
   - **Password**: `password`

### 3. Essential Graph Visualization Queries

Once connected to Neo4j Browser, you can explore your codebase using Cypher queries.

**📚 For a comprehensive collection of queries, see [docs/QUERIES.md](docs/QUERIES.md)**

Here are a few essential queries to get you started:

```cypher
// View the entire project structure
MATCH (n)
WHERE n.project_id = 'my-awesome-project'
RETURN n
LIMIT 500

// Find the most connected functions
MATCH (f:Function)
WHERE f.project_id = 'my-awesome-project'
OPTIONAL MATCH (f)-[:CALLS]->(called)
WITH f, count(called) as outgoing_calls
OPTIONAL MATCH (f)<-[:CALLS]-(caller)
RETURN f.name, f.package, outgoing_calls, count(caller) as incoming_calls,
       (outgoing_calls + count(caller)) as total_connections
ORDER BY total_connections DESC
LIMIT 20
```

### 4. Interactive Exploration Tips

**In Neo4j Browser:**

1. **Expand Nodes**: Click on any node to see its properties
2. **Follow Relationships**: Double-click relationships to explore connections
3. **Filter Results**: Use the sidebar to filter node types and relationships
4. **Adjust Layout**: Use the layout options to better organize the graph
5. **Export Views**: Save interesting queries and graph views

**Useful Browser Commands:**

See [docs/QUERIES.md](docs/QUERIES.md) for more queries including:

- Node and relationship type discovery
- Circular dependency detection
- Package dependency analysis
- Test coverage analysis
- And many more!

### 5. Advanced Analysis Examples

For advanced analysis queries, see [docs/QUERIES.md](docs/QUERIES.md) which includes:

- Finding unused functions
- Analyzing test coverage by package
- Finding interface implementations
- Detecting circular dependencies
- Identifying code complexity hotspots
- And many more analysis patterns!

### 6. Export and Share Results

```bash
# Export query results to JSON
gograph query "MATCH (n:Package) RETURN n" --format json --output packages.json

# Export to CSV for further analysis
gograph query "
  MATCH (f:Function)<-[:CALLS]-(caller)
  RETURN f.name, f.package, count(caller) as call_count
  ORDER BY call_count DESC
" --format csv --output function_popularity.csv
```

This workflow gives you a complete view of your codebase structure, dependencies, and relationships, making it easy to understand complex Go projects and identify architectural patterns or issues.

## 📖 Usage

### Commands

#### `gograph init`

Initialize a new project configuration in the current directory.

```bash
gograph init [flags]

Flags:
  --project-id string      Unique project identifier (required)
  --project-name string    Human-readable project name (defaults to project-id)
  --project-path string    Project root path (defaults to current directory)
  --force                  Overwrite existing configuration
```

**Examples:**

```bash
# Basic initialization with required project ID
gograph init --project-id my-backend-api

# Full initialization with custom settings
gograph init \
  --project-id my-backend-api \
  --project-name "My Backend API" \
  --project-path ./src

# Force overwrite existing configuration
gograph init --project-id my-api --force
```

#### `gograph analyze`

Analyze a Go project and store the results in Neo4j. The project ID is automatically loaded from the `gograph.yaml` configuration file.

```bash
gograph analyze [path] [flags]

Flags:
  --path string              Path to analyze (default: current directory)
  --neo4j-uri string         Neo4j connection URI (overrides config)
  --neo4j-user string        Neo4j username (overrides config)
  --neo4j-password string    Neo4j password (overrides config)
  --project-id string        Project identifier (overrides config)
  --concurrency int          Number of concurrent workers (default: 4)
  --include-tests           Include test files in analysis
  --include-vendor          Include vendor directory
```

**Examples:**

```bash
# Analyze current directory (uses config from gograph.yaml)
gograph analyze

# Analyze specific directory with all options
gograph analyze /path/to/project --include-tests --include-vendor --concurrency 8

# Override project ID for one-time analysis
gograph analyze --project-id temporary-analysis
```

#### `gograph query`

Execute Cypher queries against the graph database.

```bash
gograph query "CYPHER_QUERY" [flags]

Flags:
  --format string    Output format: table, json, csv (default: table)
  --output string    Output file (default: stdout)
  --params string    Query parameters as JSON
```

#### `gograph serve-mcp`

Start the MCP (Model Context Protocol) server for LLM integration.

```bash
gograph serve-mcp [flags]

Flags:
  --http             Use HTTP transport (when available)
  --port int         HTTP server port (default: 8080)
  --config string    MCP configuration file
```

#### `gograph clear`

Clear project data from the database. Uses project isolation to safely remove only the specified project's data.

```bash
gograph clear [project-id] [flags]

Flags:
  --all    Clear all projects (dangerous - use with caution)
  --force  Skip confirmation prompt
```

**Examples:**

```bash
# Clear current project (reads project ID from gograph.yaml)
gograph clear

# Clear specific project by ID
gograph clear my-backend-api

# Clear with confirmation skip
gograph clear my-backend-api --force

# Clear all projects (dangerous - removes all data)
gograph clear --all --force
```

**Safety**: The clear command only removes data tagged with the specified project_id, ensuring other projects remain untouched.

### Examples

```bash
# Initialize a new project
gograph init --project-id myproject --project-name "My Go Project"

# Analyze current directory (uses project ID from gograph.yaml)
gograph analyze

# Analyze specific project with custom settings
gograph analyze /path/to/project \
  --concurrency 8 \
  --include-tests

# Query function dependencies
gograph query "
  MATCH (f:Function)-[:CALLS]->(g:Function)
  WHERE f.name = 'main'
  RETURN g.name, g.package
"

# Find circular dependencies
gograph query "
  MATCH path=(p1:Package)-[:IMPORTS*]->(p1)
  RETURN path
  LIMIT 5
"

# Start MCP server for Claude Desktop
gograph serve-mcp
```

## ⚙️ Configuration

Create a `gograph.yaml` file in your project root:

```yaml
project:
  id: my-project-id # Required: Unique project identifier
  name: my-project # Optional: Human-readable name (defaults to id)
  root_path: . # Optional: Project root path (defaults to ".")

neo4j:
  uri: bolt://localhost:7687 # Neo4j connection URI
  username: neo4j # Neo4j username
  password: password # Neo4j password
  database: "" # Optional: Database name (uses default if empty)

analysis:
  ignore_dirs:
    - .git
    - vendor
    - node_modules
    - tmp
  ignore_files:
    - "*.pb.go"
    - "*_mock.go"
  include_tests: true
  include_vendor: false
  max_concurrency: 4

mcp:
  server:
    port: 8080
    host: localhost
    max_connections: 100
  auth:
    enabled: false
    token: ""
  performance:
    max_context_size: 1000000
    cache_ttl: 3600
    batch_size: 100
    request_timeout: 30s
  security:
    allowed_paths:
      - "."
    forbidden_paths:
      - ".git"
      - "vendor"
    rate_limit: 100
    max_query_time: 30
```

### Environment Variables

You can override configuration using environment variables:

```bash
export GOGRAPH_NEO4J_URI=bolt://localhost:7687
export GOGRAPH_NEO4J_USERNAME=neo4j
export GOGRAPH_NEO4J_PASSWORD=password
export GOGRAPH_PROJECT_ID=my-project         # Overrides config project ID
export GOGRAPH_MCP_PORT=8080
```

**Important**: The `GOGRAPH_PROJECT_ID` environment variable will override the project ID from your `gograph.yaml` file. This is useful for CI/CD environments or temporary analysis.

## 📊 Graph Schema

### Node Types

| Node Type   | Description             | Properties                                    |
| ----------- | ----------------------- | --------------------------------------------- |
| `Package`   | Go packages             | `name`, `path`, `project_id`                  |
| `File`      | Go source files         | `name`, `path`, `lines`, `project_id`         |
| `Function`  | Function declarations   | `name`, `signature`, `line`, `project_id`     |
| `Struct`    | Struct type definitions | `name`, `fields`, `line`, `project_id`        |
| `Interface` | Interface definitions   | `name`, `methods`, `line`, `project_id`       |
| `Method`    | Methods on types        | `name`, `receiver`, `signature`, `project_id` |
| `Constant`  | Constant declarations   | `name`, `value`, `type`, `project_id`         |
| `Variable`  | Variable declarations   | `name`, `type`, `line`, `project_id`          |
| `Import`    | Import statements       | `path`, `alias`, `project_id`                 |

### Relationship Types

| Relationship | Description                                               |
| ------------ | --------------------------------------------------------- |
| `CONTAINS`   | Package contains file, file contains function/struct/etc. |
| `IMPORTS`    | File imports package                                      |
| `CALLS`      | Function calls another function                           |
| `IMPLEMENTS` | Struct implements interface                               |
| `HAS_METHOD` | Struct/interface has method                               |
| `DEPENDS_ON` | File depends on another file                              |
| `DEFINES`    | File defines function/struct/interface                    |
| `USES`       | Function uses variable/constant                           |

### Example Queries

For a comprehensive collection of Cypher queries organized by use case, see [docs/QUERIES.md](docs/QUERIES.md).

Quick examples:

```cypher
-- Find most called functions
MATCH (f:Function)<-[:CALLS]-(caller)
WHERE f.project_id = 'my-project'
RETURN f.name, count(caller) as call_count
ORDER BY call_count DESC
LIMIT 10

-- Find interface implementations
MATCH (s:Struct)-[:IMPLEMENTS]->(i:Interface)
WHERE s.project_id = 'my-project'
RETURN i.name as Interface, collect(s.name) as Implementations
```

## 🤖 MCP Integration

gograph provides a Model Context Protocol (MCP) server that enables LLM applications to analyze Go codebases directly.

### Quick Setup with Claude Desktop

Add to your Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gograph": {
      "command": "gograph",
      "args": ["serve-mcp"],
      "env": {
        "GOGRAPH_CONFIG": "/path/to/your/gograph.yaml"
      }
    }
  }
}
```

### Available MCP Tools

**Note:** All MCP tools now support automatic project ID discovery from your `gograph.yaml` configuration file. When using MCP tools, you no longer need to provide the `project_id` parameter - it will be automatically derived from the project's configuration file.

**Project Management:**

- `list_projects`: List all projects in the database
- `validate_project`: Validate project existence and configuration
- `analyze_project`: Analyze a Go project with full isolation

**Code Analysis:**

- `query_dependencies`: Query project dependencies with filtering
- `get_function_info`: Get detailed function information
- `list_packages`: List all packages in a specific project
- `get_package_structure`: Get detailed package structure
- `find_implementations`: Find interface implementations
- `trace_call_chain`: Trace function call chains

**Querying & Search:**

- `execute_cypher`: Execute custom Cypher queries with project filtering
- `natural_language_query`: Translate natural language to Cypher
- `get_code_context`: Get code context for LLM understanding

**Code Quality:**

- `detect_code_patterns`: Detect common design patterns and anti-patterns
- `check_test_coverage`: Analyze test coverage by package
- `detect_circular_deps`: Find circular dependencies

**Verification (Anti-Hallucination):**

- `verify_code_exists`: Verify function/type existence
- `validate_import_path`: Verify import relationships

For detailed MCP integration guide, see [docs/MCP_INTEGRATION.md](docs/MCP_INTEGRATION.md).

## 🧠 LLM Project Configuration (CLAUDE.md)

For optimal LLM assistance with Go projects, add gograph MCP integration to your project's `CLAUDE.md` file. This enables LLMs to understand your codebase structure accurately and provide better assistance.

### Why Use GoGraph MCP for LLMs?

**🎯 Prevents Hallucination**: LLMs can verify actual code structure instead of guessing
**🔍 Deep Code Understanding**: Access to function calls, dependencies, and architectural patterns  
**📊 Real-time Analysis**: Up-to-date codebase information for accurate suggestions
**🏗️ Architectural Insights**: Understanding of package dependencies and design patterns

### When to Use GoGraph MCP

✅ **Recommended for:**

- Large Go codebases (>50 files)
- Complex microservices with multiple packages
- Legacy code exploration and refactoring
- Architectural analysis and design decisions
- Code review and quality assessment
- Dependency management and circular dependency detection

✅ **Especially valuable when:**

- Working with unfamiliar codebases
- Analyzing call chains and function relationships
- Identifying unused code or dead functions
- Understanding interface implementations
- Planning refactoring or architectural changes

### CLAUDE.md Configuration

Add this section to your project's `CLAUDE.md` file:

```markdown
# Go Codebase Analysis with GoGraph

This project uses GoGraph for deep codebase analysis and LLM integration.

## Available Analysis Tools

When working with this Go codebase, you have access to powerful analysis tools through GoGraph MCP:

### 🔍 Code Structure Analysis

- `analyze_project`: Analyze the entire Go project structure
- `list_packages`: List all packages in the project
- `get_package_structure`: Get detailed package information
- `get_function_info`: Get comprehensive function details

### 🔗 Dependency Analysis

- `query_dependencies`: Analyze package and file dependencies
- `detect_circular_deps`: Find circular dependencies
- `trace_call_chain`: Trace function call relationships
- `find_implementations`: Find interface implementations

### 🎯 Code Quality & Patterns

- `detect_code_patterns`: Identify design patterns and anti-patterns
- `check_test_coverage`: Analyze test coverage by package
- `verify_code_exists`: Verify function/type existence (prevents hallucination)

### 🔎 Smart Querying

- `natural_language_query`: Ask questions in natural language about the code
- `execute_cypher`: Run custom graph queries for complex analysis
- `get_database_schema`: Understand the code graph structure

## Usage Guidelines for LLMs

### ✅ Always Do:

1. **Verify before suggesting**: Use `verify_code_exists` before making suggestions about functions/types
2. **Understand structure first**: Use `get_package_structure` when exploring new areas
3. **Check dependencies**: Use `query_dependencies` before suggesting architectural changes
4. **Analyze patterns**: Use `detect_code_patterns` to understand existing design approaches

### 🔍 For Code Reviews:

- Use `detect_circular_deps` to identify architectural issues
- Use `check_test_coverage` to assess test quality
- Use `trace_call_chain` to understand impact of changes

### 🏗️ For Architectural Decisions:

- Use `analyze_project` for overview understanding
- Use `find_implementations` to understand interface usage
- Use `natural_language_query` for complex architectural questions

### 📊 Example Queries:

- "Find all functions that are never called"
- "Show me the dependency graph for the auth package"
- "What interfaces are implemented by UserService?"
- "Find all functions with high complexity"
```

### Configuration Steps

1. **Install GoGraph**: Follow the [installation instructions](#installation)

2. **Initialize your project**:

   ```bash
   cd /path/to/your/go/project
   gograph init --project-id your-project-name
   ```

3. **Add MCP configuration** to Claude Desktop (`~/Library/Application Support/Claude/claude_desktop_config.json`):

   ```json
   {
     "mcpServers": {
       "gograph": {
         "command": "gograph",
         "args": ["serve-mcp"],
         "env": {
           "GOGRAPH_CONFIG": "/path/to/your/project/gograph.yaml"
         }
       }
     }
   }
   ```

4. **Analyze your project**:

   ```bash
   gograph analyze
   ```

5. **Start using LLM assistance** with full codebase awareness!

### Benefits for LLM Interactions

- **Accurate Suggestions**: LLMs can verify function existence and signatures
- **Context-Aware Recommendations**: Understanding of actual package structure
- **Dependency-Aware Refactoring**: Knowledge of what calls what
- **Pattern Recognition**: Identification of existing architectural patterns
- **Quality Insights**: Test coverage and code quality metrics

## 🏗 Architecture

The project follows Domain-Driven Design with clean architecture principles:

```
gograph/
├── cmd/gograph/           # CLI application entry point
│   ├── commands/          # Cobra command implementations
│   └── main.go           # Application main
├── engine/               # Core business logic
│   ├── core/            # Shared domain entities and errors
│   ├── parser/          # Go AST parsing domain
│   ├── graph/           # Graph operations domain
│   ├── analyzer/        # Code analysis domain
│   ├── query/           # Query building and execution
│   ├── llm/             # LLM integration (Cypher translation)
│   ├── mcp/             # Model Context Protocol server
│   └── infra/           # Infrastructure (Neo4j implementation)
├── pkg/                 # Shared utilities
│   ├── config/          # Configuration management
│   ├── logger/          # Structured logging
│   ├── progress/        # Progress reporting
│   └── testhelpers/     # Test utilities
├── test/                # Integration tests
├── testdata/            # Test fixtures
└── docs/                # Documentation
```

### Key Design Principles

- **Clean Architecture**: Dependencies point inward toward the domain
- **Domain-Driven Design**: Clear domain boundaries and ubiquitous language
- **Interface Segregation**: Small, focused interfaces
- **Dependency Injection**: Constructor-based dependency injection
- **Error Handling**: Structured error handling with context
- **Testing**: Comprehensive test coverage with testify

## 🔧 Development

### Prerequisites

- Go 1.24+
- Neo4j 5.x
- Make
- Docker (for integration tests)

### Setup Development Environment

```bash
# Clone and setup
git clone https://github.com/compozy/gograph.git
cd gograph

# Install dependencies
make deps

# Start development environment (Neo4j)
make dev

# Run tests
make test

# Run linting
make lint

# Build
make build
```

### Available Make Targets

```bash
make help                 # Show all available targets
make build               # Build the binary
make test                # Run all tests
make test-integration    # Run integration tests
make test-coverage       # Generate coverage report
make lint                # Run linter
make fmt                 # Format code
make clean               # Clean build artifacts
make dev                 # Start development environment
make ci-all              # Run full CI pipeline
```

### Testing

The project uses comprehensive testing with:

- **Unit Tests**: Fast, isolated tests for business logic
- **Integration Tests**: Tests with real Neo4j database
- **E2E Tests**: End-to-end CLI testing
- **Testify**: Assertions and mocking framework

```bash
# Run specific test suites
make test                    # Unit tests
make test-integration        # Integration tests
make test-coverage          # Coverage report

# Run tests with verbose output
go test -v ./...

# Run specific test
go test -run TestAnalyzer ./engine/analyzer/
```

### Code Quality

The project enforces high code quality standards:

- **golangci-lint**: Comprehensive linting
- **gosec**: Security analysis
- **nancy**: Vulnerability scanning
- **Test Coverage**: Aim for 80%+ coverage
- **Code Review**: All changes require review

### Project Standards

- Follow the coding standards in `.cursor/rules/`
- Use conventional commit messages
- Ensure all tests pass before submitting PRs
- Update documentation for new features
- Add integration tests for new domains

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Quick Contribution Steps

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes following the project standards
4. Add tests for new functionality
5. Ensure all tests pass (`make ci-all`)
6. Commit your changes (`git commit -m 'Add amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

### Development Workflow

1. **Issue First**: Open an issue to discuss new features or bugs
2. **Branch**: Create a feature branch from `main`
3. **Develop**: Follow the coding standards and add tests
4. **Test**: Ensure all tests pass and coverage is maintained
5. **Document**: Update documentation for new features
6. **Review**: Submit PR for code review
7. **Merge**: Squash and merge after approval

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

Inspired by:

- [Barista](https://github.com/smacke/barista) - Client-side codebase graph generator
- [Code2Flow](https://github.com/scottrogowski/code2flow) - Dynamic language call graph generator
- [Neo4j MCP Server](https://github.com/neo4j/mcp-server) - Model Context Protocol integration

## 🔗 Links

- [Documentation](docs/)
- [Query Reference Guide](docs/QUERIES.md)
- [MCP Integration Guide](docs/MCP_INTEGRATION.md)
- [Issue Tracker](https://github.com/compozy/gograph/issues)
- [Discussions](https://github.com/compozy/gograph/discussions)
- [Releases](https://github.com/compozy/gograph/releases)

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/compozy">Pedro Nauck</a>
</p>
