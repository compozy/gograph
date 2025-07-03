# GoGraph Query Reference Guide

This document contains all the Cypher queries you can use to analyze your Go codebase with GoGraph. Each query is organized by category with examples and explanations.

## Table of Contents

- [Project Overview Queries](#project-overview-queries)
- [Architecture Analysis](#architecture-analysis)
- [Code Complexity Analysis](#code-complexity-analysis)
- [Dependency Analysis](#dependency-analysis)
- [Function Analysis](#function-analysis)
- [Interface Analysis](#interface-analysis)
- [Test Coverage Analysis](#test-coverage-analysis)
- [Code Quality Queries](#code-quality-queries)
- [Advanced Analysis](#advanced-analysis)

## Project Overview Queries

<details>
<summary><strong>View Entire Project Structure</strong></summary>

```cypher
// View the entire project structure (limit to 500 nodes for performance)
MATCH (n)
WHERE n.project_id = 'my-awesome-project'
RETURN n
LIMIT 500
```

This query returns all nodes in your project, giving you a complete overview of your codebase structure in the Neo4j browser.
</details>

<details>
<summary><strong>Count Nodes by Type</strong></summary>

```cypher
// Get a summary of all node types in your project
MATCH (n)
WHERE n.project_id = 'my-awesome-project'
RETURN labels(n)[0] as NodeType, count(n) as Count
ORDER BY Count DESC
```

This helps you understand the composition of your codebase - how many packages, files, functions, structs, etc.
</details>

<details>
<summary><strong>Show Database Labels and Relationships</strong></summary>

```cypher
// Show all available node labels
CALL db.labels()

// Show all available relationship types
CALL db.relationshipTypes()
```

These queries help you understand what types of nodes and relationships are available in the database.
</details>

## Architecture Analysis

<details>
<summary><strong>Package Overview</strong></summary>

```cypher
// View all packages and their file counts
MATCH (p:Package)-[:CONTAINS]->(f:File)
WHERE p.project_id = 'my-awesome-project'
RETURN p.name as package, count(f) as files
ORDER BY files DESC
```

This query shows you the size of each package in terms of number of files, helping identify large packages that might need refactoring.
</details>

<details>
<summary><strong>Package Dependencies</strong></summary>

```cypher
// Visualize what each package imports
MATCH (p1:Package)-[:CONTAINS]->(f:File)-[:IMPORTS]->(i:Import)
WHERE p1.project_id = 'my-awesome-project'
RETURN DISTINCT p1.name as importer, i.name as imported
ORDER BY importer, imported
```

This shows the import relationships between packages, helping you understand your project's dependency structure.
</details>

<details>
<summary><strong>Package Import Statistics</strong></summary>

```cypher
// Count imports per package
MATCH (p:Package)-[:CONTAINS]->(f:File)-[:IMPORTS]->(i:Import)
WHERE p.project_id = 'my-awesome-project'
RETURN p.name as package, count(DISTINCT i.name) as import_count
ORDER BY import_count DESC
```

Identifies packages with many dependencies, which might indicate high coupling.
</details>

## Code Complexity Analysis

<details>
<summary><strong>Function Complexity Hotspots</strong></summary>

```cypher
// Find the most connected functions (potential complexity hotspots)
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

Functions with many connections (both calling and being called) are often complexity hotspots that might benefit from refactoring.
</details>

<details>
<summary><strong>Most Called Functions</strong></summary>

```cypher
// Find the most frequently called functions
MATCH (f:Function)<-[:CALLS]-(caller)
WHERE f.project_id = 'my-awesome-project'
RETURN f.name, f.package, count(caller) as call_count
ORDER BY call_count DESC
LIMIT 20
```

These are your most reused functions - changes to them can have wide-reaching effects.
</details>

<details>
<summary><strong>Functions with Most Dependencies</strong></summary>

```cypher
// Find functions that call many other functions
MATCH (f:Function)-[:CALLS]->(called)
WHERE f.project_id = 'my-awesome-project'
RETURN f.name, f.package, count(called) as dependencies
ORDER BY dependencies DESC
LIMIT 20
```

Functions with many dependencies might be doing too much and could benefit from decomposition.
</details>

## Dependency Analysis

<details>
<summary><strong>Circular Dependencies Detection</strong></summary>

```cypher
// Find circular dependencies at function level
MATCH path = (f:Function)-[:CALLS*2..]->(f)
WHERE f.project_id = 'my-awesome-project'
RETURN path
LIMIT 10
```

Circular dependencies can make code hard to understand and maintain. This query helps identify them.
</details>

<details>
<summary><strong>Deep Call Chains</strong></summary>

```cypher
// Find deep call chains (more than 5 levels)
MATCH path = (f1:Function)-[:CALLS*5..]->(f2:Function)
WHERE f1.project_id = 'my-awesome-project'
  AND f2.project_id = 'my-awesome-project'
RETURN path
LIMIT 10
```

Deep call chains can indicate complex control flow that might be simplified.
</details>

<details>
<summary><strong>Import Analysis by File</strong></summary>

```cypher
// Show which files import the most packages
MATCH (f:File)-[:IMPORTS]->(i:Import)
WHERE f.project_id = 'my-awesome-project'
RETURN f.path, count(i) as import_count
ORDER BY import_count DESC
LIMIT 20
```

Files with many imports might be doing too much or could benefit from better organization.
</details>

## Function Analysis

<details>
<summary><strong>Find All Functions in a Package</strong></summary>

```cypher
// List all functions in a specific package
MATCH (p:Package {name: "main"})-[:CONTAINS]->(f:File)-[:DEFINES]->(fn:Function)
WHERE p.project_id = 'my-awesome-project'
RETURN fn.name, fn.signature, f.name as file
ORDER BY fn.name
```

This helps you explore what functions are available in a specific package.
</details>

<details>
<summary><strong>Unused Functions</strong></summary>

```cypher
// Find functions that are never called (except main)
MATCH (f:Function)
WHERE f.project_id = 'my-awesome-project'
  AND NOT (f)<-[:CALLS]-()
  AND NOT f.name = 'main'
RETURN f.name, f.package, f.file_path
ORDER BY f.package, f.name
```

Unused functions might be dead code that can be removed to simplify the codebase.
</details>

<details>
<summary><strong>Entry Points</strong></summary>

```cypher
// Find all entry points (functions not called by any other function)
MATCH (f:Function)
WHERE f.project_id = 'my-awesome-project'
  AND NOT (f)<-[:CALLS]-()
  AND f.is_exported = true
RETURN f.name, f.package, f.signature
ORDER BY f.package, f.name
```

These are typically main functions, API handlers, or exported library functions.
</details>

<details>
<summary><strong>Function Call Chains</strong></summary>

```cypher
// Trace call chain from a specific function
MATCH path = (f:Function {name: "HandleRequest"})-[:CALLS*1..5]->(target)
WHERE f.project_id = 'my-awesome-project'
RETURN path
LIMIT 20
```

Understand how a specific function interacts with the rest of your codebase.
</details>

## Interface Analysis

<details>
<summary><strong>Interface Implementations</strong></summary>

```cypher
// Find all interface implementations
MATCH (s:Struct)-[:IMPLEMENTS]->(i:Interface)
WHERE s.project_id = 'my-awesome-project'
RETURN i.name as Interface,
       collect(s.name) as Implementations,
       count(s) as ImplementationCount
ORDER BY ImplementationCount DESC
```

This shows which interfaces are most widely implemented, indicating key abstractions in your design.
</details>

<details>
<summary><strong>Interfaces by Package</strong></summary>

```cypher
// List interfaces grouped by package
MATCH (p:Package)-[:CONTAINS]->(f:File)-[:DEFINES]->(i:Interface)
WHERE p.project_id = 'my-awesome-project'
RETURN p.name as package, collect(i.name) as interfaces
ORDER BY p.name
```

Helps understand how interfaces are distributed across your packages.
</details>

<details>
<summary><strong>Unimplemented Interfaces</strong></summary>

```cypher
// Find interfaces with no implementations
MATCH (i:Interface)
WHERE i.project_id = 'my-awesome-project'
  AND NOT (i)<-[:IMPLEMENTS]-()
RETURN i.name, i.package
ORDER BY i.package, i.name
```

Interfaces without implementations might be dead code or work in progress.
</details>

## Test Coverage Analysis

<details>
<summary><strong>Test Coverage by Package</strong></summary>

```cypher
// Analyze test file ratio by package
MATCH (p:Package)
WHERE p.project_id = 'my-awesome-project'
OPTIONAL MATCH (p)-[:CONTAINS]->(f:File)
WHERE f.name ENDS WITH '_test.go'
WITH p, count(f) as test_files
OPTIONAL MATCH (p)-[:CONTAINS]->(f2:File)
WHERE NOT f2.name ENDS WITH '_test.go'
RETURN p.name, count(f2) as source_files, test_files,
       CASE WHEN count(f2) > 0
            THEN round(100.0 * test_files / count(f2), 2)
            ELSE 0 END as test_ratio
ORDER BY test_ratio DESC
```

This query helps identify packages with low test coverage.
</details>

<details>
<summary><strong>Functions with Tests</strong></summary>

```cypher
// Find functions that have corresponding test functions
MATCH (f:Function)<-[:CALLS]-(test:Function)
WHERE f.project_id = 'my-awesome-project'
  AND test.name STARTS WITH 'Test'
RETURN f.name, f.package, collect(test.name) as test_functions
ORDER BY f.package, f.name
```

Identifies which functions have direct test coverage.
</details>

<details>
<summary><strong>Packages Without Tests</strong></summary>

```cypher
// Find packages with no test files
MATCH (p:Package)
WHERE p.project_id = 'my-awesome-project'
  AND NOT EXISTS {
    MATCH (p)-[:CONTAINS]->(f:File)
    WHERE f.name ENDS WITH '_test.go'
  }
RETURN p.name
ORDER BY p.name
```

Quickly identify packages that lack any test files.
</details>

## Code Quality Queries

<details>
<summary><strong>Large Files</strong></summary>

```cypher
// Find files with many lines of code
MATCH (f:File)
WHERE f.project_id = 'my-awesome-project'
  AND f.lines > 500
RETURN f.path, f.lines
ORDER BY f.lines DESC
```

Large files might benefit from being split into smaller, more focused files.
</details>

<details>
<summary><strong>Large Functions</strong></summary>

```cypher
// Find functions with many lines
MATCH (f:Function)
WHERE f.project_id = 'my-awesome-project'
  AND (f.line_end - f.line_start) > 50
RETURN f.name, f.package, (f.line_end - f.line_start) as lines
ORDER BY lines DESC
LIMIT 20
```

Large functions are often hard to understand and test.
</details>

<details>
<summary><strong>God Objects</strong></summary>

```cypher
// Find structs with many methods (potential god objects)
MATCH (s:Struct)-[:HAS_METHOD]->(m:Method)
WHERE s.project_id = 'my-awesome-project'
RETURN s.name, s.package, count(m) as method_count
ORDER BY method_count DESC
LIMIT 20
```

Structs with too many methods might have too many responsibilities.
</details>

## Advanced Analysis

<details>
<summary><strong>Package Coupling Analysis</strong></summary>

```cypher
// Analyze coupling between packages
MATCH (p1:Package)-[:CONTAINS]->(f1:File)-[:IMPORTS]->(i:Import),
      (p2:Package)-[:CONTAINS]->(f2:File)
WHERE p1.project_id = 'my-awesome-project'
  AND p2.project_id = 'my-awesome-project'
  AND f2.path = i.name
  AND p1.name <> p2.name
RETURN p1.name as from_package, p2.name as to_package, count(*) as import_count
ORDER BY import_count DESC
```

High coupling between packages might indicate they should be merged or better separated.
</details>

<details>
<summary><strong>Find Similar Functions</strong></summary>

```cypher
// Find functions with similar names (potential duplicates)
MATCH (f1:Function), (f2:Function)
WHERE f1.project_id = 'my-awesome-project'
  AND f2.project_id = 'my-awesome-project'
  AND f1.name CONTAINS f2.name
  AND f1 <> f2
  AND length(f1.name) > 5
RETURN f1.name, f1.package, f2.name, f2.package
ORDER BY f1.name
LIMIT 20
```

Functions with similar names might be duplicates or could be consolidated.
</details>

<details>
<summary><strong>External Dependency Analysis</strong></summary>

```cypher
// Find all external (non-stdlib) imports
MATCH (f:File)-[:IMPORTS]->(i:Import)
WHERE f.project_id = 'my-awesome-project'
  AND NOT i.name STARTS WITH 'github.com/compozy/gograph'
  AND i.name CONTAINS '.'
  AND NOT i.name IN ['fmt', 'os', 'io', 'strings', 'errors', 'context', 'time', 'encoding/json']
RETURN DISTINCT i.name as external_import, count(f) as usage_count
ORDER BY usage_count DESC
```

Understand your project's external dependencies and how widely they're used.
</details>

<details>
<summary><strong>Method Receiver Analysis</strong></summary>

```cypher
// Analyze method receivers by type
MATCH (m:Method)
WHERE m.project_id = 'my-awesome-project'
RETURN m.receiver_type as receiver, count(m) as method_count
ORDER BY method_count DESC
```

Helps understand which types have the most methods defined on them.
</details>

## Query Writing Tips

### Performance Tips
- Always include `WHERE n.project_id = 'your-project-id'` to filter by project
- Use `LIMIT` to restrict results for better performance
- Use indexes by querying on indexed properties (name, project_id)

### Common Patterns
- Use `OPTIONAL MATCH` when relationships might not exist
- Use `collect()` to aggregate results
- Use `DISTINCT` to remove duplicates
- Use `count()` instead of `size()` for counting pattern matches

### Debugging Queries
- Start with simple queries and build complexity
- Use `RETURN *` to see all available data
- Check relationship directions with `-->`, `<--`, or `--`