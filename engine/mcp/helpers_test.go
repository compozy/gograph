package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewToolResultFromResponse(t *testing.T) {
	t.Run("Should handle nil response", func(t *testing.T) {
		result, err := newToolResultFromResponse(nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "No content available", textContent.Text)
	})
	t.Run("Should handle empty content", func(t *testing.T) {
		resp := &ToolResponse{Content: []any{}}
		result, err := newToolResultFromResponse(resp)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "No content available", textContent.Text)
	})
	t.Run("Should handle simple string content", func(t *testing.T) {
		resp := &ToolResponse{
			Content: []any{"simple string content"},
		}
		result, err := newToolResultFromResponse(resp)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "text", textContent.Type)
		assert.Equal(t, "simple string content", textContent.Text)
	})
	t.Run("Should handle text content type", func(t *testing.T) {
		resp := &ToolResponse{
			Content: []any{
				map[string]any{
					"type": "text",
					"text": "hello world",
				},
			},
		}
		result, err := newToolResultFromResponse(resp)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "text", textContent.Type)
		assert.Equal(t, "hello world", textContent.Text)
	})
	t.Run("Should handle resource content type", func(t *testing.T) {
		resp := &ToolResponse{
			Content: []any{
				map[string]any{
					"type": "resource",
					"resource": map[string]any{
						"uri": "/my/resource/uri",
						"data": map[string]any{
							"key":   "value",
							"count": 42,
						},
					},
				},
			},
		}
		result, err := newToolResultFromResponse(resp)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Content, 1)
		embeddedResource, ok := result.Content[0].(mcp.EmbeddedResource)
		require.True(t, ok)
		assert.Equal(t, "resource", embeddedResource.Type)
		textResource, ok := embeddedResource.Resource.(*mcp.TextResourceContents)
		require.True(t, ok)
		assert.Equal(t, "/my/resource/uri", textResource.URI)
		assert.Equal(t, "application/json", textResource.MIMEType)
		assert.JSONEq(t, `{"key":"value","count":42}`, textResource.Text)
	})
	t.Run("Should handle mixed content types", func(t *testing.T) {
		resp := &ToolResponse{
			Content: []any{
				"plain string",
				map[string]any{
					"type": "text",
					"text": "structured text",
				},
				map[string]any{
					"custom": "data",
					"value":  123,
				},
			},
		}
		result, err := newToolResultFromResponse(resp)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Content, 3)
		// First item: plain string
		textContent1, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "plain string", textContent1.Text)
		// Second item: structured text
		textContent2, ok := result.Content[1].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "structured text", textContent2.Text)
		// Third item: custom object converted to JSON
		textContent3, ok := result.Content[2].(mcp.TextContent)
		require.True(t, ok)
		assert.JSONEq(t, `{"custom":"data","value":123}`, textContent3.Text)
	})
	t.Run("Should handle malformed resource content", func(t *testing.T) {
		resp := &ToolResponse{
			Content: []any{
				map[string]any{
					"type": "resource",
					// Missing resource data
				},
			},
		}
		result, err := newToolResultFromResponse(resp)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Content, 1)
		// Should fall back to JSON serialization
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "resource")
	})
	t.Run("Should handle unknown content type", func(t *testing.T) {
		resp := &ToolResponse{
			Content: []any{
				map[string]any{
					"type": "unknown",
					"data": "some data",
				},
			},
		}
		result, err := newToolResultFromResponse(resp)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Content, 1)
		// Should serialize as JSON
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.JSONEq(t, `{"type":"unknown","data":"some data"}`, textContent.Text)
	})
	t.Run("Should handle complex nested structures", func(t *testing.T) {
		resp := &ToolResponse{
			Content: []any{
				map[string]any{
					"projects": []map[string]any{
						{
							"id":   "proj1",
							"name": "Project One",
							"stats": map[string]int{
								"files":     100,
								"functions": 250,
							},
						},
						{
							"id":   "proj2",
							"name": "Project Two",
							"stats": map[string]int{
								"files":     200,
								"functions": 500,
							},
						},
					},
				},
			},
		}
		result, err := newToolResultFromResponse(resp)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		// Verify the JSON contains expected data
		assert.Contains(t, textContent.Text, "proj1")
		assert.Contains(t, textContent.Text, "Project One")
		assert.Contains(t, textContent.Text, "250")
	})
}
