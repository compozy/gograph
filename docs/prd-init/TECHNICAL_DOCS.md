# gograph Technical Documentation & Progress Tracker

## Project Purpose

gograph is a Go codebase analyzer that creates Neo4j graph representations of Go projects. The primary goals are:

1. **LLM Integration**: Provide structured codebase understanding to prevent hallucination
2. **Dependency Visualization**: Map all code relationships in a queryable graph
3. **Project Discovery**: Enable semantic search and relationship queries
4. **Per-Project Isolation**: Support multiple projects with separate graph namespaces

## Technical Architecture

### Domain-Driven Design Structure

```
engine/
├── core/           # Shared domain primitives
│   ├── types.go    # Core entities (Node, Relationship, etc.)
│   └── errors.go   # Unified error handling
├── parser/         # AST parsing domain
│   ├── interfaces.go   # Parser contracts
│   └── service.go      # Go AST implementation
├── graph/          # Graph operations domain
│   └── interfaces.go   # Repository contracts
├── analyzer/       # Analysis domain (TODO)
└── infra/          # Infrastructure adapters
    └── neo4j_repository.go  # Neo4j implementation

pkg/
├── config/         # Configuration management
│   └── config.go   # Viper-based config
└── logger/         # Structured logging
    └── logger.go   # Charmbracelet/log wrapper

cmd/
└── gograph/        # CLI application (TODO)
    └── main.go     # Cobra commands
```

### Core Technologies

- **Language**: Go 1.22
- **CLI Framework**: Cobra + Viper
- **Graph Database**: Neo4j v5 (using official driver)
- **Logging**: Charmbracelet/log
- **Testing**: Testify (assert + mock)
- **Linting**: golangci-lint with project rules

### Key Design Patterns

1. **Repository Pattern**: Interfaces in domain, implementations in infra
2. **Factory Pattern**: Service constructors with default configs
3. **Error Handling**: Internal `fmt.Errorf`, public `core.NewError`
4. **Dependency Injection**: Constructor-based injection
5. **Context Propagation**: First parameter in all async operations

## Implementation Status

### ✅ Completed Components

#### 1. Core Domain (`engine/core/`)

- [x] Core types (ID, NodeType, RelationType, Node, Relationship)
- [x] Domain entities (Project, AnalysisResult, GraphMetadata)
- [x] Error handling with codes and context
- [x] Timestamp tracking for all entities

#### 2. Parser Domain (`engine/parser/`)

- [x] Parser interface definition
- [x] Complete Go AST implementation
- [x] Concurrent file parsing
- [x] Extract all Go constructs:
  - Packages and imports
  - Functions and methods
  - Structs and interfaces
  - Types, constants, variables
  - Dependencies and call relationships
- [x] Configurable ignore patterns

#### 3. Graph Domain (`engine/graph/`)

- [x] Repository interface for Neo4j operations
- [x] Node and relationship CRUD operations
- [x] Batch operations support
- [x] Query execution interface
- [x] Project namespace management

#### 4. Infrastructure (`engine/infra/`)

- [x] Neo4j repository implementation
- [x] Connection management
- [x] Transaction support
- [x] Batch create operations
- [x] Cypher query builder helpers

#### 5. Packages (`pkg/`)

- [x] Configuration management with Viper
- [x] Structured logging with Charmbracelet
- [x] Environment variable support
- [x] Default configuration values

### 🚧 In Progress

#### 1. Analyzer Domain (`engine/analyzer/`)

- [ ] Service interface definition
- [ ] Dependency resolution
- [ ] Call graph construction
- [ ] Interface implementation detection
- [ ] Circular dependency detection

#### 2. CLI Application (`cmd/gograph/`)

- [ ] Main command structure
- [ ] Init command (create config)
- [ ] Analyze command (parse & store)
- [ ] Query command (execute Cypher)
- [ ] Clear command (cleanup project)
- [ ] Version command

### 📋 Pending Tasks

#### Phase 1: Core Functionality

1. **Analyzer Implementation**

   - Create analyzer service
   - Build dependency graph
   - Detect interface implementations
   - Map function call chains

2. **CLI Commands**

   - Implement Cobra command structure
   - Add flag parsing
   - Progress indicators
   - Error handling and user feedback

3. **Graph Service**
   - Create graph service layer
   - Implement graph building from parser results
   - Add relationship inference
   - Optimize batch operations

#### Phase 2: Testing & Quality

1. **Unit Tests**

   - Parser service tests
   - Graph repository tests
   - Analyzer service tests
   - Configuration tests

2. **Integration Tests**

   - End-to-end parsing tests
   - Neo4j integration tests
   - CLI command tests

3. **Benchmarks**
   - Parser performance
   - Graph write performance
   - Memory usage optimization

#### Phase 3: Advanced Features

1. **Query Builder**

   - Predefined useful queries
   - Query templates
   - Export capabilities

2. **Visualization**

   - Graph visualization endpoints
   - D3.js integration
   - Export to GraphML/GEXF

3. **LLM Integration**
   - MCP server implementation
   - Natural language to Cypher
   - Context generation for LLMs

## Technical Specifications

### Graph Schema

#### Node Types

```cypher
// Package node
(:Package {
  id: "unique-id",
  name: "package/name",
  path: "/absolute/path",
  created_at: timestamp,
  updated_at: timestamp
})

// File node
(:File {
  id: "unique-id",
  path: "/path/to/file.go",
  package: "package-name",
  size: 1234,
  created_at: timestamp,
  updated_at: timestamp
})

// Function node
(:Function {
  id: "unique-id",
  name: "functionName",
  signature: "func(args) returns",
  receiver: "optional-receiver",
  is_exported: true,
  line_start: 10,
  line_end: 20,
  created_at: timestamp,
  updated_at: timestamp
})

// Similar for Struct, Interface, Method, Constant, Variable, Import
```

#### Relationship Types

```cypher
(:Package)-[:CONTAINS]->(:File)
(:File)-[:IMPORTS]->(:Package)
(:File)-[:DECLARES]->(:Function|Struct|Interface)
(:Struct)-[:HAS_METHOD]->(:Method)
(:Function)-[:CALLS]->(:Function)
(:Struct)-[:IMPLEMENTS]->(:Interface)
(:Struct)-[:EMBEDS]->(:Struct)
(:File)-[:DEPENDS_ON]->(:File)
```

### Configuration Schema

```yaml
project:
  name: string          # Project identifier
  root_path: string     # Root directory to analyze

neo4j:
  uri: string          # Neo4j connection URI
  username: string     # Authentication username
  password: string     # Authentication password
  database: string     # Database name (auto-generated)

analysis:
  ignore_dirs: []string     # Directories to skip
  ignore_files: []string    # Files to skip
  include_tests: bool       # Parse test files
  include_vendor: bool      # Parse vendor directory
  max_concurrency: int      # Parser goroutines
```

### API Contracts

#### Parser Interface

```go
type Parser interface {
    ParseFile(ctx context.Context, filePath string) (*FileInfo, error)
    ParseDirectory(ctx context.Context, dirPath string) ([]*FileInfo, error)
    ParseProject(ctx context.Context, projectPath string, config *ParserConfig) (*core.AnalysisResult, error)
}
```

#### Graph Repository Interface

```go
type Repository interface {
    Connect(ctx context.Context, uri, username, password string) error
    Close() error
    CreateNode(ctx context.Context, node *core.Node) error
    CreateNodes(ctx context.Context, nodes []core.Node) error
    CreateRelationship(ctx context.Context, rel *core.Relationship) error
    CreateRelationships(ctx context.Context, rels []core.Relationship) error
    ClearProject(ctx context.Context, projectID string) error
    ExecuteQuery(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error)
}
```

## Development Guidelines

### Code Standards

- Follow all rules in `.cursor/rules/`
- Use `fmt.Errorf` for internal errors
- Use `core.NewError` for public API boundaries
- Always handle contexts and cancellations
- Structured logging only (no fmt.Printf)
- Test coverage minimum 80% for business logic

### Git Workflow

1. Feature branches from main
2. Conventional commits
3. PR with review required
4. Run `make all` before commit
5. Update technical docs with changes

### Performance Targets

- Parse 10K files in < 30 seconds
- Graph write 100K nodes in < 1 minute
- Query response < 100ms for common queries
- Memory usage < 1GB for large codebases

## Debugging & Troubleshooting

### Common Issues

1. **Neo4j Connection Failed**

   - Check URI format (bolt://host:port)
   - Verify credentials
   - Ensure Neo4j is running

2. **Parser Memory Issues**

   - Reduce max_concurrency
   - Increase ignore patterns
   - Check for circular imports

3. **Slow Analysis**
   - Enable debug logging
   - Check Neo4j performance
   - Profile with pprof

### Debug Commands

```bash
# Enable debug logging
export GOGRAPH_LOG_LEVEL=debug

# Profile CPU usage
go tool pprof cpu.prof

# Check Neo4j metrics
MATCH (n) RETURN count(n) as nodeCount
```

## Future Enhancements

1. **Multi-language Support**

   - TypeScript/JavaScript parser
   - Python parser
   - Java parser

2. **Real-time Updates**

   - File watcher integration
   - Incremental parsing
   - Graph diff updates

3. **Cloud Integration**

   - Neo4j Aura support
   - Multi-tenant architecture
   - REST API server mode

4. **AI Features**
   - Natural language queries
   - Code smell detection
   - Architecture recommendations

## References

- [Neo4j Go Driver Docs](https://neo4j.com/docs/go-manual/current/)
- [Go AST Package](https://pkg.go.dev/go/ast)
- [Cobra CLI Framework](https://cobra.dev/)
- [Viper Configuration](https://github.com/spf13/viper)
