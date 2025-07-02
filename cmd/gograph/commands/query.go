package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/infra"
	"github.com/compozy/gograph/pkg/logger"
	"github.com/compozy/gograph/pkg/progress"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	formatJSON  = "json"
	formatTable = "table"
	formatCSV   = "csv"
)

var initQueryOnce sync.Once

// InitQueryCommand registers the query command
func InitQueryCommand() {
	initQueryOnce.Do(func() {
		rootCmd.AddCommand(queryCmd)
		queryCmd.Flags().String("format", formatTable, "Output format: table, json, csv")
		queryCmd.Flags().BoolP("raw", "r", false, "Output raw Neo4j response")
		queryCmd.Flags().Int("limit", 100, "Maximum number of results to return")
		queryCmd.Flags().BoolP("count", "c", false, "Show result count and timing")
		queryCmd.Flags().Bool("no-progress", false, "Disable progress indicators")
	})
}

var queryCmd = &cobra.Command{
	Use:   "query [cypher query]",
	Short: "Execute Cypher queries against the Neo4j database",
	Long: `Execute Cypher queries to explore the code graph stored in Neo4j. This
command allows you to run any valid Cypher query and view results in either
table or JSON format.

The graph contains these node types:
  • Package: Go packages in your project
  • File: Individual Go source files
  • Function: Functions and methods
  • Struct: Struct definitions
  • Interface: Interface definitions
  • Method: Methods on structs/interfaces

And these relationship types:
  • CONTAINS: Hierarchical containment (Package->File, File->Function, etc.)
  • IMPORTS: Import dependencies between files
  • IMPLEMENTS: Struct implements interface
  • CALLS: Function calls another function
  • DEPENDS_ON: General dependency relationships

Common queries:
  • Find all packages:
    MATCH (p:Package) RETURN p.name

  • Find interfaces and their implementations:
    MATCH (i:Interface)<-[:IMPLEMENTS]-(s:Struct) 
    RETURN i.name as interface, collect(s.name) as implementors

  • Find circular dependencies:
    MATCH (p1:Package)-[:DEPENDS_ON]->(p2:Package)-[:DEPENDS_ON]->(p1)
    RETURN p1.name, p2.name

  • Find most called functions:
    MATCH (f:Function)<-[:CALLS]-()
    RETURN f.name, count(*) as calls
    ORDER BY calls DESC LIMIT 10`,
	Example: `  # Find all packages
  gograph query "MATCH (p:Package) RETURN p.name"
  
  # Get function call statistics with JSON output
  gograph query "MATCH (f:Function)<-[:CALLS]-() RETURN f.name, count(*) as calls" -f json
  
  # Show query result count
  gograph query "MATCH (n) RETURN n" -c
  
  # Complex query without progress indicator
  gograph query "MATCH path = (p:Package)-[:CONTAINS*]->(f:Function) RETURN path" --no-progress`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		format, err := cmd.Flags().GetString("format")
		if err != nil {
			return fmt.Errorf("failed to get format flag: %w", err)
		}
		showCount, err := cmd.Flags().GetBool("count")
		if err != nil {
			return fmt.Errorf("failed to get count flag: %w", err)
		}
		noProgress, err := cmd.Flags().GetBool("no-progress")
		if err != nil {
			return fmt.Errorf("failed to get no-progress flag: %w", err)
		}

		// Validate format
		if format != formatTable && format != formatJSON {
			return fmt.Errorf("invalid format: %s (must be 'table' or 'json')", format)
		}

		// Get Neo4j configuration with fallback to defaults
		neo4jURI := viper.GetString("neo4j.uri")
		if neo4jURI == "" {
			neo4jURI = DefaultNeo4jURI // Default only if not set via env vars
		}
		neo4jUsername := viper.GetString("neo4j.username")
		if neo4jUsername == "" {
			neo4jUsername = DefaultNeo4jUsername // Default only if not set via env vars
		}
		neo4jPassword := viper.GetString("neo4j.password")
		if neo4jPassword == "" {
			neo4jPassword = DefaultNeo4jPassword // Default only if not set via env vars
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
			return fmt.Errorf("Neo4j URI not configured. Run 'gograph init' or set NEO4J_URI environment variable")
		}

		if noProgress {
			return runQueryWithoutProgress(query, format, showCount, neo4jConfig)
		}
		return runQueryWithProgress(query, format, showCount, neo4jConfig)
	},
}

func outputJSON(results []map[string]any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

func outputTable(results []map[string]any) error {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	// Get column names from first result
	var columns []string
	for key := range results[0] {
		columns = append(columns, key)
	}

	// Create tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print header
	fmt.Fprintf(w, "%s\n", strings.Join(columns, "\t"))
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", len(strings.Join(columns, "\t"))))

	// Print rows
	for _, row := range results {
		var values []string
		for _, col := range columns {
			val := formatValue(row[col])
			values = append(values, val)
		}
		fmt.Fprintf(w, "%s\n", strings.Join(values, "\t"))
	}

	return w.Flush()
}

func formatValue(val any) string {
	if val == nil {
		return "<nil>"
	}

	switch v := val.(type) {
	case string:
		// Truncate long strings
		if len(v) > 50 {
			return v[:47] + "..."
		}
		return v
	case map[string]any:
		// For node/relationship objects, try to display a meaningful summary
		if name, ok := v["name"].(string); ok {
			return fmt.Sprintf("{name: %s}", name)
		}
		if id, ok := v["id"].(string); ok {
			return fmt.Sprintf("{id: %s}", id)
		}
		return fmt.Sprintf("{%d props}", len(v))
	case []any:
		return fmt.Sprintf("[%d items]", len(v))
	default:
		return fmt.Sprintf("%v", v)
	}
}

func runQueryWithoutProgress(query, format string, showCount bool, neo4jConfig *infra.Neo4jConfig) error {
	ctx := context.Background()

	// Initialize Neo4j repository
	logger.Debug("connecting to Neo4j", "uri", neo4jConfig.URI)
	repo, err := infra.NewNeo4jRepository(neo4jConfig)
	if err != nil {
		return fmt.Errorf("failed to create Neo4j repository: %w", err)
	}
	defer repo.Close()

	// Execute query
	logger.Debug("executing query", "query", query)
	start := time.Now()
	results, err := repo.ExecuteQuery(ctx, query, nil)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	duration := time.Since(start)

	// Display results
	if showCount {
		fmt.Printf("Query returned %d results in %v\n\n", len(results), duration)
	}

	switch format {
	case formatJSON:
		return outputJSON(results)
	case formatTable:
		return outputTable(results)
	default:
		return outputTable(results)
	}
}

func runQueryWithProgress(query, format string, showCount bool, neo4jConfig *infra.Neo4jConfig) error {
	ctx := context.Background()
	var results []map[string]any
	var duration time.Duration

	// Connect to Neo4j with progress
	var repo graph.Repository
	err := progress.WithProgress("Connecting to Neo4j", func() error {
		var err error
		repo, err = infra.NewNeo4jRepository(neo4jConfig)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to create Neo4j repository: %w", err)
	}
	defer repo.Close()

	// Execute query with progress
	err = progress.WithProgress("Executing query", func() error {
		start := time.Now()
		var err error
		results, err = repo.ExecuteQuery(ctx, query, nil)
		duration = time.Since(start)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	// Display results
	if showCount {
		fmt.Printf("Query returned %d results in %v\n\n", len(results), duration)
	}

	switch format {
	case formatJSON:
		return outputJSON(results)
	case formatTable:
		return outputTable(results)
	default:
		return outputTable(results)
	}
}
