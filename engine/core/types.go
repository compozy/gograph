package core

import (
	"time"

	"github.com/google/uuid"
)

// ID represents a unique identifier
type ID string

// NewID generates a new unique ID
func NewID() ID {
	return ID(uuid.New().String())
}

// String returns the string representation of the ID
func (id ID) String() string {
	return string(id)
}

// NodeType represents the type of a node in the graph
type NodeType string

const (
	NodeTypePackage   NodeType = "Package"
	NodeTypeFile      NodeType = "File"
	NodeTypeFunction  NodeType = "Function"
	NodeTypeStruct    NodeType = "Struct"
	NodeTypeInterface NodeType = "Interface"
	NodeTypeMethod    NodeType = "Method"
	NodeTypeConstant  NodeType = "Constant"
	NodeTypeVariable  NodeType = "Variable"
	NodeTypeImport    NodeType = "Import"
)

// RelationType represents the type of relationship between nodes
type RelationType string

const (
	RelationContains   RelationType = "CONTAINS"
	RelationDefines    RelationType = "DEFINES"
	RelationCalls      RelationType = "CALLS"
	RelationImplements RelationType = "IMPLEMENTS"
	RelationEmbeds     RelationType = "EMBEDS"
	RelationImports    RelationType = "IMPORTS"
	RelationBelongsTo  RelationType = "BELONGS_TO"
	RelationReferences RelationType = "REFERENCES"
	RelationDependsOn  RelationType = "DEPENDS_ON"
)

// Node represents a node in the code graph
type Node struct {
	ID         ID             `json:"id"`
	Type       NodeType       `json:"type"`
	Name       string         `json:"name"`
	Path       string         `json:"path,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Relationship represents a relationship between nodes
type Relationship struct {
	ID         ID             `json:"id"`
	Type       RelationType   `json:"type"`
	FromNodeID ID             `json:"from_node_id"`
	ToNodeID   ID             `json:"to_node_id"`
	Properties map[string]any `json:"properties,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Project represents a project configuration
type Project struct {
	ID         ID        `json:"id"`
	Name       string    `json:"name"`
	RootPath   string    `json:"root_path"`
	Neo4jURI   string    `json:"neo4j_uri"`
	Neo4jUser  string    `json:"neo4j_user"`
	ConfigPath string    `json:"config_path"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// AnalysisResult represents the result of analyzing a codebase
type AnalysisResult struct {
	ProjectID      ID             `json:"project_id"`
	Nodes          []Node         `json:"nodes"`
	Relationships  []Relationship `json:"relationships"`
	TotalFiles     int            `json:"total_files"`
	TotalPackages  int            `json:"total_packages"`
	TotalFunctions int            `json:"total_functions"`
	TotalStructs   int            `json:"total_structs"`
	AnalyzedAt     time.Time      `json:"analyzed_at"`
	Duration       time.Duration  `json:"duration"`
}
