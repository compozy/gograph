package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/infra"
	"github.com/compozy/gograph/pkg/config"
	"github.com/compozy/gograph/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	DirectionBackward = "backward"
	DirectionForward  = "forward"
)

var (
	callChainDepth     int
	callChainDirection string
	callChainOutput    string
	callChainProject   string
)

// callChainCmd represents the call-chain command
var callChainCmd = &cobra.Command{
	Use:   "call-chain <function_name>",
	Short: "Trace function call chains in the project",
	Long: `Trace function call chains to understand how functions are connected.
By default, shows functions that call the target function (backward direction).
Use --direction forward to show functions that the target function calls.

Examples:
  # Show functions that call 'Execute' (default: backward)
  gograph call-chain Execute

  # Show functions that 'Execute' calls (forward)
  gograph call-chain Execute --direction forward

  # Limit depth and output as JSON
  gograph call-chain Execute --depth 3 --output json

  # Search for method calls
  gograph call-chain "(*Analyzer).ProcessFile"

  # Use a specific project
  gograph call-chain Execute --project my-backend-api`,
	Args: cobra.ExactArgs(1),
	RunE: runCallChain,
}

// RegisterCallChainCommand registers the call-chain command
func RegisterCallChainCommand() {
	callChainCmd.Flags().IntVarP(&callChainDepth, "depth", "d", 5, "Maximum depth to trace (default: 5)")
	callChainCmd.Flags().
		StringVar(&callChainDirection, "direction", DirectionBackward, "Direction: backward (callers) or forward (callees)")
	callChainCmd.Flags().StringVarP(&callChainOutput, "output", "o", "tree", "Output format: tree, json, or list")
	callChainCmd.Flags().
		StringVarP(&callChainProject, "project", "p", "", "Project ID to use (defaults to current project)")
	rootCmd.AddCommand(callChainCmd)
}

func runCallChain(_ *cobra.Command, args []string) error {
	functionName := args[0]

	// Validate direction flag
	if callChainDirection != DirectionForward && callChainDirection != DirectionBackward {
		return fmt.Errorf(
			"direction must be '%s' or '%s', got: %s",
			DirectionForward,
			DirectionBackward,
			callChainDirection,
		)
	}

	// Determine project ID to use
	var projectID string
	if callChainProject != "" {
		// Use the project ID from the flag
		projectID = callChainProject
	} else {
		// Load project configuration from current directory
		cfg, err := config.LoadProjectConfig(".")
		if err != nil {
			return fmt.Errorf("failed to load project config: %w", err)
		}
		projectID = cfg.Project.ID
	}

	// Get Neo4j configuration
	neo4jConfig, err := getNeo4jConfig()
	if err != nil {
		return err
	}

	// Initialize Neo4j repository
	logger.Debug("connecting to Neo4j", "uri", neo4jConfig.URI)
	repo, err := infra.NewNeo4jRepository(neo4jConfig)
	if err != nil {
		return fmt.Errorf("failed to create Neo4j repository: %w", err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Execute the call chain query
	results, err := executeCallChainQuery(ctx, repo, projectID, functionName)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		if callChainDirection == DirectionBackward {
			fmt.Printf("No functions found that call '%s'\n", functionName)
		} else {
			fmt.Printf("No functions found that '%s' calls\n", functionName)
		}
		return nil
	}

	// Display results based on output format
	switch callChainOutput {
	case "json":
		return outputCallChainJSON(results, functionName, callChainDirection)
	case "list":
		return outputCallChainList(results, functionName, callChainDirection)
	default: // tree
		return outputCallChainTree(results, functionName, callChainDirection)
	}
}

func outputCallChainJSON(results []map[string]any, functionName string, direction string) error {
	output := map[string]any{
		"function":   functionName,
		"direction":  direction,
		"max_depth":  callChainDepth,
		"call_count": len(results),
		"chains":     results,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputCallChainList(
	results []map[string]any,
	functionName string,
	direction string,
) error { //nolint:unparam // always returns nil
	fmt.Printf("Call chains for '%s' (%s, max depth: %d)\n", functionName, direction, callChainDepth)
	fmt.Printf("Found %d call chain(s)\n\n", len(results))

	for i, result := range results {
		chain, ok := result["call_chain"].([]any)
		if !ok || len(chain) == 0 {
			continue
		}

		fmt.Printf("Chain %d (depth: %d):\n", i+1, len(chain)-1)
		for j, node := range chain {
			nodeMap, ok := node.(map[string]any)
			if !ok {
				continue
			}

			indent := strings.Repeat("  ", j)
			name := getString(nodeMap, "name")
			pkg := getString(nodeMap, "package")
			file := getString(nodeMap, "file_path")
			line := getInt(nodeMap, "line_start")

			fmt.Printf("%s%s (%s)", indent, name, pkg)
			if file != "" && line > 0 {
				fmt.Printf(" at %s:%d", file, line)
			}
			fmt.Println()
		}
		fmt.Println()
	}

	return nil
}

func outputCallChainTree(
	results []map[string]any,
	functionName string,
	direction string,
) error { //nolint:unparam // always returns nil
	fmt.Printf("Call tree for '%s' (%s, max depth: %d)\n", functionName, direction, callChainDepth)
	fmt.Printf("Found %d call path(s)\n\n", len(results))

	// Build tree structure
	tree := buildCallTree(results, direction == DirectionBackward)

	// Print tree
	printCallTree(tree, "", true)

	return nil
}

func executeCallChainQuery(
	ctx context.Context,
	repo graph.Repository,
	projectID, functionName string,
) ([]map[string]any, error) {
	var cypherQuery string
	params := map[string]any{
		"project_id":    projectID,
		"function_name": functionName,
	}

	if callChainDirection == DirectionBackward {
		// Find functions that call the target
		cypherQuery = buildBackwardCallChainQuery()
	} else {
		// Find functions that the target calls
		cypherQuery = buildForwardCallChainQuery()
	}

	results, err := repo.ExecuteQuery(ctx, cypherQuery, params)
	if err != nil {
		return nil, fmt.Errorf("failed to trace call chain: %w", err)
	}

	return results, nil
}

func buildBackwardCallChainQuery() string {
	return `
		MATCH (start)
		WHERE (start:Function OR start:Method) 
		  AND start.project_id = $project_id
		  AND (start.name = $function_name OR toLower(start.name) CONTAINS toLower($function_name))
		WITH start
		MATCH path = (caller)-[:CALLS*1..` + fmt.Sprintf("%d", callChainDepth) + `]->(start)
		WHERE (caller:Function OR caller:Method) AND caller.project_id = $project_id
		RETURN [node in nodes(path) | {
			name: node.name, 
			package: node.package,
			file_path: node.file_path,
			signature: node.signature,
			line_start: node.line_start
		}] as call_chain,
		length(path) as depth,
		start.name as target_function
		ORDER BY depth
		LIMIT 100
	`
}

func buildForwardCallChainQuery() string {
	return `
		MATCH (start)
		WHERE (start:Function OR start:Method) 
		  AND start.project_id = $project_id
		  AND (start.name = $function_name OR toLower(start.name) CONTAINS toLower($function_name))
		WITH start
		MATCH path = (start)-[:CALLS*1..` + fmt.Sprintf("%d", callChainDepth) + `]->(callee)
		WHERE (callee:Function OR callee:Method) AND callee.project_id = $project_id
		RETURN [node in nodes(path) | {
			name: node.name, 
			package: node.package,
			file_path: node.file_path,
			signature: node.signature,
			line_start: node.line_start
		}] as call_chain,
		length(path) as depth,
		start.name as target_function
		ORDER BY depth
		LIMIT 100
	`
}

func getNeo4jConfig() (*infra.Neo4jConfig, error) {
	neo4jURI := viper.GetString("neo4j.uri")
	neo4jUsername := viper.GetString("neo4j.username")
	neo4jPassword := viper.GetString("neo4j.password")

	// Use environment variables as fallback
	if neo4jURI == "" {
		neo4jURI = os.Getenv("NEO4J_URI")
	}
	if neo4jUsername == "" {
		neo4jUsername = os.Getenv("NEO4J_USERNAME")
		if neo4jUsername == "" {
			neo4jUsername = DefaultNeo4jUsername
		}
	}
	if neo4jPassword == "" {
		neo4jPassword = os.Getenv("NEO4J_PASSWORD")
		if neo4jPassword == "" {
			neo4jPassword = DefaultNeo4jPassword
		}
	}

	neo4jConfig := &infra.Neo4jConfig{
		URI:        neo4jURI,
		Username:   neo4jUsername,
		Password:   neo4jPassword,
		Database:   viper.GetString("neo4j.database"),
		MaxRetries: 3,
		BatchSize:  1000,
	}

	// Check if Neo4j is configured
	if neo4jConfig.URI == "" {
		return nil, fmt.Errorf("Neo4j URI not configured. Run 'gograph init' or set NEO4J_URI environment variable")
	}

	return neo4jConfig, nil
}

// CallNode represents a node in the call tree
type CallNode struct {
	Name     string
	Package  string
	FilePath string
	Line     int
	Children map[string]*CallNode
}

func buildCallTree(results []map[string]any, _ bool) *CallNode {
	root := &CallNode{
		Name:     "root",
		Children: make(map[string]*CallNode),
	}

	for _, result := range results {
		callChain, ok := result["call_chain"].([]any)
		if !ok || len(callChain) == 0 {
			continue
		}

		// Build tree from chain
		currentNode := root
		for _, node := range callChain {
			nodeMap, ok := node.(map[string]any)
			if !ok {
				continue
			}

			name := getString(nodeMap, "name")
			pkg := getString(nodeMap, "package")
			file := getString(nodeMap, "file_path")
			line := getInt(nodeMap, "line_start")
			key := fmt.Sprintf("%s.%s", pkg, name)

			// Create node if it doesn't exist
			if _, exists := currentNode.Children[key]; !exists {
				currentNode.Children[key] = &CallNode{
					Name:     name,
					Package:  pkg,
					FilePath: file,
					Line:     line,
					Children: make(map[string]*CallNode),
				}
			}

			// Move to the child node
			currentNode = currentNode.Children[key]
		}
	}

	return root
}

func printCallTree(node *CallNode, prefix string, isLast bool) {
	if node.Name == "root" {
		// Print children of root
		children := getSortedChildren(node)
		for i, child := range children {
			printCallTree(child, "", i == len(children)-1)
		}
		return
	}

	// Print current node
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	fmt.Printf("%s%s%s (%s)", prefix, connector, node.Name, node.Package)
	if node.FilePath != "" && node.Line > 0 {
		fmt.Printf(" at %s:%d", node.FilePath, node.Line)
	}
	fmt.Println()

	// Prepare prefix for children
	childPrefix := prefix
	if isLast {
		childPrefix += "    "
	} else {
		childPrefix += "│   "
	}

	// Print children
	children := getSortedChildren(node)
	for i, child := range children {
		printCallTree(child, childPrefix, i == len(children)-1)
	}
}

func getSortedChildren(node *CallNode) []*CallNode {
	var children []*CallNode
	for _, child := range node.Children {
		children = append(children, child)
	}
	// Sort by package and name for consistent output
	for i := 0; i < len(children); i++ {
		for j := i + 1; j < len(children); j++ {
			if children[i].Package > children[j].Package ||
				(children[i].Package == children[j].Package && children[i].Name > children[j].Name) {
				children[i], children[j] = children[j], children[i]
			}
		}
	}
	return children
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}
