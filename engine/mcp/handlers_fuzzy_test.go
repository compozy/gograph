package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/compozy/gograph/engine/analyzer"
	"github.com/compozy/gograph/engine/core"
	"github.com/compozy/gograph/engine/graph"
	"github.com/compozy/gograph/engine/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestHandleTraceCallChainInternal_FuzzyMatching tests fuzzy matching for call chain tracing
func TestHandleTraceCallChainInternal_FuzzyMatching(t *testing.T) {
	tests := []struct {
		name             string
		input            map[string]any
		mockQueryReturns []map[string]any
		mockQueryError   error
		expectedCount    int
		wantError        bool
		checkQuery       func(t *testing.T, query string, params map[string]any)
	}{
		{
			name: "Should use fuzzy matching for from_function only",
			input: map[string]any{
				"project_id":    "test-proj",
				"from_function": "handleget",
				"max_depth":     3.0,
			},
			mockQueryReturns: []map[string]any{
				{
					"call_chain": []map[string]any{
						{"name": "HandleGetFunction", "package": "mcp", "file_path": "/mcp/handlers.go"},
						{"name": "GetFunction", "package": "service", "file_path": "/service/functions.go"},
					},
					"depth":        2,
					"actual_start": "HandleGetFunction",
				},
			},
			expectedCount: 1,
			checkQuery: func(t *testing.T, query string, params map[string]any) {
				assert.Contains(t, query, "toLower(start.name) CONTAINS toLower($from_function)")
				assert.Equal(t, "test-proj", params["project_id"])
				assert.Equal(t, "handleget", params["from_function"])
				assert.Equal(t, 3, params["max_depth"])
			},
		},
		{
			name: "Should use fuzzy matching for both from and to functions",
			input: map[string]any{
				"project_id":    "test-proj",
				"from_function": "handle",
				"to_function":   "execute",
				"max_depth":     5.0,
			},
			mockQueryReturns: []map[string]any{
				{
					"call_chain": []map[string]any{
						{"name": "HandleRequest", "package": "mcp"},
						{"name": "ProcessRequest", "package": "service"},
						{"name": "ExecuteQuery", "package": "query"},
					},
					"depth":        3,
					"actual_start": "HandleRequest",
					"actual_end":   "ExecuteQuery",
				},
			},
			expectedCount: 1,
			checkQuery: func(t *testing.T, query string, params map[string]any) {
				assert.Contains(t, query, "toLower(start.name) CONTAINS toLower($from_function)")
				assert.Contains(t, query, "toLower(end.name) CONTAINS toLower($to_function)")
				assert.Equal(t, "handle", params["from_function"])
				assert.Equal(t, "execute", params["to_function"])
			},
		},
		{
			name: "Should handle exact match with fuzzy fallback",
			input: map[string]any{
				"project_id":    "test-proj",
				"from_function": "HandleAnalyzeProject",
			},
			mockQueryReturns: []map[string]any{
				{
					"call_chain": []map[string]any{
						{"name": "HandleAnalyzeProject", "package": "mcp"},
					},
					"depth":        1,
					"actual_start": "HandleAnalyzeProject",
				},
			},
			expectedCount: 1,
			checkQuery: func(t *testing.T, query string, params map[string]any) {
				// Should include both exact match and fuzzy match
				assert.Contains(t, query, "start.name = $from_function OR")
				assert.Contains(t, query, "toLower(start.name) CONTAINS toLower($from_function)")
				assert.Equal(t, "test-proj", params["project_id"])
				assert.Equal(t, "HandleAnalyzeProject", params["from_function"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdapter := new(MockServiceAdapter)
			server := &Server{
				serviceAdapter: mockAdapter,
			}

			// Setup mock expectations
			if tt.mockQueryError != nil {
				mockAdapter.On("ExecuteQuery", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).
					Return(nil, tt.mockQueryError).
					Once()
			} else {
				mockAdapter.On("ExecuteQuery", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).
					Run(func(args mock.Arguments) {
						query := args.Get(1).(string)
						params := args.Get(2).(map[string]any)
						if tt.checkQuery != nil {
							tt.checkQuery(t, query, params)
						}
					}).
					Return(tt.mockQueryReturns, nil).Once()
			}

			// Execute the handler
			resp, err := server.HandleTraceCallChainInternal(context.Background(), tt.input)

			// Verify results
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Extract the data from the response
				require.Len(t, resp.Content, 2)
				resourceContent := resp.Content[1].(map[string]any)
				data := resourceContent["resource"].(map[string]any)["data"].(map[string]any)

				assert.Equal(t, tt.expectedCount, data["count"])
			}

			mockAdapter.AssertExpectations(t)
		})
	}
}

// TestHandleGetFunctionInfoInternal_EnhancedQueries tests enhanced function info queries
func TestHandleGetFunctionInfoInternal_EnhancedQueries(t *testing.T) {
	tests := []struct {
		name                 string
		input                map[string]any
		mockFunctionQuery    []map[string]any
		mockCallersQuery     []map[string]any
		mockCallsQuery       []map[string]any
		expectedFunctionName string
		expectedCallerCount  int
		expectedCallsCount   int
		checkQueries         func(t *testing.T, queries []string)
	}{
		{
			name: "Should return comprehensive function information with callers and calls",
			input: map[string]any{
				"project_id":      "test-proj",
				"function_name":   "ProcessRequest",
				"include_callers": true,
				"include_calls":   true,
			},
			mockFunctionQuery: []map[string]any{
				{
					"f": map[string]any{
						"name":        "ProcessRequest",
						"package":     "service",
						"signature":   "func (s *Service) ProcessRequest(ctx context.Context, req Request) error",
						"is_exported": true,
						"line_start":  100,
						"line_end":    150,
						"file_path":   "/service/process.go",
					},
					"file_path": "/service/process.go",
				},
			},
			mockCallersQuery: []map[string]any{
				{
					"name":        "HandleRequest",
					"package":     "handler",
					"file_path":   "/handler/main.go",
					"line_start":  50,
					"signature":   "func HandleRequest(ctx context.Context, req Request) error",
					"is_exported": true,
				},
				{
					"name":        "TestProcessRequest",
					"package":     "service",
					"file_path":   "/service/process_test.go",
					"line_start":  20,
					"signature":   "func TestProcessRequest(t *testing.T)",
					"is_exported": true,
				},
			},
			mockCallsQuery: []map[string]any{
				{
					"name":        "ValidateRequest",
					"package":     "validator",
					"file_path":   "/validator/validate.go",
					"line_start":  10,
					"signature":   "func ValidateRequest(req Request) error",
					"is_exported": true,
				},
				{
					"name":        "SaveRequest",
					"package":     "database",
					"file_path":   "/database/save.go",
					"line_start":  200,
					"signature":   "func SaveRequest(ctx context.Context, req Request) error",
					"is_exported": true,
				},
			},
			expectedFunctionName: "ProcessRequest",
			expectedCallerCount:  2,
			expectedCallsCount:   2,
			checkQueries: func(t *testing.T, queries []string) {
				require.Len(t, queries, 3)
				// The first query fetches the function itself and returns the full node
				// The next queries fetch callers and calls with specific fields
				for i, query := range queries {
					if i == 0 {
						// First query returns the full function node
						assert.Contains(t, query, "RETURN f")
					} else if strings.Contains(query, "RETURN") {
						// Caller and calls queries return specific fields
						assert.Contains(t, query, "file_path")
						assert.Contains(t, query, "signature")
						assert.Contains(t, query, "line_start")
						assert.Contains(t, query, "is_exported")
					}
				}
			},
		},
		{
			name: "Should handle fuzzy matching for function names",
			input: map[string]any{
				"project_id":      "test-proj",
				"function_name":   "handle",
				"include_callers": false,
				"include_calls":   false,
			},
			mockFunctionQuery: []map[string]any{
				{
					"f": map[string]any{
						"name":        "HandleRequest",
						"package":     "handler",
						"signature":   "func HandleRequest(ctx context.Context, req Request) error",
						"is_exported": true,
						"line_start":  50,
						"line_end":    100,
						"file_path":   "/handler/main.go",
					},
					"file_path": "/handler/main.go",
				},
			},
			expectedFunctionName: "handle", // The handler returns the searched name, not the actual name
			expectedCallerCount:  0,
			expectedCallsCount:   0,
			checkQueries: func(t *testing.T, queries []string) {
				require.Len(t, queries, 1)
				assert.Contains(t, queries[0], "toLower(f.name) CONTAINS toLower($function_name)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdapter := new(MockServiceAdapter)
			server := &Server{
				serviceAdapter: mockAdapter,
			}

			var capturedQueries []string

			// Setup mock for main function query
			mockAdapter.On("ExecuteQuery", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).
				Run(func(args mock.Arguments) {
					query := args.Get(1).(string)
					capturedQueries = append(capturedQueries, query)
				}).
				Return(tt.mockFunctionQuery, nil).
				Once()

			// Setup mock for callers query if needed
			if tt.input["include_callers"].(bool) {
				mockAdapter.On("ExecuteQuery", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).
					Run(func(args mock.Arguments) {
						query := args.Get(1).(string)
						capturedQueries = append(capturedQueries, query)
					}).
					Return(tt.mockCallersQuery, nil).
					Once()
			}

			// Setup mock for calls query if needed
			if tt.input["include_calls"].(bool) {
				mockAdapter.On("ExecuteQuery", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}")).
					Run(func(args mock.Arguments) {
						query := args.Get(1).(string)
						capturedQueries = append(capturedQueries, query)
					}).
					Return(tt.mockCallsQuery, nil).
					Once()
			}

			// Execute the handler
			resp, err := server.HandleGetFunctionInfoInternal(context.Background(), tt.input)

			// Verify results
			require.NoError(t, err)
			require.NotNil(t, resp)

			// Extract the data from the response
			require.Len(t, resp.Content, 2)
			resourceContent := resp.Content[1].(map[string]any)
			data := resourceContent["resource"].(map[string]any)["data"].(map[string]any)

			// The function info is directly in data, not under a "function" key
			assert.Equal(t, tt.expectedFunctionName, data["function_name"])

			// Handle cases where callers/calls might be nil
			if callers, ok := data["callers"].([]map[string]any); ok {
				assert.Equal(t, tt.expectedCallerCount, len(callers))
			} else {
				assert.Equal(t, 0, tt.expectedCallerCount)
			}

			if calls, ok := data["calls"].([]map[string]any); ok {
				assert.Equal(t, tt.expectedCallsCount, len(calls))
			} else {
				assert.Equal(t, 0, tt.expectedCallsCount)
			}

			// Check queries if verifier provided
			if tt.checkQueries != nil {
				tt.checkQueries(t, capturedQueries)
			}

			mockAdapter.AssertExpectations(t)
		})
	}
}

// MockServiceAdapter is a mock implementation of ServiceAdapter
type MockServiceAdapter struct {
	mock.Mock
}

func (m *MockServiceAdapter) ParseProject(ctx context.Context, projectPath string) (*parser.ParseResult, error) {
	args := m.Called(ctx, projectPath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*parser.ParseResult), args.Error(1)
}

func (m *MockServiceAdapter) AnalyzeProject(
	ctx context.Context,
	projectID core.ID,
	parseResult *parser.ParseResult,
) (*analyzer.AnalysisReport, error) {
	args := m.Called(ctx, projectID, parseResult)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*analyzer.AnalysisReport), args.Error(1)
}

func (m *MockServiceAdapter) InitializeProject(ctx context.Context, project *core.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *MockServiceAdapter) BuildAnalysisResult(
	ctx context.Context,
	projectID core.ID,
	parseResult *parser.ParseResult,
	analysisReport *analyzer.AnalysisReport,
) (*core.AnalysisResult, error) {
	args := m.Called(ctx, projectID, parseResult, analysisReport)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.AnalysisResult), args.Error(1)
}

func (m *MockServiceAdapter) ImportAnalysisResult(
	ctx context.Context,
	result *core.AnalysisResult,
) (*graph.ProjectGraph, error) {
	args := m.Called(ctx, result)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*graph.ProjectGraph), args.Error(1)
}

func (m *MockServiceAdapter) GetProjectStatistics(
	ctx context.Context,
	projectID core.ID,
) (*graph.ProjectStatistics, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*graph.ProjectStatistics), args.Error(1)
}

func (m *MockServiceAdapter) ExecuteQuery(
	ctx context.Context,
	query string,
	params map[string]any,
) ([]map[string]any, error) {
	args := m.Called(ctx, query, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]map[string]any), args.Error(1)
}

func (m *MockServiceAdapter) ListProjects(ctx context.Context) ([]core.Project, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]core.Project), args.Error(1)
}

func (m *MockServiceAdapter) ValidateProject(ctx context.Context, projectID core.ID) (bool, error) {
	args := m.Called(ctx, projectID)
	return args.Bool(0), args.Error(1)
}

func (m *MockServiceAdapter) ClearProject(ctx context.Context, projectID core.ID) error {
	args := m.Called(ctx, projectID)
	return args.Error(0)
}
