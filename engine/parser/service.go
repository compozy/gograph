package parser

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/gograph/pkg/logger"
)

// Service implements the Parser interface
type Service struct {
	config *Config
}

// NewService creates a new parser service
func NewService(config *Config) Parser {
	if config == nil {
		config = &Config{
			IgnoreDirs:     []string{".git", ".idea", ".vscode", "node_modules"},
			IgnoreFiles:    []string{},
			IncludeTests:   true,
			IncludeVendor:  false,
			MaxConcurrency: 4,
		}
	}
	return &Service{
		config: config,
	}
}

// ParseFile parses a single Go file
func (s *Service) ParseFile(ctx context.Context, filePath string) (*FileInfo, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	info := &FileInfo{
		Path:         filePath,
		Package:      node.Name.Name,
		Imports:      []ImportInfo{},
		Functions:    []FunctionInfo{},
		Structs:      []StructInfo{},
		Interfaces:   []InterfaceInfo{},
		Constants:    []ConstantInfo{},
		Variables:    []VariableInfo{},
		Dependencies: []string{},
	}

	// Parse imports
	for _, imp := range node.Imports {
		impInfo := ImportInfo{
			Path: strings.Trim(imp.Path.Value, `"`),
		}
		if imp.Name != nil {
			impInfo.Alias = imp.Name.Name
		}
		info.Imports = append(info.Imports, impInfo)
		info.Dependencies = append(info.Dependencies, impInfo.Path)
	}

	// Walk the AST
	ast.Inspect(node, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			info.Functions = append(info.Functions, s.parseFunctionDecl(fset, decl))
		case *ast.GenDecl:
			s.parseGenDecl(fset, decl, info)
		}
		return true
	})

	return info, nil
}

// ParseDirectory parses all Go files in a directory
func (s *Service) ParseDirectory(ctx context.Context, dirPath string) ([]*FileInfo, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var files []*FileInfo
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			// Check if should ignore directory
			dirName := filepath.Base(path)
			for _, ignore := range s.config.IgnoreDirs {
				if dirName == ignore {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Skip non-Go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files if configured
		if !s.config.IncludeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip vendor if configured
		if !s.config.IncludeVendor && strings.Contains(path, "vendor/") {
			return nil
		}

		// Parse file
		info, err := s.ParseFile(ctx, path)
		if err != nil {
			logger.Warn("failed to parse file", "file", path, "error", err)
			return nil // Continue with other files
		}

		files = append(files, info)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return files, nil
}

// ParseProject parses an entire Go project
func (s *Service) ParseProject(ctx context.Context, projectPath string, config *Config) (*ParseResult, error) {
	startTime := time.Now()

	if config != nil {
		s.config = config
	}

	// Use a channel to collect file infos
	filesChan := make(chan *FileInfo, 100)
	var wg sync.WaitGroup
	var collectionWg sync.WaitGroup

	// Start file collection goroutine
	var allFiles []*FileInfo
	collectionWg.Add(1)
	go func() {
		defer collectionWg.Done()
		for file := range filesChan {
			allFiles = append(allFiles, file)
		}
	}()

	// Walk the project directory and parse files
	if err := s.walkAndParseFiles(ctx, projectPath, filesChan, &wg); err != nil {
		return nil, err
	}

	// Wait for all parsing to complete
	wg.Wait()
	close(filesChan)

	// Wait for collection goroutine to finish
	collectionWg.Wait()

	// Build parse result
	result := &ParseResult{
		ProjectPath: projectPath,
		Files:       allFiles,
		ParseTime:   time.Since(startTime).Milliseconds(),
	}

	return result, nil
}

// walkAndParseFiles walks the directory tree and parses Go files concurrently
func (s *Service) walkAndParseFiles(
	ctx context.Context,
	projectPath string,
	filesChan chan<- *FileInfo,
	wg *sync.WaitGroup,
) error {
	return filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			dirName := filepath.Base(path)
			for _, ignore := range s.config.IgnoreDirs {
				if dirName == ignore {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Skip non-Go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files if configured
		if !s.config.IncludeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip vendor if configured
		if !s.config.IncludeVendor && strings.Contains(path, "vendor/") {
			return nil
		}

		// Parse file in goroutine
		wg.Add(1)
		go func(filePath string) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			info, err := s.ParseFile(ctx, filePath)
			if err != nil {
				logger.Warn("failed to parse file", "file", filePath, "error", err)
				return
			}

			filesChan <- info
		}(path)

		return nil
	})
}

// parseFunctionDecl parses a function declaration
func (s *Service) parseFunctionDecl(fset *token.FileSet, decl *ast.FuncDecl) FunctionInfo {
	info := FunctionInfo{
		Name:       decl.Name.Name,
		IsExported: ast.IsExported(decl.Name.Name),
		LineStart:  fset.Position(decl.Pos()).Line,
		LineEnd:    fset.Position(decl.End()).Line,
		Parameters: []Parameter{},
		Returns:    []string{},
		Calls:      []FunctionCall{},
	}

	// Parse receiver
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		if t := s.getTypeName(decl.Recv.List[0].Type); t != "" {
			info.Receiver = t
		}
	}

	// Parse parameters
	if decl.Type.Params != nil {
		for _, field := range decl.Type.Params.List {
			paramType := s.getTypeName(field.Type)
			for _, name := range field.Names {
				info.Parameters = append(info.Parameters, Parameter{
					Name: name.Name,
					Type: paramType,
				})
			}
		}
	}

	// Parse return types
	if decl.Type.Results != nil {
		for _, field := range decl.Type.Results.List {
			info.Returns = append(info.Returns, s.getTypeName(field.Type))
		}
	}

	// Parse function calls within the body
	if decl.Body != nil {
		calls := s.extractFunctionCalls(decl.Body)
		info.Calls = calls
	}

	return info
}

// parseGenDecl parses a general declaration
func (s *Service) parseGenDecl(fset *token.FileSet, decl *ast.GenDecl, info *FileInfo) {
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			switch t := spec.Type.(type) {
			case *ast.StructType:
				info.Structs = append(info.Structs, s.parseStructType(fset, spec.Name.Name, t))
			case *ast.InterfaceType:
				info.Interfaces = append(info.Interfaces, s.parseInterfaceType(fset, spec.Name.Name, t))
			}
		case *ast.ValueSpec:
			switch decl.Tok {
			case token.CONST:
				for _, name := range spec.Names {
					info.Constants = append(info.Constants, ConstantInfo{
						Name:       name.Name,
						Type:       s.getTypeName(spec.Type),
						IsExported: ast.IsExported(name.Name),
					})
				}
			case token.VAR:
				for _, name := range spec.Names {
					info.Variables = append(info.Variables, VariableInfo{
						Name:       name.Name,
						Type:       s.getTypeName(spec.Type),
						IsExported: ast.IsExported(name.Name),
					})
				}
			}
		}
	}
}

// parseStructType parses a struct type
func (s *Service) parseStructType(fset *token.FileSet, name string, structType *ast.StructType) StructInfo {
	info := StructInfo{
		Name:       name,
		IsExported: ast.IsExported(name),
		LineStart:  fset.Position(structType.Pos()).Line,
		LineEnd:    fset.Position(structType.End()).Line,
		Fields:     []FieldInfo{},
		Embeds:     []string{},
	}

	for _, field := range structType.Fields.List {
		fieldType := s.getTypeName(field.Type)

		if len(field.Names) == 0 {
			// Embedded field
			info.Embeds = append(info.Embeds, fieldType)
		} else {
			// Regular fields
			for _, name := range field.Names {
				fieldInfo := FieldInfo{
					Name:       name.Name,
					Type:       fieldType,
					IsExported: ast.IsExported(name.Name),
				}
				if field.Tag != nil {
					fieldInfo.Tag = field.Tag.Value
				}
				info.Fields = append(info.Fields, fieldInfo)
			}
		}
	}

	return info
}

// parseInterfaceType parses an interface type
func (s *Service) parseInterfaceType(fset *token.FileSet, name string, ifaceType *ast.InterfaceType) InterfaceInfo {
	info := InterfaceInfo{
		Name:       name,
		IsExported: ast.IsExported(name),
		LineStart:  fset.Position(ifaceType.Pos()).Line,
		LineEnd:    fset.Position(ifaceType.End()).Line,
		Methods:    []MethodInfo{},
		Embeds:     []string{},
	}

	for _, method := range ifaceType.Methods.List {
		switch t := method.Type.(type) {
		case *ast.FuncType:
			// Method signature
			for _, name := range method.Names {
				methodInfo := MethodInfo{
					Name:       name.Name,
					Parameters: []Parameter{},
					Returns:    []string{},
				}

				// Parse parameters
				if t.Params != nil {
					for _, field := range t.Params.List {
						paramType := s.getTypeName(field.Type)
						for _, name := range field.Names {
							methodInfo.Parameters = append(methodInfo.Parameters, Parameter{
								Name: name.Name,
								Type: paramType,
							})
						}
					}
				}

				// Parse returns
				if t.Results != nil {
					for _, field := range t.Results.List {
						methodInfo.Returns = append(methodInfo.Returns, s.getTypeName(field.Type))
					}
				}

				info.Methods = append(info.Methods, methodInfo)
			}
		default:
			// Embedded interface
			if typeName := s.getTypeName(method.Type); typeName != "" {
				info.Embeds = append(info.Embeds, typeName)
			}
		}
	}

	return info
}

// getTypeName extracts the type name from an expression
func (s *Service) getTypeName(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
	case *ast.StarExpr:
		return "*" + s.getTypeName(t.X)
	case *ast.ArrayType:
		return "[]" + s.getTypeName(t.Elt)
	case *ast.MapType:
		return "map[" + s.getTypeName(t.Key) + "]" + s.getTypeName(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{}"
	case *ast.FuncType:
		return "func"
	}

	return ""
}

// extractFunctionCalls walks the AST to find function calls
func (s *Service) extractFunctionCalls(node ast.Node) []FunctionCall {
	var calls []FunctionCall

	ast.Inspect(node, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if call := s.parseCallExpr(callExpr); call != nil {
				calls = append(calls, *call)
			}
		}
		return true
	})

	return calls
}

// parseCallExpr parses a function call expression
func (s *Service) parseCallExpr(callExpr *ast.CallExpr) *FunctionCall {
	if callExpr.Fun == nil {
		return nil
	}

	switch fun := callExpr.Fun.(type) {
	case *ast.Ident:
		// Simple function call: foo()
		return &FunctionCall{
			Name:    fun.Name,
			Package: "",
		}
	case *ast.SelectorExpr:
		// Package function call: pkg.Foo() or method call: obj.Method()
		if x, ok := fun.X.(*ast.Ident); ok {
			return &FunctionCall{
				Name:    fun.Sel.Name,
				Package: x.Name,
			}
		}
	}

	return nil
}
