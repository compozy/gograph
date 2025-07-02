package infra_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/infra"
	"github.com/compozy/gograph/pkg/testhelpers"
	"github.com/stretchr/testify/suite"
)

// Neo4jRepositoryTestSuite runs all tests with a single container
type Neo4jRepositoryTestSuite struct {
	suite.Suite
	container *testhelpers.Neo4jTestContainer
	repo      *infra.Neo4jRepository
	ctx       context.Context
}

// SetupSuite runs once before all tests
func (s *Neo4jRepositoryTestSuite) SetupSuite() {
	s.ctx = context.Background()

	// Start container once for all tests
	container, err := testhelpers.StartNeo4jContainer(s.ctx)
	s.Require().NoError(err)
	s.container = container

	// Create repository
	repo, err := container.CreateRepository()
	s.Require().NoError(err)
	s.repo = repo
}

// TearDownSuite runs once after all tests
func (s *Neo4jRepositoryTestSuite) TearDownSuite() {
	if s.repo != nil {
		s.repo.Close()
	}
	if s.container != nil {
		s.container.Stop(s.ctx)
	}
}

// SetupTest runs before each test - clears the database
func (s *Neo4jRepositoryTestSuite) SetupTest() {
	// Clear all data before each test using ExecuteQuery
	query := "MATCH (n) DETACH DELETE n"
	_, err := s.repo.ExecuteQuery(s.ctx, query, nil)
	s.Require().NoError(err)
}

func TestNeo4jRepositoryTestSuite(t *testing.T) {
	// Skip if running in CI without Docker
	if os.Getenv("CI") == "true" && os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration tests in CI")
	}

	suite.Run(t, new(Neo4jRepositoryTestSuite))
}

// -----
// Connection Tests
// -----

func (s *Neo4jRepositoryTestSuite) TestConnect() {
	// Connection is already established in SetupSuite
	err := s.container.VerifyConnection(s.ctx)
	s.NoError(err)
}

func (s *Neo4jRepositoryTestSuite) TestConnectWithInvalidCredentials() {
	// Test invalid credentials with a new config
	config := &infra.Neo4jConfig{
		URI:        s.container.URI,
		Username:   "invalid",
		Password:   "invalid",
		Database:   "",
		MaxRetries: 1,
		BatchSize:  1000,
	}

	_, err := infra.NewNeo4jRepository(config)
	s.Error(err)
}

// -----
// Node Operations Tests
// -----

func (s *Neo4jRepositoryTestSuite) TestCreateNode() {
	node := &core.Node{
		ID:   core.NewID(),
		Type: core.NodeType("Function"),
		Name: "TestFunction",
		Path: "/src/test.go",
		Properties: map[string]any{
			"exported": true,
		},
		CreatedAt: time.Now().UTC(),
	}

	err := s.repo.CreateNode(s.ctx, node)
	s.NoError(err)

	// Verify node was created
	result, err := s.repo.GetNode(s.ctx, node.ID)
	s.NoError(err)
	s.Equal(node.ID, result.ID)
	s.Equal(node.Name, result.Name)
	s.Equal(node.Type, result.Type)
}

func (s *Neo4jRepositoryTestSuite) TestCreateMultipleNodes() {
	nodes := []core.Node{
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Package"),
			Name:      "main",
			Path:      "/src/main",
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Function"),
			Name:      "main",
			Path:      "/src/main.go",
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Function"),
			Name:      "helper",
			Path:      "/src/helper.go",
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Struct"),
			Name:      "User",
			Path:      "/src/types.go",
			CreatedAt: time.Now().UTC(),
		},
	}

	// Create all nodes using batch operation
	err := s.repo.CreateNodes(s.ctx, nodes)
	s.NoError(err)

	// Verify all nodes were created
	for _, node := range nodes {
		result, err := s.repo.GetNode(s.ctx, node.ID)
		s.NoError(err)
		s.Equal(node.ID, result.ID)
		s.Equal(node.Name, result.Name)
	}
}

func (s *Neo4jRepositoryTestSuite) TestGetNodeNotFound() {
	nonExistentID := core.NewID()
	_, err := s.repo.GetNode(s.ctx, nonExistentID)
	s.Error(err)
}

func (s *Neo4jRepositoryTestSuite) TestUpdateNode() {
	// Create original node
	originalNode := &core.Node{
		ID:        core.NewID(),
		Type:      core.NodeType("Function"),
		Name:      "original",
		Path:      "/src/original",
		CreatedAt: time.Now().UTC(),
	}

	err := s.repo.CreateNode(s.ctx, originalNode)
	s.NoError(err)

	// Update node
	updatedNode := &core.Node{
		ID:        originalNode.ID,
		Type:      core.NodeType("Function"),
		Name:      "updated",
		Path:      "/src/updated",
		CreatedAt: originalNode.CreatedAt,
	}

	err = s.repo.UpdateNode(s.ctx, updatedNode)
	s.NoError(err)

	// Verify update
	result, err := s.repo.GetNode(s.ctx, originalNode.ID)
	s.NoError(err)
	s.Equal("updated", result.Name)
	s.Equal("/src/updated", result.Path)
}

func (s *Neo4jRepositoryTestSuite) TestDeleteNode() {
	// Create a temporary node
	tempNode := &core.Node{
		ID:        core.NewID(),
		Type:      core.NodeType("Function"),
		Name:      "temp",
		Path:      "/src/temp",
		CreatedAt: time.Now().UTC(),
	}

	err := s.repo.CreateNode(s.ctx, tempNode)
	s.NoError(err)

	// Delete the node
	err = s.repo.DeleteNode(s.ctx, tempNode.ID)
	s.NoError(err)

	// Verify deletion
	_, err = s.repo.GetNode(s.ctx, tempNode.ID)
	s.Error(err)
}

func (s *Neo4jRepositoryTestSuite) TestFindNodesByType() {
	// Create nodes of different types
	nodes := []core.Node{
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Function"),
			Name:      "func1",
			Path:      "/src/func1.go",
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Function"),
			Name:      "func2",
			Path:      "/src/func2.go",
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Struct"),
			Name:      "type1",
			Path:      "/src/types.go",
			CreatedAt: time.Now().UTC(),
		},
	}

	// Create all nodes
	err := s.repo.CreateNodes(s.ctx, nodes)
	s.NoError(err)

	// Get only Function nodes
	functionNodes, err := s.repo.FindNodesByType(s.ctx, core.NodeType("Function"))
	s.NoError(err)
	s.Len(functionNodes, 2)

	// Get only Struct nodes
	structNodes, err := s.repo.FindNodesByType(s.ctx, core.NodeType("Struct"))
	s.NoError(err)
	s.Len(structNodes, 1)
}

func (s *Neo4jRepositoryTestSuite) TestFindNodesByName() {
	// Create nodes with different names
	nodes := []core.Node{
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Function"),
			Name:      "TestFunction",
			Path:      "/src/test.go",
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Function"),
			Name:      "TestFunction",
			Path:      "/src/another.go",
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Function"),
			Name:      "OtherFunction",
			Path:      "/src/other.go",
			CreatedAt: time.Now().UTC(),
		},
	}

	// Create all nodes
	err := s.repo.CreateNodes(s.ctx, nodes)
	s.NoError(err)

	// Search for nodes with exact name
	results, err := s.repo.FindNodesByName(s.ctx, "TestFunction")
	s.NoError(err)
	s.Len(results, 2)

	// Verify results contain the right nodes
	for _, node := range results {
		s.Equal("TestFunction", node.Name)
	}
}

// -----
// Relationship Operations Tests
// -----

func (s *Neo4jRepositoryTestSuite) TestCreateRelationship() {
	// Create two nodes
	node1 := &core.Node{
		ID:        core.NewID(),
		Type:      core.NodeType("Package"),
		Name:      "pkg1",
		Path:      "/src/pkg1",
		CreatedAt: time.Now().UTC(),
	}
	node2 := &core.Node{
		ID:        core.NewID(),
		Type:      core.NodeType("Package"),
		Name:      "pkg2",
		Path:      "/src/pkg2",
		CreatedAt: time.Now().UTC(),
	}

	err := s.repo.CreateNode(s.ctx, node1)
	s.NoError(err)
	err = s.repo.CreateNode(s.ctx, node2)
	s.NoError(err)

	// Create relationship
	rel := &core.Relationship{
		ID:         core.NewID(),
		Type:       core.RelationType("IMPORTS"),
		FromNodeID: node1.ID,
		ToNodeID:   node2.ID,
		Properties: map[string]any{
			"count": 5,
		},
		CreatedAt: time.Now().UTC(),
	}

	err = s.repo.CreateRelationship(s.ctx, rel)
	s.NoError(err)

	// Verify relationship
	result, err := s.repo.GetRelationship(s.ctx, rel.ID)
	s.NoError(err)
	s.Equal(rel.Type, result.Type)
	s.Equal(rel.FromNodeID, result.FromNodeID)
	s.Equal(rel.ToNodeID, result.ToNodeID)
}

func (s *Neo4jRepositoryTestSuite) TestCreateMultipleRelationships() {
	// Create nodes
	nodes := []core.Node{
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Function"),
			Name:      "main",
			Path:      "/main.go",
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Function"),
			Name:      "helper1",
			Path:      "/helper1.go",
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        core.NewID(),
			Type:      core.NodeType("Function"),
			Name:      "helper2",
			Path:      "/helper2.go",
			CreatedAt: time.Now().UTC(),
		},
	}

	err := s.repo.CreateNodes(s.ctx, nodes)
	s.NoError(err)

	// Create relationships
	relationships := []core.Relationship{
		{
			ID:         core.NewID(),
			Type:       core.RelationType("CALLS"),
			FromNodeID: nodes[0].ID,
			ToNodeID:   nodes[1].ID,
			CreatedAt:  time.Now().UTC(),
		},
		{
			ID:         core.NewID(),
			Type:       core.RelationType("CALLS"),
			FromNodeID: nodes[0].ID,
			ToNodeID:   nodes[2].ID,
			CreatedAt:  time.Now().UTC(),
		},
		{
			ID:         core.NewID(),
			Type:       core.RelationType("CALLS"),
			FromNodeID: nodes[1].ID,
			ToNodeID:   nodes[2].ID,
			CreatedAt:  time.Now().UTC(),
		},
	}

	err = s.repo.CreateRelationships(s.ctx, relationships)
	s.NoError(err)

	// Verify all relationships
	for _, rel := range relationships {
		result, err := s.repo.GetRelationship(s.ctx, rel.ID)
		s.NoError(err)
		s.Equal(rel.Type, result.Type)
	}
}

func (s *Neo4jRepositoryTestSuite) TestDeleteRelationship() {
	// Create nodes and relationship
	node1 := &core.Node{
		ID:        core.NewID(),
		Type:      core.NodeType("Package"),
		Name:      "pkg1",
		Path:      "/pkg1",
		CreatedAt: time.Now().UTC(),
	}
	node2 := &core.Node{
		ID:        core.NewID(),
		Type:      core.NodeType("Package"),
		Name:      "pkg2",
		Path:      "/pkg2",
		CreatedAt: time.Now().UTC(),
	}

	err := s.repo.CreateNode(s.ctx, node1)
	s.NoError(err)
	err = s.repo.CreateNode(s.ctx, node2)
	s.NoError(err)

	rel := &core.Relationship{
		ID:         core.NewID(),
		Type:       core.RelationType("IMPORTS"),
		FromNodeID: node1.ID,
		ToNodeID:   node2.ID,
		CreatedAt:  time.Now().UTC(),
	}

	err = s.repo.CreateRelationship(s.ctx, rel)
	s.NoError(err)

	// Delete relationship
	err = s.repo.DeleteRelationship(s.ctx, rel.ID)
	s.NoError(err)

	// Verify deletion
	_, err = s.repo.GetRelationship(s.ctx, rel.ID)
	s.Error(err)
}

func (s *Neo4jRepositoryTestSuite) TestFindRelationshipsByType() {
	// Create nodes
	nodes := []core.Node{
		{ID: core.NewID(), Type: core.NodeType("Package"), Name: "pkg1", Path: "/pkg1", CreatedAt: time.Now().UTC()},
		{ID: core.NewID(), Type: core.NodeType("Package"), Name: "pkg2", Path: "/pkg2", CreatedAt: time.Now().UTC()},
		{ID: core.NewID(), Type: core.NodeType("Package"), Name: "pkg3", Path: "/pkg3", CreatedAt: time.Now().UTC()},
	}

	err := s.repo.CreateNodes(s.ctx, nodes)
	s.NoError(err)

	// Create relationships: pkg1 -> pkg2, pkg1 -> pkg3, pkg2 -> pkg3
	relationships := []core.Relationship{
		{
			ID:         core.NewID(),
			Type:       core.RelationType("IMPORTS"),
			FromNodeID: nodes[0].ID,
			ToNodeID:   nodes[1].ID,
			CreatedAt:  time.Now().UTC(),
		},
		{
			ID:         core.NewID(),
			Type:       core.RelationType("IMPORTS"),
			FromNodeID: nodes[0].ID,
			ToNodeID:   nodes[2].ID,
			CreatedAt:  time.Now().UTC(),
		},
		{
			ID:         core.NewID(),
			Type:       core.RelationType("DEPENDS_ON"),
			FromNodeID: nodes[1].ID,
			ToNodeID:   nodes[2].ID,
			CreatedAt:  time.Now().UTC(),
		},
	}

	err = s.repo.CreateRelationships(s.ctx, relationships)
	s.NoError(err)

	// Get IMPORTS relationships
	imports, err := s.repo.FindRelationshipsByType(s.ctx, core.RelationType("IMPORTS"))
	s.NoError(err)
	s.Len(imports, 2)

	// Get DEPENDS_ON relationships
	deps, err := s.repo.FindRelationshipsByType(s.ctx, core.RelationType("DEPENDS_ON"))
	s.NoError(err)
	s.Len(deps, 1)
}

// -----
// Analysis Operations Tests
// -----

func (s *Neo4jRepositoryTestSuite) TestImportAnalysisResult() {
	// Create analysis result
	pkgID := core.NewID()
	funcID := core.NewID()

	analysisResult := &core.AnalysisResult{
		ProjectID: core.NewID(),
		Nodes: []core.Node{
			{
				ID:        pkgID,
				Type:      core.NodeType("Package"),
				Name:      "main",
				Path:      "/src/main",
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        funcID,
				Type:      core.NodeType("Function"),
				Name:      "main",
				Path:      "/src/main.go",
				CreatedAt: time.Now().UTC(),
			},
		},
		Relationships: []core.Relationship{
			{
				ID:         core.NewID(),
				Type:       core.RelationType("CONTAINS"),
				FromNodeID: pkgID,
				ToNodeID:   funcID,
				CreatedAt:  time.Now().UTC(),
			},
		},
		TotalFiles:     1,
		TotalPackages:  1,
		TotalFunctions: 1,
		TotalStructs:   0,
		AnalyzedAt:     time.Now().UTC(),
	}

	// Fix relationship IDs
	analysisResult.Relationships[0].FromNodeID = analysisResult.Nodes[0].ID
	analysisResult.Relationships[0].ToNodeID = analysisResult.Nodes[1].ID

	// Import analysis
	err := s.repo.ImportAnalysisResult(s.ctx, analysisResult)
	s.NoError(err)

	// Verify nodes were created
	for _, node := range analysisResult.Nodes {
		result, err := s.repo.GetNode(s.ctx, node.ID)
		s.NoError(err)
		s.Equal(node.Name, result.Name)
	}

	// Verify relationships were created
	for _, rel := range analysisResult.Relationships {
		result, err := s.repo.GetRelationship(s.ctx, rel.ID)
		s.NoError(err)
		s.Equal(rel.Type, result.Type)
	}
}

// -----
// Advanced Query Tests
// -----

func (s *Neo4jRepositoryTestSuite) TestExecuteQuery() {
	// Create test data
	node := &core.Node{
		ID:        core.NewID(),
		Type:      core.NodeType("Function"),
		Name:      "testFunc",
		Path:      "/test.go",
		CreatedAt: time.Now().UTC(),
	}

	err := s.repo.CreateNode(s.ctx, node)
	s.NoError(err)

	// Execute custom query
	query := "MATCH (n:Function) WHERE n.name = $name RETURN n.id as id, n.name as name"
	params := map[string]any{"name": "testFunc"}

	results, err := s.repo.ExecuteQuery(s.ctx, query, params)
	s.NoError(err)
	s.Len(results, 1)

	// Verify result
	s.Equal(node.ID.String(), results[0]["id"])
	s.Equal("testFunc", results[0]["name"])
}

// -----
// Batch Operations Tests
// -----

func (s *Neo4jRepositoryTestSuite) TestBatchOperations() {
	// Create a large number of nodes
	nodeCount := 100
	nodes := make([]core.Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = core.Node{
			ID:        core.NewID(),
			Type:      core.NodeType("Function"),
			Name:      fmt.Sprintf("func%d", i),
			Path:      fmt.Sprintf("/src/func%d.go", i),
			CreatedAt: time.Now().UTC(),
		}
	}

	// Create nodes in batch
	err := s.repo.CreateNodes(s.ctx, nodes)
	s.NoError(err)

	// Verify all nodes were created
	functions, err := s.repo.FindNodesByType(s.ctx, core.NodeType("Function"))
	s.NoError(err)
	s.Len(functions, nodeCount)
}

// -----
// Error Handling Tests
// -----

func (s *Neo4jRepositoryTestSuite) TestDuplicateNodeCreation() {
	// Note: Neo4j doesn't enforce uniqueness by default unless we create constraints
	// This test verifies the actual behavior and shows how to detect duplicates
	nodeID := core.NewID()
	node := &core.Node{
		ID:        nodeID,
		Type:      core.NodeType("Function"),
		Name:      "duplicate",
		Path:      "/duplicate.go",
		CreatedAt: time.Now().UTC(),
	}

	err := s.repo.CreateNode(s.ctx, node)
	s.NoError(err)

	// Neo4j allows duplicate nodes by default
	err = s.repo.CreateNode(s.ctx, node)
	s.NoError(err)

	// Verify we can detect duplicates with a query
	query := `MATCH (n {id: $id}) RETURN count(n) as count`
	result, err := s.repo.ExecuteQuery(s.ctx, query, map[string]any{"id": nodeID.String()})
	s.NoError(err)
	s.NotEmpty(result)

	// Should have 2 nodes with the same ID
	s.Equal(int64(2), result[0]["count"])
}

func (s *Neo4jRepositoryTestSuite) TestRelationshipWithNonExistentNodes() {
	// Note: Neo4j's MATCH clause will simply not create the relationship if nodes don't exist
	// This test verifies that behavior
	rel := &core.Relationship{
		ID:         core.NewID(),
		Type:       core.RelationType("CALLS"),
		FromNodeID: core.NewID(), // Non-existent
		ToNodeID:   core.NewID(), // Non-existent
		CreatedAt:  time.Now().UTC(),
	}

	// The CreateRelationship uses MATCH, so it won't create anything if nodes don't exist
	err := s.repo.CreateRelationship(s.ctx, rel)
	s.NoError(err) // No error is returned, but the relationship won't be created

	// Verify the relationship wasn't created
	query := `MATCH ()-[r {id: $id}]->() RETURN r`
	result, err := s.repo.ExecuteQuery(s.ctx, query, map[string]any{"id": rel.ID.String()})
	s.NoError(err)
	s.Empty(result) // No relationship should exist
}
