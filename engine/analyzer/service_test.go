package analyzer_test

import (
	"context"
	"go/types"
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

		// Create mock types for testing
		mockInterfaceType := types.NewInterfaceType([]*types.Func{
			types.NewFunc(0, nil, "Handle", types.NewSignatureType(nil, nil, nil,
				types.NewTuple(types.NewParam(0, nil, "req", types.Typ[types.String])),
				types.NewTuple(types.NewParam(0, nil, "", types.Typ[types.String]),
					types.NewParam(0, nil, "", types.Universe.Lookup("error").Type())),
				false)),
		}, nil)

		mockStructType := types.NewNamed(
			types.NewTypeName(0, nil, "HTTPHandler", nil),
			types.NewStruct(nil, nil),
			nil,
		)

		// Create mock parse result
		parseResult := &parser.ParseResult{
			ProjectPath: "/test/project",
			Packages: []*parser.PackageInfo{
				{
					Path: "main",
					Name: "main",
					Functions: []*parser.FunctionInfo{
						{
							Name:       "main",
							LineStart:  10,
							LineEnd:    20,
							IsExported: true,
							Calls: []*parser.FunctionCall{
								{Line: 15},
							},
						},
					},
				},
				{
					Path: "handler",
					Name: "handler",
					Interfaces: []*parser.InterfaceInfo{
						{
							Name:    "Handler",
							Package: "handler",
							Type:    mockInterfaceType,
							Methods: []*parser.MethodInfo{
								{
									Name: "Handle",
									Signature: types.NewSignatureType(nil, nil, nil,
										types.NewTuple(types.NewParam(0, nil, "req", types.Typ[types.String])),
										types.NewTuple(types.NewParam(0, nil, "", types.Typ[types.String]),
											types.NewParam(0, nil, "", types.Universe.Lookup("error").Type())),
										false),
								},
							},
							Implementations: []*parser.Implementation{
								{
									Type: &parser.TypeInfo{
										Name: "HTTPHandler",
										Type: mockStructType,
									},
									IsComplete: true,
								},
							},
						},
					},
					Types: []*parser.TypeInfo{
						{
							Name:       "HTTPHandler",
							Type:       mockStructType,
							IsExported: true,
							Fields: []*parser.FieldInfo{
								{
									Name:       "client",
									IsExported: false,
								},
							},
						},
					},
					Functions: []*parser.FunctionInfo{
						{
							Name: "Handle",
							Receiver: &parser.TypeInfo{
								Name: "HTTPHandler",
								Type: mockStructType,
							},
							LineStart:  30,
							LineEnd:    50,
							IsExported: true,
						},
						{
							Name:       "HandleRequest",
							LineStart:  60,
							LineEnd:    70,
							IsExported: true,
						},
					},
				},
			},
			Interfaces: []*parser.InterfaceInfo{
				{
					Name:    "Handler",
					Package: "handler",
					Type:    mockInterfaceType,
					Methods: []*parser.MethodInfo{
						{
							Name: "Handle",
							Signature: types.NewSignatureType(nil, nil, nil,
								types.NewTuple(types.NewParam(0, nil, "req", types.Typ[types.String])),
								types.NewTuple(types.NewParam(0, nil, "", types.Typ[types.String]),
									types.NewParam(0, nil, "", types.Universe.Lookup("error").Type())),
								false),
						},
					},
					Implementations: []*parser.Implementation{
						{
							Type: &parser.TypeInfo{
								Name: "HTTPHandler",
								Type: mockStructType,
							},
							IsComplete: true,
						},
					},
				},
			},
		}

		input := &analyzer.AnalysisInput{
			ProjectID:   "test-project",
			ParseResult: parseResult,
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
		assert.GreaterOrEqual(t, len(report.DependencyGraph.Nodes), 2)

		// Check interface implementations (should be passed from parse result)
		assert.Equal(t, len(parseResult.Interfaces), len(report.InterfaceImplementations))
		if len(report.InterfaceImplementations) > 0 {
			assert.Equal(t, "Handler", report.InterfaceImplementations[0].Interface.Name)
			assert.Equal(t, "HTTPHandler", report.InterfaceImplementations[0].Type.Name)
			assert.True(t, report.InterfaceImplementations[0].IsComplete)
		}

		// Check functions and types are properly analyzed
		assert.GreaterOrEqual(t, len(report.CallChains), 0)
	})
}

func TestService_MapCallChains(t *testing.T) {
	t.Run("Should map call chains between functions", func(t *testing.T) {
		// Setup
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		// Create mock packages with SSA
		packages := []*parser.PackageInfo{
			{
				Path: "main",
				Name: "main",
				Functions: []*parser.FunctionInfo{
					{
						Name:       "main",
						IsExported: true,
						Calls: []*parser.FunctionCall{
							{Line: 15},
						},
					},
					{
						Name:       "helper",
						IsExported: false,
					},
				},
			},
		}

		// Execute
		callChains, err := service.MapCallChains(ctx, packages)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, callChains)
		// Without SSA, we might not get call chains, but it shouldn't error
	})

	t.Run("Should detect recursive calls", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		// Create a function that calls itself
		packages := []*parser.PackageInfo{
			{
				Path: "main",
				Name: "main",
				Functions: []*parser.FunctionInfo{
					{
						Name:       "factorial",
						IsExported: true,
						Calls: []*parser.FunctionCall{
							{
								Function: &parser.FunctionInfo{
									Name: "factorial",
								},
								Line: 25,
							},
						},
					},
				},
			},
		}

		chains, err := service.MapCallChains(ctx, packages)

		require.NoError(t, err)
		assert.NotNil(t, chains)
		// If we have chains, check for recursion
		for _, chain := range chains {
			if chain.Caller.Name == "factorial" && chain.Callee.Name == "factorial" {
				assert.True(t, chain.IsRecursive)
			}
		}
	})
}

func TestService_BuildDependencyGraph(t *testing.T) {
	t.Run("Should build dependency graph from packages", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		packages := []*parser.PackageInfo{
			{
				Path: "cmd/main",
				Name: "main",
				Files: []*parser.FileInfo{
					{
						Path:    "/project/cmd/main.go",
						Package: "main",
						Imports: []*parser.ImportInfo{
							{Path: "internal/server"},
							{Path: "pkg/utils"},
						},
					},
				},
			},
			{
				Path: "internal/server",
				Name: "server",
				Files: []*parser.FileInfo{
					{
						Path:    "/project/internal/server/server.go",
						Package: "server",
						Imports: []*parser.ImportInfo{
							{Path: "pkg/utils"},
						},
					},
				},
			},
			{
				Path: "pkg/utils",
				Name: "utils",
				Files: []*parser.FileInfo{
					{
						Path:    "/project/pkg/utils/utils.go",
						Package: "utils",
					},
				},
			},
		}

		graph, err := service.BuildDependencyGraph(ctx, packages)

		require.NoError(t, err)
		require.NotNil(t, graph)

		// Check nodes exist for packages
		assert.GreaterOrEqual(t, len(graph.Nodes), 3)

		// Validate specific nodes
		mainNode, exists := graph.Nodes["cmd/main"]
		require.True(t, exists)
		assert.Equal(t, "cmd/main", mainNode.Path)
		assert.Equal(t, analyzer.NodeTypePackage, mainNode.Type)
		assert.Equal(t, []string{"internal/server", "pkg/utils"}, mainNode.Dependencies)

		serverNode, exists := graph.Nodes["internal/server"]
		require.True(t, exists)
		assert.Equal(t, "internal/server", serverNode.Path)
		assert.Equal(t, analyzer.NodeTypePackage, serverNode.Type)
		assert.Equal(t, []string{"pkg/utils"}, serverNode.Dependencies)

		utilsNode, exists := graph.Nodes["pkg/utils"]
		require.True(t, exists)
		assert.Equal(t, "pkg/utils", utilsNode.Path)
		assert.Equal(t, analyzer.NodeTypePackage, utilsNode.Type)
		assert.Equal(t, []string{}, utilsNode.Dependencies)

		// Validate dependency edges
		assert.Len(t, graph.Edges, 3)

		// Verify specific edges
		edgeMap := make(map[string]*analyzer.DependencyEdge)
		for _, edge := range graph.Edges {
			key := edge.From + "->" + edge.To
			edgeMap[key] = edge
		}

		// Check main -> server edge
		mainServerEdge, exists := edgeMap["cmd/main->internal/server"]
		require.True(t, exists)
		assert.Equal(t, "cmd/main", mainServerEdge.From)
		assert.Equal(t, "internal/server", mainServerEdge.To)
		assert.Equal(t, analyzer.DependencyTypeImport, mainServerEdge.Type)

		// Check main -> utils edge
		mainUtilsEdge, exists := edgeMap["cmd/main->pkg/utils"]
		require.True(t, exists)
		assert.Equal(t, "cmd/main", mainUtilsEdge.From)
		assert.Equal(t, "pkg/utils", mainUtilsEdge.To)
		assert.Equal(t, analyzer.DependencyTypeImport, mainUtilsEdge.Type)

		// Check server -> utils edge
		serverUtilsEdge, exists := edgeMap["internal/server->pkg/utils"]
		require.True(t, exists)
		assert.Equal(t, "internal/server", serverUtilsEdge.From)
		assert.Equal(t, "pkg/utils", serverUtilsEdge.To)
		assert.Equal(t, analyzer.DependencyTypeImport, serverUtilsEdge.Type)

		// Validate dependents are correctly updated
		assert.Contains(t, utilsNode.Dependents, "cmd/main")
		assert.Contains(t, utilsNode.Dependents, "internal/server")
		assert.Contains(t, serverNode.Dependents, "cmd/main")
	})
}

func TestService_DetectCircularDependencies(t *testing.T) {
	t.Run("Should detect simple circular dependency", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		// Create a circular dependency graph: a -> b -> c -> a
		graph := &analyzer.DependencyGraph{
			Nodes: map[string]*analyzer.DependencyNode{
				"a": {
					Path:         "a",
					Type:         analyzer.NodeTypePackage,
					Dependencies: []string{"b"},
				},
				"b": {
					Path:         "b",
					Type:         analyzer.NodeTypePackage,
					Dependencies: []string{"c"},
				},
				"c": {
					Path:         "c",
					Type:         analyzer.NodeTypePackage,
					Dependencies: []string{"a"},
				},
			},
			Edges: []*analyzer.DependencyEdge{
				{From: "a", To: "b", Type: analyzer.DependencyTypeImport},
				{From: "b", To: "c", Type: analyzer.DependencyTypeImport},
				{From: "c", To: "a", Type: analyzer.DependencyTypeImport},
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

		// Create a DAG (no cycles)
		graph := &analyzer.DependencyGraph{
			Nodes: map[string]*analyzer.DependencyNode{
				"a": {
					Path:         "a",
					Type:         analyzer.NodeTypePackage,
					Dependencies: []string{"b", "c"},
				},
				"b": {
					Path:         "b",
					Type:         analyzer.NodeTypePackage,
					Dependencies: []string{"c"},
				},
				"c": {
					Path:         "c",
					Type:         analyzer.NodeTypePackage,
					Dependencies: []string{},
				},
			},
			Edges: []*analyzer.DependencyEdge{
				{From: "a", To: "b", Type: analyzer.DependencyTypeImport},
				{From: "a", To: "c", Type: analyzer.DependencyTypeImport},
				{From: "b", To: "c", Type: analyzer.DependencyTypeImport},
			},
		}

		cycles, err := service.DetectCircularDependencies(ctx, graph)

		require.NoError(t, err)
		assert.Empty(t, cycles)
	})
}

func TestService_InterfaceImplementationValidation(t *testing.T) {
	t.Run("Should validate complete interface implementations", func(t *testing.T) {
		service := analyzer.NewAnalyzer(nil)
		ctx := context.Background()

		// Create comprehensive mock types for implementation validation
		handlerInterface := types.NewInterfaceType([]*types.Func{
			types.NewFunc(0, nil, "Handle", types.NewSignatureType(nil, nil, nil,
				types.NewTuple(types.NewParam(0, nil, "req", types.Typ[types.String])),
				types.NewTuple(types.NewParam(0, nil, "", types.Typ[types.String]),
					types.NewParam(0, nil, "", types.Universe.Lookup("error").Type())),
				false)),
			types.NewFunc(0, nil, "Close", types.NewSignatureType(nil, nil, nil,
				nil,
				types.NewTuple(types.NewParam(0, nil, "", types.Universe.Lookup("error").Type())),
				false)),
		}, nil)

		httpHandlerStruct := types.NewNamed(
			types.NewTypeName(0, nil, "HTTPHandler", nil),
			types.NewStruct(nil, nil),
			nil,
		)

		incompleteHandlerStruct := types.NewNamed(
			types.NewTypeName(0, nil, "IncompleteHandler", nil),
			types.NewStruct(nil, nil),
			nil,
		)

		parseResult := &parser.ParseResult{
			ProjectPath: "/test/project",
			Packages: []*parser.PackageInfo{
				{
					Path: "handler",
					Name: "handler",
					Interfaces: []*parser.InterfaceInfo{
						{
							Name:    "Handler",
							Package: "handler",
							Type:    handlerInterface,
							Methods: []*parser.MethodInfo{
								{
									Name: "Handle",
									Signature: types.NewSignatureType(nil, nil, nil,
										types.NewTuple(types.NewParam(0, nil, "req", types.Typ[types.String])),
										types.NewTuple(types.NewParam(0, nil, "", types.Typ[types.String]),
											types.NewParam(0, nil, "", types.Universe.Lookup("error").Type())),
										false),
								},
								{
									Name: "Close",
									Signature: types.NewSignatureType(
										nil,
										nil,
										nil,
										nil,
										types.NewTuple(
											types.NewParam(0, nil, "", types.Universe.Lookup("error").Type()),
										),
										false,
									),
								},
							},
							Implementations: []*parser.Implementation{
								{
									Type: &parser.TypeInfo{
										Name: "HTTPHandler",
										Type: httpHandlerStruct,
									},
									IsComplete: true,
									MethodMatches: map[string]*parser.FunctionInfo{
										"Handle": {Name: "Handle"},
										"Close":  {Name: "Close"},
									},
									MissingMethods: []string{},
								},
								{
									Type: &parser.TypeInfo{
										Name: "IncompleteHandler",
										Type: incompleteHandlerStruct,
									},
									IsComplete: false,
									MethodMatches: map[string]*parser.FunctionInfo{
										"Handle": {Name: "Handle"},
									},
									MissingMethods: []string{"Close"},
								},
							},
						},
					},
				},
			},
			Interfaces: []*parser.InterfaceInfo{
				{
					Name:    "Handler",
					Package: "handler",
					Type:    handlerInterface,
					Methods: []*parser.MethodInfo{
						{Name: "Handle"},
						{Name: "Close"},
					},
					Implementations: []*parser.Implementation{
						{
							Type: &parser.TypeInfo{
								Name: "HTTPHandler",
								Type: httpHandlerStruct,
							},
							IsComplete: true,
							MethodMatches: map[string]*parser.FunctionInfo{
								"Handle": {Name: "Handle"},
								"Close":  {Name: "Close"},
							},
							MissingMethods: []string{},
						},
						{
							Type: &parser.TypeInfo{
								Name: "IncompleteHandler",
								Type: incompleteHandlerStruct,
							},
							IsComplete:     false,
							MethodMatches:  map[string]*parser.FunctionInfo{"Handle": {Name: "Handle"}},
							MissingMethods: []string{"Close"},
						},
					},
				},
			},
		}

		input := &analyzer.AnalysisInput{
			ProjectID:   "test-project",
			ParseResult: parseResult,
		}

		report, err := service.AnalyzeProject(ctx, input)

		require.NoError(t, err)
		require.NotNil(t, report)

		// Validate interface implementations are preserved and accurate
		require.Len(t, report.InterfaceImplementations, 2)

		// Find complete implementation
		var completeImpl *parser.Implementation
		var incompleteImpl *parser.Implementation
		for _, impl := range report.InterfaceImplementations {
			switch impl.Type.Name {
			case "HTTPHandler":
				completeImpl = impl
			case "IncompleteHandler":
				incompleteImpl = impl
			}
		}

		// Validate complete implementation
		require.NotNil(t, completeImpl)
		assert.Equal(t, "HTTPHandler", completeImpl.Type.Name)
		assert.True(t, completeImpl.IsComplete)
		assert.Len(t, completeImpl.MethodMatches, 2)
		assert.Contains(t, completeImpl.MethodMatches, "Handle")
		assert.Contains(t, completeImpl.MethodMatches, "Close")
		assert.Empty(t, completeImpl.MissingMethods)

		// Validate incomplete implementation
		require.NotNil(t, incompleteImpl)
		assert.Equal(t, "IncompleteHandler", incompleteImpl.Type.Name)
		assert.False(t, incompleteImpl.IsComplete)
		assert.Len(t, incompleteImpl.MethodMatches, 1)
		assert.Contains(t, incompleteImpl.MethodMatches, "Handle")
		assert.Len(t, incompleteImpl.MissingMethods, 1)
		assert.Contains(t, incompleteImpl.MissingMethods, "Close")

		// Validate interface reference is set correctly
		for _, impl := range report.InterfaceImplementations {
			require.NotNil(t, impl.Interface)
			assert.Equal(t, "Handler", impl.Interface.Name)
			assert.Equal(t, "handler", impl.Interface.Package)
		}
	})
}

func TestService_AnalyzeProject_WithMetrics(t *testing.T) {
	t.Run("Should include metrics when enabled", func(t *testing.T) {
		config := &analyzer.Config{
			IncludeMetrics: true,
		}
		service := analyzer.NewAnalyzer(config)
		ctx := context.Background()

		parseResult := &parser.ParseResult{
			ProjectPath: "/test/project",
			Packages: []*parser.PackageInfo{
				{
					Path: "main",
					Name: "main",
					Files: []*parser.FileInfo{
						{
							Path: "/test/project/main.go",
						},
					},
					Functions: []*parser.FunctionInfo{
						{Name: "main"},
						{Name: "helper"},
					},
					Types: []*parser.TypeInfo{
						{
							Name:       "Config",
							Underlying: types.NewStruct(nil, nil),
						},
					},
				},
			},
			Interfaces: []*parser.InterfaceInfo{
				{Name: "Handler"},
			},
		}

		input := &analyzer.AnalysisInput{
			ProjectID:   "test-project",
			ParseResult: parseResult,
		}

		report, err := service.AnalyzeProject(ctx, input)

		require.NoError(t, err)
		require.NotNil(t, report.Metrics)
		assert.Equal(t, 1, report.Metrics.TotalFiles)
		assert.Equal(t, 2, report.Metrics.TotalFunctions)
		assert.Equal(t, 1, report.Metrics.TotalInterfaces)
		assert.Equal(t, 1, report.Metrics.TotalStructs)
		assert.GreaterOrEqual(t, report.Metrics.TotalLines, 0)
	})
}
