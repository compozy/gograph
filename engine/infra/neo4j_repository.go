package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/pkg/errors"
	"github.com/compozy/gograph/pkg/logger"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/config"
)

// Neo4jConfig holds Neo4j connection configuration
type Neo4jConfig struct {
	URI        string // Neo4j connection URI
	Username   string // Username for authentication
	Password   string // Password for authentication
	Database   string // Database name (optional)
	MaxRetries int    // Maximum retry attempts
	BatchSize  int    // Batch size for bulk operations
}

// Global mutex to prevent concurrent index creation across all repository instances
var indexCreationMutex sync.Mutex

// Neo4jRepository implements the graph.Repository interface
type Neo4jRepository struct {
	driver neo4j.DriverWithContext
	config *Neo4jConfig
}

// NewNeo4jRepository creates a new Neo4j repository
func NewNeo4jRepository(config *Neo4jConfig) (graph.Repository, error) {
	if config == nil {
		return nil, fmt.Errorf("Neo4j config is required")
	}

	driver, err := neo4j.NewDriverWithContext(
		config.URI,
		neo4j.BasicAuth(config.Username, config.Password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create neo4j driver: %w", err)
	}

	// Verify connectivity
	ctx := context.Background()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		driver.Close(ctx)
		return nil, fmt.Errorf("failed to verify neo4j connectivity: %w", err)
	}

	logger.Info("connected to neo4j", "uri", config.URI)

	return &Neo4jRepository{
		driver: driver,
		config: config,
	}, nil
}

// StoreAnalysis stores the complete analysis result in Neo4j
func (r *Neo4jRepository) StoreAnalysis(ctx context.Context, result *core.AnalysisResult) error {
	// Clear existing data first
	if err := r.ClearProject(ctx, result.ProjectID); err != nil {
		return fmt.Errorf("failed to clear existing data: %w", err)
	}

	// Import the new analysis result
	return r.ImportAnalysisResult(ctx, result)
}

// Connect establishes a connection to Neo4j with retry logic
func (r *Neo4jRepository) Connect(ctx context.Context, uri, username, password string) error {
	logger.Info("connecting to Neo4j", "uri", uri)

	// Use retry logic for connection attempts
	retryConfig := &errors.RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    2 * time.Second,
		MaxDelay:        10 * time.Second,
		Multiplier:      2.0,
		RetryableErrors: []string{"NEO4J_CONNECTION_ERROR"},
	}

	err := errors.WithRetry(ctx, "neo4j_connect", retryConfig, func() error {
		driver, err := neo4j.NewDriverWithContext(
			uri,
			neo4j.BasicAuth(username, password, ""),
			func(c *config.Config) {
				c.MaxConnectionPoolSize = 50
				c.MaxConnectionLifetime = 5 * time.Minute
				c.ConnectionAcquisitionTimeout = 30 * time.Second
			},
		)
		if err != nil {
			return core.NewError(err, "NEO4J_CONNECTION_ERROR", map[string]any{
				"uri": uri,
			})
		}

		// Verify connectivity
		if err := driver.VerifyConnectivity(ctx); err != nil {
			driver.Close(ctx)
			return core.NewError(err, "NEO4J_CONNECTION_ERROR", map[string]any{
				"uri":   uri,
				"error": "connectivity verification failed",
			})
		}

		r.driver = driver
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to connect to Neo4j after retries: %w", err)
	}

	logger.Info("successfully connected to Neo4j")
	return nil
}

// Close closes the Neo4j connection
func (r *Neo4jRepository) Close() error {
	if r.driver != nil {
		return r.driver.Close(context.Background())
	}
	return nil
}

// CreateNode creates a new node in the graph
func (r *Neo4jRepository) CreateNode(ctx context.Context, node *core.Node) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	// Build the query with dynamic properties
	params := map[string]any{
		"id":         node.ID.String(),
		"name":       node.Name,
		"path":       node.Path,
		"created_at": node.CreatedAt.UTC(),
	}

	// Serialize complex properties
	if node.Properties != nil {
		serializedProps := r.serializeComplexProperties(node.Properties)
		for k, v := range serializedProps {
			params[k] = v
		}
	}

	query := fmt.Sprintf(`
		CREATE (n:%s)
		SET n = $props
		RETURN n
	`, node.Type)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{"props": params})
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})

	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}
	return nil
}

// CreateNodes creates multiple nodes in a batch using optimized batch processing
func (r *Neo4jRepository) CreateNodes(ctx context.Context, nodes []core.Node) error {
	if len(nodes) == 0 {
		return nil
	}

	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	// Determine batch size (default to 1000 if not configured)
	batchSize := r.config.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	// Process nodes in batches
	for i := 0; i < len(nodes); i += batchSize {
		end := i + batchSize
		if end > len(nodes) {
			end = len(nodes)
		}
		batch := nodes[i:end]

		// Group nodes by type for more efficient batch creation
		nodesByType := make(map[core.NodeType][]core.Node)
		for _, node := range batch {
			nodesByType[node.Type] = append(nodesByType[node.Type], node)
		}

		// Create nodes for each type using UNWIND
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			for nodeType, typedNodes := range nodesByType {
				// Convert nodes to parameter list
				nodeParams := make([]map[string]any, len(typedNodes))
				for i, node := range typedNodes {
					params := map[string]any{
						"id":         node.ID.String(),
						"name":       node.Name,
						"path":       node.Path,
						"created_at": node.CreatedAt.UTC(),
					}

					// Serialize complex properties
					if node.Properties != nil {
						serializedProps := r.serializeComplexProperties(node.Properties)
						for k, v := range serializedProps {
							params[k] = v
						}
					}
					nodeParams[i] = params
				}

				// Use UNWIND for efficient batch creation with dynamic properties
				query := fmt.Sprintf(`
					UNWIND $nodes AS node
					CREATE (n:%s)
					SET n = node
				`, nodeType)

				_, err := tx.Run(ctx, query, map[string]any{
					"nodes": nodeParams,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to create batch of %s nodes: %w", nodeType, err)
				}
			}
			return nil, nil
		})

		if err != nil {
			return fmt.Errorf("failed to create node batch %d-%d: %w", i, end, err)
		}

		logger.Debug("created node batch", "batch_start", i, "batch_end", end, "total", len(nodes))
	}

	return nil
}

// GetNode retrieves a node by ID
func (r *Neo4jRepository) GetNode(ctx context.Context, id core.ID) (*core.Node, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	query := `
		MATCH (n)
		WHERE n.id = $id
		RETURN n, labels(n) as labels
	`

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{
			"id": id.String(),
		})
		if err != nil {
			return nil, err
		}

		record, err := result.Single(ctx)
		if err != nil {
			return nil, err
		}

		return r.recordToNode(record)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	node, ok := result.(*core.Node)
	if !ok {
		return nil, fmt.Errorf("unexpected result type")
	}

	return node, nil
}

// UpdateNode updates an existing node
func (r *Neo4jRepository) UpdateNode(ctx context.Context, node *core.Node) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	query := `
		MATCH (n)
		WHERE n.id = $id
		SET n.name = $name, n.path = $path
		RETURN n
	`

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{
			"id":   node.ID.String(),
			"name": node.Name,
			"path": node.Path,
		})
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})

	if err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}
	return nil
}

// DeleteNode deletes a node by ID
func (r *Neo4jRepository) DeleteNode(ctx context.Context, id core.ID) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	query := `
		MATCH (n)
		WHERE n.id = $id
		DETACH DELETE n
	`

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{
			"id": id.String(),
		})
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})

	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}
	return nil
}

// CreateRelationship creates a new relationship
func (r *Neo4jRepository) CreateRelationship(ctx context.Context, rel *core.Relationship) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	// Build the relationship properties map (excluding node matching properties)
	relProps := map[string]any{
		"id":         rel.ID.String(),
		"created_at": rel.CreatedAt.UTC(),
	}

	// Serialize complex properties
	if rel.Properties != nil {
		serializedProps := r.serializeComplexProperties(rel.Properties)
		for k, v := range serializedProps {
			relProps[k] = v
		}
	}

	query := fmt.Sprintf(`
		MATCH (from {id: $from_id}), (to {id: $to_id})
		CREATE (from)-[r:%s]->(to)
		SET r = $props
		RETURN r
	`, rel.Type)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{
			"from_id": rel.FromNodeID.String(),
			"to_id":   rel.ToNodeID.String(),
			"props":   relProps,
		})
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})

	if err != nil {
		return fmt.Errorf("failed to create relationship: %w", err)
	}
	return nil
}

// CreateRelationships creates multiple relationships in a batch using optimized batch processing
func (r *Neo4jRepository) CreateRelationships(ctx context.Context, rels []core.Relationship) error {
	if len(rels) == 0 {
		return nil
	}

	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	// Determine batch size (default to 1000 if not configured)
	batchSize := r.config.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	// Process relationships in batches
	for i := 0; i < len(rels); i += batchSize {
		end := i + batchSize
		if end > len(rels) {
			end = len(rels)
		}
		batch := rels[i:end]

		// Group relationships by type for more efficient batch creation
		relsByType := make(map[core.RelationType][]core.Relationship)
		for _, rel := range batch {
			relsByType[rel.Type] = append(relsByType[rel.Type], rel)
		}

		// Create relationships for each type using UNWIND
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			for relType, typedRels := range relsByType {
				// Convert relationships to parameter list
				relParams := make([]map[string]any, len(typedRels))
				for i, rel := range typedRels {
					// Create the relationship data (excluding node matching fields)
					relData := map[string]any{
						"id":         rel.ID.String(),
						"created_at": rel.CreatedAt.UTC(),
					}

					// Serialize complex properties
					if rel.Properties != nil {
						serializedProps := r.serializeComplexProperties(rel.Properties)
						for k, v := range serializedProps {
							relData[k] = v
						}
					}

					// Create the full parameter object
					relParams[i] = map[string]any{
						"from_id": rel.FromNodeID.String(),
						"to_id":   rel.ToNodeID.String(),
						"props":   relData,
					}
				}

				// Use UNWIND for efficient batch creation with index hints
				query := fmt.Sprintf(`
					UNWIND $rels AS rel
					MATCH (from {id: rel.from_id}), (to {id: rel.to_id})
					CREATE (from)-[r:%s]->(to)
					SET r = rel.props
				`, relType)

				_, err := tx.Run(ctx, query, map[string]any{
					"rels": relParams,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to create batch of %s relationships: %w", relType, err)
				}
			}
			return nil, nil
		})

		if err != nil {
			return fmt.Errorf("failed to create relationship batch %d-%d: %w", i, end, err)
		}

		logger.Debug("created relationship batch", "batch_start", i, "batch_end", end, "total", len(rels))
	}

	return nil
}

// GetRelationship retrieves a relationship by ID
func (r *Neo4jRepository) GetRelationship(ctx context.Context, id core.ID) (*core.Relationship, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	query := `
		MATCH (from)-[r]->(to)
		WHERE r.id = $id
		RETURN r, type(r) as type, from.id as from_id, to.id as to_id
	`

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{
			"id": id.String(),
		})
		if err != nil {
			return nil, err
		}

		record, err := result.Single(ctx)
		if err != nil {
			return nil, err
		}

		return r.recordToRelationship(record)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get relationship: %w", err)
	}

	rel, ok := result.(*core.Relationship)
	if !ok {
		return nil, fmt.Errorf("unexpected result type")
	}

	return rel, nil
}

// DeleteRelationship deletes a relationship by ID
func (r *Neo4jRepository) DeleteRelationship(ctx context.Context, id core.ID) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	query := `
		MATCH ()-[r]->()
		WHERE r.id = $id
		DELETE r
	`

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{
			"id": id.String(),
		})
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})

	if err != nil {
		return fmt.Errorf("failed to delete relationship: %w", err)
	}
	return nil
}

// ExecuteQuery runs a Cypher query and returns results
func (r *Neo4jRepository) ExecuteQuery(
	ctx context.Context,
	query string,
	params map[string]any,
) ([]map[string]any, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	result, err := session.Run(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	records, err := result.Collect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect results: %w", err)
	}

	// Convert records to map
	var results []map[string]any
	for _, record := range records {
		recordMap := make(map[string]any)
		for _, key := range record.Keys {
			val, ok := record.Get(key)
			if ok {
				recordMap[key] = val
			}
		}
		results = append(results, recordMap)
	}

	return results, nil
}

// ImportAnalysisResult imports an entire analysis result with optimized batch processing
func (r *Neo4jRepository) ImportAnalysisResult(ctx context.Context, result *core.AnalysisResult) error {
	startTime := time.Now()

	// Create indexes for better performance (if not already created)
	if err := r.ensureIndexes(ctx); err != nil {
		logger.Warn("failed to create indexes, continuing anyway", "error", err)
	}

	// Use a single session for the entire import to ensure consistency
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	// Execute the entire import in a single write transaction to ensure consistency
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Create all nodes first using batch operations
		logger.Info("importing nodes", "count", len(result.Nodes))
		if err := r.createNodesInTransaction(ctx, tx, result.Nodes); err != nil {
			return nil, fmt.Errorf("failed to create nodes: %w", err)
		}

		// Then create all relationships using batch operations
		logger.Info("importing relationships", "count", len(result.Relationships))
		if err := r.createRelationshipsInTransaction(ctx, tx, result.Relationships); err != nil {
			return nil, fmt.Errorf("failed to create relationships: %w", err)
		}

		return nil, nil
	})

	if err != nil {
		return fmt.Errorf("failed to import analysis result: %w", err)
	}

	// Create project metadata node (outside main transaction as it's not critical)
	if err := r.createProjectMetadata(ctx, result); err != nil {
		logger.Warn("failed to create project metadata", "error", err)
	}

	duration := time.Since(startTime)
	logger.Info("imported analysis result",
		"project_id", result.ProjectID,
		"nodes", len(result.Nodes),
		"relationships", len(result.Relationships),
		"duration", duration,
		"nodes_per_second", float64(len(result.Nodes))/duration.Seconds(),
		"relationships_per_second", float64(len(result.Relationships))/duration.Seconds())

	return nil
}

// createNodesInTransaction creates multiple nodes within an existing transaction
func (r *Neo4jRepository) createNodesInTransaction(
	ctx context.Context,
	tx neo4j.ManagedTransaction,
	nodes []core.Node,
) error {
	if len(nodes) == 0 {
		return nil
	}

	// Determine batch size (default to 1000 if not configured)
	batchSize := r.config.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	// Process nodes in batches
	for i := 0; i < len(nodes); i += batchSize {
		end := i + batchSize
		if end > len(nodes) {
			end = len(nodes)
		}
		batch := nodes[i:end]

		// Group nodes by type for more efficient batch creation
		nodesByType := make(map[core.NodeType][]core.Node)
		for _, node := range batch {
			nodesByType[node.Type] = append(nodesByType[node.Type], node)
		}

		// Create nodes for each type using UNWIND
		for nodeType, typedNodes := range nodesByType {
			// Convert nodes to parameter list
			nodeParams := make([]map[string]any, len(typedNodes))
			for i, node := range typedNodes {
				params := map[string]any{
					"id":         node.ID.String(),
					"name":       node.Name,
					"path":       node.Path,
					"created_at": node.CreatedAt.UTC(),
				}
				// Serialize complex properties
				if node.Properties != nil {
					serializedProps := r.serializeComplexProperties(node.Properties)
					for k, v := range serializedProps {
						params[k] = v
					}
				}
				nodeParams[i] = params
			}

			// Use UNWIND for efficient batch creation with dynamic properties
			query := fmt.Sprintf(`
				UNWIND $nodes AS node
				CREATE (n:%s)
				SET n = node
			`, nodeType)

			_, err := tx.Run(ctx, query, map[string]any{
				"nodes": nodeParams,
			})
			if err != nil {
				return fmt.Errorf("failed to create batch of %s nodes: %w", nodeType, err)
			}
		}

		logger.Debug("created node batch", "batch_start", i, "batch_end", end, "total", len(nodes))
	}

	return nil
}

// createRelationshipsInTransaction creates multiple relationships within an existing transaction
func (r *Neo4jRepository) createRelationshipsInTransaction(
	ctx context.Context,
	tx neo4j.ManagedTransaction,
	rels []core.Relationship,
) error {
	if len(rels) == 0 {
		return nil
	}

	// Determine batch size (default to 1000 if not configured)
	batchSize := r.config.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	// Process relationships in batches
	for i := 0; i < len(rels); i += batchSize {
		end := i + batchSize
		if end > len(rels) {
			end = len(rels)
		}
		batch := rels[i:end]

		// Group relationships by type for more efficient batch creation
		relsByType := make(map[core.RelationType][]core.Relationship)
		for _, rel := range batch {
			relsByType[rel.Type] = append(relsByType[rel.Type], rel)
		}

		// Create relationships for each type using UNWIND
		for relType, typedRels := range relsByType {
			// Convert relationships to parameter list
			relParams := make([]map[string]any, len(typedRels))
			for i, rel := range typedRels {
				// Create the relationship data (excluding node matching fields)
				relData := map[string]any{
					"id":         rel.ID.String(),
					"created_at": rel.CreatedAt.UTC(),
				}

				// Serialize complex properties
				if rel.Properties != nil {
					serializedProps := r.serializeComplexProperties(rel.Properties)
					for k, v := range serializedProps {
						relData[k] = v
					}
				}

				// Create the full parameter object
				relParams[i] = map[string]any{
					"from_id": rel.FromNodeID.String(),
					"to_id":   rel.ToNodeID.String(),
					"props":   relData,
				}
			}

			// Use UNWIND for efficient batch creation with index hints
			query := fmt.Sprintf(`
				UNWIND $rels AS rel
				MATCH (from {id: rel.from_id}), (to {id: rel.to_id})
				CREATE (from)-[r:%s]->(to)
				SET r = rel.props
			`, relType)

			_, err := tx.Run(ctx, query, map[string]any{
				"rels": relParams,
			})
			if err != nil {
				return fmt.Errorf("failed to create batch of %s relationships: %w", relType, err)
			}
		}

		logger.Debug("created relationship batch", "batch_start", i, "batch_end", end, "total", len(rels))
	}

	return nil
}

// ensureIndexes creates indexes for better query performance on large codebases
func (r *Neo4jRepository) ensureIndexes(ctx context.Context) error {
	// Use global mutex to prevent concurrent index creation across all instances
	indexCreationMutex.Lock()
	defer indexCreationMutex.Unlock()

	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	// Create single-property indexes
	r.createSinglePropertyIndexes(ctx, session)

	// Create project_id indexes for performance optimization
	r.createProjectIDIndexes(ctx, session)

	// Create composite indexes
	r.createCompositeIndexes(ctx, session)

	// Create text indexes
	r.createTextIndexes(ctx, session)

	// Create constraints
	r.createConstraints(ctx, session)

	logger.Info("database indexes and constraints created for optimized performance")
	return nil
}

// createSinglePropertyIndexes creates single-property indexes for common queries
func (r *Neo4jRepository) createSinglePropertyIndexes(ctx context.Context, session neo4j.SessionWithContext) {
	singleIndexes := []struct {
		label    string
		property string
	}{
		{"File", "id"},
		{"File", "path"},
		{"Package", "id"},
		{"Package", "name"},
		{"Function", "id"},
		{"Function", "name"},
		{"Struct", "id"},
		{"Struct", "name"},
		{"Interface", "id"},
		{"Interface", "name"},
		{"Field", "id"},
		{"Field", "name"},
		{"Method", "id"},
		{"Method", "name"},
		{"Constant", "id"},
		{"Constant", "name"},
		{"Import", "path"},
		{"ProjectMetadata", "project_id"},
	}

	for _, idx := range singleIndexes {
		query := fmt.Sprintf("CREATE INDEX IF NOT EXISTS FOR (n:%s) ON (n.%s)", idx.label, idx.property)
		_, err := session.Run(ctx, query, nil)
		if err != nil {
			logger.Debug("failed to create single index",
				"label", idx.label,
				"property", idx.property,
				"error", err)
		}
	}
}

// createProjectIDIndexes creates indexes specifically for project_id performance optimization
func (r *Neo4jRepository) createProjectIDIndexes(ctx context.Context, session neo4j.SessionWithContext) {
	// Node indexes for project_id - covering all major node types
	nodeTypes := []string{
		"File", "Package", "Function", "Struct", "Interface",
		"Method", "Import", "Constant", "Variable",
	}

	for _, nodeType := range nodeTypes {
		query := fmt.Sprintf("CREATE INDEX IF NOT EXISTS FOR (n:%s) ON (n.project_id)", nodeType)
		_, err := session.Run(ctx, query, nil)
		if err != nil {
			logger.Debug("failed to create project_id index",
				"node_type", nodeType,
				"error", err)
		}
	}

	// Relationship indexes for project_id - covering all major relationship types
	relationshipTypes := []string{
		"CONTAINS", "DEFINES", "CALLS", "IMPLEMENTS",
		"IMPORTS", "DEPENDS_ON", "BELONGS_TO", "REFERENCES",
	}

	for _, relType := range relationshipTypes {
		query := fmt.Sprintf("CREATE INDEX IF NOT EXISTS FOR ()-[r:%s]-() ON (r.project_id)", relType)
		_, err := session.Run(ctx, query, nil)
		if err != nil {
			logger.Debug("failed to create relationship project_id index",
				"rel_type", relType,
				"error", err)
		}
	}

	logger.Info("project_id indexes created for performance optimization")
}

// createCompositeIndexes creates composite indexes for optimized queries
func (r *Neo4jRepository) createCompositeIndexes(ctx context.Context, session neo4j.SessionWithContext) {
	compositeIndexes := []string{
		// Package + name combinations for quick lookups
		"CREATE INDEX IF NOT EXISTS FOR (n:Function) ON (n.package, n.name)",
		"CREATE INDEX IF NOT EXISTS FOR (n:Struct) ON (n.package, n.name)",
		"CREATE INDEX IF NOT EXISTS FOR (n:Interface) ON (n.package, n.name)",
		"CREATE INDEX IF NOT EXISTS FOR (n:Method) ON (n.receiver, n.name)",

		// File + package for directory operations
		"CREATE INDEX IF NOT EXISTS FOR (n:File) ON (n.package, n.path)",

		// For dependency analysis
		"CREATE INDEX IF NOT EXISTS FOR (n:Import) ON (n.source_package, n.path)",

		// For visibility-based queries
		"CREATE INDEX IF NOT EXISTS FOR (n:Function) ON (n.exported, n.name)",
		"CREATE INDEX IF NOT EXISTS FOR (n:Struct) ON (n.exported, n.name)",
	}

	for _, query := range compositeIndexes {
		_, err := session.Run(ctx, query, nil)
		if err != nil {
			logger.Debug("failed to create composite index", "query", query, "error", err)
		}
	}
}

// createTextIndexes creates text indexes for search functionality
func (r *Neo4jRepository) createTextIndexes(ctx context.Context, session neo4j.SessionWithContext) {
	textIndexes := []string{
		"CREATE FULLTEXT INDEX functionNameSearch IF NOT EXISTS FOR (n:Function) ON EACH [n.name, n.comment]",
		"CREATE FULLTEXT INDEX structNameSearch IF NOT EXISTS FOR (n:Struct) ON EACH [n.name, n.comment]",
		"CREATE FULLTEXT INDEX filePathSearch IF NOT EXISTS FOR (n:File) ON EACH [n.path]",
	}

	for _, query := range textIndexes {
		_, err := session.Run(ctx, query, nil)
		if err != nil {
			// Text indexes might not be supported in all Neo4j versions
			logger.Debug("failed to create text index (may require Neo4j 4.0+)", "query", query, "error", err)
		}
	}
}

// createConstraints creates constraints for data integrity
func (r *Neo4jRepository) createConstraints(ctx context.Context, session neo4j.SessionWithContext) {
	constraints := []string{
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:File) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:Package) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:Function) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:Struct) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:Interface) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:Method) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:ProjectMetadata) REQUIRE n.project_id IS UNIQUE",
	}

	for _, query := range constraints {
		_, err := session.Run(ctx, query, nil)
		if err != nil {
			// Constraints might fail if indexes already exist
			logger.Debug("failed to create constraint", "query", query, "error", err)
		}
	}
}

// createProjectMetadata creates a metadata node for the project
func (r *Neo4jRepository) createProjectMetadata(ctx context.Context, result *core.AnalysisResult) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	query := `
		MERGE (p:ProjectMetadata {project_id: $project_id})
		SET p.analyzed_at = $analyzed_at,
		    p.total_files = $total_files,
		    p.total_packages = $total_packages,
		    p.total_functions = $total_functions,
		    p.total_structs = $total_structs,
		    p.node_count = $node_count,
		    p.relationship_count = $relationship_count,
		    p.updated_at = timestamp()
		RETURN p
	`

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, query, map[string]any{
			"project_id":         result.ProjectID.String(),
			"analyzed_at":        result.AnalyzedAt.UTC(),
			"total_files":        result.TotalFiles,
			"total_packages":     result.TotalPackages,
			"total_functions":    result.TotalFunctions,
			"total_structs":      result.TotalStructs,
			"node_count":         len(result.Nodes),
			"relationship_count": len(result.Relationships),
		})
		return nil, err
	})

	return err
}

// ClearProject removes all nodes and relationships for a specific project
func (r *Neo4jRepository) ClearProject(ctx context.Context, projectID core.ID) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	// Only delete nodes and relationships for the specified project_id
	query := `
		MATCH (n {project_id: $project_id})
		DETACH DELETE n
	`

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{
			"project_id": projectID.String(),
		})
		if err != nil {
			return nil, err
		}
		return result.Consume(ctx)
	})

	if err != nil {
		return fmt.Errorf("failed to clear project %s: %w", projectID, err)
	}

	logger.Info("successfully cleared project", "project_id", projectID)
	return nil
}

// FindNodesByType finds all nodes of a specific type
func (r *Neo4jRepository) FindNodesByType(
	ctx context.Context,
	nodeType core.NodeType,
	projectID core.ID,
) ([]core.Node, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	query := fmt.Sprintf(`
		MATCH (n:%s)
		WHERE n.project_id = $project_id
		RETURN n, labels(n) as labels
	`, nodeType)

	results, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{
			"project_id": projectID.String(),
		})
		if err != nil {
			return nil, err
		}

		var nodes []core.Node
		for result.Next(ctx) {
			node, err := r.recordToNode(result.Record())
			if err != nil {
				logger.Warn("failed to convert record to node", "error", err)
				continue
			}
			nodes = append(nodes, *node)
		}

		return nodes, result.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("failed to find nodes by type: %w", err)
	}

	nodes, ok := results.([]core.Node)
	if !ok {
		return nil, fmt.Errorf("unexpected result type")
	}

	return nodes, nil
}

// FindNodesByName finds nodes by name
func (r *Neo4jRepository) FindNodesByName(
	ctx context.Context,
	name string,
	projectID core.ID,
) ([]core.Node, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	query := `
		MATCH (n)
		WHERE n.name = $name AND n.project_id = $project_id
		RETURN n, labels(n) as labels
	`

	results, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{
			"name":       name,
			"project_id": projectID.String(),
		})
		if err != nil {
			return nil, err
		}

		var nodes []core.Node
		for result.Next(ctx) {
			node, err := r.recordToNode(result.Record())
			if err != nil {
				logger.Warn("failed to convert record to node", "error", err)
				continue
			}
			nodes = append(nodes, *node)
		}

		return nodes, result.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("failed to find nodes by name: %w", err)
	}

	nodes, ok := results.([]core.Node)
	if !ok {
		return nil, fmt.Errorf("unexpected result type")
	}

	return nodes, nil
}

// FindRelationshipsByType finds all relationships of a specific type
func (r *Neo4jRepository) FindRelationshipsByType(
	ctx context.Context,
	relType core.RelationType,
	projectID core.ID,
) ([]core.Relationship, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: r.config.Database,
	})
	defer session.Close(ctx)

	query := fmt.Sprintf(`
		MATCH (from)-[r:%s]->(to)
		WHERE r.project_id = $project_id
		RETURN r, type(r) as type, from.id as from_id, to.id as to_id
	`, relType)

	results, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{
			"project_id": projectID.String(),
		})
		if err != nil {
			return nil, err
		}

		var relationships []core.Relationship
		for result.Next(ctx) {
			rel, err := r.recordToRelationship(result.Record())
			if err != nil {
				logger.Warn("failed to convert record to relationship", "error", err)
				continue
			}
			relationships = append(relationships, *rel)
		}

		return relationships, result.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("failed to find relationships by type: %w", err)
	}

	relationships, ok := results.([]core.Relationship)
	if !ok {
		return nil, fmt.Errorf("unexpected result type")
	}

	return relationships, nil
}

// Helper functions

func (r *Neo4jRepository) recordToNode(record *neo4j.Record) (*core.Node, error) {
	nodeVal, ok := record.Get("n")
	if !ok {
		return nil, fmt.Errorf("node not found in record")
	}

	nodeData, ok := nodeVal.(neo4j.Node)
	if !ok {
		return nil, fmt.Errorf("invalid node type")
	}

	labelsVal, labelsOK := record.Get("labels")
	var labels []any
	if labelsOK {
		labelsList, ok := labelsVal.([]any)
		if ok {
			labels = labelsList
		}
	}

	var nodeType core.NodeType
	if len(labels) > 0 {
		if labelStr, ok := labels[0].(string); ok {
			nodeType = core.NodeType(labelStr)
		}
	}

	idVal, ok := nodeData.Props["id"]
	if !ok {
		return nil, fmt.Errorf("node missing required 'id' property")
	}
	idStr, ok := idVal.(string)
	if !ok {
		return nil, fmt.Errorf("node 'id' property is not a string")
	}

	nameVal, ok := nodeData.Props["name"]
	if !ok {
		return nil, fmt.Errorf("node missing required 'name' property")
	}
	nameStr, ok := nameVal.(string)
	if !ok {
		return nil, fmt.Errorf("node 'name' property is not a string")
	}

	node := &core.Node{
		ID:   core.ID(idStr),
		Type: nodeType,
		Name: nameStr,
	}

	if path, ok := nodeData.Props["path"].(string); ok {
		node.Path = path
	}

	// Extract all properties except the system ones
	systemProps := map[string]bool{
		"id":   true,
		"name": true,
		"path": true,
	}

	properties := make(map[string]any)
	for k, v := range nodeData.Props {
		if !systemProps[k] {
			properties[k] = v
		}
	}

	if len(properties) > 0 {
		node.Properties = properties
	}

	return node, nil
}

func (r *Neo4jRepository) recordToRelationship(record *neo4j.Record) (*core.Relationship, error) {
	relVal, ok := record.Get("r")
	if !ok {
		return nil, fmt.Errorf("relationship not found in record")
	}

	relData, ok := relVal.(neo4j.Relationship)
	if !ok {
		return nil, fmt.Errorf("invalid relationship type")
	}

	typeVal, typeOK := record.Get("type")
	if !typeOK {
		return nil, fmt.Errorf("relationship type not found")
	}
	relTypeStr, ok := typeVal.(string)
	if !ok {
		return nil, fmt.Errorf("relationship type is not a string")
	}

	fromIDVal, fromOK := record.Get("from_id")
	if !fromOK {
		return nil, fmt.Errorf("from_id not found")
	}
	fromIDStr, ok := fromIDVal.(string)
	if !ok {
		return nil, fmt.Errorf("from_id is not a string")
	}

	toIDVal, toOK := record.Get("to_id")
	if !toOK {
		return nil, fmt.Errorf("to_id not found")
	}
	toIDStr, ok := toIDVal.(string)
	if !ok {
		return nil, fmt.Errorf("to_id is not a string")
	}

	idVal, ok := relData.Props["id"]
	if !ok {
		return nil, fmt.Errorf("relationship missing required 'id' property")
	}
	idStr, ok := idVal.(string)
	if !ok {
		return nil, fmt.Errorf("relationship 'id' property is not a string")
	}

	rel := &core.Relationship{
		ID:         core.ID(idStr),
		Type:       core.RelationType(relTypeStr),
		FromNodeID: core.ID(fromIDStr),
		ToNodeID:   core.ID(toIDStr),
	}

	// Extract all properties except the system ones
	systemProps := map[string]bool{
		"id": true,
	}

	properties := make(map[string]any)
	for k, v := range relData.Props {
		if !systemProps[k] {
			properties[k] = v
		}
	}

	if len(properties) > 0 {
		rel.Properties = properties
	}

	return rel, nil
}

// serializeComplexProperties converts complex properties to JSON strings for Neo4j compatibility
func (r *Neo4jRepository) serializeComplexProperties(properties map[string]any) map[string]any {
	if properties == nil {
		return nil
	}
	serialized := make(map[string]any)
	for k, v := range properties {
		serialized[k] = r.serializeValue(v)
	}
	return serialized
}

// serializeValue recursively serializes a single value
func (r *Neo4jRepository) serializeValue(v any) any {
	if v == nil {
		return nil
	}
	// Convert time values to UTC
	if t, ok := v.(time.Time); ok {
		return t.UTC()
	}
	// Check if it's a supported primitive type
	if primitiveVal := r.handlePrimitiveTypes(v); primitiveVal != nil {
		return primitiveVal
	}
	// Check if it's a supported array type
	if arrayVal := r.handleArrayTypes(v); arrayVal != nil {
		return arrayVal
	}
	// Everything else needs to be serialized to JSON
	return r.serializeToJSON(v)
}

// handlePrimitiveTypes handles Neo4j-supported primitive types
func (r *Neo4jRepository) handlePrimitiveTypes(v any) any {
	switch val := v.(type) {
	case bool, int64, float64, string:
		return v
	case int, int8, int16, int32:
		return reflect.ValueOf(v).Int()
	case uint, uint8, uint16, uint32:
		return r.handleUintConversion(reflect.ValueOf(v).Uint(), v)
	case uint64:
		return r.handleUintConversion(reflect.ValueOf(v).Uint(), v)
	case float32:
		return float64(val)
	default:
		return nil
	}
}

// handleArrayTypes handles Neo4j-supported array types
func (r *Neo4jRepository) handleArrayTypes(v any) any {
	switch val := v.(type) {
	case []bool, []int64, []float64, []string:
		return v
	case []int:
		result := make([]int64, len(val))
		for i, item := range val {
			result[i] = int64(item)
		}
		return result
	default:
		return nil
	}
}

// handleUintConversion safely converts uint values to int64, serializing large values
func (r *Neo4jRepository) handleUintConversion(uintVal uint64, originalVal any) any {
	if uintVal > 9223372036854775807 { // max int64
		return r.serializeToJSON(originalVal)
	}
	return int64(uintVal)
}

// serializeToJSON serializes a value to JSON string
func (r *Neo4jRepository) serializeToJSON(v any) string {
	if jsonBytes, err := json.Marshal(v); err == nil {
		return string(jsonBytes)
	}
	return fmt.Sprintf("%v", v)
}
