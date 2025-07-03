package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/sashabaranov/go-openai"
)

// OpenAICypherTranslator implements CypherTranslator using OpenAI
type OpenAICypherTranslator struct {
	client       *openai.Client
	graphService graph.Service
	model        string
}

// CypherTranslatorConfig holds configuration for the translator
type CypherTranslatorConfig struct {
	APIKey string
	Model  string
}

// NewOpenAICypherTranslator creates a new OpenAI-based Cypher translator
func NewOpenAICypherTranslator(config CypherTranslatorConfig, graphService graph.Service) *OpenAICypherTranslator {
	model := config.Model
	if model == "" {
		model = openai.GPT4Dot120250414 // Use gpt-4.1-2025-04-14 as default for better Cypher generation
	}
	return &OpenAICypherTranslator{
		client:       openai.NewClient(config.APIKey),
		graphService: graphService,
		model:        model,
	}
}

// Translate converts natural language to Cypher query
func (t *OpenAICypherTranslator) Translate(ctx context.Context, query string, schema string) (*CypherQuery, error) {
	if query == "" {
		return nil, core.NewError(fmt.Errorf("query cannot be empty"), "EMPTY_QUERY", nil)
	}
	prompt := t.buildTranslationPrompt(query, schema)
	resp, err := t.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: t.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: t.getSystemPrompt(),
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.1, // Low temperature for consistent results
		MaxTokens:   1000,
	})
	if err != nil {
		return nil, core.NewError(fmt.Errorf("failed to call OpenAI API: %w", err), "OPENAI_API_ERROR", map[string]any{
			"query": query,
		})
	}
	if len(resp.Choices) == 0 {
		return nil, core.NewError(fmt.Errorf("no response from OpenAI"), "NO_OPENAI_RESPONSE", nil)
	}
	content := resp.Choices[0].Message.Content
	cypherQuery, confidence := t.parseCypherResponse(content)
	return &CypherQuery{
		Query:       cypherQuery,
		Parameters:  make(map[string]any),
		Description: t.generateDescription(query, cypherQuery),
		Confidence:  confidence,
	}, nil
}

// GetSchema retrieves the graph schema for a project
func (t *OpenAICypherTranslator) GetSchema(ctx context.Context, projectID string) (string, error) {
	if projectID == "" {
		return "", core.NewError(fmt.Errorf("project ID cannot be empty"), "EMPTY_PROJECT_ID", nil)
	}
	// Get schema information from the graph service
	stats, err := t.graphService.GetProjectStatistics(ctx, core.ID(projectID))
	if err != nil {
		return "", core.NewError(
			fmt.Errorf("failed to get project statistics: %w", err),
			"GRAPH_SERVICE_ERROR",
			map[string]any{
				"project_id": projectID,
			},
		)
	}
	schema := t.buildSchemaDescription(stats)
	return schema, nil
}

// getSystemPrompt returns the system prompt for Cypher translation
func (t *OpenAICypherTranslator) getSystemPrompt() string {
	return `You are a expert Neo4j Cypher query generator for Go code analysis. ` +
		`Your task is to convert natural language questions into precise Cypher queries.

IMPORTANT RULES:
1. Always return ONLY the Cypher query, no explanations or markdown formatting
2. Use the exact node labels and relationship types provided in the schema
3. For case-sensitive searches, use exact matching
4. For case-insensitive searches, use toLower() function
5. Always include LIMIT clauses to prevent excessive results (default: 50)
6. Use parameterized queries when possible
7. Focus on performance and accuracy
8. Functions have a 'package' property - use it directly instead of traversing relationships

WORKING QUERY EXAMPLES:
Query: "Show me all handler functions in the mcp package"
Cypher: MATCH (f:Function) WHERE f.package = 'mcp' AND toLower(f.name) CONTAINS 'handle' RETURN f LIMIT 50

Query: "Find functions that handle MCP requests"
Cypher: MATCH (f:Function) WHERE toLower(f.name) CONTAINS 'mcp' RETURN f LIMIT 50

Query: "List all functions in package mcp"
Cypher: MATCH (f:Function) WHERE f.package = 'mcp' RETURN f LIMIT 50

Query: "Show functions with names starting with handle"
Cypher: MATCH (f:Function) WHERE f.name STARTS WITH 'handle' RETURN f LIMIT 50

Query: "Find all structs in the analyzer package"
Cypher: MATCH (s:Struct) WHERE s.package = 'analyzer' RETURN s LIMIT 50

Query: "Show interfaces and their implementations"
Cypher: MATCH (s:Struct)-[:IMPLEMENTS]->(i:Interface) RETURN s.name as struct, i.name as interface LIMIT 50

Query: "Find functions that call a specific function"
Cypher: MATCH (f1:Function)-[:CALLS]->(f2:Function {name: 'Debug'}) RETURN f1.name, f1.package LIMIT 50

Query: "Show all packages and their file count"
Cypher: MATCH (p:Package)-[:CONTAINS]->(f:File) RETURN p.name, count(f) as file_count ORDER BY file_count DESC

Return only the Cypher query without any formatting or explanations.`
}

// buildTranslationPrompt creates the translation prompt
func (t *OpenAICypherTranslator) buildTranslationPrompt(query string, schema string) string {
	return fmt.Sprintf(`Convert this natural language question into a Cypher query:

QUESTION: %s

GRAPH SCHEMA:
%s

Generate the Cypher query:`, query, schema)
}

// buildSchemaDescription builds a schema description from statistics
func (t *OpenAICypherTranslator) buildSchemaDescription(stats *graph.ProjectStatistics) string {
	var schema strings.Builder
	schema.WriteString("Neo4j Graph Schema:\n\n")
	schema.WriteString("NODE TYPES:\n")
	schema.WriteString("- Project: Represents a code project\n")
	schema.WriteString("- Package: Go packages with properties: name, path\n")
	schema.WriteString("- File: Go source files with properties: path, name, size\n")
	schema.WriteString(
		"- Function: Functions/methods with properties: name, signature, is_exported, line_start, line_end\n",
	)
	schema.WriteString("- Struct: Struct types with properties: name, is_exported\n")
	schema.WriteString("- Interface: Interface types with properties: name, is_exported\n")
	schema.WriteString("- Variable: Variables with properties: name, type, is_exported\n")
	schema.WriteString("- Constant: Constants with properties: name, type, value, is_exported\n")
	schema.WriteString("- Import: Import statements with properties: path, alias\n")
	schema.WriteString("\nRELATIONSHIP TYPES:\n")
	schema.WriteString("- CONTAINS: Package->File (packages contain files)\n")
	schema.WriteString("- DEFINES: File->Function/Struct/Interface/Variable/Constant (files define code elements)\n")
	schema.WriteString("- IMPORTS: File->Import (files have imports)\n")
	schema.WriteString("- CALLS: Function->Function (function call relationships)\n")
	schema.WriteString("- IMPLEMENTS: Struct->Interface (struct implements interface)\n")
	schema.WriteString("- HAS_METHOD: Struct->Function (struct has methods)\n")
	schema.WriteString("- DEPENDS_ON: File->File (file dependencies)\n")
	schema.WriteString("- HAS_FIELD: Struct->Variable (struct has fields)\n")
	schema.WriteString(fmt.Sprintf("\nDatabase contains %d nodes total.\n", stats.TotalNodes))
	schema.WriteString(fmt.Sprintf("Database contains %d relationships total.\n", stats.TotalRelationships))
	schema.WriteString("\nIMPORTANT NOTES:\n")
	schema.WriteString("- Functions have a 'package' property that can be queried directly\n")
	schema.WriteString("- To find functions in a package, use: MATCH (f:Function) WHERE f.package = 'packagename'\n")
	schema.WriteString("- Files DEFINE code elements (not CONTAINS)\n")
	schema.WriteString("- All nodes have a 'project_id' property for filtering\n")
	return schema.String()
}

// parseCypherResponse extracts Cypher query from OpenAI response
func (t *OpenAICypherTranslator) parseCypherResponse(content string) (string, float64) {
	// Clean up the response
	content = strings.TrimSpace(content)
	// Remove markdown code blocks if present
	content = strings.TrimPrefix(content, "```cypher")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	// Simple confidence scoring based on content analysis
	confidence := 0.8 // Base confidence
	if strings.Contains(strings.ToUpper(content), "MATCH") {
		confidence += 0.1
	}
	if strings.Contains(strings.ToUpper(content), "RETURN") {
		confidence += 0.1
	}
	if confidence > 1.0 {
		confidence = 1.0
	}
	return content, confidence
}

// generateDescription creates a human-readable description
func (t *OpenAICypherTranslator) generateDescription(naturalQuery, _ string) string {
	return fmt.Sprintf("Translated '%s' to Cypher query", naturalQuery)
}
