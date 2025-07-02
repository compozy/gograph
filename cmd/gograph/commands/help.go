package commands

import (
	"sync"

	"github.com/spf13/cobra"
)

var initHelpOnce sync.Once

// InitHelpCommands registers the help commands
func InitHelpCommands() {
	initHelpOnce.Do(func() {
		rootCmd.AddCommand(helpGraphSchema)
		rootCmd.AddCommand(helpCypherExamples)
		rootCmd.AddCommand(helpConfig)
	})
}

// helpGraphSchema provides information about the graph schema
var helpGraphSchema = &cobra.Command{
	Use:   "graph-schema",
	Short: "Information about the graph schema used by gograph",
	Long: `GoGraph creates a rich graph structure in Neo4j to represent your Go codebase.

NODE TYPES:
-----------
• Package
  - Properties: id, name, path
  - Represents: Go packages in your project
  
• File  
  - Properties: id, name, path, package
  - Represents: Individual Go source files
  
• Function
  - Properties: id, name, signature, file, visibility, line_number
  - Represents: Functions and methods in your code
  
• Struct
  - Properties: id, name, file, visibility, fields
  - Represents: Go struct definitions
  
• Interface
  - Properties: id, name, file, visibility, methods
  - Represents: Go interface definitions
  
• Method
  - Properties: id, name, signature, receiver
  - Represents: Methods defined on structs/interfaces

RELATIONSHIP TYPES:
------------------
• CONTAINS
  - Direction: Parent -> Child
  - Examples: Package -[:CONTAINS]-> File, File -[:CONTAINS]-> Function
  
• IMPORTS
  - Direction: File -> File
  - Represents: Import dependencies between files
  
• IMPLEMENTS
  - Direction: Struct -> Interface
  - Represents: Which structs implement which interfaces
  
• CALLS
  - Direction: Function -> Function
  - Represents: Function call relationships
  
• DEPENDS_ON
  - Direction: Package -> Package
  - Represents: Package-level dependencies

EXAMPLE QUERIES:
---------------
# Find all structs in a package:
MATCH (p:Package {name: "mypackage"})-[:CONTAINS]->(f:File)-[:CONTAINS]->(s:Struct)
RETURN s.name, f.path

# Find unused functions (no incoming CALLS):
MATCH (f:Function)
WHERE NOT (f)<-[:CALLS]-()
AND f.visibility = "private"
RETURN f.name, f.file`,
}

// helpCypherExamples provides common Cypher query examples
var helpCypherExamples = &cobra.Command{
	Use:   "cypher-examples",
	Short: "Common Cypher queries for exploring your code graph",
	Long: `Here are some useful Cypher queries for analyzing your Go codebase:

BASIC QUERIES:
-------------
# Count all nodes by type:
MATCH (n)
RETURN labels(n)[0] as type, count(*) as count
ORDER BY count DESC

# List all packages:
MATCH (p:Package)
RETURN p.name, p.path
ORDER BY p.name

CODE ANALYSIS:
-------------
# Find most complex functions (by lines of code):
MATCH (f:Function)
WHERE f.line_count > 50
RETURN f.name, f.file, f.line_count
ORDER BY f.line_count DESC

# Find potential God objects (structs with many methods):
MATCH (s:Struct)<-[:RECEIVER]-(m:Method)
WITH s, count(m) as method_count
WHERE method_count > 10
RETURN s.name, method_count
ORDER BY method_count DESC

DEPENDENCY ANALYSIS:
-------------------
# Find circular dependencies between packages:
MATCH path = (p1:Package)-[:DEPENDS_ON*]->(p1)
RETURN path

# Find most depended-upon packages:
MATCH (p:Package)<-[:DEPENDS_ON]-(dependent)
RETURN p.name, count(dependent) as dependents
ORDER BY dependents DESC

# Show import graph for a specific file:
MATCH (f:File {name: "main.go"})-[:IMPORTS]->(imported)
RETURN f.name, collect(imported.path) as imports

INTERFACE ANALYSIS:
------------------
# Find all implementations of an interface:
MATCH (i:Interface {name: "Reader"})<-[:IMPLEMENTS]-(s:Struct)
RETURN i.name, i.package, collect(s.name) as implementors

# Find interfaces with most implementations:
MATCH (i:Interface)<-[:IMPLEMENTS]-(s:Struct)
RETURN i.name, count(s) as impl_count
ORDER BY impl_count DESC

CALL CHAIN ANALYSIS:
-------------------
# Find all functions called by main:
MATCH path = (main:Function {name: "main"})-[:CALLS*]->(f:Function)
RETURN path

# Find recursive functions:
MATCH (f:Function)-[:CALLS]->(f)
RETURN f.name, f.file

# Find dead code (unreachable functions):
MATCH (f:Function)
WHERE NOT (f)<-[:CALLS]-() 
AND NOT f.name = "main"
AND NOT f.name = "init"
AND f.visibility = "private"
RETURN f.name, f.file

VISUALIZATION:
-------------
# Get subgraph for a package (for visualization):
MATCH (p:Package {name: "mypackage"})-[r*..3]-(connected)
RETURN p, r, connected`,
}

// helpConfig provides information about configuration options
var helpConfig = &cobra.Command{
	Use:   "config",
	Short: "Configuration file format and options",
	Long: `GoGraph uses a YAML configuration file (gograph.yaml) to customize its behavior.

CONFIGURATION FILE STRUCTURE:
----------------------------
neo4j:
  uri: "bolt://localhost:7687"     # Neo4j connection URI
  username: "neo4j"                # Neo4j username
  password: "password"             # Neo4j password
  database: "neo4j"                # Database name (optional)

parser:
  ignore_dirs:                     # Directories to skip
    - ".git"
    - ".idea"
    - "vendor"
    - "node_modules"
  ignore_files:                    # File patterns to skip
    - "*_test.go"                  # Skip test files
    - "*.pb.go"                    # Skip generated files
  include_tests: true              # Include test files
  include_vendor: false            # Include vendor directory
  max_concurrency: 4               # Parallel parsing threads

analyzer:
  max_dependency_depth: 5          # Max depth for dependency analysis
  ignore_test_files: false         # Skip test files in analysis
  ignore_vendor: true              # Skip vendor in analysis
  include_metrics: true            # Calculate code metrics
  parallel_workers: 4              # Parallel analysis threads

logging:
  level: "info"                    # Log level (debug, info, warn, error)
  format: "text"                   # Log format (text, json)

ENVIRONMENT VARIABLES:
---------------------
You can override config values with environment variables:
- GOGRAPH_NEO4J_URI
- GOGRAPH_NEO4J_USERNAME
- GOGRAPH_NEO4J_PASSWORD
- GOGRAPH_LOG_LEVEL

MULTIPLE CONFIGURATIONS:
-----------------------
You can use different config files for different environments:

# Development
gograph analyze . -c dev.yaml

# Production
gograph analyze . -c prod.yaml

DEFAULT LOCATIONS:
-----------------
GoGraph looks for configuration in these locations (in order):
1. -c/--config flag
2. ./gograph.yaml
3. $HOME/.gograph.yaml
4. /etc/gograph/config.yaml`,
}
