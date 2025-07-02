package core

import (
	"fmt"
)

// ErrorCode represents a unique error code
type ErrorCode string

const (
	// Parser errors
	ErrorCodeParseFailure      ErrorCode = "PARSE_FAILURE"
	ErrorCodeInvalidSyntax     ErrorCode = "INVALID_SYNTAX"
	ErrorCodeFileNotFound      ErrorCode = "FILE_NOT_FOUND"
	ErrorCodeDirectoryNotFound ErrorCode = "DIRECTORY_NOT_FOUND"

	// Graph errors
	ErrorCodeGraphConnection  ErrorCode = "GRAPH_CONNECTION_FAILED"
	ErrorCodeGraphQuery       ErrorCode = "GRAPH_QUERY_FAILED"
	ErrorCodeGraphWrite       ErrorCode = "GRAPH_WRITE_FAILED"
	ErrorCodeGraphTransaction ErrorCode = "GRAPH_TRANSACTION_FAILED"

	// Configuration errors
	ErrorCodeConfigNotFound ErrorCode = "CONFIG_NOT_FOUND"
	ErrorCodeConfigInvalid  ErrorCode = "CONFIG_INVALID"
	ErrorCodeConfigWrite    ErrorCode = "CONFIG_WRITE_FAILED"

	// Analysis errors
	ErrorCodeAnalysisFailed ErrorCode = "ANALYSIS_FAILED"
	ErrorCodeNoGoFiles      ErrorCode = "NO_GO_FILES_FOUND"

	// Validation errors
	ErrorCodeValidationFailed ErrorCode = "VALIDATION_FAILED"
	ErrorCodeInvalidInput     ErrorCode = "INVALID_INPUT"
)

// Error represents a structured error with code and metadata
type Error struct {
	Err      error          `json:"error"`
	Code     ErrorCode      `json:"code"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewError creates a new structured error for domain boundaries
func NewError(err error, code ErrorCode, metadata map[string]any) *Error {
	return &Error{
		Err:      err,
		Code:     code,
		Metadata: metadata,
	}
}

// Error implements the error interface
func (e *Error) Error() string {
	if len(e.Metadata) > 0 {
		return fmt.Sprintf("[%s] %v (metadata: %v)", e.Code, e.Err, e.Metadata)
	}
	return fmt.Sprintf("[%s] %v", e.Code, e.Err)
}

// Unwrap returns the wrapped error
func (e *Error) Unwrap() error {
	return e.Err
}

// Is checks if the error matches the target error
func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.Code == t.Code
	}
	return false
}
