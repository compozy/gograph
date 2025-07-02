package query

import (
	"fmt"
	"strings"

	"github.com/compozy/gograph/engine/core"
)

// Builder provides a fluent interface for building Cypher queries
type Builder struct {
	query      strings.Builder
	parameters map[string]any
	errors     []error
}

// NewBuilder creates a new query builder
func NewBuilder() *Builder {
	return &Builder{
		parameters: make(map[string]any),
		errors:     make([]error, 0),
	}
}

// Match adds a MATCH clause to the query
func (b *Builder) Match(pattern string) *Builder {
	if b.query.Len() > 0 {
		b.query.WriteString(" ")
	}
	b.query.WriteString("MATCH ")
	b.query.WriteString(pattern)
	return b
}

// OptionalMatch adds an OPTIONAL MATCH clause
func (b *Builder) OptionalMatch(pattern string) *Builder {
	if b.query.Len() > 0 {
		b.query.WriteString(" ")
	}
	b.query.WriteString("OPTIONAL MATCH ")
	b.query.WriteString(pattern)
	return b
}

// Where adds a WHERE clause
func (b *Builder) Where(condition string) *Builder {
	b.query.WriteString(" WHERE ")
	b.query.WriteString(condition)
	return b
}

// And adds an AND condition to the WHERE clause
func (b *Builder) And(condition string) *Builder {
	b.query.WriteString(" AND ")
	b.query.WriteString(condition)
	return b
}

// Or adds an OR condition to the WHERE clause
func (b *Builder) Or(condition string) *Builder {
	b.query.WriteString(" OR ")
	b.query.WriteString(condition)
	return b
}

// Return adds a RETURN clause
func (b *Builder) Return(fields string) *Builder {
	b.query.WriteString(" RETURN ")
	b.query.WriteString(fields)
	return b
}

// OrderBy adds an ORDER BY clause
func (b *Builder) OrderBy(fields string) *Builder {
	b.query.WriteString(" ORDER BY ")
	b.query.WriteString(fields)
	return b
}

// Limit adds a LIMIT clause
func (b *Builder) Limit(count int) *Builder {
	b.query.WriteString(fmt.Sprintf(" LIMIT %d", count))
	return b
}

// Skip adds a SKIP clause
func (b *Builder) Skip(count int) *Builder {
	b.query.WriteString(fmt.Sprintf(" SKIP %d", count))
	return b
}

// With adds a WITH clause
func (b *Builder) With(fields string) *Builder {
	b.query.WriteString(" WITH ")
	b.query.WriteString(fields)
	return b
}

// Create adds a CREATE clause
func (b *Builder) Create(pattern string) *Builder {
	if b.query.Len() > 0 {
		b.query.WriteString(" ")
	}
	b.query.WriteString("CREATE ")
	b.query.WriteString(pattern)
	return b
}

// Merge adds a MERGE clause
func (b *Builder) Merge(pattern string) *Builder {
	if b.query.Len() > 0 {
		b.query.WriteString(" ")
	}
	b.query.WriteString("MERGE ")
	b.query.WriteString(pattern)
	return b
}

// Set adds a SET clause
func (b *Builder) Set(assignments string) *Builder {
	b.query.WriteString(" SET ")
	b.query.WriteString(assignments)
	return b
}

// Delete adds a DELETE clause
func (b *Builder) Delete(nodes string) *Builder {
	b.query.WriteString(" DELETE ")
	b.query.WriteString(nodes)
	return b
}

// DetachDelete adds a DETACH DELETE clause
func (b *Builder) DetachDelete(nodes string) *Builder {
	b.query.WriteString(" DETACH DELETE ")
	b.query.WriteString(nodes)
	return b
}

// SetParameter adds a parameter to the query
func (b *Builder) SetParameter(name string, value any) *Builder {
	b.parameters[name] = value
	return b
}

// SetParameters adds multiple parameters to the query
func (b *Builder) SetParameters(params map[string]any) *Builder {
	for name, value := range params {
		b.parameters[name] = value
	}
	return b
}

// ProjectFilter adds a project ID filter condition
func (b *Builder) ProjectFilter(projectID core.ID) *Builder {
	b.SetParameter("project_id", string(projectID))
	return b
}

// Build returns the final query and parameters
func (b *Builder) Build() (string, map[string]any, error) {
	if len(b.errors) > 0 {
		return "", nil, fmt.Errorf("query build errors: %v", b.errors)
	}
	return strings.TrimSpace(b.query.String()), b.parameters, nil
}

// String returns the query string
func (b *Builder) String() string {
	return strings.TrimSpace(b.query.String())
}

// HighLevelBuilder provides high-level query building methods for common patterns
type HighLevelBuilder struct{}

// NewHighLevelBuilder creates a new high-level query builder
func NewHighLevelBuilder() *HighLevelBuilder {
	return &HighLevelBuilder{}
}

// FindNodesByType creates a query to find nodes by type and project
func (hlb *HighLevelBuilder) FindNodesByType(nodeType core.NodeType, projectID core.ID) *Builder {
	return NewBuilder().
		Match(fmt.Sprintf("(n:%s)", nodeType)).
		Where("n.project_id = $project_id").
		ProjectFilter(projectID).
		Return("n").
		OrderBy("n.name")
}

// FindRelationshipsByType creates a query to find relationships by type
func (hlb *HighLevelBuilder) FindRelationshipsByType(relType core.RelationType, projectID core.ID) *Builder {
	return NewBuilder().
		Match(fmt.Sprintf("(a)-[r:%s]->(b)", relType)).
		Where("r.project_id = $project_id").
		ProjectFilter(projectID).
		Return("a, r, b")
}

// FindNodesByName creates a query to find nodes by name pattern
func (hlb *HighLevelBuilder) FindNodesByName(namePattern string, projectID core.ID) *Builder {
	return NewBuilder().
		Match("(n)").
		Where("n.project_id = $project_id").
		And("toLower(n.name) CONTAINS toLower($name_pattern)").
		ProjectFilter(projectID).
		SetParameter("name_pattern", namePattern).
		Return("n").
		OrderBy("labels(n)[0], n.name")
}

// FindDependencies creates a query to find dependencies for a specific node
func (hlb *HighLevelBuilder) FindDependencies(nodeID core.ID, projectID core.ID) *Builder {
	return NewBuilder().
		Match("(n)-[:DEPENDS_ON*1..3]->(dep)").
		Where("n.id = $node_id").
		And("n.project_id = $project_id").
		ProjectFilter(projectID).
		SetParameter("node_id", string(nodeID)).
		Return("dep").
		OrderBy("dep.name")
}

// FindDependents creates a query to find what depends on a specific node
func (hlb *HighLevelBuilder) FindDependents(nodeID core.ID, projectID core.ID) *Builder {
	return NewBuilder().
		Match("(dependent)-[:DEPENDS_ON*1..3]->(n)").
		Where("n.id = $node_id").
		And("n.project_id = $project_id").
		ProjectFilter(projectID).
		SetParameter("node_id", string(nodeID)).
		Return("dependent").
		OrderBy("dependent.name")
}

// FindPath creates a query to find the shortest path between two nodes
func (hlb *HighLevelBuilder) FindPath(fromID, toID core.ID, projectID core.ID) *Builder {
	return NewBuilder().
		Match("(from), (to)").
		Where("from.id = $from_id").
		And("to.id = $to_id").
		And("from.project_id = $project_id").
		And("to.project_id = $project_id").
		With("from, to").
		Match("path = shortestPath((from)-[*]-(to))").
		ProjectFilter(projectID).
		SetParameter("from_id", string(fromID)).
		SetParameter("to_id", string(toID)).
		Return("path")
}

// CountNodesByType creates a query to count nodes by type
func (hlb *HighLevelBuilder) CountNodesByType(projectID core.ID) *Builder {
	return NewBuilder().
		Match("(n)").
		Where("n.project_id = $project_id").
		ProjectFilter(projectID).
		Return("labels(n)[0] as node_type, count(n) as count").
		OrderBy("count DESC")
}

// CountRelationshipsByType creates a query to count relationships by type
func (hlb *HighLevelBuilder) CountRelationshipsByType(projectID core.ID) *Builder {
	return NewBuilder().
		Match("()-[r]->()").
		Where("r.project_id = $project_id").
		ProjectFilter(projectID).
		Return("type(r) as relationship_type, count(r) as count").
		OrderBy("count DESC")
}

// FindComplexFunctions creates a query to find functions with high complexity
func (hlb *HighLevelBuilder) FindComplexFunctions(projectID core.ID, minComplexity int) *Builder {
	return NewBuilder().
		Match("(f:Function)").
		Where("f.project_id = $project_id").
		With("f, (f.line_end - f.line_start) as complexity").
		Where("complexity >= $min_complexity").
		ProjectFilter(projectID).
		SetParameter("min_complexity", minComplexity).
		Return("f.package as package, f.name as function, complexity, f.signature as signature").
		OrderBy("complexity DESC")
}

// FindUnusedFunctions creates a query to find potentially unused functions
func (hlb *HighLevelBuilder) FindUnusedFunctions(projectID core.ID) *Builder {
	return NewBuilder().
		Match("(f:Function)").
		Where("f.project_id = $project_id").
		And("NOT EXISTS { MATCH ()-[:CALLS]->(f) }").
		And("f.name <> 'main'").
		And("f.name <> 'init'").
		And("NOT f.name STARTS WITH 'Test'").
		ProjectFilter(projectID).
		Return("f.package as package, f.name as function, f.signature as signature").
		OrderBy("f.package, f.name")
}

// FindCircularDependencies creates a query to detect circular dependencies
func (hlb *HighLevelBuilder) FindCircularDependencies(projectID core.ID) *Builder {
	return NewBuilder().
		Match("(a:File)-[:DEPENDS_ON*2..10]->(a)").
		Where("a.project_id = $project_id").
		ProjectFilter(projectID).
		Return("DISTINCT a.path as file_path").
		OrderBy("file_path")
}

// FindInterfaceImplementations creates a query to find interface implementations
func (hlb *HighLevelBuilder) FindInterfaceImplementations(projectID core.ID) *Builder {
	return NewBuilder().
		Match("(s:Struct)-[:IMPLEMENTS]->(i:Interface)").
		Where("s.project_id = $project_id").
		ProjectFilter(projectID).
		Return("i.package as interface_package, i.name as interface_name, " +
			"s.package as struct_package, s.name as struct_name").
		OrderBy("i.name, s.name")
}

// FindMostCalledFunctions creates a query to find the most called functions
func (hlb *HighLevelBuilder) FindMostCalledFunctions(projectID core.ID, limit int) *Builder {
	return NewBuilder().
		Match("(f:Function)<-[:CALLS]-()").
		Where("f.project_id = $project_id").
		ProjectFilter(projectID).
		Return("f.package as package, f.name as function, count(*) as call_count, f.signature as signature").
		OrderBy("call_count DESC").
		Limit(limit)
}
