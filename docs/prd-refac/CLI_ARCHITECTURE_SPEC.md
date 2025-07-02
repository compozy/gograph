# CLI Architecture Improvement Specification

**Version**: 1.1  
**Date**: December 2024  
**Status**: Draft - Enhanced with LLM Integration Requirements

## Executive Summary

This specification addresses the critical architectural issues identified in `ANALYSIS.md` by decomposing the monolithic MCP handlers and creating an intuitive CLI interface that abstracts Neo4j complexity from users. **ENHANCED**: Based on comprehensive research into LLM codebase exploration requirements, this specification now includes critical LLM-specific capabilities needed for effective AI-powered code analysis and interaction.

The solution leverages existing capabilities in the `query`, `parser`, `analyzer`, and `graph` packages while providing both a clean, user-friendly command structure AND the advanced semantic search, context retrieval, and progressive analysis capabilities that modern LLM tools require.

## 🎯 Goals

1. **Greenfield Architecture**: Complete redesign with optimal architecture (no backwards compatibility constraints)
2. **Solve Handler Monolith**: Break down the 2,189-line `handlers.go` into focused, domain-specific files
3. **User-Friendly CLI**: Create intuitive subcommands that don't require Neo4j knowledge
4. **Leverage Existing Code**: Utilize the rich functionality already available in other packages
5. **Optimal Design**: Design the best possible architecture without legacy constraints (alpha version)
6. **Follow Architecture Standards**: Adhere to project's clean architecture principles
7. **🆕 LLM-Ready Foundation**: Build semantic search, context retrieval, and progressive analysis capabilities for effective LLM integration
8. **🆕 Multi-Modal Support**: Enable CLI, MCP, IDE integration, and conversational interfaces
9. **🆕 Advanced Pattern Detection**: Implement design pattern recognition, security analysis, and code smell detection

## 🔍 Current State Analysis

### Existing Capabilities (Underutilized)

From my analysis, the project already has extensive capabilities that aren't exposed via CLI:

#### Query Package (`engine/query/`)

- **Builder**: Fluent Cypher query builder with high-level abstractions
- **Templates**: 20+ predefined query templates for common analysis tasks
- **Exporter**: JSON, CSV, TSV export with formatting options
- **High-Level Builder**: Pre-built queries for common patterns

#### Parser Package (`engine/parser/`)

- **AST Analysis**: Deep Go code parsing with function calls, dependencies
- **Concurrent Processing**: Multi-threaded file parsing
- **Comprehensive Extraction**: Functions, structs, interfaces, imports, calls

#### Analyzer Package (`engine/analyzer/`)

- **Dependency Analysis**: Circular dependency detection
- **Interface Detection**: Automatic interface implementation mapping
- **Call Chain Analysis**: Function call relationship mapping
- **Code Metrics**: Complexity analysis and quality metrics

#### Graph Package (`engine/graph/`)

- **Project Statistics**: Comprehensive project metrics
- **Dependency Graphs**: Package and function dependency visualization
- **Call Graphs**: Function call relationship analysis
- **Path Finding**: Shortest path between code elements

### 🆕 LLM Integration Requirements Analysis

Based on comprehensive research into modern LLM codebase exploration tools (GitHub Copilot, Cursor, Aider, Continue.dev), the following capabilities are **CRITICAL** for effective LLM integration:

#### ✅ **Current Strengths for LLM Integration**

1. **Comprehensive Codebase Indexing**: Our analyzer provides excellent structural analysis (dependency graphs, interface implementations, call chains) that LLMs need as foundation
2. **Rich Query Templates**: 20+ templates covering architectural exploration queries that LLMs require
3. **Multi-Level Analysis**: ProjectStatistics, DependencyGraph, CallGraph support hierarchical exploration workflows
4. **Flexible Query Infrastructure**: Both low-level and high-level abstractions support different LLM interaction patterns

#### ❌ **Critical Gaps for LLM Integration**

1. **No Semantic Search/Context Retrieval**: LLMs need multi-stage retrieval with semantic search and relevance ranking. We only have exact/pattern matching.
2. **No Context Ranking/Relevance Scoring**: LLMs need intelligent context selection to manage token limitations. Our queries return raw results without relevance scoring.
3. **No Progressive Context Management**: LLMs need iterative refinement cycles to handle token limitations. We don't support progressive analysis.
4. **Limited Natural Language Interface**: Current natural language query is basic fallback generation, not true NL understanding.
5. **No Advanced Pattern Detection**: Beyond structural analysis, LLMs need design pattern detection, security pattern recognition, and code smell identification.
6. **No Multi-Modal Integration**: LLMs need various interfaces (chat, CLI, IDE integration). We only have CLI/MCP.

### Current MCP Handlers (17 tools)

The monolithic `handlers.go` contains these tools that need to be decomposed:

1. **Project Analysis**: `analyze_project`, `get_project_metadata`
2. **Query Execution**: `execute_cypher`, `natural_language_query`
3. **Function Analysis**: `get_function_info`, `trace_call_chain`, `find_tests_for_code`
4. **Dependency Analysis**: `query_dependencies`, `detect_circular_deps`, `validate_import_path`
5. **Code Navigation**: `get_code_context`, `verify_code_exists`
6. **Pattern Detection**: `detect_code_patterns`, `get_naming_conventions`
7. **Package Analysis**: `list_packages`, `get_package_structure`, `find_implementations`
8. **Testing Analysis**: `check_test_coverage`

## 🏗 Proposed Architecture

### 1. Complete Handler Redesign (Greenfield)

**Alpha Version Approach**: Complete redesign without backwards compatibility constraints.

Replace `engine/mcp/handlers.go` with optimal domain-focused architecture:

```
engine/mcp/
├── handlers/
│   ├── analysis.go          # Project analysis tools (redesigned)
│   ├── dependencies.go      # Dependency analysis tools (redesigned)
│   ├── functions.go         # Function-related tools (redesigned)
│   ├── navigation.go        # Code navigation tools (redesigned)
│   ├── patterns.go          # Pattern detection tools (redesigned)
│   ├── packages.go          # Package analysis tools (redesigned)
│   ├── queries.go           # Query execution tools (redesigned)
│   ├── testing.go           # Test-related tools (redesigned)
│   └── llm.go              # 🆕 LLM-specific tools (semantic search, context retrieval)
├── server.go                # MCP server (redesigned for optimal architecture)
├── service_adapter.go       # Service adapter (redesigned)
└── types.go                 # Types (redesigned with optimal structure)
```

### 2. 🆕 LLM Integration Architecture

#### Core LLM Services

```
engine/llm/
├── semantic/
│   ├── embeddings.go        # Code element vector embeddings
│   ├── search.go           # Semantic search engine
│   └── similarity.go       # Similarity scoring algorithms
├── context/
│   ├── retrieval.go        # Multi-stage context retrieval
│   ├── ranking.go          # Context relevance scoring
│   ├── progressive.go      # Progressive context building
│   └── tokenizer.go        # Token-aware context management
├── patterns/
│   ├── design.go           # Design pattern detection (Factory, Observer, etc.)
│   ├── security.go         # Security pattern analysis
│   ├── smells.go           # Code smell detection
│   └── antipatterns.go     # Anti-pattern identification
├── query/
│   ├── natural.go          # Natural language query understanding
│   ├── intent.go           # Intent classification and parsing
│   └── translator.go       # NL to graph query translation
└── interfaces.go           # LLM service interfaces
```

#### Enhanced MCP Tools for LLM Integration

```
New LLM-specific MCP tools:
├── semantic_search         # Semantic code search with embeddings
├── context_retrieve        # Intelligent context retrieval
├── progressive_analyze     # Progressive analysis with token management
├── pattern_detect          # Advanced pattern detection
├── natural_query           # Natural language query processing
├── context_rank            # Context relevance ranking
├── exploration_guide       # Guided codebase exploration
└── conversation_context    # Conversational context management
```

### 3. CLI Command Structure

Create intuitive subcommands that abstract Neo4j complexity AND provide LLM-ready interfaces:

```
gograph
├── analyze [path]           # Analyze project (existing)
├── query [cypher]          # Execute Cypher (existing)
├── clear [project]         # Clear data (existing)
├── init                    # Initialize config (existing)
├── serve-mcp               # Start MCP server (existing)
├── explore/                # NEW: High-level exploration commands
│   ├── overview            # Project overview and statistics
│   ├── packages            # List and analyze packages
│   ├── functions           # Function analysis and search
│   ├── dependencies        # Dependency analysis
│   ├── interfaces          # Interface implementations
│   ├── patterns            # Code pattern detection
│   ├── calls               # Call chain analysis
│   ├── tests               # Test coverage analysis
│   └── 🆕 semantic        # Semantic code exploration
├── find/                   # NEW: Search and navigation
│   ├── function <name>     # Find functions by name
│   ├── struct <name>       # Find structs by name
│   ├── interface <name>    # Find interfaces by name
│   ├── package <name>      # Find packages by name
│   ├── usage <symbol>      # Find where symbol is used
│   ├── definition <symbol> # Find symbol definition
│   └── 🆕 similar <code>   # Find semantically similar code
├── report/                 # NEW: Generate reports
│   ├── complexity          # Complexity analysis
│   ├── coverage            # Test coverage report
│   ├── dependencies        # Dependency report
│   ├── unused              # Unused code detection
│   ├── metrics             # Code quality metrics
│   ├── 🆕 patterns         # Design pattern analysis report
│   ├── 🆕 security         # Security pattern analysis
│   └── 🆕 smells           # Code smell detection report
├── export/                 # NEW: Export functionality
│   ├── graph               # Export graph data
│   ├── dependencies        # Export dependency data
│   ├── metrics             # Export metrics
│   ├── templates           # Export using templates
│   └── 🆕 embeddings       # Export semantic embeddings
├── 🆕 llm/                 # NEW: LLM-specific commands
│   ├── context <query>     # Get relevant context for LLM
│   ├── search <query>      # Semantic search
│   ├── patterns <type>     # Detect specific patterns
│   ├── explain <element>   # Generate explanations
│   ├── suggest <intent>    # Get suggestions
│   └── conversation        # Interactive conversation mode
└── 🆕 chat                 # NEW: Interactive chat interface
```

### 4. Implementation Strategy

#### Phase 1: Greenfield Handler Redesign (Week 1)

1. **Complete Redesign**: Redesign handlers from scratch with optimal architecture
2. **Remove Legacy Constraints**: Eliminate all backwards compatibility requirements
3. **Optimize Performance**: Design for maximum performance and usability
4. **Modern Patterns**: Implement latest Go patterns and best practices
5. **Comprehensive Testing**: Build test-first with 90%+ coverage

#### Phase 2: Core LLM Infrastructure (Week 2-3)

1. 🆕 Implement semantic search engine
2. 🆕 Build context retrieval system
3. 🆕 Add progressive analysis framework
4. 🆕 Create pattern detection services

#### Phase 3: CLI Framework (Week 3-4)

1. Create shared utilities
2. Implement base command structure
3. Add output formatting
4. Create template integration
5. 🆕 Integrate LLM services

#### Phase 4: Advanced Features (Week 4-5)

1. Implement `explore/` commands
2. Implement `find/` commands
3. Add rich output formatting
4. Create export capabilities
5. 🆕 Implement `llm/` commands
6. 🆕 Add semantic search integration

#### Phase 5: Integration & Polish (Week 5-6)

1. Implement `report/` commands
2. Add advanced export formats
3. Performance optimization
4. Documentation and examples
5. 🆕 Multi-modal interface preparation
6. 🆕 Comprehensive LLM testing

## 📋 Detailed Command Specifications

### 🆕 LLM-Specific Commands

#### `gograph llm context <query>`

**Purpose**: Get relevant context for LLM analysis  
**Uses**: Semantic search, context retrieval, relevance ranking  
**Output**: Ranked, token-aware context for LLM consumption

```bash
# Examples
gograph llm context "How does authentication work?"
gograph llm context "Find all database operations" --max-tokens 4000
gograph llm context "Circular dependencies" --format json
```

#### `gograph llm search <query>`

**Purpose**: Semantic code search using embeddings  
**Uses**: Vector embeddings, similarity search  
**Output**: Semantically similar code elements with relevance scores

```bash
# Examples
gograph llm search "error handling patterns"
gograph llm search "factory pattern implementations"
gograph llm search "authentication middleware" --similarity 0.8
```

#### `gograph explore semantic [--query <text>]`

**Purpose**: Semantic exploration of codebase  
**Uses**: Embeddings, progressive context building  
**Output**: Semantically related code elements and relationships

```bash
# Examples
gograph explore semantic --query "user management"
gograph explore semantic --interactive
gograph explore semantic --focus security
```

#### `gograph report patterns [--type design|security|smells]`

**Purpose**: Advanced pattern detection and analysis  
**Uses**: Pattern recognition algorithms, security analysis  
**Output**: Detected patterns with recommendations and examples

```bash
# Examples
gograph report patterns --type design
gograph report patterns --type security --severity high
gograph report patterns --type smells --package mypackage
```

### Enhanced Existing Commands

#### `gograph find similar <code-snippet>`

**Purpose**: Find semantically similar code using embeddings  
**Uses**: Vector similarity search  
**Output**: Similar code with similarity scores

#### `gograph export embeddings [--format json|binary]`

**Purpose**: Export semantic embeddings for external use  
**Uses**: Embedding generation and serialization  
**Output**: Vector embeddings in specified format

### Explore Commands

#### `gograph explore overview [project-id]`

**Purpose**: Get a high-level overview of the project  
**Uses**: `graph.GetProjectStatistics()`, query templates  
**Output**: Summary statistics, top packages, complexity metrics

```bash
# Examples
gograph explore overview
gograph explore overview my-project --format json
gograph explore overview --export overview.json
```

#### `gograph explore packages [--pattern <glob>]`

**Purpose**: List and analyze packages  
**Uses**: Query templates, `query.Builder`  
**Output**: Package list with dependencies, file counts, complexity

```bash
# Examples
gograph explore packages
gograph explore packages --pattern "internal/*"
gograph explore packages --sort complexity --limit 10
```

#### `gograph explore functions [--package <name>] [--complexity <min>]`

**Purpose**: Analyze functions across the project  
**Uses**: Query templates, `analyzer` metrics  
**Output**: Function list with complexity, call counts, signatures

```bash
# Examples
gograph explore functions --complexity 20
gograph explore functions --package main --exported-only
gograph explore functions --most-called --limit 10
```

#### `gograph explore dependencies [--direction in|out|both] [--recursive]`

**Purpose**: Analyze dependency relationships  
**Uses**: `graph.GetDependencyGraph()`, query builder  
**Output**: Dependency tree, circular dependencies, external deps

```bash
# Examples
gograph explore dependencies --direction out --recursive
gograph explore dependencies --circular-only
gograph explore dependencies --external --format csv
```

#### `gograph explore interfaces [--unimplemented]`

**Purpose**: Analyze interface implementations  
**Uses**: `analyzer.DetectInterfaceImplementations()`, query templates  
**Output**: Interface-implementation mappings, unused interfaces

```bash
# Examples
gograph explore interfaces
gograph explore interfaces --unimplemented
gograph explore interfaces --package mypackage
```

#### `gograph explore patterns [--type factory|singleton|observer]`

**Purpose**: Detect code patterns and anti-patterns  
**Uses**: Enhanced pattern detection from LLM services  
**Output**: Pattern instances, recommendations, code smells

```bash
# Examples
gograph explore patterns
gograph explore patterns --type factory
gograph explore patterns --anti-patterns
```

#### `gograph explore calls <function> [--depth <n>]`

**Purpose**: Analyze function call chains  
**Uses**: `graph.GetCallGraph()`, query builder  
**Output**: Call graph, recursive calls, call depth analysis

```bash
# Examples
gograph explore calls main.main --depth 5
gograph explore calls --recursive-only
gograph explore calls myFunc --callers
```

#### `gograph explore tests [--coverage] [--missing]`

**Purpose**: Analyze test coverage and patterns  
**Uses**: Existing test analysis from MCP handlers  
**Output**: Coverage reports, missing tests, test patterns

```bash
# Examples
gograph explore tests --coverage
gograph explore tests --missing --package mypackage
gograph explore tests --patterns
```

### Find Commands

#### `gograph find function <name> [--package <pkg>]`

**Purpose**: Search for functions by name (fuzzy matching)  
**Uses**: Query templates with CONTAINS matching  
**Output**: Matching functions with signatures and locations

```bash
# Examples
gograph find function Handle
gograph find function "New*" --package service
gograph find function main --exact
```

#### `gograph find struct <name>`

**Purpose**: Search for struct definitions  
**Uses**: Query templates  
**Output**: Matching structs with fields and methods

#### `gograph find interface <name>`

**Purpose**: Search for interface definitions  
**Uses**: Query templates  
**Output**: Matching interfaces with methods and implementations

#### `gograph find package <name>`

**Purpose**: Search for packages  
**Uses**: Query templates  
**Output**: Matching packages with file counts and dependencies

#### `gograph find usage <symbol>`

**Purpose**: Find where a symbol is used  
**Uses**: Query builder with relationship traversal  
**Output**: Usage locations with context

#### `gograph find definition <symbol>`

**Purpose**: Find symbol definition  
**Uses**: Query templates  
**Output**: Definition location with signature

### Report Commands

#### `gograph report complexity [--threshold <n>] [--package <pkg>]`

**Purpose**: Generate complexity analysis report  
**Uses**: `analyzer` metrics, query templates  
**Output**: Complexity rankings, hotspots, recommendations

```bash
# Examples
gograph report complexity --threshold 15
gograph report complexity --package main --format json
gograph report complexity --export complexity-report.html
```

#### `gograph report coverage [--package <pkg>]`

**Purpose**: Generate test coverage report  
**Uses**: Test analysis capabilities  
**Output**: Coverage percentages, missing tests, recommendations

#### `gograph report dependencies [--circular] [--external]`

**Purpose**: Generate dependency analysis report  
**Uses**: `analyzer.DetectCircularDependencies()`, dependency graph  
**Output**: Dependency tree, circular deps, external dependencies

#### `gograph report unused [--functions] [--structs] [--interfaces]`

**Purpose**: Detect unused code  
**Uses**: Query templates for unused detection  
**Output**: Unused code items with recommendations

#### `gograph report metrics [--detailed]`

**Purpose**: Generate comprehensive code quality metrics  
**Uses**: `analyzer` metrics, project statistics  
**Output**: Quality scores, trends, recommendations

### Export Commands

#### `gograph export graph [--format json|csv|graphml]`

**Purpose**: Export complete graph data  
**Uses**: `query.Exporter`, graph service  
**Output**: Graph data in specified format

#### `gograph export dependencies [--format json|csv|dot]`

**Purpose**: Export dependency data  
**Uses**: Dependency analysis, exporter  
**Output**: Dependency data for external tools

#### `gograph export metrics [--format json|csv]`

**Purpose**: Export metrics data  
**Uses**: Project statistics, exporter  
**Output**: Metrics data for analysis tools

#### `gograph export templates <template-name>`

**Purpose**: Export using predefined query templates  
**Uses**: `query.CommonTemplates`, exporter  
**Output**: Template-based exports

```bash
# Examples
gograph export templates function_complexity --format csv
gograph export templates interface_implementations --format json
```

## 🔧 Implementation Details

### 1. 🆕 LLM Service Integration

Create `engine/llm/` package for LLM-specific functionality:

```go
// engine/llm/interfaces.go
type SemanticSearchService interface {
    GenerateEmbeddings(ctx context.Context, elements []CodeElement) error
    SearchSimilar(ctx context.Context, query string, limit int) ([]SimilarityResult, error)
    FindRelated(ctx context.Context, elementID core.ID, threshold float64) ([]CodeElement, error)
}

type ContextRetrievalService interface {
    GetRelevantContext(ctx context.Context, query string, maxTokens int) (*ContextResult, error)
    RankContext(ctx context.Context, contexts []Context, query string) ([]RankedContext, error)
    BuildProgressiveContext(ctx context.Context, query string, iterations int) (*ProgressiveContext, error)
}

type PatternDetectionService interface {
    DetectDesignPatterns(ctx context.Context, projectID core.ID) ([]DesignPattern, error)
    AnalyzeSecurity(ctx context.Context, projectID core.ID) ([]SecurityIssue, error)
    FindCodeSmells(ctx context.Context, projectID core.ID) ([]CodeSmell, error)
}
```

### 2. Shared CLI Utilities

Create `cmd/gograph/internal/` package for shared functionality:

```go
// cmd/gograph/internal/client.go
type GraphClient struct {
    serviceAdapter   mcp.ServiceAdapter
    queryBuilder     *query.HighLevelBuilder
    exporter         *query.Exporter
    semanticSearch   llm.SemanticSearchService     // 🆕
    contextRetrieval llm.ContextRetrievalService   // 🆕
    patternDetection llm.PatternDetectionService   // 🆕
}

func NewGraphClient(config *Config) (*GraphClient, error) {
    // Initialize services and return client
}

func (c *GraphClient) ExecuteTemplate(templateName string, params map[string]any) ([]map[string]any, error) {
    // Execute predefined template
}

func (c *GraphClient) SemanticSearch(query string, options SearchOptions) (*SemanticResult, error) {
    // 🆕 Semantic search functionality
}

func (c *GraphClient) GetLLMContext(query string, maxTokens int) (*ContextResult, error) {
    // 🆕 LLM context retrieval
}
```

### 3. Command Structure

```go
// cmd/gograph/commands/llm/context.go
func NewContextCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "context <query>",
        Short: "Get relevant context for LLM analysis",
        RunE:  runContext,
    }

    cmd.Flags().Int("max-tokens", 4000, "Maximum tokens for context")
    cmd.Flags().String("format", "json", "Output format: json, text")
    cmd.Flags().Float64("relevance", 0.7, "Minimum relevance score")

    return cmd
}

func runContext(cmd *cobra.Command, args []string) error {
    client, err := internal.NewGraphClient(getConfig())
    if err != nil {
        return err
    }

    maxTokens, _ := cmd.Flags().GetInt("max-tokens")
    context, err := client.GetLLMContext(args[0], maxTokens)
    if err != nil {
        return err
    }

    return formatAndDisplay(context, getOutputOptions(cmd))
}
```

### 4. Template Integration

Leverage existing query templates with LLM enhancements:

```go
// cmd/gograph/internal/templates.go
func (c *GraphClient) ListFunctions(projectID string, options FunctionListOptions) ([]map[string]any, error) {
    template, err := query.GetTemplate("functions_by_package")
    if err != nil {
        return nil, err
    }

    params := map[string]any{
        "project_id": projectID,
    }

    results, err := c.serviceAdapter.ExecuteQuery(ctx, template.Query, params)
    if err != nil {
        return nil, err
    }

    // 🆕 Enhance with semantic information if available
    if c.semanticSearch != nil {
        results = c.enhanceWithSemanticData(results)
    }

    return results, nil
}
```

### 5. Output Formatting

Create consistent output formatting with LLM-friendly options:

```go
// cmd/gograph/internal/formatter.go
type OutputFormatter struct {
    format string
    writer io.Writer
}

func (f *OutputFormatter) FormatProjectStats(stats *graph.ProjectStatistics) error {
    switch f.format {
    case "table":
        return f.formatAsTable(stats)
    case "json":
        return f.formatAsJSON(stats)
    case "csv":
        return f.formatAsCSV(stats)
    case "llm":           // 🆕 LLM-optimized format
        return f.formatForLLM(stats)
    }
}

func (f *OutputFormatter) FormatSemanticResults(results *SemanticResult) error {
    // 🆕 Format semantic search results with relevance scores
}
```

## 🧪 Testing Strategy

### 1. Handler Tests

- Unit tests for each decomposed handler file
- Integration tests for MCP compatibility
- Performance tests for large codebases
- 🆕 LLM service integration tests

### 2. CLI Tests

- Command execution tests
- Output format validation
- Error handling verification
- Integration tests with real projects
- 🆕 Semantic search accuracy tests

### 3. Template Tests

- Query template validation
- Parameter substitution tests
- Result format verification
- 🆕 LLM context quality tests

### 4. 🆕 LLM Integration Tests

- Embedding generation and similarity tests
- Context retrieval accuracy tests
- Pattern detection validation
- Natural language query translation tests
- Performance benchmarks for large codebases

## 📊 Success Metrics

### Architecture Quality

- ✅ No single file exceeds 500 lines (except main entry points)
- ✅ Each handler file has single responsibility
- ✅ All handlers have comprehensive test coverage
- ✅ MCP API compatibility maintained
- 🆕 ✅ LLM services achieve >80% context relevance accuracy
- 🆕 ✅ Semantic search provides >90% precision for common queries

### User Experience

- ✅ Users can analyze projects without Neo4j knowledge
- ✅ Common analysis tasks require single commands
- ✅ Rich output formatting available
- ✅ Export capabilities for external tools
- 🆕 ✅ LLMs can effectively explore codebases using provided context
- 🆕 ✅ Natural language queries work for 90% of common use cases

### Performance

- ✅ Command response times under 5 seconds for typical projects
- ✅ Memory usage optimized for large codebases
- ✅ Concurrent processing utilized effectively
- 🆕 ✅ Semantic search responds within 2 seconds for typical queries
- 🆕 ✅ Context retrieval handles token limitations efficiently

### 🆕 LLM Integration Metrics

- ✅ Context relevance score >0.8 for architectural queries
- ✅ Pattern detection accuracy >85% on known patterns
- ✅ Natural language query success rate >90%
- ✅ Token utilization efficiency >75%
- ✅ Multi-modal interface compatibility

## 🚀 Migration Plan

### Phase 1: Foundation (Week 1)

1. Create handler package structure
2. Decompose existing handlers
3. Add comprehensive tests
4. Ensure MCP compatibility

### Phase 2: LLM Infrastructure (Week 2-3)

1. 🆕 Implement semantic search engine
2. 🆕 Build context retrieval system
3. 🆕 Add progressive analysis framework
4. 🆕 Create pattern detection services

### Phase 3: CLI Framework (Week 3-4)

1. Create shared utilities
2. Implement base command structure
3. Add output formatting
4. Create template integration
5. 🆕 Integrate LLM services

### Phase 4: Advanced Features (Week 4-5)

1. Implement `explore/` commands
2. Implement `find/` commands
3. Add rich output formatting
4. Create export capabilities
5. 🆕 Implement `llm/` commands
6. 🆕 Add semantic search integration

### Phase 5: Integration & Polish (Week 5-6)

1. Implement `report/` commands
2. Add advanced export formats
3. Performance optimization
4. Documentation and examples
5. 🆕 Multi-modal interface preparation
6. 🆕 Comprehensive LLM testing

## 📚 Documentation Plan

### User Documentation

- Command reference with examples
- Common workflow guides
- Output format specifications
- Integration examples
- 🆕 LLM integration guide
- 🆕 Semantic search tutorial
- 🆕 Pattern detection reference

### Developer Documentation

- Handler decomposition guide
- Template creation guide
- Extension patterns
- Performance tuning
- 🆕 LLM service development guide
- 🆕 Embedding generation process
- 🆕 Context retrieval algorithms

### 🆕 LLM Integration Documentation

- Semantic search API reference
- Context retrieval best practices
- Pattern detection customization
- Natural language query examples
- Multi-modal integration guide
- Performance optimization for LLM workloads

## 🔗 Dependencies

### External Dependencies

- No new external dependencies required for CLI redesign
- Leverage existing Neo4j, Cobra, and other dependencies
- 🆕 **New LLM Dependencies**:
  - Vector embedding library (e.g., `sentence-transformers` via Python bridge or Go native)
  - Similarity search engine (e.g., `faiss` or Neo4j vector index)
  - Natural language processing library
  - Token counting utilities

### Internal Dependencies

- Utilize existing `query`, `parser`, `analyzer`, `graph` packages
- Maintain compatibility with existing MCP server
- Follow established architecture patterns
- 🆕 New `engine/llm/` package with semantic services
- 🆕 Enhanced MCP handlers with LLM integration
- 🆕 Extended CLI commands with LLM capabilities

## 🎯 LLM Integration Priority Matrix

| Capability              | Implementation Effort | LLM Impact | Priority |
| ----------------------- | --------------------- | ---------- | -------- |
| Semantic Search         | High                  | Critical   | **P0**   |
| Context Retrieval       | Medium                | Critical   | **P0**   |
| Progressive Context     | Medium                | High       | **P1**   |
| Pattern Detection       | High                  | High       | **P1**   |
| Natural Language Query  | High                  | Medium     | **P2**   |
| Multi-Modal Integration | High                  | Medium     | **P2**   |

---

**Next Steps**: Begin Phase 1 implementation with handler decomposition, followed immediately by Phase 2 LLM infrastructure development to establish the foundation for effective LLM integration.
