package analyzer_test

import (
	"context"
	"testing"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/compozy/gograph/engine/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_AnalyzeProject(t *testing.T) {
	t.Run("Should analyze project with all components", func(t *testing.T) {
		// Setup
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		input := &analyzer.AnalysisInput{
			ProjectID: "test-project",
			Files: []*parser.FileInfo{
				{
					Path:         "/project/main.go",
					Package:      "main",
					Dependencies: []string{"/project/handler/handler.go"},
					Functions: []parser.FunctionInfo{
						{
							Name:      "main",
							LineStart: 10,
							LineEnd:   20,
							Calls: []parser.FunctionCall{
								{Package: "handler", Name: "HandleRequest", Line: 15},
							},
						},
					},
				},
				{
					Path:         "/project/handler/handler.go",
					Package:      "handler",
					Dependencies: []string{},
					Interfaces: []parser.InterfaceInfo{
						{
							Name: "Handler",
							Methods: []parser.MethodInfo{
								{
									Name:       "Handle",
									Parameters: []parser.Parameter{{Name: "req", Type: "Request"}},
									Returns:    []string{"Response", "error"},
								},
							},
						},
					},
					Structs: []parser.StructInfo{
						{
							Name: "HTTPHandler",
							Fields: []parser.FieldInfo{
								{Name: "client", Type: "http.Client", IsExported: false},
							},
						},
					},
					Functions: []parser.FunctionInfo{
						{
							Name:       "Handle",
							Receiver:   "HTTPHandler",
							Parameters: []parser.Parameter{{Name: "req", Type: "Request"}},
							Returns:    []string{"Response", "error"},
							LineStart:  30,
							LineEnd:    50,
						},
						{
							Name:       "HandleRequest",
							Parameters: []parser.Parameter{{Name: "req", Type: "Request"}},
							Returns:    []string{"error"},
							LineStart:  60,
							LineEnd:    70,
						},
					},
				},
			},
		}

		// Execute
		report, err := service.AnalyzeProject(ctx, input)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, report)
		assert.Equal(t, "test-project", report.ProjectID)
		assert.Greater(t, report.Timestamp, int64(0))

		// Check dependency graph
		assert.NotNil(t, report.DependencyGraph)
		assert.Len(t, report.DependencyGraph.Nodes, 2)
		assert.Len(t, report.DependencyGraph.Edges, 1)

		// Check interface implementations
		assert.Len(t, report.InterfaceImplementations, 1)
		assert.Equal(t, "Handler", report.InterfaceImplementations[0].Interface.Name)
		assert.Equal(t, "HTTPHandler", report.InterfaceImplementations[0].Implementor.Name)
		assert.True(t, report.InterfaceImplementations[0].IsComplete)

		// Check call chains
		assert.Len(t, report.CallChains, 1)
		assert.Equal(t, "main", report.CallChains[0].Caller.Name)
		assert.Equal(t, "HandleRequest", report.CallChains[0].Callee.Name)

		// Check circular dependencies
		assert.Empty(t, report.CircularDependencies)
	})

	t.Run("Should include metrics when enabled", func(t *testing.T) {
		config := &analyzer.Config{
			IncludeMetrics: true,
		}
		service := analyzer.NewAnalyzer(config)
		ctx := context.Background()

		input := &analyzer.AnalysisInput{
			ProjectID: "test-project",
			Files: []*parser.FileInfo{
				{
					Path:    "/project/main.go",
					Package: "main",
					Functions: []parser.FunctionInfo{
						{Name: "main", LineStart: 1, LineEnd: 10},
						{Name: "helper", LineStart: 12, LineEnd: 20},
					},
					Interfaces: []parser.InterfaceInfo{
						{Name: "Worker"},
					},
					Structs: []parser.StructInfo{
						{Name: "Config"},
					},
				},
			},
		}

		report, err := service.AnalyzeProject(ctx, input)

		require.NoError(t, err)
		require.NotNil(t, report.Metrics)
		assert.Equal(t, 1, report.Metrics.TotalFiles)
		assert.Equal(t, 2, report.Metrics.TotalFunctions)
		assert.Equal(t, 1, report.Metrics.TotalInterfaces)
		assert.Equal(t, 1, report.Metrics.TotalStructs)
		assert.Greater(t, report.Metrics.TotalLines, 0)
	})
}

func TestService_BuildDependencyGraph(t *testing.T) {
	t.Run("Should build dependency graph correctly", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		files := []*parser.FileInfo{
			{
				Path:         "/project/cmd/main.go",
				Package:      "main",
				Dependencies: []string{"/project/internal/server/server.go", "/project/pkg/utils/utils.go"},
			},
			{
				Path:         "/project/internal/server/server.go",
				Package:      "server",
				Dependencies: []string{"/project/pkg/utils/utils.go"},
			},
			{
				Path:         "/project/pkg/utils/utils.go",
				Package:      "utils",
				Dependencies: []string{},
			},
		}

		graph, err := service.BuildDependencyGraph(ctx, files)

		require.NoError(t, err)
		require.NotNil(t, graph)

		// Check nodes
		assert.Len(t, graph.Nodes, 3)

		mainNode := graph.Nodes["/project/cmd/main.go"]
		require.NotNil(t, mainNode)
		assert.Equal(t, analyzer.NodeTypeFile, mainNode.Type)
		assert.Len(t, mainNode.Dependencies, 2)
		assert.Empty(t, mainNode.Dependents)

		serverNode := graph.Nodes["/project/internal/server/server.go"]
		require.NotNil(t, serverNode)
		assert.Len(t, serverNode.Dependencies, 1)
		assert.Len(t, serverNode.Dependents, 1)
		assert.Contains(t, serverNode.Dependents, "/project/cmd/main.go")

		utilsNode := graph.Nodes["/project/pkg/utils/utils.go"]
		require.NotNil(t, utilsNode)
		assert.Empty(t, utilsNode.Dependencies)
		assert.Len(t, utilsNode.Dependents, 2)

		// Check edges
		assert.Len(t, graph.Edges, 3)
	})
}

func TestService_DetectInterfaceImplementations(t *testing.T) {
	t.Run("Should detect complete interface implementation", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		files := []*parser.FileInfo{
			{
				Path:    "/project/interfaces.go",
				Package: "project",
				Interfaces: []parser.InterfaceInfo{
					{
						Name: "Writer",
						Methods: []parser.MethodInfo{
							{
								Name:       "Write",
								Parameters: []parser.Parameter{{Name: "data", Type: "[]byte"}},
								Returns:    []string{"int", "error"},
							},
						},
					},
				},
			},
			{
				Path:    "/project/impl.go",
				Package: "project",
				Structs: []parser.StructInfo{
					{
						Name: "FileWriter",
						Fields: []parser.FieldInfo{
							{Name: "path", Type: "string"},
						},
					},
				},
				Functions: []parser.FunctionInfo{
					{
						Name:       "Write",
						Receiver:   "FileWriter",
						Parameters: []parser.Parameter{{Name: "data", Type: "[]byte"}},
						Returns:    []string{"int", "error"},
					},
				},
			},
		}

		implementations, err := service.DetectInterfaceImplementations(ctx, files)

		require.NoError(t, err)
		require.Len(t, implementations, 1)

		impl := implementations[0]
		assert.Equal(t, "Writer", impl.Interface.Name)
		assert.Equal(t, "FileWriter", impl.Implementor.Name)
		assert.True(t, impl.IsComplete)
		assert.Empty(t, impl.MissingMethods)
		assert.Len(t, impl.Methods, 1)
		assert.Equal(t, "Write", impl.Methods[0].InterfaceMethod)
	})

	t.Run("Should detect partial interface implementation", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		files := []*parser.FileInfo{
			{
				Path:    "/project/interfaces.go",
				Package: "project",
				Interfaces: []parser.InterfaceInfo{
					{
						Name: "ReadWriter",
						Methods: []parser.MethodInfo{
							{
								Name:       "Read",
								Parameters: []parser.Parameter{{Name: "buf", Type: "[]byte"}},
								Returns:    []string{"int", "error"},
							},
							{
								Name:       "Write",
								Parameters: []parser.Parameter{{Name: "data", Type: "[]byte"}},
								Returns:    []string{"int", "error"},
							},
						},
					},
				},
			},
			{
				Path:    "/project/impl.go",
				Package: "project",
				Structs: []parser.StructInfo{
					{
						Name: "PartialImpl",
					},
				},
				Functions: []parser.FunctionInfo{
					{
						Name:       "Write",
						Receiver:   "PartialImpl",
						Parameters: []parser.Parameter{{Name: "data", Type: "[]byte"}},
						Returns:    []string{"int", "error"},
					},
				},
			},
		}

		implementations, err := service.DetectInterfaceImplementations(ctx, files)

		require.NoError(t, err)
		require.Len(t, implementations, 1)

		impl := implementations[0]
		assert.Equal(t, "ReadWriter", impl.Interface.Name)
		assert.Equal(t, "PartialImpl", impl.Implementor.Name)
		assert.False(t, impl.IsComplete)
		assert.Len(t, impl.MissingMethods, 1)
		assert.Equal(t, "Read", impl.MissingMethods[0])
	})
}

func TestService_MapCallChains(t *testing.T) {
	t.Run("Should map function call chains", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		files := []*parser.FileInfo{
			{
				Path:    "/project/main.go",
				Package: "main",
				Functions: []parser.FunctionInfo{
					{
						Name: "main",
						Calls: []parser.FunctionCall{
							{Package: "main", Name: "processData", Line: 15},
						},
					},
					{
						Name:       "processData",
						Parameters: []parser.Parameter{{Name: "data", Type: "string"}},
						Returns:    []string{"error"},
						Calls: []parser.FunctionCall{
							{Package: "main", Name: "validateData", Line: 25},
						},
					},
					{
						Name:       "validateData",
						Parameters: []parser.Parameter{{Name: "data", Type: "string"}},
						Returns:    []string{"bool"},
					},
				},
			},
		}

		chains, err := service.MapCallChains(ctx, files)

		require.NoError(t, err)
		require.Len(t, chains, 2)

		// Find chains by caller/callee names (order independent)
		var mainToProcess, processToValidate *analyzer.CallChain
		for _, chain := range chains {
			if chain.Caller.Name == "main" && chain.Callee.Name == "processData" {
				mainToProcess = chain
			} else if chain.Caller.Name == "processData" && chain.Callee.Name == "validateData" {
				processToValidate = chain
			}
		}

		// Check main -> processData chain
		require.NotNil(t, mainToProcess, "Should find main->processData chain")
		assert.False(t, mainToProcess.IsRecursive)
		assert.Len(t, mainToProcess.CallSites, 1)
		assert.Equal(t, 15, mainToProcess.CallSites[0].Line)

		// Check processData -> validateData chain
		require.NotNil(t, processToValidate, "Should find processData->validateData chain")
		assert.False(t, processToValidate.IsRecursive)
		assert.Len(t, processToValidate.CallSites, 1)
		assert.Equal(t, 25, processToValidate.CallSites[0].Line)
	})

	t.Run("Should detect recursive calls", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		files := []*parser.FileInfo{
			{
				Path:    "/project/recursive.go",
				Package: "main",
				Functions: []parser.FunctionInfo{
					{
						Name:       "factorial",
						Parameters: []parser.Parameter{{Name: "n", Type: "int"}},
						Returns:    []string{"int"},
						Calls: []parser.FunctionCall{
							{Package: "main", Name: "factorial", Line: 10},
						},
					},
				},
			},
		}

		chains, err := service.MapCallChains(ctx, files)

		require.NoError(t, err)
		require.Len(t, chains, 1)

		chain := chains[0]
		assert.Equal(t, "factorial", chain.Caller.Name)
		assert.Equal(t, "factorial", chain.Callee.Name)
		assert.True(t, chain.IsRecursive)
	})
}

func TestService_DetectCircularDependencies(t *testing.T) {
	t.Run("Should detect simple circular dependency", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		graph := &analyzer.DependencyGraph{
			Nodes: map[string]*analyzer.DependencyNode{
				"a": {Path: "a", Dependencies: []string{"b"}},
				"b": {Path: "b", Dependencies: []string{"c"}},
				"c": {Path: "c", Dependencies: []string{"a"}},
			},
		}

		cycles, err := service.DetectCircularDependencies(ctx, graph)

		require.NoError(t, err)
		require.Len(t, cycles, 1)

		cycle := cycles[0]
		assert.Equal(t, analyzer.SeverityHigh, cycle.Severity)
		assert.Len(t, cycle.Cycle, 3)
		assert.Contains(t, cycle.Cycle, "a")
		assert.Contains(t, cycle.Cycle, "b")
		assert.Contains(t, cycle.Cycle, "c")
	})

	t.Run("Should handle no circular dependencies", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		graph := &analyzer.DependencyGraph{
			Nodes: map[string]*analyzer.DependencyNode{
				"a": {Path: "a", Dependencies: []string{"b", "c"}},
				"b": {Path: "b", Dependencies: []string{"c"}},
				"c": {Path: "c", Dependencies: []string{}},
			},
		}

		cycles, err := service.DetectCircularDependencies(ctx, graph)

		require.NoError(t, err)
		assert.Empty(t, cycles)
	})
}

func TestService_EdgeCases(t *testing.T) {
	t.Run("Should handle empty input gracefully", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		input := &analyzer.AnalysisInput{
			ProjectID: "empty-project",
			Files:     []*parser.FileInfo{},
		}

		report, err := service.AnalyzeProject(ctx, input)

		require.NoError(t, err)
		require.NotNil(t, report)
		assert.Equal(t, "empty-project", report.ProjectID)
		assert.NotNil(t, report.DependencyGraph)
		assert.Len(t, report.DependencyGraph.Nodes, 0)
		assert.Empty(t, report.InterfaceImplementations)
		assert.Empty(t, report.CallChains)
		assert.Empty(t, report.CircularDependencies)
	})

	t.Run("Should handle nil config with defaults", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		assert.NotNil(t, service)
	})

	t.Run("Should handle files with no dependencies", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		files := []*parser.FileInfo{
			{Path: "/project/a.go", Package: "a", Dependencies: []string{}},
			{Path: "/project/b.go", Package: "b", Dependencies: []string{}},
			{Path: "/project/c.go", Package: "c", Dependencies: []string{}},
		}

		graph, err := service.BuildDependencyGraph(ctx, files)

		require.NoError(t, err)
		assert.Len(t, graph.Nodes, 3)
		assert.Empty(t, graph.Edges)
		for _, node := range graph.Nodes {
			assert.Empty(t, node.Dependencies)
			assert.Empty(t, node.Dependents)
		}
	})

	t.Run("Should handle complex method signatures in interface implementation", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		files := []*parser.FileInfo{
			{
				Path:    "/project/interfaces.go",
				Package: "project",
				Interfaces: []parser.InterfaceInfo{
					{
						Name: "ComplexInterface",
						Methods: []parser.MethodInfo{
							{
								Name: "ProcessData",
								Parameters: []parser.Parameter{
									{Name: "ctx", Type: "context.Context"},
									{Name: "data", Type: "[]byte"},
									{Name: "opts", Type: "*ProcessOptions"},
								},
								Returns: []string{"*Result", "error"},
							},
						},
					},
				},
			},
			{
				Path:    "/project/impl.go",
				Package: "project",
				Structs: []parser.StructInfo{
					{Name: "Processor"},
				},
				Functions: []parser.FunctionInfo{
					{
						Name:     "ProcessData",
						Receiver: "Processor",
						Parameters: []parser.Parameter{
							{Name: "ctx", Type: "context.Context"},
							{Name: "data", Type: "[]byte"},
							{Name: "opts", Type: "*ProcessOptions"},
						},
						Returns: []string{"*Result", "error"},
					},
				},
			},
		}

		implementations, err := service.DetectInterfaceImplementations(ctx, files)

		require.NoError(t, err)
		require.Len(t, implementations, 1)
		assert.True(t, implementations[0].IsComplete)
	})

	t.Run("Should detect complex circular dependencies", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		// Create a graph with multiple cycles
		graph := &analyzer.DependencyGraph{
			Nodes: map[string]*analyzer.DependencyNode{
				"a": {Path: "a", Dependencies: []string{"b", "d"}},
				"b": {Path: "b", Dependencies: []string{"c"}},
				"c": {Path: "c", Dependencies: []string{"a"}},
				"d": {Path: "d", Dependencies: []string{"e"}},
				"e": {Path: "e", Dependencies: []string{"d"}},
			},
		}

		cycles, err := service.DetectCircularDependencies(ctx, graph)

		require.NoError(t, err)
		// Should detect at least 2 cycles: a->b->c->a and d->e->d
		assert.GreaterOrEqual(t, len(cycles), 2)
	})

	t.Run("Should handle external dependencies in graph", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		files := []*parser.FileInfo{
			{
				Path:    "/project/main.go",
				Package: "main",
				Dependencies: []string{
					"/project/internal/service.go",
					"fmt",                      // External stdlib
					"github.com/gin-gonic/gin", // External package
				},
			},
			{
				Path:         "/project/internal/service.go",
				Package:      "service",
				Dependencies: []string{"database/sql"}, // External stdlib
			},
		}

		graph, err := service.BuildDependencyGraph(ctx, files)

		require.NoError(t, err)
		// Should only create nodes for internal files
		assert.Len(t, graph.Nodes, 2)
		// Should still create edges for all dependencies
		assert.Len(t, graph.Edges, 4)
	})

	t.Run("Should handle methods with pointer receivers", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		files := []*parser.FileInfo{
			{
				Path:    "/project/interfaces.go",
				Package: "project",
				Interfaces: []parser.InterfaceInfo{
					{
						Name: "Stringer",
						Methods: []parser.MethodInfo{
							{
								Name:    "String",
								Returns: []string{"string"},
							},
						},
					},
				},
			},
			{
				Path:    "/project/types.go",
				Package: "project",
				Structs: []parser.StructInfo{
					{Name: "MyType"},
				},
				Functions: []parser.FunctionInfo{
					{
						Name:     "String",
						Receiver: "MyType", // Value receiver
						Returns:  []string{"string"},
					},
				},
			},
			{
				Path:    "/project/other_types.go",
				Package: "project",
				Structs: []parser.StructInfo{
					{Name: "OtherType"},
				},
				Functions: []parser.FunctionInfo{
					{
						Name:     "String",
						Receiver: "*OtherType", // Pointer receiver
						Returns:  []string{"string"},
					},
				},
			},
		}

		implementations, err := service.DetectInterfaceImplementations(ctx, files)

		require.NoError(t, err)
		// Note: The current implementation only matches exact receiver types
		// So it will only find MyType with value receiver, not *OtherType
		assert.Len(t, implementations, 1)
		assert.Equal(t, "MyType", implementations[0].Implementor.Name)
	})

	t.Run("Should handle multiple calls from same function", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		files := []*parser.FileInfo{
			{
				Path:    "/project/main.go",
				Package: "main",
				Functions: []parser.FunctionInfo{
					{
						Name: "main",
						Calls: []parser.FunctionCall{
							{Package: "main", Name: "init", Line: 10},
							{Package: "main", Name: "run", Line: 11},
							{Package: "main", Name: "cleanup", Line: 15},
							{Package: "main", Name: "cleanup", Line: 20}, // Called twice
						},
					},
					{Name: "init"},
					{Name: "run"},
					{Name: "cleanup"},
				},
			},
		}

		chains, err := service.MapCallChains(ctx, files)

		require.NoError(t, err)
		// Should create separate chain entries for each unique call
		assert.Len(t, chains, 4)

		// Find the cleanup calls
		var cleanupChains []*analyzer.CallChain
		for _, chain := range chains {
			if chain.Callee.Name == "cleanup" {
				cleanupChains = append(cleanupChains, chain)
			}
		}
		assert.Len(t, cleanupChains, 2)
	})
}

func TestService_GetStructMethods(t *testing.T) {
	t.Run("Should get methods from multiple files", func(t *testing.T) {
		// This test would test the private getStructMethods method
		// In production code, we test this through the public API
		// (DetectInterfaceImplementations uses getStructMethods internally)
		t.Skip("Private method - tested through public API")
	})
}
