package graph

import (
	"context"
	"fmt"
	"go/types"
	"path/filepath"
	"time"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/parser"
	"github.com/compozy/gograph/pkg/logger"
)

// Builder defines the interface for building graph structures from analysis results
type Builder interface {
	// BuildFromAnalysis creates graph nodes and relationships from analysis results
	BuildFromAnalysis(
		ctx context.Context,
		projectID core.ID,
		parseResult *parser.ParseResult,
		analysis *analyzer.AnalysisReport,
	) (*core.AnalysisResult, error)

	// BuildFromParseResult creates basic graph structure from parser results only
	BuildFromParseResult(
		ctx context.Context,
		projectID core.ID,
		parseResult *parser.ParseResult,
	) (*core.AnalysisResult, error)
}

// builder implements the Builder interface
type builder struct {
	config *BuilderConfig
}

// BuilderConfig holds builder configuration
type BuilderConfig struct {
	IncludeLineNumbers bool // Include line numbers in properties
	IncludeComments    bool // Include comment text in properties
	CreateFileNodes    bool // Create nodes for files
	BatchSize          int  // Batch size for node/relationship creation
	ChunkSize          int  // Number of files to process in one chunk
	MaxConcurrency     int  // Maximum concurrent goroutines for processing
}

// DefaultBuilderConfig returns default builder configuration
func DefaultBuilderConfig() *BuilderConfig {
	return &BuilderConfig{
		IncludeLineNumbers: true,
		IncludeComments:    false,
		CreateFileNodes:    true,
		BatchSize:          1000,
		ChunkSize:          500, // Process 500 files at a time
		MaxConcurrency:     4,   // Use 4 goroutines for parallel processing
	}
}

// NewBuilder creates a new graph builder instance
func NewBuilder(config *BuilderConfig) Builder {
	if config == nil {
		config = DefaultBuilderConfig()
	}
	return &builder{
		config: config,
	}
}

// BuildFromAnalysis creates graph nodes and relationships from analysis results with optimization for large codebases
func (b *builder) BuildFromAnalysis(
	ctx context.Context,
	projectID core.ID,
	parseResult *parser.ParseResult,
	analysis *analyzer.AnalysisReport,
) (*core.AnalysisResult, error) {
	startTime := time.Now()

	// Count total files
	totalFiles := 0
	for _, pkg := range parseResult.Packages {
		totalFiles += len(pkg.Files)
	}

	// Pre-allocate with estimated capacity for better memory efficiency
	estimatedNodes := totalFiles * 10 // Conservative estimate: ~10 nodes per file

	// Log processing start
	logger.Info("building graph from analysis",
		"project_id", projectID,
		"packages", len(parseResult.Packages),
		"files", totalFiles,
		"estimated_nodes", estimatedNodes)

	// Start with basic graph structure from parser
	result, err := b.BuildFromParseResult(ctx, projectID, parseResult)
	if err != nil {
		return nil, fmt.Errorf("failed to build from parse result: %w", err)
	}

	// Enhance with analyzer results
	b.addAnalyzerRelationships(result, analysis)

	// Update totals with analysis info
	if analysis.Metrics != nil {
		result.TotalFiles = analysis.Metrics.TotalFiles
	}

	// Log completion statistics
	logger.Info("graph building completed",
		"duration", time.Since(startTime),
		"nodes", len(result.Nodes),
		"relationships", len(result.Relationships),
		"nodes_per_file", float64(len(result.Nodes))/float64(totalFiles))

	return result, nil
}

// BuildFromParseResult creates basic graph structure from parser results only
func (b *builder) BuildFromParseResult(
	_ context.Context,
	projectID core.ID,
	parseResult *parser.ParseResult,
) (*core.AnalysisResult, error) {
	// Count total files
	totalFiles := 0
	for _, pkg := range parseResult.Packages {
		totalFiles += len(pkg.Files)
	}

	// Pre-allocate capacity for better performance with large codebases
	estimatedNodes := totalFiles * 10 // Rough estimate
	estimatedRels := estimatedNodes * 3

	result := &core.AnalysisResult{
		ProjectID:     projectID,
		Nodes:         make([]core.Node, 0, estimatedNodes),
		Relationships: make([]core.Relationship, 0, estimatedRels),
		AnalyzedAt:    time.Now(),
	}

	// Process packages
	packageNodeMap := make(map[string]core.ID)
	functionNodeMap := make(map[string]core.ID) // For linking calls
	typeNodeMap := make(map[string]core.ID)     // For linking implementations

	for _, pkg := range parseResult.Packages {
		pkgID := b.createPackageNode(result, pkg)
		packageNodeMap[pkg.Path] = pkgID

		// Process files in the package
		for _, file := range pkg.Files {
			if b.config.CreateFileNodes {
				fileID := b.createFileNode(result, file, pkgID)
				b.processFileContents(result, pkg, file, fileID, functionNodeMap, typeNodeMap)
			}
		}

		// Process package-level types and functions
		b.processPackageTypes(result, pkg, pkgID, typeNodeMap)
		b.processPackageFunctions(result, pkg, pkgID, functionNodeMap)
	}

	// Update result totals
	result.TotalPackages = len(packageNodeMap)
	result.TotalFunctions = 0
	result.TotalStructs = 0
	result.TotalFiles = totalFiles

	// Count types
	for _, node := range result.Nodes {
		switch node.Type {
		case core.NodeTypeFunction, core.NodeTypeMethod:
			result.TotalFunctions++
		case core.NodeTypeStruct:
			result.TotalStructs++
		}
	}

	return result, nil
}

// createPackageNode creates a package node
func (b *builder) createPackageNode(result *core.AnalysisResult, pkg *parser.PackageInfo) core.ID {
	pkgID := core.NewID()
	pkgNode := core.Node{
		ID:   pkgID,
		Type: core.NodeTypePackage,
		Name: pkg.Name,
		Path: pkg.Path,
		Properties: map[string]any{
			"project_id":  result.ProjectID.String(),
			"analyzed_at": result.AnalyzedAt,
			"import_path": pkg.Path,
		},
		CreatedAt: time.Now(),
	}
	result.Nodes = append(result.Nodes, pkgNode)
	return pkgID
}

// createFileNode creates a file node
func (b *builder) createFileNode(result *core.AnalysisResult, file *parser.FileInfo, pkgID core.ID) core.ID {
	fileID := core.NewID()
	fileNode := core.Node{
		ID:   fileID,
		Type: core.NodeTypeFile,
		Name: filepath.Base(file.Path),
		Properties: map[string]any{
			"path":       file.Path,
			"package":    file.Package,
			"project_id": result.ProjectID.String(),
		},
		CreatedAt: time.Now(),
	}
	result.Nodes = append(result.Nodes, fileNode)

	// Create package->file relationship
	result.Relationships = append(result.Relationships, core.Relationship{
		ID:         core.NewID(),
		Type:       core.RelationContains,
		FromNodeID: pkgID,
		ToNodeID:   fileID,
		Properties: map[string]any{
			"project_id": result.ProjectID.String(),
		},
		CreatedAt: time.Now(),
	})

	return fileID
}

// processFileContents processes all content within a file
func (b *builder) processFileContents(
	result *core.AnalysisResult,
	pkg *parser.PackageInfo,
	file *parser.FileInfo,
	fileID core.ID,
	functionNodeMap map[string]core.ID,
	typeNodeMap map[string]core.ID,
) {
	// Process imports
	b.processImports(result, file, fileID)

	// Process functions defined in this file
	for _, fn := range file.Functions {
		fnID := b.createFunctionNode(result, pkg, fn, fileID)
		key := fmt.Sprintf("%s.%s", pkg.Path, fn.Name)
		functionNodeMap[key] = fnID
	}

	// Process types defined in this file
	for _, t := range file.Types {
		typeID := b.createTypeNode(result, pkg, t, fileID)
		key := fmt.Sprintf("%s.%s", pkg.Path, t.Name)
		typeNodeMap[key] = typeID
	}
}

// processImports creates import nodes and relationships
func (b *builder) processImports(result *core.AnalysisResult, file *parser.FileInfo, fileID core.ID) {
	for _, imp := range file.Imports {
		impID := core.NewID()
		impNode := core.Node{
			ID:   impID,
			Type: core.NodeTypeImport,
			Name: imp.Path,
			Properties: map[string]any{
				"name":       imp.Name,
				"project_id": result.ProjectID.String(),
			},
			CreatedAt: time.Now(),
		}
		result.Nodes = append(result.Nodes, impNode)

		// Create file->import relationship
		result.Relationships = append(result.Relationships, core.Relationship{
			ID:         core.NewID(),
			Type:       core.RelationImports,
			FromNodeID: fileID,
			ToNodeID:   impID,
			Properties: map[string]any{
				"project_id": result.ProjectID.String(),
			},
			CreatedAt: time.Now(),
		})
	}
}

// createFunctionNode creates a function node
func (b *builder) createFunctionNode(
	result *core.AnalysisResult,
	pkg *parser.PackageInfo,
	fn *parser.FunctionInfo,
	fileID core.ID,
) core.ID {
	fnID := core.NewID()

	// Determine node type based on receiver
	nodeType := core.NodeTypeFunction
	if fn.Receiver != nil {
		nodeType = core.NodeTypeMethod
	}

	fnNode := core.Node{
		ID:   fnID,
		Type: nodeType,
		Name: fn.Name,
		Properties: map[string]any{
			"is_exported": fn.IsExported,
			"package":     pkg.Path,
			"project_id":  result.ProjectID.String(),
		},
		CreatedAt: time.Now(),
	}

	// Add signature if available
	if fn.Signature != nil {
		fnNode.Properties["signature"] = fn.Signature.String()
	}

	// Add receiver info for methods
	if fn.Receiver != nil {
		fnNode.Properties["receiver"] = fn.Receiver.Name
		fnNode.Properties["receiver_type"] = getTypeString(fn.Receiver.Type)
	}

	if b.config.IncludeLineNumbers {
		fnNode.Properties["line_start"] = fn.LineStart
		fnNode.Properties["line_end"] = fn.LineEnd
	}

	result.Nodes = append(result.Nodes, fnNode)

	// Create file->function relationship
	result.Relationships = append(result.Relationships, core.Relationship{
		ID:         core.NewID(),
		Type:       core.RelationDefines,
		FromNodeID: fileID,
		ToNodeID:   fnID,
		Properties: map[string]any{
			"project_id": result.ProjectID.String(),
		},
		CreatedAt: time.Now(),
	})

	return fnID
}

// createTypeNode creates a type node (struct, interface, etc.)
func (b *builder) createTypeNode(
	result *core.AnalysisResult,
	pkg *parser.PackageInfo,
	t *parser.TypeInfo,
	fileID core.ID,
) core.ID {
	typeID := core.NewID()

	// Determine node type based on underlying type
	nodeType := core.NodeTypeStruct
	if t.Underlying != nil {
		if _, ok := t.Underlying.(*types.Interface); ok {
			nodeType = core.NodeTypeInterface
		}
	}

	typeNode := core.Node{
		ID:   typeID,
		Type: nodeType,
		Name: t.Name,
		Properties: map[string]any{
			"is_exported": t.IsExported,
			"package":     pkg.Path,
			"project_id":  result.ProjectID.String(),
		},
		CreatedAt: time.Now(),
	}

	if b.config.IncludeLineNumbers {
		typeNode.Properties["line_start"] = t.LineStart
		typeNode.Properties["line_end"] = t.LineEnd
	}

	// Add fields for structs
	if len(t.Fields) > 0 {
		fields := make([]map[string]any, 0, len(t.Fields))
		for _, field := range t.Fields {
			fieldMap := map[string]any{
				"name":        field.Name,
				"type":        getTypeString(field.Type),
				"is_exported": field.IsExported,
				"anonymous":   field.Anonymous,
			}
			if field.Tag != "" {
				fieldMap["tag"] = field.Tag
			}
			fields = append(fields, fieldMap)
		}
		typeNode.Properties["fields"] = fields
	}

	result.Nodes = append(result.Nodes, typeNode)

	// Create file->type relationship
	result.Relationships = append(result.Relationships, core.Relationship{
		ID:         core.NewID(),
		Type:       core.RelationDefines,
		FromNodeID: fileID,
		ToNodeID:   typeID,
		Properties: map[string]any{
			"project_id": result.ProjectID.String(),
		},
		CreatedAt: time.Now(),
	})

	return typeID
}

// processPackageTypes processes types at package level (methods, etc.)
func (b *builder) processPackageTypes(
	result *core.AnalysisResult,
	pkg *parser.PackageInfo,
	_ core.ID,
	typeNodeMap map[string]core.ID,
) {
	// Process interfaces with their methods
	for _, iface := range pkg.Interfaces {
		key := fmt.Sprintf("%s.%s", pkg.Path, iface.Name)
		if ifaceID, exists := typeNodeMap[key]; exists {
			// Add interface methods as properties
			if len(iface.Methods) > 0 {
				methods := make([]map[string]any, 0, len(iface.Methods))
				for _, method := range iface.Methods {
					methodMap := map[string]any{
						"name": method.Name,
					}
					if method.Signature != nil {
						methodMap["signature"] = method.Signature.String()
					}
					methods = append(methods, methodMap)
				}

				// Update the interface node with methods
				for i, node := range result.Nodes {
					if node.ID == ifaceID {
						result.Nodes[i].Properties["methods"] = methods
						break
					}
				}
			}
		}
	}
}

// processPackageFunctions processes functions at package level (linking methods to types)
func (b *builder) processPackageFunctions(
	result *core.AnalysisResult,
	pkg *parser.PackageInfo,
	_ core.ID,
	functionNodeMap map[string]core.ID,
) {
	// Link methods to their receiver types
	for _, fn := range pkg.Functions {
		if fn.Receiver != nil {
			fnKey := fmt.Sprintf("%s.%s", pkg.Path, fn.Name)
			if fnID, exists := functionNodeMap[fnKey]; exists {
				// Find the receiver type node
				receiverTypeName := fn.Receiver.Name

				// Create method->type relationship
				for _, node := range result.Nodes {
					if node.Type == core.NodeTypeStruct && node.Name == receiverTypeName &&
						node.Properties["package"] == pkg.Path {
						result.Relationships = append(result.Relationships, core.Relationship{
							ID:         core.NewID(),
							Type:       core.RelationBelongsTo,
							FromNodeID: fnID,
							ToNodeID:   node.ID,
							Properties: map[string]any{
								"project_id": result.ProjectID.String(),
							},
							CreatedAt: time.Now(),
						})
						break
					}
				}
			}
		}
	}
}

// addAnalyzerRelationships adds relationships discovered by the analyzer
func (b *builder) addAnalyzerRelationships(result *core.AnalysisResult, analysis *analyzer.AnalysisReport) {
	// Add interface implementation relationships
	if len(analysis.InterfaceImplementations) > 0 {
		b.addImplementationRelationships(result, analysis.InterfaceImplementations)
	}

	// Add call chain relationships
	if len(analysis.CallChains) > 0 {
		b.addCallRelationships(result, analysis.CallChains)
	}
}

// addImplementationRelationships adds IMPLEMENTS relationships
func (b *builder) addImplementationRelationships(
	result *core.AnalysisResult,
	implementations []*parser.Implementation,
) {
	// Build lookup maps for O(1) access
	structNodes := make(map[string]*core.Node)
	ifaceNodes := make(map[string]*core.Node)

	// Index nodes by type and key
	for i := range result.Nodes {
		node := &result.Nodes[i]
		key := fmt.Sprintf("%s.%s", node.Properties["package"], node.Name)

		switch node.Type {
		case core.NodeTypeStruct:
			structNodes[key] = node
		case core.NodeTypeInterface:
			ifaceNodes[key] = node
		}
	}

	// Process implementations using lookups
	for _, impl := range implementations {
		if impl.Type == nil || impl.Interface == nil {
			continue
		}

		// Build lookup keys
		structKey := fmt.Sprintf("%s.%s", getPackageFromType(impl.Type), impl.Type.Name)
		ifaceKey := fmt.Sprintf("%s.%s", getPackageFromInterface(impl.Interface), impl.Interface.Name)

		// Look up nodes in O(1)
		structNode, structExists := structNodes[structKey]
		ifaceNode, ifaceExists := ifaceNodes[ifaceKey]

		if structExists && ifaceExists {
			result.Relationships = append(result.Relationships, core.Relationship{
				ID:         core.NewID(),
				Type:       core.RelationImplements,
				FromNodeID: structNode.ID,
				ToNodeID:   ifaceNode.ID,
				Properties: map[string]any{
					"is_complete":     impl.IsComplete,
					"missing_methods": impl.MissingMethods,
					"project_id":      result.ProjectID.String(),
				},
				CreatedAt: time.Now(),
			})
		}
	}
}

// addCallRelationships adds CALLS relationships
func (b *builder) addCallRelationships(
	result *core.AnalysisResult,
	callChains []*analyzer.CallChain,
) {
	// Build node lookup maps for O(1) access
	functionNodes := b.buildFunctionNodeMap(result)

	for _, chain := range callChains {
		callerNode := b.findFunctionNode(functionNodes, chain.Caller)
		calleeNode := b.findFunctionNode(functionNodes, chain.Callee)

		if callerNode != nil && calleeNode != nil {
			// Create CALLS relationship
			props := map[string]any{
				"project_id":   result.ProjectID.String(),
				"is_recursive": chain.IsRecursive,
			}

			// Add call site information
			if len(chain.CallSites) > 0 {
				sites := make([]map[string]any, 0, len(chain.CallSites))
				for _, site := range chain.CallSites {
					sites = append(sites, map[string]any{
						"file":   site.File,
						"line":   site.Line,
						"column": site.Column,
					})
				}
				props["call_sites"] = sites
			}

			result.Relationships = append(result.Relationships, core.Relationship{
				ID:         core.NewID(),
				Type:       core.RelationCalls,
				FromNodeID: callerNode.ID,
				ToNodeID:   calleeNode.ID,
				Properties: props,
				CreatedAt:  time.Now(),
			})
		}
	}
}

// buildFunctionNodeMap creates a map for O(1) function node lookup
func (b *builder) buildFunctionNodeMap(result *core.AnalysisResult) map[string]*core.Node {
	functionNodes := make(map[string]*core.Node)
	for i := range result.Nodes {
		node := &result.Nodes[i]
		if node.Type == core.NodeTypeFunction || node.Type == core.NodeTypeMethod {
			// Build key from package, name, and receiver
			key := fmt.Sprintf("%s.%s", node.Properties["package"], node.Name)
			if receiver, ok := node.Properties["receiver"]; ok && receiver != "" {
				key = fmt.Sprintf("%s.%s.%s", node.Properties["package"], receiver, node.Name)
			}
			functionNodes[key] = node
		}
	}
	return functionNodes
}

// findFunctionNode looks up a function node by reference
func (b *builder) findFunctionNode(
	functionNodes map[string]*core.Node,
	ref *analyzer.FunctionReference,
) *core.Node {
	if ref == nil {
		return nil
	}

	// Build lookup key
	key := fmt.Sprintf("%s.%s", ref.Package, ref.Name)
	if ref.Receiver != "" {
		key = fmt.Sprintf("%s.%s.%s", ref.Package, ref.Receiver, ref.Name)
	}

	return functionNodes[key]
}

// Helper functions

// getTypeString converts a types.Type to string
func getTypeString(t types.Type) string {
	if t == nil {
		return ""
	}
	return t.String()
}

// getPackageFromType extracts package path from TypeInfo
func getPackageFromType(t *parser.TypeInfo) string {
	if t.Type == nil {
		return ""
	}

	// Try to get package from named type
	if named, ok := t.Type.(*types.Named); ok {
		if pkg := named.Obj().Pkg(); pkg != nil {
			return pkg.Path()
		}
	}

	return ""
}

// getPackageFromInterface extracts package path from InterfaceInfo
func getPackageFromInterface(iface *parser.InterfaceInfo) string {
	return iface.Package
}
