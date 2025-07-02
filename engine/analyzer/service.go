package analyzer

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/compozy/gograph/engine/parser"
	"github.com/compozy/gograph/pkg/logger"
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
	logger.Info("starting project analysis", "project_id", input.ProjectID, "files", len(input.Files))
	startTime := time.Now()

	report := &AnalysisReport{
		ProjectID: input.ProjectID,
		Timestamp: time.Now().Unix(),
	}

	// Build dependency graph
	depGraph, err := s.BuildDependencyGraph(ctx, input.Files)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}
	report.DependencyGraph = depGraph

	// Detect interface implementations
	implementations, err := s.DetectInterfaceImplementations(ctx, input.Files)
	if err != nil {
		return nil, fmt.Errorf("failed to detect interface implementations: %w", err)
	}
	report.InterfaceImplementations = implementations

	// Map call chains
	callChains, err := s.MapCallChains(ctx, input.Files)
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
		report.Metrics = s.calculateMetrics(input.Files)
	}

	logger.Info("project analysis complete", "duration", time.Since(startTime))
	return report, nil
}

// BuildDependencyGraph constructs a dependency graph from file imports
func (s *service) BuildDependencyGraph(_ context.Context, files []*parser.FileInfo) (*DependencyGraph, error) {
	graph := &DependencyGraph{
		Nodes: make(map[string]*DependencyNode),
		Edges: []*DependencyEdge{},
	}

	// Create nodes for each file
	for _, file := range files {
		node := &DependencyNode{
			Path:         file.Path,
			Type:         NodeTypeFile,
			Dependencies: file.Dependencies,
			Dependents:   []string{},
		}
		graph.Nodes[file.Path] = node
	}

	// Build edges and update dependents
	for _, file := range files {
		for _, dep := range file.Dependencies {
			edge := &DependencyEdge{
				From: file.Path,
				To:   dep,
				Type: DependencyTypeImport,
			}
			graph.Edges = append(graph.Edges, edge)

			// Update dependent information if the dependency is internal
			if depNode, exists := graph.Nodes[dep]; exists {
				depNode.Dependents = append(depNode.Dependents, file.Path)
			}
		}
	}

	return graph, nil
}

// DetectInterfaceImplementations finds all structs that implement interfaces
func (s *service) DetectInterfaceImplementations(
	_ context.Context,
	files []*parser.FileInfo,
) ([]*InterfaceImplementation, error) {
	var implementations []*InterfaceImplementation

	// Collect all interfaces and structs with their package information
	interfaces := make(map[string]*InterfaceRef)
	structs := make(map[string]*StructRef)

	for _, file := range files {
		// Process interfaces
		for i := range file.Interfaces {
			iface := &file.Interfaces[i]
			key := fmt.Sprintf("%s.%s", file.Package, iface.Name)
			interfaces[key] = &InterfaceRef{
				InterfaceInfo: iface,
				Package:       file.Package,
				FilePath:      file.Path,
			}
		}

		// Process structs
		for i := range file.Structs {
			st := &file.Structs[i]
			key := fmt.Sprintf("%s.%s", file.Package, st.Name)
			structs[key] = &StructRef{
				StructInfo: st,
				Package:    file.Package,
				FilePath:   file.Path,
			}
		}
	}

	// Check each struct against each interface
	for _, structRef := range structs {
		for _, ifaceRef := range interfaces {
			if impl := s.checkImplementation(structRef, ifaceRef, files); impl != nil {
				implementations = append(implementations, impl)
			}
		}
	}

	return implementations, nil
}

// checkImplementation checks if a struct implements an interface
func (s *service) checkImplementation(
	structRef *StructRef,
	ifaceRef *InterfaceRef,
	files []*parser.FileInfo,
) *InterfaceImplementation {
	impl := &InterfaceImplementation{
		Interface:      ifaceRef,
		Implementor:    structRef,
		Methods:        []MethodMatch{},
		IsComplete:     true,
		MissingMethods: []string{},
	}

	// Get all methods for the struct
	structMethods := s.getStructMethods(structRef, files)

	// Check each interface method
	for _, ifaceMethod := range ifaceRef.Methods {
		found := false
		for i := range structMethods {
			if s.methodsMatch(ifaceMethod, &structMethods[i]) {
				impl.Methods = append(impl.Methods, MethodMatch{
					InterfaceMethod: ifaceMethod.Name,
					StructMethod:    structMethods[i].Name,
					Signature:       s.getMethodSignature(&structMethods[i]),
				})
				found = true
				break
			}
		}

		if !found {
			impl.IsComplete = false
			impl.MissingMethods = append(impl.MissingMethods, ifaceMethod.Name)
		}
	}

	// Only return if there's at least one matching method
	if len(impl.Methods) == 0 {
		return nil
	}

	return impl
}

// getStructMethods returns all methods for a struct
func (s *service) getStructMethods(structRef *StructRef, files []*parser.FileInfo) []parser.FunctionInfo {
	var methods []parser.FunctionInfo

	for _, file := range files {
		if file.Package != structRef.Package {
			continue
		}

		for i := range file.Functions {
			if file.Functions[i].Receiver == structRef.Name {
				methods = append(methods, file.Functions[i])
			}
		}
	}

	// Also include methods from the struct definition itself
	methods = append(methods, structRef.Methods...)

	return methods
}

// methodsMatch checks if a struct method matches an interface method
func (s *service) methodsMatch(ifaceMethod parser.MethodInfo, structMethod *parser.FunctionInfo) bool {
	if ifaceMethod.Name != structMethod.Name {
		return false
	}

	// Check parameter count
	if len(ifaceMethod.Parameters) != len(structMethod.Parameters) {
		return false
	}

	// Check parameter types
	for i, ifaceParam := range ifaceMethod.Parameters {
		if ifaceParam.Type != structMethod.Parameters[i].Type {
			return false
		}
	}

	// Check return types
	if len(ifaceMethod.Returns) != len(structMethod.Returns) {
		return false
	}

	for i, ifaceReturn := range ifaceMethod.Returns {
		if ifaceReturn != structMethod.Returns[i] {
			return false
		}
	}

	return true
}

// getMethodSignature returns a string representation of a method signature
func (s *service) getMethodSignature(method *parser.FunctionInfo) string {
	sig := method.Name + "("
	for i, param := range method.Parameters {
		if i > 0 {
			sig += ", "
		}
		sig += param.Name + " " + param.Type
	}
	sig += ")"

	if len(method.Returns) > 0 {
		sig += " "
		if len(method.Returns) > 1 {
			sig += "("
		}
		for i, ret := range method.Returns {
			if i > 0 {
				sig += ", "
			}
			sig += ret
		}
		if len(method.Returns) > 1 {
			sig += ")"
		}
	}

	return sig
}

// MapCallChains traces function call relationships
func (s *service) MapCallChains(_ context.Context, files []*parser.FileInfo) ([]*CallChain, error) {
	var callChains []*CallChain

	// Build function index
	functions := make(map[string]*parser.FunctionInfo)
	functionPackages := make(map[string]string)

	for _, file := range files {
		for i := range file.Functions {
			fn := &file.Functions[i]
			key := s.getFunctionKey(file.Package, fn.Name, fn.Receiver)
			functions[key] = fn
			functionPackages[key] = file.Package
		}
	}

	// Analyze function calls
	for fnKey, fn := range functions {
		for _, call := range fn.Calls {
			calleeKey := s.getFunctionKey(call.Package, call.Name, "")
			if callee, exists := functions[calleeKey]; exists {
				chain := &CallChain{
					Caller: &FunctionReference{
						Name:      fn.Name,
						Package:   functionPackages[fnKey],
						Receiver:  fn.Receiver,
						Signature: s.getMethodSignature(fn),
					},
					Callee: &FunctionReference{
						Name:      callee.Name,
						Package:   functionPackages[calleeKey],
						Receiver:  callee.Receiver,
						Signature: s.getMethodSignature(callee),
					},
					CallSites: []CallSite{
						{
							Line: call.Line,
						},
					},
					IsRecursive: fnKey == calleeKey,
				}
				callChains = append(callChains, chain)
			}
		}
	}

	return callChains, nil
}

// getFunctionKey creates a unique key for a function
func (s *service) getFunctionKey(pkg, name, receiver string) string {
	if receiver != "" {
		return fmt.Sprintf("%s.%s.%s", pkg, receiver, name)
	}
	return fmt.Sprintf("%s.%s", pkg, name)
}

// DetectCircularDependencies identifies circular import cycles
func (s *service) DetectCircularDependencies(
	_ context.Context,
	graph *DependencyGraph,
) ([]*CircularDependency, error) {
	var cycles []*CircularDependency
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	for path := range graph.Nodes {
		if !visited[path] {
			if cycle := s.detectCycle(path, graph, visited, recursionStack, []string{}); cycle != nil {
				cycles = append(cycles, cycle)
			}
		}
	}

	return cycles, nil
}

// detectCycle uses DFS to detect cycles in the dependency graph
func (s *service) detectCycle(
	node string,
	graph *DependencyGraph,
	visited map[string]bool,
	recursionStack map[string]bool,
	path []string,
) *CircularDependency {
	visited[node] = true
	recursionStack[node] = true
	path = append(path, node)

	if nodeData, exists := graph.Nodes[node]; exists {
		for _, dep := range nodeData.Dependencies {
			if !visited[dep] {
				if cycle := s.detectCycle(dep, graph, visited, recursionStack, path); cycle != nil {
					return cycle
				}
			} else if recursionStack[dep] {
				// Found a cycle
				cycleStart := 0
				for i, p := range path {
					if p == dep {
						cycleStart = i
						break
					}
				}

				return &CircularDependency{
					Cycle:    path[cycleStart:],
					Severity: SeverityHigh,
					Impact:   path, // All nodes in the path are impacted
				}
			}
		}
	}

	recursionStack[node] = false
	return nil
}

// calculateMetrics computes code quality metrics
func (s *service) calculateMetrics(files []*parser.FileInfo) *CodeMetrics {
	metrics := &CodeMetrics{
		TotalFiles:           len(files),
		CyclomaticComplexity: make(map[string]int),
	}

	for _, file := range files {
		metrics.TotalFunctions += len(file.Functions)
		metrics.TotalInterfaces += len(file.Interfaces)
		metrics.TotalStructs += len(file.Structs)

		// Simple line count (would need actual file reading for accurate count)
		for i := range file.Functions {
			metrics.TotalLines += file.Functions[i].LineEnd - file.Functions[i].LineStart + 1
		}
	}

	return metrics
}
