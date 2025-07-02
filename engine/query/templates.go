package query

import (
	"fmt"
	"strings"
)

// Template represents a query template with parameters
type Template struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Query       string            `json:"query"`
	Parameters  map[string]string `json:"parameters"`
	Category    string            `json:"category"`
}

// CommonTemplates contains frequently used query templates
var CommonTemplates = map[string]*Template{
	// Project overview queries
	"project_overview": {
		Name:        "Project Overview",
		Description: "Get basic statistics about a project",
		Category:    "overview",
		Query: `MATCH (n) WHERE n.project_id = $project_id
		RETURN 
		  labels(n)[0] as node_type,
		  count(n) as count
		ORDER BY count DESC`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	"project_files": {
		Name:        "List Project Files",
		Description: "List all files in a project with their package information",
		Category:    "overview",
		Query: `MATCH (f:File) WHERE f.project_id = $project_id
		RETURN f.path as file_path, f.name as file_name, f.package as package_name
		ORDER BY f.package, f.name`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	"project_packages": {
		Name:        "List Project Packages",
		Description: "List all packages in a project with file counts",
		Category:    "overview",
		Query: `MATCH (f:File) WHERE f.project_id = $project_id
		RETURN f.package as package_name, count(f) as file_count
		ORDER BY file_count DESC, package_name`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	// Function analysis queries
	"functions_by_package": {
		Name:        "Functions by Package",
		Description: "List all functions grouped by package",
		Category:    "functions",
		Query: `MATCH (f:Function) WHERE f.project_id = $project_id
		RETURN f.package as package_name, f.name as function_name, 
		       f.signature as signature, f.is_exported as is_exported
		ORDER BY f.package, f.name`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	"exported_functions": {
		Name:        "Exported Functions",
		Description: "List all exported functions in a project",
		Category:    "functions",
		Query: `MATCH (f:Function) 
		WHERE f.project_id = $project_id AND f.is_exported = true
		RETURN f.package as package_name, f.name as function_name, f.signature as signature
		ORDER BY f.package, f.name`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	"function_complexity": {
		Name:        "Function Complexity",
		Description: "List functions ordered by complexity (line count)",
		Category:    "functions",
		Query: `MATCH (f:Function) WHERE f.project_id = $project_id
		WITH f, (f.line_end - f.line_start) as complexity
		RETURN f.package as package_name, f.name as function_name, 
		       complexity, f.signature as signature
		ORDER BY complexity DESC
		LIMIT 20`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	// Dependency analysis queries
	"package_dependencies": {
		Name:        "Package Dependencies",
		Description: "Show dependencies between packages",
		Category:    "dependencies",
		Query: `MATCH (f1:File)-[:DEPENDS_ON]->(f2:File) 
		WHERE f1.project_id = $project_id
		RETURN DISTINCT f1.package as from_package, f2.package as to_package
		ORDER BY from_package, to_package`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	"external_dependencies": {
		Name:        "External Dependencies",
		Description: "List external package imports",
		Category:    "dependencies",
		Query: `MATCH (i:Import) WHERE i.project_id = $project_id
		AND NOT i.path STARTS WITH "."
		RETURN DISTINCT i.path as external_package, count(*) as usage_count
		ORDER BY usage_count DESC, external_package`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	"dependency_graph": {
		Name:        "Dependency Graph",
		Description: "Full dependency graph with relationships",
		Category:    "dependencies",
		Query: `MATCH (f1:File)-[r:DEPENDS_ON]->(f2:File) 
		WHERE f1.project_id = $project_id
		RETURN f1.path as from_file, f2.path as to_file, type(r) as relationship_type`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	// Interface and struct queries
	"interface_implementations": {
		Name:        "Interface Implementations",
		Description: "List all interfaces and their implementations",
		Category:    "types",
		Query: `MATCH (s:Struct)-[:IMPLEMENTS]->(i:Interface) 
		WHERE s.project_id = $project_id
		RETURN i.package as interface_package, i.name as interface_name,
		       s.package as struct_package, s.name as struct_name
		ORDER BY i.name, s.name`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	"unimplemented_interfaces": {
		Name:        "Unimplemented Interfaces",
		Description: "Find interfaces with no implementations",
		Category:    "types",
		Query: `MATCH (i:Interface) WHERE i.project_id = $project_id
		AND NOT EXISTS {
		  MATCH (s:Struct)-[:IMPLEMENTS]->(i)
		}
		RETURN i.package as package_name, i.name as interface_name
		ORDER BY i.package, i.name`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	"struct_methods": {
		Name:        "Struct Methods",
		Description: "List all structs and their methods",
		Category:    "types",
		Query: `MATCH (s:Struct)-[:HAS_METHOD]->(m:Function) 
		WHERE s.project_id = $project_id
		RETURN s.package as struct_package, s.name as struct_name,
		       m.name as method_name, m.signature as method_signature
		ORDER BY s.name, m.name`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	// Call chain analysis
	"function_calls": {
		Name:        "Function Call Relationships",
		Description: "Show which functions call which other functions",
		Category:    "calls",
		Query: `MATCH (f1:Function)-[:CALLS]->(f2:Function) 
		WHERE f1.project_id = $project_id
		RETURN f1.package as caller_package, f1.name as caller_name,
		       f2.package as callee_package, f2.name as callee_name
		ORDER BY caller_package, caller_name`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	"most_called_functions": {
		Name:        "Most Called Functions",
		Description: "Functions ordered by how often they are called",
		Category:    "calls",
		Query: `MATCH (f:Function)<-[:CALLS]-() WHERE f.project_id = $project_id
		RETURN f.package as package_name, f.name as function_name, 
		       count(*) as call_count, f.signature as signature
		ORDER BY call_count DESC
		LIMIT 20`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	"unused_functions": {
		Name:        "Unused Functions",
		Description: "Functions that are never called (potential dead code)",
		Category:    "calls",
		Query: `MATCH (f:Function) WHERE f.project_id = $project_id
		AND NOT EXISTS {
		  MATCH ()-[:CALLS]->(f)
		}
		AND f.name <> "main" AND f.name <> "init"
		RETURN f.package as package_name, f.name as function_name, f.signature as signature
		ORDER BY f.package, f.name`,
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	},
	// Search queries
	"find_function": {
		Name:        "Find Function by Name",
		Description: "Search for functions by name (case-insensitive)",
		Category:    "search",
		Query: `MATCH (f:Function) WHERE f.project_id = $project_id
		AND toLower(f.name) CONTAINS toLower($function_name)
		RETURN f.package as package_name, f.name as function_name, 
		       f.signature as signature, f.is_exported as is_exported
		ORDER BY f.package, f.name`,
		Parameters: map[string]string{
			"project_id":    "string - The project identifier",
			"function_name": "string - Function name to search for",
		},
	},
	"find_struct": {
		Name:        "Find Struct by Name",
		Description: "Search for structs by name (case-insensitive)",
		Category:    "search",
		Query: `MATCH (s:Struct) WHERE s.project_id = $project_id
		AND toLower(s.name) CONTAINS toLower($struct_name)
		RETURN s.package as package_name, s.name as struct_name, s.is_exported as is_exported
		ORDER BY s.package, s.name`,
		Parameters: map[string]string{
			"project_id":  "string - The project identifier",
			"struct_name": "string - Struct name to search for",
		},
	},
	"find_interface": {
		Name:        "Find Interface by Name",
		Description: "Search for interfaces by name (case-insensitive)",
		Category:    "search",
		Query: `MATCH (i:Interface) WHERE i.project_id = $project_id
		AND toLower(i.name) CONTAINS toLower($interface_name)
		RETURN i.package as package_name, i.name as interface_name, i.is_exported as is_exported
		ORDER BY i.package, i.name`,
		Parameters: map[string]string{
			"project_id":     "string - The project identifier",
			"interface_name": "string - Interface name to search for",
		},
	},
	"search_code": {
		Name:        "Search in Code",
		Description: "Search for text patterns in function signatures",
		Category:    "search",
		Query: `MATCH (f:Function) WHERE f.project_id = $project_id
		AND toLower(f.signature) CONTAINS toLower($search_term)
		RETURN f.package as package_name, f.name as function_name, f.signature as signature
		ORDER BY f.package, f.name`,
		Parameters: map[string]string{
			"project_id":  "string - The project identifier",
			"search_term": "string - Text to search for in function signatures",
		},
	},
}

// GetTemplate retrieves a template by name
func GetTemplate(name string) (*Template, error) {
	template, exists := CommonTemplates[name]
	if !exists {
		return nil, fmt.Errorf("template '%s' not found", name)
	}
	return template, nil
}

// ListTemplates returns all available templates grouped by category
func ListTemplates() map[string][]*Template {
	categories := make(map[string][]*Template)
	for _, template := range CommonTemplates {
		categories[template.Category] = append(categories[template.Category], template)
	}
	return categories
}

// GetTemplatesByCategory returns templates for a specific category
func GetTemplatesByCategory(category string) []*Template {
	var templates []*Template
	for _, template := range CommonTemplates {
		if template.Category == category {
			templates = append(templates, template)
		}
	}
	return templates
}

// ValidateParameters checks if all required parameters are provided
func (t *Template) ValidateParameters(params map[string]any) error {
	for paramName := range t.Parameters {
		if _, exists := params[paramName]; !exists {
			return fmt.Errorf("missing required parameter: %s", paramName)
		}
	}
	return nil
}

// BuildQuery builds the final query with parameter substitution validation
func (t *Template) BuildQuery(params map[string]any) (string, error) {
	if err := t.ValidateParameters(params); err != nil {
		return "", err
	}
	// Return the template query - actual parameter substitution will be handled by Neo4j driver
	return t.Query, nil
}

// GetParameterHelp returns help text for template parameters
func (t *Template) GetParameterHelp() string {
	if len(t.Parameters) == 0 {
		return "No parameters required"
	}
	var help strings.Builder
	help.WriteString("Required parameters:\n")
	for name, description := range t.Parameters {
		help.WriteString(fmt.Sprintf("  %s: %s\n", name, description))
	}
	return help.String()
}
