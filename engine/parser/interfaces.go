package parser

import (
	"context"
)

// Parser defines the interface for parsing Go source code
type Parser interface {
	ParseFile(ctx context.Context, filePath string) (*FileInfo, error)
	ParseDirectory(ctx context.Context, dirPath string) ([]*FileInfo, error)
	ParseProject(ctx context.Context, projectPath string, config *Config) (*ParseResult, error)
}

// ParseResult contains the results of parsing a project
type ParseResult struct {
	ProjectPath string      // Root path of the project
	Files       []*FileInfo // All parsed files
	ParseTime   int64       // Time taken to parse in milliseconds
}

// FileInfo represents parsed information from a Go file
type FileInfo struct {
	Path         string
	Package      string
	Imports      []ImportInfo
	Functions    []FunctionInfo
	Structs      []StructInfo
	Interfaces   []InterfaceInfo
	Constants    []ConstantInfo
	Variables    []VariableInfo
	Dependencies []string
}

// ImportInfo represents an import statement
type ImportInfo struct {
	Name  string
	Path  string
	Alias string
}

// FunctionInfo represents a function or method
type FunctionInfo struct {
	Name       string
	Receiver   string
	Signature  string
	Parameters []Parameter
	Returns    []string
	Body       string
	LineStart  int
	LineEnd    int
	IsExported bool
	Calls      []FunctionCall
}

// Parameter represents a function parameter
type Parameter struct {
	Name string
	Type string
}

// FunctionCall represents a function call within a function
type FunctionCall struct {
	Name    string
	Package string
	Line    int
}

// StructInfo represents a struct type
type StructInfo struct {
	Name       string
	Fields     []FieldInfo
	Methods    []FunctionInfo
	Embeds     []string
	LineStart  int
	LineEnd    int
	IsExported bool
}

// FieldInfo represents a struct field
type FieldInfo struct {
	Name       string
	Type       string
	Tag        string
	IsExported bool
}

// InterfaceInfo represents an interface type
type InterfaceInfo struct {
	Name       string
	Methods    []MethodInfo
	Embeds     []string
	LineStart  int
	LineEnd    int
	IsExported bool
}

// MethodInfo represents an interface method
type MethodInfo struct {
	Name       string
	Parameters []Parameter
	Returns    []string
}

// ConstantInfo represents a constant declaration
type ConstantInfo struct {
	Name       string
	Type       string
	Value      string
	IsExported bool
}

// VariableInfo represents a variable declaration
type VariableInfo struct {
	Name       string
	Type       string
	IsExported bool
}

// Config represents parser configuration
type Config struct {
	IgnoreDirs     []string
	IgnoreFiles    []string
	IncludeTests   bool
	IncludeVendor  bool
	MaxConcurrency int
}
