# gograph Architecture

This document provides a comprehensive overview of the gograph architecture, design decisions, and implementation patterns.

## 📋 Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Architecture Layers](#architecture-layers)
- [Domain Structure](#domain-structure)
- [Data Flow](#data-flow)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Patterns and Conventions](#patterns-and-conventions)

## 🏗 Overview

gograph follows **Clean Architecture** principles with **Domain-Driven Design** (DDD) to create a maintainable, testable, and extensible codebase analysis tool.

### Core Objectives

- **Separation of Concerns**: Clear boundaries between business logic and infrastructure
- **Dependency Inversion**: Dependencies point inward toward the domain
- **Testability**: Easy to test business logic in isolation
- **Extensibility**: Easy to add new analyzers and output formats
- **Performance**: Efficient parsing and graph operations

## 🎯 Design Principles

### 1. Clean Architecture

```
┌─────────────────────────────────────────┐
│                UI Layer                 │
│            (CLI Commands)               │
├─────────────────────────────────────────┤
│             Application Layer           │
│         (Use Cases & Services)          │
├─────────────────────────────────────────┤
│              Domain Layer               │
│          (Business Logic)               │
├─────────────────────────────────────────┤
│           Infrastructure Layer          │
│        (Database, External APIs)       │
└─────────────────────────────────────────┘
```

### 2. Domain-Driven Design

- **Ubiquitous Language**: Consistent terminology across the codebase
- **Bounded Contexts**: Clear domain boundaries
- **Aggregates**: Consistent data integrity
- **Domain Services**: Complex business logic

### 3. SOLID Principles

- **Single Responsibility**: Each component has one reason to change
- **Open/Closed**: Open for extension, closed for modification
- **Liskov Substitution**: Interfaces are properly implemented
- **Interface Segregation**: Small, focused interfaces
- **Dependency Inversion**: Depend on abstractions, not concretions

## 🏛 Architecture Layers

### 1. Presentation Layer (`cmd/`)

**Responsibility**: User interface and command handling

```go
cmd/gograph/
├── commands/          # Cobra command implementations
│   ├── analyze.go    # Analysis command
│   ├── query.go      # Query command
│   ├── init.go       # Project initialization
│   └── mcp.go        # MCP server command
└── main.go           # Application entry point
```

**Key Patterns**:

- Command pattern with Cobra
- Dependency injection from main
- Minimal business logic

### 2. Application Layer (`engine/`)

**Responsibility**: Use cases and application services

```go
engine/
├── analyzer/         # Code analysis orchestration
├── graph/           # Graph operations
├── parser/          # Go AST parsing
├── query/           # Query building and execution
├── llm/             # LLM integration
└── mcp/             # MCP server implementation
```

**Key Patterns**:

- Service pattern
- Use case pattern
- Repository pattern (interfaces)

### 3. Domain Layer (`engine/core/`)

**Responsibility**: Core business entities and rules

```go
engine/core/
├── types.go         # Domain entities and value objects
└── errors.go        # Domain-specific errors
```

**Key Patterns**:

- Entity pattern
- Value object pattern
- Domain services

### 4. Infrastructure Layer (`engine/infra/`)

**Responsibility**: External concerns and adapters

```go
engine/infra/
├── neo4j_repository.go    # Neo4j database adapter
└── ...                    # Other external adapters
```

**Key Patterns**:

- Adapter pattern
- Repository implementation
- External service integration

## 🔗 Domain Structure

### Core Domain (`engine/core/`)

**Entities**:

```go
type ID string
type Ref struct {
    Type string
    ID   string
}
```

**Error Handling**:

```go
type Error struct {
    Err     error
    Code    string
    Context map[string]any
}
```

### Parser Domain (`engine/parser/`)

**Responsibility**: Go AST parsing and analysis

```go
type Service interface {
    ParseProject(ctx context.Context, config *Config) (*ProjectInfo, error)
    ParseFile(ctx context.Context, filePath string) (*FileInfo, error)
}
```

**Key Components**:

- AST parsing service
- File information extraction
- Import relationship detection

### Graph Domain (`engine/graph/`)

**Responsibility**: Graph operations and Neo4j interaction

```go
type Service interface {
    StoreProject(ctx context.Context, project *ProjectInfo) error
    QueryGraph(ctx context.Context, query string) (*QueryResult, error)
}
```

**Key Components**:

- Graph builder service
- Neo4j repository
- Query execution

### Analyzer Domain (`engine/analyzer/`)

**Responsibility**: High-level analysis orchestration

```go
type Service interface {
    AnalyzeProject(ctx context.Context, config *Config) (*AnalysisResult, error)
}
```

**Key Components**:

- Analysis orchestration
- Progress reporting
- Result aggregation

### Query Domain (`engine/query/`)

**Responsibility**: Query building and execution

```go
type Builder interface {
    BuildQuery(template string, params map[string]any) (string, error)
}

type Exporter interface {
    Export(result *QueryResult, format string) ([]byte, error)
}
```

### LLM Domain (`engine/llm/`)

**Responsibility**: LLM integration and Cypher translation

```go
type CypherTranslator interface {
    TranslateToQuery(ctx context.Context, question string) (string, error)
}
```

### MCP Domain (`engine/mcp/`)

**Responsibility**: Model Context Protocol server

```go
type Server interface {
    Start(ctx context.Context) error
    Stop() error
}
```

## 🔄 Data Flow

### 1. Analysis Flow

```
CLI Command → Analyzer Service → Parser Service → Graph Service → Neo4j
     ↓              ↓                ↓               ↓
Configuration → Project Info → AST Data → Graph Data → Storage
```

### 2. Query Flow

```
CLI Command → Query Service → Graph Service → Neo4j
     ↓             ↓              ↓
Query String → Cypher Query → Graph Data → Results
```

### 3. MCP Flow

```
MCP Client → MCP Server → Analyzer/Query Service → Graph Service → Neo4j
     ↓          ↓              ↓                        ↓
Tool Call → Handler → Business Logic → Graph Data → Response
```

## 🔧 Key Components

### 1. Configuration Management (`pkg/config/`)

```go
type Config struct {
    Project  ProjectConfig  `yaml:"project"`
    Neo4j    Neo4jConfig    `yaml:"neo4j"`
    Analysis AnalysisConfig `yaml:"analysis"`
    MCP      MCPConfig      `yaml:"mcp"`
}
```

**Features**:

- YAML-based configuration
- Environment variable overrides
- Default value handling
- Validation

### 2. Error Handling

**Unified Strategy**:

- Internal: `fmt.Errorf()` for error propagation
- Domain boundaries: `core.NewError()` for structured errors
- Always wrap errors with context

```go
// Internal error propagation
if err := s.validate(input); err != nil {
    return fmt.Errorf("validation failed: %w", err)
}

// Domain boundary error
if err := s.process(input); err != nil {
    return core.NewError(err, "PROCESSING_FAILED", map[string]any{
        "input_type": reflect.TypeOf(input).String(),
    })
}
```

### 3. Dependency Injection

**Constructor Pattern**:

```go
func NewService(repo Repository, config *Config) *Service {
    if config == nil {
        config = DefaultConfig()
    }
    return &Service{
        repo:   repo,
        config: config,
    }
}
```

### 4. Testing Strategy

**Test Types**:

- **Unit Tests**: Business logic with mocks
- **Integration Tests**: Real Neo4j database
- **E2E Tests**: Full CLI workflow

**Test Structure**:

```go
func TestService(t *testing.T) {
    t.Run("Should handle valid input", func(t *testing.T) {
        // Arrange
        service := setupTestService()

        // Act
        result, err := service.Process(context.Background(), validInput)

        // Assert
        assert.NoError(t, err)
        assert.Equal(t, expectedResult, result)
    })
}
```

## 🔌 Integration Points

### 1. Neo4j Database

**Connection Management**:

- Connection pooling
- Health checks
- Graceful shutdown

**Graph Schema**:

- Nodes: Package, File, Function, Struct, Interface, etc.
- Relationships: CONTAINS, IMPORTS, CALLS, IMPLEMENTS, etc.

### 2. MCP Server

**Protocol Implementation**:

- Tool registration
- Resource management
- Request/response handling

**Transport Support**:

- Stdio (current)
- HTTP (planned)

### 3. CLI Interface

**Command Structure**:

- Cobra-based commands
- Flag handling
- Output formatting

## 📐 Patterns and Conventions

### 1. Service Pattern

```go
type Service struct {
    dependency Dependency
    config     *Config
}

func NewService(dependency Dependency, config *Config) *Service {
    return &Service{
        dependency: dependency,
        config:     config,
    }
}

func (s *Service) DoSomething(ctx context.Context, input Input) (Output, error) {
    // Implementation
}
```

### 2. Repository Pattern

```go
type Repository interface {
    Save(ctx context.Context, entity *Entity) error
    FindByID(ctx context.Context, id ID) (*Entity, error)
}

type repositoryImpl struct {
    db Database
}

func NewRepository(db Database) Repository {
    return &repositoryImpl{db: db}
}
```

### 3. Builder Pattern

```go
type QueryBuilder struct {
    query  strings.Builder
    params map[string]any
}

func NewQueryBuilder() *QueryBuilder {
    return &QueryBuilder{
        params: make(map[string]any),
    }
}

func (b *QueryBuilder) Match(pattern string) *QueryBuilder {
    b.query.WriteString("MATCH " + pattern)
    return b
}

func (b *QueryBuilder) Build() string {
    return b.query.String()
}
```

### 4. Factory Pattern

```go
type ServiceFactory struct{}

func (f *ServiceFactory) CreateService(serviceType string) (Service, error) {
    switch serviceType {
    case "parser":
        return NewParserService(), nil
    case "graph":
        return NewGraphService(), nil
    default:
        return nil, fmt.Errorf("unknown service type: %s", serviceType)
    }
}
```

## 🚀 Performance Considerations

### 1. Concurrent Processing

- Parallel file parsing
- Concurrent graph operations
- Worker pools for large projects

### 2. Memory Management

- Streaming for large files
- Batch processing for graph operations
- Resource cleanup with defer

### 3. Caching Strategy

- AST parsing results
- Query results
- Configuration data

## 🔒 Security Considerations

### 1. Input Validation

- Path traversal prevention
- Query injection protection
- Rate limiting

### 2. Access Control

- Project isolation
- MCP authentication
- Resource restrictions

## 📈 Extensibility

### 1. Adding New Analyzers

1. Create new domain package
2. Define service interface
3. Implement service
4. Register with factory

### 2. Adding New Output Formats

1. Implement Exporter interface
2. Register with query service
3. Add CLI flag support

### 3. Adding New Transports

1. Implement transport interface
2. Add to MCP server
3. Update configuration

## 🔮 Future Enhancements

### 1. Planned Features

- Real-time analysis updates
- Multi-language support
- Advanced pattern detection
- Performance metrics

### 2. Architectural Improvements

- Event-driven architecture
- Microservice decomposition
- Distributed graph storage
- Advanced caching layer

---

This architecture documentation serves as a guide for understanding and extending the gograph system. For implementation details, refer to the individual package documentation and code comments.
