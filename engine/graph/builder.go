package graph

import (
	"context"
	"fmt"
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

	// Pre-allocate with estimated capacity for better memory efficiency
	estimatedNodes := len(parseResult.Files) * 10 // Conservative estimate: ~10 nodes per file
	if len(parseResult.Files) > 0 {
		// Estimate based on average nodes per file (funcs, structs, constants, etc.)
		avgNodesPerFile := 10 // Conservative estimate
		estimatedNodes = len(parseResult.Files) * (1 + avgNodesPerFile)
	}

	// Log processing start
	logger.Info("building graph from analysis",
		"project_id", projectID,
		"files", len(parseResult.Files),
		"estimated_nodes", estimatedNodes)

	// Start with basic graph structure from parser
	result, err := b.BuildFromParseResult(ctx, projectID, parseResult)
	if err != nil {
		return nil, fmt.Errorf("failed to build from parse result: %w", err)
	}

	// Enhance with analyzer results
	b.addAnalyzerRelationships(result, analysis)

	// Update totals with analysis info
	result.TotalFiles = analysis.Metrics.TotalFiles

	// Log completion statistics
	logger.Info("graph building completed",
		"duration", time.Since(startTime),
		"nodes", len(result.Nodes),
		"relationships", len(result.Relationships),
		"nodes_per_file", float64(len(result.Nodes))/float64(len(parseResult.Files)))

	return result, nil
}

// BuildFromParseResult creates basic graph structure from parser results only
func (b *builder) BuildFromParseResult(
	ctx context.Context,
	projectID core.ID,
	parseResult *parser.ParseResult,
) (*core.AnalysisResult, error) {
	// Pre-allocate capacity for better performance with large codebases
	estimatedNodes := len(parseResult.Files) * 10 // Rough estimate
	estimatedRels := estimatedNodes * 3           // Rough estimate

	result := &core.AnalysisResult{
		ProjectID:     projectID,
		Nodes:         make([]core.Node, 0, estimatedNodes),
		Relationships: make([]core.Relationship, 0, estimatedRels),
		AnalyzedAt:    time.Now(),
	}

	// Process packages and files
	packageNodes := make(map[string]core.ID)

	// Check if we should use chunked processing for large codebases
	if len(parseResult.Files) > b.config.ChunkSize*2 {
		logger.Info("using chunked processing for large codebase",
			"total_files", len(parseResult.Files),
			"chunk_size", b.config.ChunkSize)

		// Process files in chunks to avoid memory issues
		for i := 0; i < len(parseResult.Files); i += b.config.ChunkSize {
			end := i + b.config.ChunkSize
			if end > len(parseResult.Files) {
				end = len(parseResult.Files)
			}

			chunk := parseResult.Files[i:end]
			logger.Debug("processing file chunk",
				"chunk_start", i,
				"chunk_end", end,
				"chunk_size", len(chunk))

			for _, file := range chunk {
				b.processFile(ctx, result, file, packageNodes)
			}

			// Allow GC to clean up memory between chunks
			if i+b.config.ChunkSize < len(parseResult.Files) {
				time.Sleep(10 * time.Millisecond)
			}
		}
	} else {
		// Process all files at once for smaller codebases
		for _, file := range parseResult.Files {
			b.processFile(ctx, result, file, packageNodes)
		}
	}

	// Update result totals
	result.TotalPackages = len(packageNodes)
	result.TotalFunctions = 0
	result.TotalStructs = 0

	// Count types
	for _, node := range result.Nodes {
		switch node.Type {
		case core.NodeTypeFunction:
			result.TotalFunctions++
		case core.NodeTypeStruct:
			result.TotalStructs++
		}
	}

	return result, nil
}

// processFile processes a single file and adds nodes/relationships
func (b *builder) processFile(
	ctx context.Context,
	result *core.AnalysisResult,
	file *parser.FileInfo,
	packageNodes map[string]core.ID,
) {
	// Create package node if not exists
	pkgID := b.createPackageNode(ctx, result, file, packageNodes)

	// Create file node
	if b.config.CreateFileNodes {
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
			CreatedAt:  time.Now(),
		})

		// Process file contents
		b.processFileContents(result, file, fileID)
	}
}

// createPackageNode creates a package node if it doesn't exist
func (b *builder) createPackageNode(
	_ context.Context,
	result *core.AnalysisResult,
	file *parser.FileInfo,
	packageNodes map[string]core.ID,
) core.ID {
	// Check if package node already exists
	if pkgID, exists := packageNodes[file.Package]; exists {
		return pkgID
	}

	// Create new package node
	pkgID := core.NewID()
	pkgNode := core.Node{
		ID:   pkgID,
		Type: core.NodeTypePackage,
		Name: file.Package,
		Path: file.Package,
		Properties: map[string]any{
			"project_id":  result.ProjectID.String(),
			"analyzed_at": result.AnalyzedAt,
		},
		CreatedAt: time.Now(),
	}
	result.Nodes = append(result.Nodes, pkgNode)

	// Cache the package node ID
	packageNodes[file.Package] = pkgID

	return pkgID
}

// processFileContents processes all content within a file
func (b *builder) processFileContents(
	result *core.AnalysisResult,
	file *parser.FileInfo,
	fileID core.ID,
) {
	// Process imports
	b.processImports(result, file, fileID)

	// Process functions
	b.processFunctions(result, file, fileID)

	// Process structs and their methods
	b.processStructs(result, file, fileID)

	// Process interfaces
	b.processInterfaces(result, file, fileID)

	// Process constants
	b.processConstants(result, file, fileID)

	// Process variables
	b.processVariables(result, file, fileID)
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
				"alias":      imp.Alias,
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
			CreatedAt:  time.Now(),
		})
	}
}

// processFunctions creates function nodes and relationships
func (b *builder) processFunctions(result *core.AnalysisResult, file *parser.FileInfo, fileID core.ID) {
	for i := range file.Functions {
		fn := &file.Functions[i]
		fnID := core.NewID()
		fnNode := core.Node{
			ID:   fnID,
			Type: core.NodeTypeFunction,
			Name: fn.Name,
			Properties: map[string]any{
				"signature":   fn.Signature,
				"receiver":    fn.Receiver,
				"is_exported": fn.IsExported,
				"package":     file.Package,
				"project_id":  result.ProjectID.String(),
			},
			CreatedAt: time.Now(),
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
			CreatedAt:  time.Now(),
		})
	}
}

// processStructs creates struct nodes and their method nodes
func (b *builder) processStructs(result *core.AnalysisResult, file *parser.FileInfo, fileID core.ID) {
	for _, st := range file.Structs {
		stID := b.createStructNode(result, file, fileID, &st)
		b.processStructMethods(result, file, stID, &st)
	}
}

// createStructNode creates a struct node and returns its ID
func (b *builder) createStructNode(
	result *core.AnalysisResult,
	file *parser.FileInfo,
	fileID core.ID,
	st *parser.StructInfo,
) core.ID {
	stID := core.NewID()
	stNode := core.Node{
		ID:   stID,
		Type: core.NodeTypeStruct,
		Name: st.Name,
		Properties: map[string]any{
			"is_exported": st.IsExported,
			"package":     file.Package,
			"project_id":  result.ProjectID.String(),
		},
		CreatedAt: time.Now(),
	}
	if b.config.IncludeLineNumbers {
		stNode.Properties["line_start"] = st.LineStart
		stNode.Properties["line_end"] = st.LineEnd
	}

	// Add fields as properties
	fields := make([]map[string]any, 0, len(st.Fields))
	for _, field := range st.Fields {
		fields = append(fields, map[string]any{
			"name": field.Name,
			"type": field.Type,
			"tag":  field.Tag,
		})
	}
	if len(fields) > 0 {
		stNode.Properties["fields"] = fields
	}

	result.Nodes = append(result.Nodes, stNode)

	// Create file->struct relationship
	result.Relationships = append(result.Relationships, core.Relationship{
		ID:         core.NewID(),
		Type:       core.RelationDefines,
		FromNodeID: fileID,
		ToNodeID:   stID,
		CreatedAt:  time.Now(),
	})

	return stID
}

// processStructMethods creates method nodes for a struct
func (b *builder) processStructMethods(
	result *core.AnalysisResult,
	file *parser.FileInfo,
	stID core.ID,
	st *parser.StructInfo,
) {
	for i := range st.Methods {
		method := &st.Methods[i]
		methodID := core.NewID()
		methodNode := core.Node{
			ID:   methodID,
			Type: core.NodeTypeMethod,
			Name: method.Name,
			Properties: map[string]any{
				"signature":   method.Signature,
				"receiver":    st.Name,
				"is_exported": method.IsExported,
				"package":     file.Package,
				"project_id":  result.ProjectID.String(),
			},
			CreatedAt: time.Now(),
		}
		if b.config.IncludeLineNumbers {
			methodNode.Properties["line_start"] = method.LineStart
			methodNode.Properties["line_end"] = method.LineEnd
		}
		result.Nodes = append(result.Nodes, methodNode)

		// Create struct->method relationship
		result.Relationships = append(result.Relationships, core.Relationship{
			ID:         core.NewID(),
			Type:       core.RelationBelongsTo,
			FromNodeID: methodID,
			ToNodeID:   stID,
			CreatedAt:  time.Now(),
		})
	}
}

// processInterfaces creates interface nodes and relationships
func (b *builder) processInterfaces(result *core.AnalysisResult, file *parser.FileInfo, fileID core.ID) {
	for _, iface := range file.Interfaces {
		ifaceID := core.NewID()
		ifaceNode := core.Node{
			ID:   ifaceID,
			Type: core.NodeTypeInterface,
			Name: iface.Name,
			Properties: map[string]any{
				"is_exported": iface.IsExported,
				"package":     file.Package,
				"project_id":  result.ProjectID.String(),
			},
			CreatedAt: time.Now(),
		}
		if b.config.IncludeLineNumbers {
			ifaceNode.Properties["line_start"] = iface.LineStart
			ifaceNode.Properties["line_end"] = iface.LineEnd
		}

		// Add methods as properties
		methods := make([]map[string]any, 0, len(iface.Methods))
		for _, method := range iface.Methods {
			params := make([]map[string]any, 0, len(method.Parameters))
			for _, p := range method.Parameters {
				params = append(params, map[string]any{
					"name": p.Name,
					"type": p.Type,
				})
			}
			methods = append(methods, map[string]any{
				"name":       method.Name,
				"parameters": params,
				"returns":    method.Returns,
			})
		}
		if len(methods) > 0 {
			ifaceNode.Properties["methods"] = methods
		}

		result.Nodes = append(result.Nodes, ifaceNode)

		// Create file->interface relationship
		result.Relationships = append(result.Relationships, core.Relationship{
			ID:         core.NewID(),
			Type:       core.RelationDefines,
			FromNodeID: fileID,
			ToNodeID:   ifaceID,
			CreatedAt:  time.Now(),
		})
	}
}

// processConstants creates constant nodes and relationships
func (b *builder) processConstants(result *core.AnalysisResult, file *parser.FileInfo, fileID core.ID) {
	for _, cnst := range file.Constants {
		cnstID := core.NewID()
		cnstNode := core.Node{
			ID:   cnstID,
			Type: core.NodeTypeConstant,
			Name: cnst.Name,
			Properties: map[string]any{
				"type":        cnst.Type,
				"value":       cnst.Value,
				"is_exported": cnst.IsExported,
				"package":     file.Package,
				"project_id":  result.ProjectID.String(),
			},
			CreatedAt: time.Now(),
		}
		result.Nodes = append(result.Nodes, cnstNode)

		// Create file->constant relationship
		result.Relationships = append(result.Relationships, core.Relationship{
			ID:         core.NewID(),
			Type:       core.RelationDefines,
			FromNodeID: fileID,
			ToNodeID:   cnstID,
			CreatedAt:  time.Now(),
		})
	}
}

// processVariables creates variable nodes and relationships
func (b *builder) processVariables(result *core.AnalysisResult, file *parser.FileInfo, fileID core.ID) {
	for _, v := range file.Variables {
		varID := core.NewID()
		varNode := core.Node{
			ID:   varID,
			Type: core.NodeTypeVariable,
			Name: v.Name,
			Properties: map[string]any{
				"type":        v.Type,
				"is_exported": v.IsExported,
				"package":     file.Package,
				"project_id":  result.ProjectID.String(),
			},
			CreatedAt: time.Now(),
		}
		result.Nodes = append(result.Nodes, varNode)

		// Create file->variable relationship
		result.Relationships = append(result.Relationships, core.Relationship{
			ID:         core.NewID(),
			Type:       core.RelationDefines,
			FromNodeID: fileID,
			ToNodeID:   varID,
			CreatedAt:  time.Now(),
		})
	}
}

// addAnalyzerRelationships adds relationships discovered by the analyzer
func (b *builder) addAnalyzerRelationships(
	result *core.AnalysisResult,
	analysisReport *analyzer.AnalysisReport,
) {
	// Add function call relationships
	b.addFunctionCallRelationships(result, analysisReport)

	// Add interface implementation relationships
	b.addInterfaceImplementationRelationships(result, analysisReport)

	// Add dependency relationships
	b.addDependencyRelationships(result, analysisReport)
}

// addFunctionCallRelationships processes function call chains
func (b *builder) addFunctionCallRelationships(result *core.AnalysisResult, analysis *analyzer.AnalysisReport) {
	for _, chain := range analysis.CallChains {
		callerID := b.findFunctionNodeID(result.Nodes, chain.Caller)
		calleeID := b.findFunctionNodeID(result.Nodes, chain.Callee)

		if callerID != "" && calleeID != "" {
			rel := core.Relationship{
				ID:         core.NewID(),
				Type:       core.RelationCalls,
				FromNodeID: callerID,
				ToNodeID:   calleeID,
				Properties: map[string]any{
					"call_count": len(chain.CallSites),
				},
				CreatedAt: time.Now(),
			}

			if chain.IsRecursive {
				rel.Properties["is_recursive"] = true
			}

			result.Relationships = append(result.Relationships, rel)
		}
	}
}

// addInterfaceImplementationRelationships processes interface implementations
func (b *builder) addInterfaceImplementationRelationships(
	result *core.AnalysisResult,
	analysis *analyzer.AnalysisReport,
) {
	for _, impl := range analysis.InterfaceImplementations {
		interfaceID := b.findInterfaceNodeID(result.Nodes, impl.Interface.Name)
		implementorID := b.findStructNodeID(result.Nodes, impl.Implementor.Name)

		if interfaceID != "" && implementorID != "" {
			rel := core.Relationship{
				ID:         core.NewID(),
				Type:       core.RelationImplements,
				FromNodeID: implementorID,
				ToNodeID:   interfaceID,
				Properties: map[string]any{
					"is_complete": impl.IsComplete,
				},
				CreatedAt: time.Now(),
			}

			if len(impl.MissingMethods) > 0 {
				rel.Properties["missing_methods"] = impl.MissingMethods
			}

			result.Relationships = append(result.Relationships, rel)
		}
	}
}

// addDependencyRelationships processes dependency graph edges
func (b *builder) addDependencyRelationships(result *core.AnalysisResult, analysis *analyzer.AnalysisReport) {
	if analysis.DependencyGraph == nil {
		return
	}

	for _, edge := range analysis.DependencyGraph.Edges {
		if edge.Type != analyzer.DependencyTypeImport {
			continue
		}

		fromID := b.findFileNodeID(result.Nodes, edge.From)
		toID := b.findFileNodeID(result.Nodes, edge.To)

		if fromID != "" && toID != "" {
			result.Relationships = append(result.Relationships, core.Relationship{
				ID:         core.NewID(),
				Type:       core.RelationDependsOn,
				FromNodeID: fromID,
				ToNodeID:   toID,
				CreatedAt:  time.Now(),
			})
		}
	}
}

// findFunctionNodeID finds a function or method node by reference
func (b *builder) findFunctionNodeID(nodes []core.Node, ref *analyzer.FunctionReference) core.ID {
	for _, node := range nodes {
		if (node.Type == core.NodeTypeFunction || node.Type == core.NodeTypeMethod) && matchesFunctionRef(&node, ref) {
			return node.ID
		}
	}
	return ""
}

// findInterfaceNodeID finds an interface node by name
func (b *builder) findInterfaceNodeID(nodes []core.Node, name string) core.ID {
	for _, node := range nodes {
		if node.Type == core.NodeTypeInterface && node.Name == name {
			return node.ID
		}
	}
	return ""
}

// findStructNodeID finds a struct node by name
func (b *builder) findStructNodeID(nodes []core.Node, name string) core.ID {
	for _, node := range nodes {
		if node.Type == core.NodeTypeStruct && node.Name == name {
			return node.ID
		}
	}
	return ""
}

// findFileNodeID finds a file node by path
func (b *builder) findFileNodeID(nodes []core.Node, path string) core.ID {
	for _, node := range nodes {
		if node.Type == core.NodeTypeFile {
			if props, ok := node.Properties["path"].(string); ok && props == path {
				return node.ID
			}
		}
	}
	return ""
}

// matchesFunctionRef checks if a node matches a function reference
func matchesFunctionRef(node *core.Node, ref *analyzer.FunctionReference) bool {
	if node.Name != ref.Name {
		return false
	}

	// Check package if specified
	if pkg, ok := node.Properties["package"].(string); ok && ref.Package != "" {
		if pkg != ref.Package {
			return false
		}
	}

	// Check receiver for methods
	if receiver, ok := node.Properties["receiver"].(string); ok && ref.Receiver != "" {
		if receiver != ref.Receiver {
			return false
		}
	}

	return true
}
