# Performance Indexes for GoGraph

## Required Indexes for Optimal Query Performance

The following indexes should be created in Neo4j to optimize query performance, especially for case-insensitive searches using `toLower()`:

### Full-Text Indexes

Create full-text indexes for text search operations:

```cypher
-- Function name and package search
CALL db.index.fulltext.createNodeIndex(
  "functionSearch",
  ["Function"],
  ["name", "package"],
  {analyzer: "standard-no-stop-words"}
);

-- Struct name and package search
CALL db.index.fulltext.createNodeIndex(
  "structSearch",
  ["Struct"],
  ["name", "package"],
  {analyzer: "standard-no-stop-words"}
);

-- Interface name and package search
CALL db.index.fulltext.createNodeIndex(
  "interfaceSearch",
  ["Interface"],
  ["name", "package"],
  {analyzer: "standard-no-stop-words"}
);

-- File path search
CALL db.index.fulltext.createNodeIndex(
  "fileSearch",
  ["File"],
  ["path", "name"],
  {analyzer: "standard-no-stop-words"}
);

-- Package search
CALL db.index.fulltext.createNodeIndex(
  "packageSearch",
  ["Package"],
  ["name", "path"],
  {analyzer: "standard-no-stop-words"}
);
```

### Composite Indexes

Create composite indexes for common query patterns:

```cypher
-- Function lookups by project and name
CREATE INDEX function_project_name FOR (n:Function) ON (n.project_id, n.name);

-- Struct lookups by project and name
CREATE INDEX struct_project_name FOR (n:Struct) ON (n.project_id, n.name);

-- Interface lookups by project and name
CREATE INDEX interface_project_name FOR (n:Interface) ON (n.project_id, n.name);

-- File lookups by project and path
CREATE INDEX file_project_path FOR (n:File) ON (n.project_id, n.path);

-- Package lookups by project and name
CREATE INDEX package_project_name FOR (n:Package) ON (n.project_id, n.name);
```

### Property Indexes

Create property indexes for frequently queried fields:

```cypher
-- Basic property indexes
CREATE INDEX ON :Function(name);
CREATE INDEX ON :Function(package);
CREATE INDEX ON :Function(is_exported);
CREATE INDEX ON :Struct(name);
CREATE INDEX ON :Struct(package);
CREATE INDEX ON :Interface(name);
CREATE INDEX ON :Interface(package);
CREATE INDEX ON :File(path);
CREATE INDEX ON :Package(name);
```

## Query Optimization Tips

### Using Full-Text Indexes

Instead of using `toLower()` for case-insensitive searches, use full-text queries:

```cypher
-- Before (slow)
MATCH (f:Function {project_id: $project_id})
WHERE toLower(f.name) CONTAINS toLower($search_term)
RETURN f

-- After (fast)
CALL db.index.fulltext.queryNodes("functionSearch", $search_term + "*")
YIELD node as f
WHERE f.project_id = $project_id
RETURN f
```

### Monitoring Index Usage

Check if indexes are being used:

```cypher
EXPLAIN MATCH (f:Function {project_id: "test"})
WHERE f.name = "HandleRequest"
RETURN f
```

### Index Maintenance

Regularly update index statistics:

```cypher
CALL db.stats.retrieve("INDEXES");
```

## Implementation Notes

1. These indexes should be created after initial data import for best performance
2. Monitor query performance using Neo4j's query log
3. Consider adding more specialized indexes based on actual query patterns
4. For very large datasets, consider partitioning strategies

## Future Optimizations

1. Implement caching layer for frequently accessed queries
2. Consider using materialized views for complex aggregations
3. Implement query result pagination for large result sets
4. Add connection pooling configuration for concurrent access