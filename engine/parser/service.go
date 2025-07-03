package parser

import (
	"context"
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/compozy/gograph/pkg/logger"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Service implements the Parser interface using modern Go analysis tools
type Service struct {
	config *Config
}

// NewService creates a new parser service
func NewService(config *Config) Parser {
	if config == nil {
		config = &Config{
			IgnoreDirs:             []string{".git", ".idea", ".vscode", "node_modules"},
			IgnoreFiles:            []string{},
			IncludeTests:           true,
			IncludeVendor:          false,
			EnableSSA:              true,
			EnableCallGraph:        true,
			EnablePerformanceStats: false, // Disabled by default to avoid overhead
		}
	}
	return &Service{
		config: config,
	}
}

// ParseProject parses an entire Go project with full type information
func (s *Service) ParseProject(ctx context.Context, projectPath string, config *Config) (*ParseResult, error) {
	startTime := time.Now()

	// Validate and sanitize project path to prevent directory traversal attacks
	cleanPath, err := s.validateProjectPath(projectPath)
	if err != nil {
		return nil, fmt.Errorf("invalid project path: %w", err)
	}

	if config == nil {
		config = s.config
	}

	// Load packages using the validated path
	pkgs, err := s.loadPackages(ctx, cleanPath, config)
	if err != nil {
		return nil, err
	}

	// Filter packages
	filteredPkgs := s.filterPackages(pkgs, config)

	// Build SSA if enabled
	ssaProg, ssaPkgs, perfStats := s.buildSSAWithMetrics(filteredPkgs, config)

	// Create result
	result := &ParseResult{
		ProjectPath:      cleanPath,
		Packages:         make([]*PackageInfo, 0, len(filteredPkgs)),
		SSAProgram:       ssaProg,
		PerformanceStats: perfStats,
	}

	// Map SSA packages
	ssaPkgMap := s.mapSSAPackages(filteredPkgs, ssaPkgs, config, ssaProg)

	// Process packages
	s.processAllPackages(filteredPkgs, ssaPkgMap, result)

	// Link methods to their receiver types
	s.linkMethodsToTypes(result)

	// Find interface implementations
	s.findImplementations(result)

	// Build call graph if enabled
	if config.EnableCallGraph && ssaProg != nil {
		result.CallGraph = s.buildCallGraph(ssaProg)
	}

	result.ParseTime = time.Since(startTime).Milliseconds()
	return result, nil
}

// validateProjectPath validates and sanitizes the project path to prevent directory traversal attacks
func (s *Service) validateProjectPath(projectPath string) (string, error) {
	if projectPath == "" {
		return "", fmt.Errorf("project path cannot be empty")
	}

	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(projectPath)

	// Convert to absolute path to eliminate any remaining relative components
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Verify the path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("project path does not exist: %s", absPath)
		}
		return "", fmt.Errorf("failed to access project path: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("project path is not a directory: %s", absPath)
	}

	return absPath, nil
}

// loadPackages loads all packages from the project
func (s *Service) loadPackages(ctx context.Context, projectPath string, config *Config) ([]*packages.Package, error) {
	// Configure package loading with full type information
	loadMode := packages.NeedName |
		packages.NeedFiles |
		packages.NeedCompiledGoFiles |
		packages.NeedImports |
		packages.NeedDeps |
		packages.NeedTypes |
		packages.NeedTypesInfo |
		packages.NeedSyntax |
		packages.NeedModule

	pkgConfig := &packages.Config{
		Mode:    loadMode,
		Context: ctx,
		Dir:     projectPath,
		Tests:   config.IncludeTests,
	}

	// Load all packages in the project
	pattern := "./..."
	pkgs, err := packages.Load(pkgConfig, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to load packages: %w", err)
	}

	// Check for errors in loaded packages
	var loadErrors []string
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			for _, e := range pkg.Errors {
				loadErrors = append(loadErrors, e.Error())
			}
		}
	}

	if len(loadErrors) > 0 {
		return nil, fmt.Errorf("package loading failed with %d errors: %v", len(loadErrors), loadErrors)
	}

	return pkgs, nil
}

// filterPackages filters out test and vendor packages based on config
func (s *Service) filterPackages(pkgs []*packages.Package, config *Config) []*packages.Package {
	var filteredPkgs []*packages.Package
	for _, pkg := range pkgs {
		if !config.IncludeTests && strings.HasSuffix(pkg.ID, ".test") {
			continue
		}
		if !config.IncludeVendor && strings.Contains(pkg.PkgPath, "/vendor/") {
			continue
		}
		filteredPkgs = append(filteredPkgs, pkg)
	}
	return filteredPkgs
}

// buildSSAWithMetrics builds SSA representation and returns performance metrics
func (s *Service) buildSSAWithMetrics(
	pkgs []*packages.Package,
	config *Config,
) (*ssa.Program, []*ssa.Package, *SSAPerformanceMetrics) {
	if !config.EnableSSA {
		return nil, nil, nil
	}

	if config.EnablePerformanceStats {
		return s.buildSSAWithMonitoring(pkgs, config)
	}

	// Fast path without monitoring overhead
	ssaProg, ssaPkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	ssaProg.Build()
	return ssaProg, ssaPkgs, nil
}

// buildSSAWithMonitoring builds SSA with comprehensive performance monitoring
//
//nolint:funlen // This function handles comprehensive monitoring which requires detailed steps
func (s *Service) buildSSAWithMonitoring(
	pkgs []*packages.Package,
	config *Config,
) (*ssa.Program, []*ssa.Package, *SSAPerformanceMetrics) {
	buildStart := time.Now()

	memProfile := &SSAMemoryProfile{}

	// Only read memory stats at key checkpoints if memory monitoring is enabled
	if config.EnableMemoryMonitoring {
		// Get initial memory stats
		var initialMem runtime.MemStats
		runtime.ReadMemStats(&initialMem)
		memProfile.InitialMemoryMB = memStatsToMB(initialMem.Alloc)

		logger.Info("starting SSA build with performance monitoring",
			"packages", len(pkgs),
			"initial_memory_mb", memProfile.InitialMemoryMB)
	} else {
		logger.Info("starting SSA build with performance monitoring",
			"packages", len(pkgs))
	}

	phaseBreakdown := make(map[string]time.Duration)

	// Phase 1: SSA Preparation
	prepStart := time.Now()
	logger.Debug("SSA preparation phase starting")

	ssaProg, ssaPkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)

	prepDuration := time.Since(prepStart)
	phaseBreakdown["preparation"] = prepDuration

	// Get memory after preparation only if monitoring is enabled
	if config.EnableMemoryMonitoring {
		var prepMem runtime.MemStats
		runtime.ReadMemStats(&prepMem)
		memProfile.PreparationMemoryMB = memStatsToMB(prepMem.Alloc)

		logger.Debug("SSA preparation completed",
			"duration", prepDuration,
			"memory_mb", memProfile.PreparationMemoryMB,
			"packages_created", len(ssaPkgs))
	} else {
		logger.Debug("SSA preparation completed",
			"duration", prepDuration,
			"packages_created", len(ssaPkgs))
	}

	// Phase 2: SSA Construction
	buildConstructionStart := time.Now()
	logger.Debug("SSA construction phase starting")

	// Monitor memory during build only if enabled
	var peakMemoryMB int64
	if config.EnableMemoryMonitoring {
		peakMemoryMB = s.performSSABuildWithMemoryMonitoring(ssaProg, memProfile.PreparationMemoryMB)
	} else {
		// Just build without monitoring
		ssaProg.Build()
	}

	constructionDuration := time.Since(buildConstructionStart)
	phaseBreakdown["construction"] = constructionDuration

	// Get final memory stats only if monitoring is enabled
	if config.EnableMemoryMonitoring {
		var finalMem runtime.MemStats
		runtime.ReadMemStats(&finalMem)
		memProfile.BuildMemoryMB = memStatsToMB(finalMem.Alloc)
		memProfile.PeakMemoryMB = peakMemoryMB
		memProfile.FinalMemoryMB = memProfile.BuildMemoryMB
	}

	// Count SSA functions across all packages
	functionsCount := countSSAFunctions(ssaProg)

	totalDuration := time.Since(buildStart)
	phaseBreakdown["total"] = totalDuration

	if config.EnableMemoryMonitoring {
		logger.Info("SSA build completed successfully",
			"total_duration", totalDuration,
			"preparation_duration", prepDuration,
			"construction_duration", constructionDuration,
			"functions_analyzed", functionsCount,
			"peak_memory_mb", memProfile.PeakMemoryMB,
			"memory_increase_mb", memProfile.PeakMemoryMB-memProfile.InitialMemoryMB)

		// Performance analysis and warnings
		s.analyzeSSAPerformance(
			len(pkgs),
			functionsCount,
			totalDuration,
			memProfile.PeakMemoryMB-memProfile.InitialMemoryMB,
		)
	} else {
		logger.Info("SSA build completed successfully",
			"total_duration", totalDuration,
			"preparation_duration", prepDuration,
			"construction_duration", constructionDuration,
			"functions_analyzed", functionsCount)
	}

	logger.Debug("SSA performance breakdown",
		"phase_timings", phaseBreakdown,
		"memory_profile", memProfile)

	// Create comprehensive performance metrics
	perfStats := createSSAPerformanceMetrics(
		totalDuration, prepDuration, constructionDuration,
		len(pkgs), functionsCount,
		memProfile, phaseBreakdown,
	)

	return ssaProg, ssaPkgs, perfStats
}

// performSSABuildWithMemoryMonitoring performs SSA build with memory monitoring
func (s *Service) performSSABuildWithMemoryMonitoring(
	ssaProg *ssa.Program,
	initialPeakMemory int64,
) int64 {
	peakMemory := initialPeakMemory
	atomic.StoreInt64(&peakMemory, initialPeakMemory)

	// Start a goroutine to monitor peak memory usage during build
	// Use a longer interval (1 second) to reduce stop-the-world pauses
	memMonitorDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-memMonitorDone:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				currentMB := memStatsToMB(m.Alloc)
				// Atomically update peak memory if current is higher
				for {
					oldPeak := atomic.LoadInt64(&peakMemory)
					if currentMB <= oldPeak {
						break
					}
					if atomic.CompareAndSwapInt64(&peakMemory, oldPeak, currentMB) {
						break
					}
				}
			}
		}
	}()

	// Perform the actual SSA build
	ssaProg.Build()

	// Stop memory monitoring
	close(memMonitorDone)

	// Do one final memory check after build completes
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	finalMB := memStatsToMB(finalMem.Alloc)
	for {
		oldPeak := atomic.LoadInt64(&peakMemory)
		if finalMB <= oldPeak {
			break
		}
		if atomic.CompareAndSwapInt64(&peakMemory, oldPeak, finalMB) {
			break
		}
	}

	return atomic.LoadInt64(&peakMemory)
}

// memStatsToMB safely converts memory stats to MB to avoid gosec warnings
func memStatsToMB(bytes uint64) int64 {
	const mbShift = 20 // 1MB = 2^20 bytes
	mbValue := bytes >> mbShift
	// Ensure the result fits in int64 to prevent overflow
	const maxInt64 = (1 << 63) - 1
	if mbValue > maxInt64 {
		return maxInt64 // Return max safe value if overflow would occur
	}
	return int64(mbValue)
}

// countSSAFunctions counts all functions in the SSA program including nested functions
func countSSAFunctions(ssaProg *ssa.Program) int {
	functionsCount := 0
	for _, pkg := range ssaProg.AllPackages() {
		for _, member := range pkg.Members {
			if fn, ok := member.(*ssa.Function); ok {
				functionsCount++
				// Count anonymous functions within this function
				for _, anon := range fn.AnonFuncs {
					functionsCount++
					countNestedFunctions(anon, &functionsCount)
				}
			}
		}
	}
	return functionsCount
}

// countNestedFunctions recursively counts anonymous functions within a function
func countNestedFunctions(fn *ssa.Function, count *int) {
	for _, anon := range fn.AnonFuncs {
		*count++
		countNestedFunctions(anon, count)
	}
}

// createSSAPerformanceMetrics creates comprehensive performance metrics
func createSSAPerformanceMetrics(
	totalDuration, prepDuration, constructionDuration time.Duration,
	packageCount, functionCount int,
	memProfile *SSAMemoryProfile,
	phaseBreakdown map[string]time.Duration,
) *SSAPerformanceMetrics {
	return &SSAPerformanceMetrics{
		BuildDuration:     totalDuration,
		PreparationTime:   prepDuration,
		ConstructionTime:  constructionDuration,
		PackagesProcessed: packageCount,
		FunctionsAnalyzed: functionCount,
		CallGraphNodes:    0, // Will be set by call graph analysis if enabled
		MemoryUsageMB:     memProfile.PeakMemoryMB,
		PhaseBreakdown:    phaseBreakdown,
		MemoryProfile:     memProfile,
	}
}

// analyzeSSAPerformance provides insights and warnings about SSA build performance
func (s *Service) analyzeSSAPerformance(
	packageCount, functionCount int,
	duration time.Duration,
	memoryIncreaseMB int64,
) {
	// Performance insights based on project size
	if duration > 30*time.Second {
		logger.Warn("SSA build took longer than expected",
			"duration", duration,
			"recommendation", "consider disabling SSA for large projects or using faster hardware")
	}

	if memoryIncreaseMB > 1024 { // > 1GB memory increase
		logger.Warn("SSA build used significant memory",
			"memory_increase_mb", memoryIncreaseMB,
			"recommendation", "monitor memory usage in production environments")
	}

	// Calculate performance ratios
	avgTimePerPackage := duration.Nanoseconds() / int64(packageCount)
	avgTimePerFunction := duration.Nanoseconds() / int64(functionCount)

	logger.Debug("SSA performance analysis",
		"avg_time_per_package_ms", avgTimePerPackage/1000000,
		"avg_time_per_function_ns", avgTimePerFunction,
		"functions_per_package", functionCount/packageCount,
		"memory_per_function_kb", (memoryIncreaseMB*1024)/int64(functionCount))
}

// mapSSAPackages creates a mapping from packages to their SSA counterparts
func (s *Service) mapSSAPackages(
	filteredPkgs []*packages.Package,
	ssaPkgs []*ssa.Package,
	config *Config,
	ssaProg *ssa.Program,
) map[*packages.Package]*ssa.Package {
	ssaPkgMap := make(map[*packages.Package]*ssa.Package)
	if !config.EnableSSA || ssaProg == nil {
		return ssaPkgMap
	}

	// Build a map of SSA packages by import path
	ssaByPath := make(map[string]*ssa.Package)
	for _, ssaPkg := range ssaPkgs {
		if ssaPkg != nil && ssaPkg.Pkg != nil {
			ssaByPath[ssaPkg.Pkg.Path()] = ssaPkg
		}
	}

	// Map filtered packages to their SSA counterparts
	for _, pkg := range filteredPkgs {
		if ssaPkg, ok := ssaByPath[pkg.PkgPath]; ok {
			ssaPkgMap[pkg] = ssaPkg
		}
	}

	return ssaPkgMap
}

// processAllPackages processes each package and collects results
func (s *Service) processAllPackages(
	filteredPkgs []*packages.Package,
	ssaPkgMap map[*packages.Package]*ssa.Package,
	result *ParseResult,
) {
	for _, pkg := range filteredPkgs {
		pkgInfo := s.processPackage(pkg, ssaPkgMap[pkg])
		result.Packages = append(result.Packages, pkgInfo)

		// Collect interfaces
		result.Interfaces = append(result.Interfaces, pkgInfo.Interfaces...)
	}
}

// processPackage processes a single package
func (s *Service) processPackage(pkg *packages.Package, ssaPkg *ssa.Package) *PackageInfo {
	pkgInfo := &PackageInfo{
		Package:    pkg,
		Path:       pkg.PkgPath,
		Name:       pkg.Name,
		Files:      make([]*FileInfo, 0),
		Functions:  make([]*FunctionInfo, 0),
		Types:      make([]*TypeInfo, 0),
		Interfaces: make([]*InterfaceInfo, 0),
		Constants:  make([]*ConstantInfo, 0),
		Variables:  make([]*VariableInfo, 0),
		SSAPackage: ssaPkg,
	}

	// Process all files in the package
	s.processPackageFiles(pkg, pkgInfo)

	// Link SSA functions to parsed functions for call graph analysis
	if ssaPkg != nil {
		s.linkSSAFunctions(pkgInfo, ssaPkg)
	}

	// Extract interfaces from types
	s.extractPackageInterfaces(pkg, pkgInfo)

	return pkgInfo
}

// linkSSAFunctions links SSA functions to parsed FunctionInfo objects for call graph analysis
func (s *Service) linkSSAFunctions(pkgInfo *PackageInfo, ssaPkg *ssa.Package) {
	// Create a map of SSA functions by name for quick lookup
	ssaFuncMap := make(map[string]*ssa.Function)

	// Index all SSA functions (including methods)
	for _, member := range ssaPkg.Members {
		if fn, ok := member.(*ssa.Function); ok {
			// Create a unique key for the function
			key := s.getSSAFunctionKey(fn)
			ssaFuncMap[key] = fn
		}
	}

	// Link parsed functions to their SSA counterparts
	for _, fn := range pkgInfo.Functions {
		key := s.getParsedFunctionKey(fn, pkgInfo.Path)
		if ssaFn, exists := ssaFuncMap[key]; exists {
			fn.SSAFunc = ssaFn
		}
	}
}

// getSSAFunctionKey creates a unique key for an SSA function
func (s *Service) getSSAFunctionKey(fn *ssa.Function) string {
	if fn.Signature.Recv() != nil {
		// Method: ReceiverType.MethodName
		receiverType := fn.Signature.Recv().Type().String()
		return receiverType + "." + fn.Name()
	}
	// Regular function: just the name
	return fn.Name()
}

// getParsedFunctionKey creates a unique key for a parsed function
func (s *Service) getParsedFunctionKey(fn *FunctionInfo, _ string) string {
	if fn.Receiver != nil {
		// Method: ReceiverType.MethodName
		receiverType := ""
		if fn.Receiver.Type != nil {
			receiverType = fn.Receiver.Type.String()
		}
		return receiverType + "." + fn.Name
	}
	// Regular function: just the name
	return fn.Name
}

// processPackageFiles processes all files in a package
func (s *Service) processPackageFiles(pkg *packages.Package, pkgInfo *PackageInfo) {
	for i, file := range pkg.Syntax {
		if i < len(pkg.CompiledGoFiles) {
			fileInfo := s.processFile(pkg, file, pkg.CompiledGoFiles[i])
			pkgInfo.Files = append(pkgInfo.Files, fileInfo)

			// Aggregate functions, types, constants, and variables
			pkgInfo.Functions = append(pkgInfo.Functions, fileInfo.Functions...)
			pkgInfo.Types = append(pkgInfo.Types, fileInfo.Types...)
			pkgInfo.Constants = append(pkgInfo.Constants, fileInfo.Constants...)
			pkgInfo.Variables = append(pkgInfo.Variables, fileInfo.Variables...)
		}
	}
}

// extractPackageInterfaces extracts interface declarations from types
func (s *Service) extractPackageInterfaces(pkg *packages.Package, pkgInfo *PackageInfo) {
	for _, typeInfo := range pkgInfo.Types {
		if iface, ok := typeInfo.Underlying.(*types.Interface); ok {
			ifaceInfo := s.createInterfaceInfo(typeInfo.Name, iface, typeInfo, pkg.PkgPath)
			pkgInfo.Interfaces = append(pkgInfo.Interfaces, ifaceInfo)
		}
	}
}

// processFile processes a single Go file
func (s *Service) processFile(pkg *packages.Package, file *ast.File, filePath string) *FileInfo {
	fileInfo := &FileInfo{
		Path:         filePath,
		Package:      pkg.Name,
		Imports:      make([]*ImportInfo, 0),
		Functions:    make([]*FunctionInfo, 0),
		Types:        make([]*TypeInfo, 0),
		Constants:    make([]*ConstantInfo, 0),
		Variables:    make([]*VariableInfo, 0),
		Dependencies: make([]string, 0),
	}

	// Process imports
	s.processImports(pkg, file, fileInfo)

	// Process declarations
	s.processDeclarations(pkg, file, fileInfo)

	return fileInfo
}

// processImports extracts import information from a file
func (s *Service) processImports(pkg *packages.Package, file *ast.File, fileInfo *FileInfo) {
	for _, imp := range file.Imports {
		impPath := strings.Trim(imp.Path.Value, `"`)
		impInfo := &ImportInfo{
			Path: impPath,
		}

		if imp.Name != nil {
			impInfo.Name = imp.Name.Name
		} else {
			// Try to resolve the actual package name from pkg.Imports
			actualPkgName := ""
			if pkg.Imports != nil {
				if importedPkg, found := pkg.Imports[impPath]; found && importedPkg != nil {
					actualPkgName = importedPkg.Name
				}
			}

			if actualPkgName != "" {
				impInfo.Name = actualPkgName
			} else {
				// TODO: This fallback heuristic may be incorrect for packages where
				// the directory name differs from the package name
				// (e.g., "github.com/user/repo/v2" might have package name "repo" not "v2")
				parts := strings.Split(impPath, "/")
				impInfo.Name = parts[len(parts)-1]
			}
		}

		fileInfo.Imports = append(fileInfo.Imports, impInfo)
		fileInfo.Dependencies = append(fileInfo.Dependencies, impPath)
	}
}

// processDeclarations processes all declarations in a file
func (s *Service) processDeclarations(pkg *packages.Package, file *ast.File, fileInfo *FileInfo) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			funcInfo := s.processFuncDecl(pkg, d)
			fileInfo.Functions = append(fileInfo.Functions, funcInfo)

		case *ast.GenDecl:
			s.processGenDecl(pkg, d, fileInfo)
		}
	}
}

// processGenDecl processes general declarations (type, const, var)
func (s *Service) processGenDecl(pkg *packages.Package, decl *ast.GenDecl, fileInfo *FileInfo) {
	for _, spec := range decl.Specs {
		switch specType := spec.(type) {
		case *ast.TypeSpec:
			typeInfo := s.processTypeSpec(pkg, specType)
			fileInfo.Types = append(fileInfo.Types, typeInfo)

		case *ast.ValueSpec:
			switch decl.Tok.String() {
			case "const":
				constants := s.processConstSpec(pkg, specType)
				fileInfo.Constants = append(fileInfo.Constants, constants...)
			case "var":
				variables := s.processVarSpec(pkg, specType)
				fileInfo.Variables = append(fileInfo.Variables, variables...)
			}
		}
	}
}

// processFuncDecl processes a function declaration
func (s *Service) processFuncDecl(pkg *packages.Package, decl *ast.FuncDecl) *FunctionInfo {
	funcInfo := &FunctionInfo{
		Name:       decl.Name.Name,
		IsExported: ast.IsExported(decl.Name.Name),
		LineStart:  pkg.Fset.Position(decl.Pos()).Line,
		LineEnd:    pkg.Fset.Position(decl.End()).Line,
		Calls:      make([]*FunctionCall, 0),
	}

	// Get type information and handle receiver
	s.extractFunctionTypeInfo(pkg, decl, funcInfo)

	// Extract function calls
	s.extractFunctionCalls(pkg, decl, funcInfo)

	return funcInfo
}

// extractFunctionTypeInfo extracts type information for a function
func (s *Service) extractFunctionTypeInfo(pkg *packages.Package, decl *ast.FuncDecl, funcInfo *FunctionInfo) {
	if obj := pkg.TypesInfo.Defs[decl.Name]; obj != nil {
		if fn, ok := obj.(*types.Func); ok {
			if sig, ok := fn.Type().(*types.Signature); ok {
				funcInfo.Signature = sig
			}

			// Handle receiver for methods
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				if recv := funcInfo.Signature.Recv(); recv != nil {
					funcInfo.Receiver = &TypeInfo{
						Name: recv.Type().String(),
						Type: recv.Type(),
					}
				}
			}
		}
	}
}

// extractFunctionCalls extracts all function calls within a function body
func (s *Service) extractFunctionCalls(pkg *packages.Package, decl *ast.FuncDecl, funcInfo *FunctionInfo) {
	if decl.Body == nil {
		// Function body is nil (e.g., due to syntax error)
		return
	}
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if callInfo := s.extractCallInfo(pkg, call); callInfo != nil {
				funcInfo.Calls = append(funcInfo.Calls, callInfo)
			}
		}
		return true
	})
}

// processTypeSpec processes a type specification
func (s *Service) processTypeSpec(pkg *packages.Package, spec *ast.TypeSpec) *TypeInfo {
	typeInfo := &TypeInfo{
		Name:       spec.Name.Name,
		IsExported: ast.IsExported(spec.Name.Name),
		LineStart:  pkg.Fset.Position(spec.Pos()).Line,
		LineEnd:    pkg.Fset.Position(spec.End()).Line,
		Methods:    make([]*FunctionInfo, 0),
		Fields:     make([]*FieldInfo, 0),
		Embeds:     make([]*TypeInfo, 0),
		Implements: make([]*InterfaceInfo, 0),
	}

	// Get type information
	if obj := pkg.TypesInfo.Defs[spec.Name]; obj != nil {
		if named, ok := obj.Type().(*types.Named); ok {
			typeInfo.Type = named
			typeInfo.Underlying = named.Underlying()

			// Process struct fields if applicable
			s.processStructFields(typeInfo)
		}
	}

	return typeInfo
}

// processConstSpec processes a constant specification
func (s *Service) processConstSpec(pkg *packages.Package, spec *ast.ValueSpec) []*ConstantInfo {
	var constants []*ConstantInfo

	for i, name := range spec.Names {
		constInfo := &ConstantInfo{
			Name:       name.Name,
			IsExported: ast.IsExported(name.Name),
			LineStart:  pkg.Fset.Position(name.Pos()).Line,
			LineEnd:    pkg.Fset.Position(name.End()).Line,
		}

		// Get type information from the type checker
		if obj := pkg.TypesInfo.Defs[name]; obj != nil {
			if constObj, ok := obj.(*types.Const); ok {
				constInfo.Type = constObj.Type()
				constInfo.Value = constObj.Val().String()
			}
		}

		// If no type info from type checker, try to get from AST
		if constInfo.Type == nil && spec.Type != nil {
			if typeObj := pkg.TypesInfo.TypeOf(spec.Type); typeObj != nil {
				constInfo.Type = typeObj
			}
		}

		// Get value from AST if available
		if constInfo.Value == "" && i < len(spec.Values) && spec.Values[i] != nil {
			constInfo.Value = types.ExprString(spec.Values[i])
		}

		constants = append(constants, constInfo)
	}

	return constants
}

// processVarSpec processes a variable specification
func (s *Service) processVarSpec(pkg *packages.Package, spec *ast.ValueSpec) []*VariableInfo {
	var variables []*VariableInfo

	for i, name := range spec.Names {
		varInfo := &VariableInfo{
			Name:       name.Name,
			IsExported: ast.IsExported(name.Name),
			LineStart:  pkg.Fset.Position(name.Pos()).Line,
			LineEnd:    pkg.Fset.Position(name.End()).Line,
		}

		// Get type information from the type checker
		if obj := pkg.TypesInfo.Defs[name]; obj != nil {
			if varObj, ok := obj.(*types.Var); ok {
				varInfo.Type = varObj.Type()
			}
		}

		// If no type info from type checker, try to get from AST
		if varInfo.Type == nil && spec.Type != nil {
			if typeObj := pkg.TypesInfo.TypeOf(spec.Type); typeObj != nil {
				varInfo.Type = typeObj
			}
		}

		// Get initial value from AST if available
		if i < len(spec.Values) && spec.Values[i] != nil {
			varInfo.Value = types.ExprString(spec.Values[i])
		}

		variables = append(variables, varInfo)
	}

	return variables
}

// processStructFields processes fields of a struct type
func (s *Service) processStructFields(typeInfo *TypeInfo) {
	strct, ok := typeInfo.Underlying.(*types.Struct)
	if !ok {
		return
	}

	for i := 0; i < strct.NumFields(); i++ {
		field := strct.Field(i)
		fieldInfo := &FieldInfo{
			Name:       field.Name(),
			Type:       field.Type(),
			IsExported: field.Exported(),
			Anonymous:  field.Anonymous(),
		}

		// Get tag
		if i < strct.NumFields() {
			fieldInfo.Tag = strct.Tag(i)
		}

		typeInfo.Fields = append(typeInfo.Fields, fieldInfo)

		// Track embedded types
		if field.Anonymous() {
			embedInfo := &TypeInfo{
				Name: field.Type().String(),
				Type: field.Type(),
			}
			typeInfo.Embeds = append(typeInfo.Embeds, embedInfo)
		}
	}
}

// extractCallInfo extracts call information from a call expression
func (s *Service) extractCallInfo(pkg *packages.Package, call *ast.CallExpr) *FunctionCall {
	callInfo := &FunctionCall{
		Position: pkg.Fset.Position(call.Pos()).String(),
		Line:     pkg.Fset.Position(call.Pos()).Line,
	}

	// Function linking happens in the analyzer phase when we have
	// the complete function map. For now, we just record the call.
	return callInfo
}

// createInterfaceInfo creates interface information
func (s *Service) createInterfaceInfo(
	name string,
	iface *types.Interface,
	typeInfo *TypeInfo,
	packagePath string,
) *InterfaceInfo {
	ifaceInfo := &InterfaceInfo{
		Name:            name,
		Package:         packagePath,
		Type:            iface,
		Methods:         make([]*MethodInfo, 0),
		Embeds:          make([]*InterfaceInfo, 0),
		Implementations: make([]*Implementation, 0),
		LineStart:       typeInfo.LineStart,
		LineEnd:         typeInfo.LineEnd,
		IsExported:      typeInfo.IsExported,
	}

	// Extract methods
	for i := 0; i < iface.NumMethods(); i++ {
		method := iface.Method(i)
		// Use safe type assertion to avoid potential panic
		sig, ok := method.Type().(*types.Signature)
		if !ok {
			// This should never happen for interface methods, but handle it gracefully
			logger.Warn("unexpected non-signature type for interface method",
				"interface", ifaceInfo.Name,
				"method", method.Name(),
				"type", method.Type())
			continue
		}
		methodInfo := &MethodInfo{
			Name:      method.Name(),
			Signature: sig,
		}
		ifaceInfo.Methods = append(ifaceInfo.Methods, methodInfo)
	}

	// Extract embedded interfaces
	for i := 0; i < iface.NumEmbeddeds(); i++ {
		if embedded, ok := iface.EmbeddedType(i).(*types.Named); ok {
			if embeddedIface, ok := embedded.Underlying().(*types.Interface); ok {
				embedInfo := s.createInterfaceInfo(embedded.Obj().Name(), embeddedIface, typeInfo, packagePath)
				ifaceInfo.Embeds = append(ifaceInfo.Embeds, embedInfo)
			}
		}
	}

	return ifaceInfo
}

// linkMethodsToTypes associates methods with their receiver types
func (s *Service) linkMethodsToTypes(result *ParseResult) {
	// Create a map of types by package and name
	typeMap := make(map[string]*TypeInfo)
	for _, pkg := range result.Packages {
		for _, t := range pkg.Types {
			key := fmt.Sprintf("%s.%s", pkg.Path, t.Name)
			typeMap[key] = t
		}
	}

	// Link methods to their receiver types
	for _, pkg := range result.Packages {
		for _, fn := range pkg.Functions {
			if fn.Receiver != nil && fn.Receiver.Name != "" {
				// Extract the base type name (remove pointer notation and package prefix)
				receiverName := fn.Receiver.Name

				// Remove pointer notation
				receiverName = strings.TrimPrefix(receiverName, "*")

				// Remove package prefix (e.g., "main.FileWriter" -> "FileWriter")
				parts := strings.Split(receiverName, ".")
				if len(parts) > 1 {
					receiverName = parts[len(parts)-1]
				}

				// Look up the type
				key := fmt.Sprintf("%s.%s", pkg.Path, receiverName)
				if t, ok := typeMap[key]; ok {
					t.Methods = append(t.Methods, fn)
				}
			}
		}
	}
}

// findImplementations finds all interface implementations
func (s *Service) findImplementations(result *ParseResult) {
	// Create maps for efficient lookup
	allTypes := make(map[string]*TypeInfo)
	typesByMethodCount := make(map[int][]*TypeInfo)

	// Index types by path and method count
	for _, pkg := range result.Packages {
		for _, t := range pkg.Types {
			key := fmt.Sprintf("%s.%s", pkg.Path, t.Name)
			allTypes[key] = t

			// Index by method count for optimization
			methodCount := len(t.Methods)
			typesByMethodCount[methodCount] = append(typesByMethodCount[methodCount], t)
		}
	}

	// Check types against interfaces
	for _, pkg := range result.Packages {
		for _, iface := range pkg.Interfaces {
			if iface.Type == nil {
				continue
			}

			// Get minimum method count for this interface
			minMethods := iface.Type.NumMethods()

			// Only check types with at least the required number of methods
			for methodCount, typesWithCount := range typesByMethodCount {
				if methodCount < minMethods {
					continue
				}

				for _, t := range typesWithCount {
					if t.Type == nil {
						continue
					}

					// Check if type implements interface
					if types.Implements(t.Type, iface.Type) ||
						types.Implements(types.NewPointer(t.Type), iface.Type) {
						impl := s.createImplementation(t, iface)
						iface.Implementations = append(iface.Implementations, impl)
						t.Implements = append(t.Implements, iface)
					}
				}
			}
		}
	}
}

// createImplementation creates implementation information
func (s *Service) createImplementation(t *TypeInfo, iface *InterfaceInfo) *Implementation {
	impl := &Implementation{
		Type:           t,
		Interface:      iface,
		IsComplete:     true,
		MethodMatches:  make(map[string]*FunctionInfo),
		MissingMethods: make([]string, 0),
	}

	// Match methods
	methodSet := types.NewMethodSet(t.Type)
	ptrMethodSet := types.NewMethodSet(types.NewPointer(t.Type))

	for _, ifaceMethod := range iface.Methods {
		found := false

		// Check both value and pointer method sets
		for _, mset := range []*types.MethodSet{methodSet, ptrMethodSet} {
			if sel := mset.Lookup(nil, ifaceMethod.Name); sel != nil {
				found = true
				// Method linking to FunctionInfo happens in a second pass
				// when all functions have been collected
				break
			}
		}

		if !found {
			impl.IsComplete = false
			impl.MissingMethods = append(impl.MissingMethods, ifaceMethod.Name)
		}
	}

	return impl
}

// buildCallGraph builds the function call graph using RTA (Rapid Type Analysis)
func (s *Service) buildCallGraph(ssaProg *ssa.Program) *CallGraph {
	if ssaProg == nil {
		logger.Warn("SSA program is nil, cannot build call graph")
		return &CallGraph{
			Functions: make(map[string]*CallNode),
		}
	}

	// Find main functions as entry points for RTA analysis
	var mainFuncs []*ssa.Function
	for _, pkg := range ssaProg.AllPackages() {
		if pkg.Func("main") != nil {
			mainFuncs = append(mainFuncs, pkg.Func("main"))
		}
		// Also include init functions as entry points
		if pkg.Func("init") != nil {
			mainFuncs = append(mainFuncs, pkg.Func("init"))
		}
	}

	// If no main functions found, use all exported functions as entry points
	if len(mainFuncs) == 0 {
		logger.Info("No main functions found, using all exported functions as entry points")
		for _, pkg := range ssaProg.AllPackages() {
			for _, member := range pkg.Members {
				if fn, ok := member.(*ssa.Function); ok && fn.Object() != nil && fn.Object().Exported() {
					mainFuncs = append(mainFuncs, fn)
				}
			}
		}
	}

	if len(mainFuncs) == 0 {
		logger.Warn("No entry points found for call graph analysis")
		return &CallGraph{
			Functions: make(map[string]*CallNode),
		}
	}

	logger.Info("Building call graph using RTA", "entry_points", len(mainFuncs))

	// Perform RTA analysis to build call graph
	result := rta.Analyze(mainFuncs, true)
	cg := result.CallGraph

	// Convert callgraph.Graph to our CallGraph format
	return s.convertToCustomCallGraph(cg)
}

// convertToCustomCallGraph converts a golang.org/x/tools callgraph to our custom format
func (s *Service) convertToCustomCallGraph(cg *callgraph.Graph) *CallGraph {
	customCG := &CallGraph{
		Functions: make(map[string]*CallNode),
	}

	// Convert nodes
	for fn, node := range cg.Nodes {
		if fn == nil {
			continue
		}

		fnKey := s.getCallGraphFunctionKey(fn)
		callNode := &CallNode{
			Function: &FunctionInfo{
				Name: fn.Name(),
			},
			Calls:    make([]*CallNode, 0),
			CalledBy: make([]*CallNode, 0),
		}

		// Set package information if available
		if fn.Pkg != nil && fn.Pkg.Pkg != nil {
			callNode.Function.Name = fn.Pkg.Pkg.Path() + "." + fn.Name()
		}

		customCG.Functions[fnKey] = callNode

		// Set root if this is the root node
		if node == cg.Root {
			customCG.Root = callNode
		}
	}

	// Convert edges
	for _, node := range cg.Nodes {
		if node.Func == nil {
			continue
		}

		callerKey := s.getCallGraphFunctionKey(node.Func)
		callerNode := customCG.Functions[callerKey]

		for _, edge := range node.Out {
			if edge.Callee.Func == nil {
				continue
			}

			calleeKey := s.getCallGraphFunctionKey(edge.Callee.Func)
			if calleeNode, exists := customCG.Functions[calleeKey]; exists {
				callerNode.Calls = append(callerNode.Calls, calleeNode)
				calleeNode.CalledBy = append(calleeNode.CalledBy, callerNode)
			}
		}
	}

	logger.Info("Call graph conversion complete", "functions", len(customCG.Functions))
	return customCG
}

// getCallGraphFunctionKey creates a unique key for call graph functions
func (s *Service) getCallGraphFunctionKey(fn *ssa.Function) string {
	if fn.Pkg != nil && fn.Pkg.Pkg != nil {
		if fn.Signature.Recv() != nil {
			// Method: Package.ReceiverType.MethodName
			receiverType := fn.Signature.Recv().Type().String()
			return fmt.Sprintf("%s.%s.%s", fn.Pkg.Pkg.Path(), receiverType, fn.Name())
		}
		// Function: Package.FunctionName
		return fmt.Sprintf("%s.%s", fn.Pkg.Pkg.Path(), fn.Name())
	}
	// Fallback: just function name
	return fn.Name()
}

// ParseFile parses a single Go file (backward compatibility method)
func (s *Service) ParseFile(ctx context.Context, filePath string, config *Config) (*FileResult, error) {
	startTime := time.Now()

	// Validate and sanitize file path
	cleanPath, err := s.validateFilePath(filePath)
	if err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	if config == nil {
		config = s.config
	}

	// Get the directory containing the file to load its package
	dir := filepath.Dir(cleanPath)

	// Load the package containing this file
	pkgs, err := s.loadPackages(ctx, dir, config)
	if err != nil {
		return nil, err
	}

	// Find the package that contains our target file
	var targetPkg *packages.Package
	var targetFileInfo *FileInfo

	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			if i < len(pkg.CompiledGoFiles) {
				if pkg.CompiledGoFiles[i] == cleanPath {
					targetPkg = pkg
					targetFileInfo = s.processFile(pkg, file, pkg.CompiledGoFiles[i])
					break
				}
			}
		}
		if targetPkg != nil {
			break
		}
	}

	if targetPkg == nil {
		return nil, fmt.Errorf("file %s not found in any package", cleanPath)
	}

	// Create package info for the target package
	pkgInfo := s.processPackage(targetPkg, nil)

	result := &FileResult{
		FilePath:  cleanPath,
		Package:   pkgInfo,
		FileInfo:  targetFileInfo,
		ParseTime: time.Since(startTime).Milliseconds(),
	}

	return result, nil
}

// ParseDirectory parses all Go files in a directory (backward compatibility method)
func (s *Service) ParseDirectory(ctx context.Context, dirPath string, config *Config) (*DirectoryResult, error) {
	startTime := time.Now()

	// Validate and sanitize directory path
	cleanPath, err := s.validateProjectPath(dirPath)
	if err != nil {
		return nil, fmt.Errorf("invalid directory path: %w", err)
	}

	if config == nil {
		config = s.config
	}

	// Load packages from the directory (non-recursive)
	pkgConfig := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedSyntax |
			packages.NeedModule,
		Context: ctx,
		Dir:     cleanPath,
		Tests:   config.IncludeTests,
	}

	// Use "." pattern to load only packages in this directory
	pattern := "."
	pkgs, err := packages.Load(pkgConfig, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to load packages from directory: %w", err)
	}

	// Check for errors in loaded packages
	var loadErrors []string
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			for _, e := range pkg.Errors {
				loadErrors = append(loadErrors, e.Error())
			}
		}
	}

	if len(loadErrors) > 0 {
		return nil, fmt.Errorf("package loading failed with %d errors: %v", len(loadErrors), loadErrors)
	}

	// Filter packages
	filteredPkgs := s.filterPackages(pkgs, config)

	// Process packages
	var packageInfos []*PackageInfo
	for _, pkg := range filteredPkgs {
		pkgInfo := s.processPackage(pkg, nil)
		packageInfos = append(packageInfos, pkgInfo)
	}

	result := &DirectoryResult{
		DirectoryPath: cleanPath,
		Packages:      packageInfos,
		ParseTime:     time.Since(startTime).Milliseconds(),
	}

	return result, nil
}

// validateFilePath validates and sanitizes a file path
func (s *Service) validateFilePath(filePath string) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(filePath)

	// Convert to absolute path to eliminate any remaining relative components
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute file path: %w", err)
	}

	// Verify the path exists and is a file
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file does not exist: %s", absPath)
		}
		return "", fmt.Errorf("failed to access file: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", absPath)
	}

	// Check if it's a Go file
	if !strings.HasSuffix(absPath, ".go") {
		return "", fmt.Errorf("file is not a Go source file: %s", absPath)
	}

	return absPath, nil
}
