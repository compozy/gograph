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

## 📋 Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Usage](#usage)
- [Configuration](#configuration)
- [Graph Schema](#graph-schema)
- [MCP Integration](#mcp-integration)
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
   gograph init
   ```

3. **Analyze your Go project**:

   ```bash
   gograph analyze
   ```

4. **Query the graph**:
   ```bash
   gograph query "MATCH (n:Function) RETURN n.name LIMIT 10"
   ```

## 📖 Usage

### Commands

#### `gograph init`

Initialize a new project configuration in the current directory.

```bash
gograph init [flags]

Flags:
  --force          Overwrite existing configuration
  --project-name   Set project name (default: directory name)
```

#### `gograph analyze`

Analyze a Go project and store the results in Neo4j.

```bash
gograph analyze [path] [flags]

Flags:
  --path string              Path to analyze (default: current directory)
  --neo4j-uri string         Neo4j connection URI
  --neo4j-user string        Neo4j username
  --neo4j-password string    Neo4j password
  --project-id string        Project identifier
  --concurrency int          Number of concurrent workers (default: 4)
  --include-tests           Include test files in analysis
  --include-vendor          Include vendor directory
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

Clear project data from the database.

```bash
gograph clear [project-id] [flags]

Flags:
  --all    Clear all projects
  --force  Skip confirmation prompt
```

### Examples

```bash
# Analyze current directory
gograph analyze

# Analyze specific project with custom settings
gograph analyze /path/to/project \
  --project-id myproject \
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
  name: my-project
  root_path: .
  id: my-project-id

neo4j:
  uri: bolt://localhost:7687
  username: neo4j
  password: password
  database: gograph_my_project

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
export GOGRAPH_PROJECT_ID=my-project
export GOGRAPH_MCP_PORT=8080
```

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
| `DECLARES`   | File declares function/struct/interface                   |
| `USES`       | Function uses variable/constant                           |

### Example Queries

```cypher
-- Find all functions in a package
MATCH (p:Package {name: "main"})-[:CONTAINS]->(f:File)-[:DECLARES]->(fn:Function)
RETURN fn.name, fn.signature

-- Find circular dependencies
MATCH path=(p1:Package)-[:IMPORTS*]->(p1)
RETURN path

-- Find most called functions
MATCH (f:Function)<-[:CALLS]-(caller)
RETURN f.name, count(caller) as call_count
ORDER BY call_count DESC

-- Find unused functions
MATCH (f:Function)
WHERE NOT (f)<-[:CALLS]-()
RETURN f.name, f.package

-- Find interface implementations
MATCH (s:Struct)-[:IMPLEMENTS]->(i:Interface)
RETURN s.name, i.name
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

- `analyze_project`: Analyze a Go project
- `query_dependencies`: Query project dependencies
- `get_function_info`: Get detailed function information
- `list_packages`: List all packages in a project
- `execute_cypher`: Execute custom Cypher queries
- `natural_language_query`: Translate natural language to Cypher
- `detect_code_patterns`: Detect common design patterns
- `verify_function_exists`: Verify function existence (prevent hallucinations)
- `verify_import_relationship`: Verify import relationships

For detailed MCP integration guide, see [docs/MCP_INTEGRATION.md](docs/MCP_INTEGRATION.md).

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
- [MCP Integration Guide](docs/MCP_INTEGRATION.md)
- [Issue Tracker](https://github.com/compozy/gograph/issues)
- [Discussions](https://github.com/compozy/gograph/discussions)
- [Releases](https://github.com/compozy/gograph/releases)

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/compozy">Pedro Nauck</a>
</p>
