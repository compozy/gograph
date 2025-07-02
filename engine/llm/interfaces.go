package llm

import (
	"context"

	"github.com/compozy/gograph/engine/core"
)

// CypherTranslator translates natural language to Cypher queries
type CypherTranslator interface {
	Translate(ctx context.Context, query string, schema string) (*CypherQuery, error)
	GetSchema(ctx context.Context, projectID string) (string, error)
}

// ContextGenerator generates context for LLMs from graph data
type ContextGenerator interface {
	GenerateContext(ctx context.Context, projectID string, query string) (*Context, error)
	GetProjectSummary(ctx context.Context, projectID string) (*ProjectSummary, error)
}

// CypherQuery represents a translated Cypher query
type CypherQuery struct {
	Query       string         `json:"query"`
	Parameters  map[string]any `json:"parameters"`
	Description string         `json:"description"`
	Confidence  float64        `json:"confidence"`
}

// Context represents context information for LLMs
type Context struct {
	ProjectID        string              `json:"project_id"`
	Summary          *ProjectSummary     `json:"summary"`
	RelevantNodes    []core.Node         `json:"relevant_nodes"`
	Relationships    []core.Relationship `json:"relationships"`
	CodeExamples     []CodeExample       `json:"code_examples"`
	QuerySuggestions []string            `json:"query_suggestions"`
}

// ProjectSummary represents a high-level summary of a project
type ProjectSummary struct {
	Name           string         `json:"name"`
	TotalFiles     int            `json:"total_files"`
	TotalPackages  int            `json:"total_packages"`
	TotalFunctions int            `json:"total_functions"`
	MainPackages   []string       `json:"main_packages"`
	Dependencies   map[string]int `json:"dependencies"`
	Metrics        map[string]any `json:"metrics"`
}

// CodeExample represents a code example with context
type CodeExample struct {
	FilePath    string `json:"file_path"`
	Function    string `json:"function"`
	Code        string `json:"code"`
	Description string `json:"description"`
	LineStart   int    `json:"line_start"`
	LineEnd     int    `json:"line_end"`
}
