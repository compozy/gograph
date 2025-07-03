package analyzer

import (
	"context"
	"fmt"
	"go/types"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/compozy/gograph/engine/parser"
	"github.com/compozy/gograph/pkg/logger"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
)

// service implements the Analyzer interface
type service struct {
	config *Config
	log    *log.Logger
}

// NewAnalyzer creates a new analyzer service
func NewAnalyzer(config *Config) Analyzer {
	if config == nil {
		config = DefaultAnalyzerConfig()
	}
	return &service{
		config: config,
		log:    log.New(os.Stderr),
	}
}

// AnalyzeProject performs comprehensive analysis on parsed project data
func (s *service) AnalyzeProject(ctx context.Context, input *AnalysisInput) (*AnalysisReport, error) {
	logger.Info("starting project analysis", "project_id", input.ProjectID, "packages", len(input.ParseResult.Packages))
	startTime := time.Now()

	report := &AnalysisReport{
		ProjectID: input.ProjectID,
		Timestamp: time.Now().Unix(),
	}

	// Build dependency graph
	depGraph, err := s.BuildDependencyGraph(ctx, input.ParseResult.Packages)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}
	report.DependencyGraph = depGraph

	// Interface implementations are already detected by the parser with proper type checking
	report.InterfaceImplementations = s.collectImplementations(input.ParseResult)

	// Map call chains
	callChains, err := s.MapCallChains(ctx, input.ParseResult.Packages)
	if err != nil {
		return nil, fmt.Errorf("failed to map call chains: %w", err)
	}
	report.CallChains = callChains

	// Detect circular dependencies
	circularDeps, err := s.DetectCircularDependencies(ctx, depGraph)
	if err != nil {
		return nil, fmt.Errorf("failed to detect circular dependencies: %w", err)
	}
	report.CircularDependencies = circularDeps

	// Calculate metrics if enabled
	if s.config.IncludeMetrics {
		report.Metrics = s.calculateMetrics(input.ParseResult)
	}

	logger.Info("project analysis complete", "duration", time.Since(startTime))
	return report, nil
}

// BuildDependencyGraph constructs a dependency graph from parsed packages
func (s *service) BuildDependencyGraph(_ context.Context, packages []*parser.PackageInfo) (*DependencyGraph, error) {
	graph := &DependencyGraph{
		Nodes: make(map[string]*DependencyNode),
		Edges: []*DependencyEdge{},
	}

	// Create nodes for each package
	for _, pkg := range packages {
		node := &DependencyNode{
			Path:         pkg.Path,
			Type:         NodeTypePackage,
			Dependencies: []string{},
			Dependents:   []string{},
		}

		// Extract unique dependencies from all files
		depMap := make(map[string]bool)
		for _, file := range pkg.Files {
			for _, imp := range file.Imports {
				depMap[imp.Path] = true
			}
		}

		for dep := range depMap {
			node.Dependencies = append(node.Dependencies, dep)
		}

		graph.Nodes[pkg.Path] = node
	}

	// Build edges and update dependents
	for pkgPath, node := range graph.Nodes {
		for _, dep := range node.Dependencies {
			edge := &DependencyEdge{
				From: pkgPath,
				To:   dep,
				Type: DependencyTypeImport,
			}
			graph.Edges = append(graph.Edges, edge)

			// Update dependents if the dependency is in our graph
			if depNode, exists := graph.Nodes[dep]; exists {
				depNode.Dependents = append(depNode.Dependents, pkgPath)
			}
		}
	}

	return graph, nil
}

// MapCallChains traces function call relationships using SSA if available
func (s *service) MapCallChains(_ context.Context, packages []*parser.PackageInfo) ([]*CallChain, error) {
	callChains := make([]*CallChain, 0)

	// Check if we have SSA information
	var ssaProg *ssa.Program
	for _, pkg := range packages {
		if pkg.SSAPackage != nil && pkg.SSAPackage.Prog != nil {
			ssaProg = pkg.SSAPackage.Prog
			break
		}
	}

	if ssaProg != nil {
		// Use SSA-based call graph analysis
		logger.Info("Building call graph using SSA analysis")

		// Find main packages
		var mains []*ssa.Package
		for _, pkg := range packages {
			if pkg.SSAPackage != nil && pkg.SSAPackage.Func("main") != nil {
				mains = append(mains, pkg.SSAPackage)
			}
		}

		if len(mains) > 0 {
			// Build call graph using Rapid Type Analysis
			// RTA requires main functions as roots
			var roots []*ssa.Function
			for _, pkg := range mains {
				if main := pkg.Func("main"); main != nil {
					roots = append(roots, main)
				}
			}

			if len(roots) > 0 {
				result := rta.Analyze(roots, true)
				cg := result.CallGraph

				// Convert callgraph to our CallChain format
				callChains = s.convertCallGraph(cg, packages, ssaProg)
			}
		}
	} else {
		// Fallback to AST-based analysis
		logger.Info("Building call graph using AST analysis")
		callChains = s.mapCallChainsFromAST(packages)
	}

	logger.Info("Mapped call chains", "count", len(callChains))
	return callChains, nil
}

// convertCallGraph converts SSA call graph to our CallChain format
func (s *service) convertCallGraph(
	cg *callgraph.Graph,
	packages []*parser.PackageInfo,
	ssaProg *ssa.Program,
) []*CallChain {
	var chains []*CallChain

	// Create a map of SSA functions to our FunctionInfo
	funcMap := make(map[*ssa.Function]*parser.FunctionInfo)
	for _, pkg := range packages {
		for _, fn := range pkg.Functions {
			if fn.SSAFunc != nil {
				funcMap[fn.SSAFunc] = fn
			}
		}
	}

	// Process each edge in the call graph
	for _, node := range cg.Nodes {
		for _, edge := range node.Out {
			caller := s.createFunctionReference(edge.Caller.Func, funcMap)
			callee := s.createFunctionReference(edge.Callee.Func, funcMap)

			if caller != nil && callee != nil {
				// Convert token.Pos to actual line number
				line := 0
				if edge.Pos().IsValid() && ssaProg != nil && ssaProg.Fset != nil {
					pos := ssaProg.Fset.Position(edge.Pos())
					line = pos.Line
				}

				chain := &CallChain{
					Caller: caller,
					Callee: callee,
					CallSites: []CallSite{
						{
							Line: line,
						},
					},
					IsRecursive: edge.Caller.Func == edge.Callee.Func,
				}
				chains = append(chains, chain)
			}
		}
	}

	return chains
}

// createFunctionReference creates a FunctionReference from SSA function
func (s *service) createFunctionReference(
	ssaFunc *ssa.Function,
	_ map[*ssa.Function]*parser.FunctionInfo,
) *FunctionReference {
	if ssaFunc == nil {
		return nil
	}

	ref := &FunctionReference{
		Name:      ssaFunc.Name(),
		Signature: ssaFunc.Signature.String(),
	}

	// Extract package name
	if ssaFunc.Pkg != nil {
		ref.Package = ssaFunc.Pkg.Pkg.Path()
	}

	// Extract receiver for methods
	if ssaFunc.Signature.Recv() != nil {
		ref.Receiver = ssaFunc.Signature.Recv().Type().String()
	}

	return ref
}

// mapCallChainsFromAST maps call chains using AST information
func (s *service) mapCallChainsFromAST(packages []*parser.PackageInfo) []*CallChain {
	chains := make([]*CallChain, 0)

	// Build function index
	funcIndex := make(map[string]*parser.FunctionInfo)
	for _, pkg := range packages {
		for _, fn := range pkg.Functions {
			key := s.getFunctionKey(pkg.Path, fn.Name, fn.Receiver)
			funcIndex[key] = fn
		}
	}

	// Process each function's calls
	for _, pkg := range packages {
		for _, fn := range pkg.Functions {
			callerRef := &FunctionReference{
				Name:      fn.Name,
				Package:   pkg.Path,
				Signature: s.getSignatureString(fn),
			}

			if fn.Receiver != nil {
				callerRef.Receiver = fn.Receiver.Name
			}

			// Process each call
			for _, call := range fn.Calls {
				if call.Function != nil {
					// Determine the actual package of the callee
					calleePackage := pkg.Path
					if call.Function.Receiver != nil && call.Function.Receiver.Type != nil {
						// For method calls, the package might be different
						if named, ok := call.Function.Receiver.Type.(*types.Named); ok {
							if pkg := named.Obj().Pkg(); pkg != nil {
								calleePackage = pkg.Path()
							}
						}
					}

					calleeRef := &FunctionReference{
						Name:      call.Function.Name,
						Package:   calleePackage,
						Signature: s.getSignatureString(call.Function),
					}

					chain := &CallChain{
						Caller: callerRef,
						Callee: calleeRef,
						CallSites: []CallSite{
							{
								File: pkg.Path,
								Line: call.Line,
							},
						},
						IsRecursive: callerRef.Name == calleeRef.Name && callerRef.Package == calleeRef.Package,
					}
					chains = append(chains, chain)
				}
			}
		}
	}

	return chains
}

// getFunctionKey creates a unique key for a function
func (s *service) getFunctionKey(pkg, name string, receiver *parser.TypeInfo) string {
	if receiver != nil {
		return fmt.Sprintf("%s.(%s).%s", pkg, receiver.Name, name)
	}
	return fmt.Sprintf("%s.%s", pkg, name)
}

// getSignatureString converts function signature to string
func (s *service) getSignatureString(fn *parser.FunctionInfo) string {
	if fn.Signature != nil {
		return fn.Signature.String()
	}
	return ""
}

// DetectCircularDependencies identifies circular import cycles
func (s *service) DetectCircularDependencies(_ context.Context, graph *DependencyGraph) ([]*CircularDependency, error) {
	var cycles []*CircularDependency
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := []string{}

	var detectCycle func(node string) bool
	detectCycle = func(node string) bool {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		if depNode, exists := graph.Nodes[node]; exists {
			for _, dep := range depNode.Dependencies {
				if !visited[dep] {
					if detectCycle(dep) {
						return true
					}
				} else if recStack[dep] {
					// Found a cycle
					cycleStart := -1
					for i, p := range path {
						if p == dep {
							cycleStart = i
							break
						}
					}
					if cycleStart >= 0 {
						cycle := &CircularDependency{
							Cycle:    append([]string{}, path[cycleStart:]...),
							Severity: s.calculateCycleSeverity(len(path) - cycleStart),
						}
						cycles = append(cycles, cycle)
					}
					return true
				}
			}
		}

		path = path[:len(path)-1]
		recStack[node] = false
		return false
	}

	// Check each unvisited node
	for node := range graph.Nodes {
		if !visited[node] {
			detectCycle(node)
		}
	}

	return cycles, nil
}

// calculateCycleSeverity determines the severity based on cycle length
func (s *service) calculateCycleSeverity(length int) SeverityLevel {
	switch {
	case length <= 3:
		return SeverityHigh
	case length <= 5:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// collectImplementations collects all interface implementations from parse result
func (s *service) collectImplementations(result *parser.ParseResult) []*parser.Implementation {
	var implementations []*parser.Implementation

	for _, iface := range result.Interfaces {
		// Set the Interface field for each implementation
		for _, impl := range iface.Implementations {
			// Create a copy to avoid modifying the original
			implCopy := *impl
			implCopy.Interface = iface
			implementations = append(implementations, &implCopy)
		}
	}

	return implementations
}

// calculateMetrics calculates code quality metrics
func (s *service) calculateMetrics(result *parser.ParseResult) *CodeMetrics {
	metrics := &CodeMetrics{
		CyclomaticComplexity: make(map[string]int),
	}

	// Count global interfaces
	metrics.TotalInterfaces = len(result.Interfaces)

	for _, pkg := range result.Packages {
		metrics.TotalFiles += len(pkg.Files)
		metrics.TotalFunctions += len(pkg.Functions)

		// Count structs from types
		for _, t := range pkg.Types {
			if t.Underlying != nil && strings.Contains(t.Underlying.String(), "struct") {
				metrics.TotalStructs++
			}
		}

		// Calculate lines (approximate from files)
		for _, file := range pkg.Files {
			// Estimate lines based on function positions
			maxLine := 0
			for _, fn := range file.Functions {
				if fn.LineEnd > maxLine {
					maxLine = fn.LineEnd
				}
			}
			metrics.TotalLines += maxLine
		}
	}

	return metrics
}
