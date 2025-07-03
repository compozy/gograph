package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Helper function to convert old-style resource handlers to new format
func wrapResourceHandler(
	handler func(ctx context.Context, params map[string]string) ([]byte, error),
) func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Extract params from the URI
		params := make(map[string]string)
		uri := request.Params.URI

		// Simple parameter extraction - this could be improved
		if strings.Contains(uri, "{") {
			// Extract template parameters
			parts := strings.Split(uri, "/")
			for i, part := range parts {
				if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
					key := strings.Trim(part, "{}")
					if i+1 < len(parts) {
						params[key] = parts[i+1]
					}
				}
			}
		}

		data, err := handler(ctx, params)
		if err != nil {
			return nil, err
		}

		// Create TextResourceContents
		textContent := mcp.TextResourceContents{
			URI:      uri,
			Text:     string(data),
			MIMEType: "application/json",
		}

		return []mcp.ResourceContents{&textContent}, nil
	}
}

// Helper function to create tool results from ToolResponse content
// This properly handles structured data by using appropriate MCP content types
// instead of base64-encoding everything as text
func newToolResultFromResponse(response *ToolResponse) (*mcp.CallToolResult, error) {
	if response == nil || len(response.Content) == 0 {
		return mcp.NewToolResultText("No content available"), nil
	}

	// Convert response.Content ([]any) to []mcp.Content
	mcpContent := make([]mcp.Content, 0, len(response.Content))

	for _, content := range response.Content {
		mcpContentItem, err := convertToMCPContent(content)
		if err != nil {
			return nil, err
		}
		mcpContent = append(mcpContent, mcpContentItem)
	}

	return &mcp.CallToolResult{
		Content: mcpContent,
	}, nil
}

// convertToMCPContent converts a single content item to MCP Content
func convertToMCPContent(content any) (mcp.Content, error) {
	switch v := content.(type) {
	case map[string]any:
		return convertMapToMCPContent(v)
	case string:
		return mcp.TextContent{
			Type: "text",
			Text: v,
		}, nil
	default:
		return convertUnknownToMCPContent(v)
	}
}

// convertMapToMCPContent handles map[string]any content
func convertMapToMCPContent(v map[string]any) (mcp.Content, error) {
	contentType, hasType := v["type"].(string)
	if !hasType {
		return convertObjectToText(v)
	}

	switch contentType {
	case "text":
		return convertTextContent(v)
	case "resource":
		return convertResourceContent(v)
	default:
		return convertObjectToText(v)
	}
}

// convertTextContent handles text content type
func convertTextContent(v map[string]any) (mcp.Content, error) {
	if text, ok := v["text"].(string); ok {
		return mcp.TextContent{
			Type: "text",
			Text: text,
		}, nil
	}
	return convertObjectToText(v)
}

// convertResourceContent handles resource content type
func convertResourceContent(v map[string]any) (mcp.Content, error) {
	resourceData, ok := v["resource"].(map[string]any)
	if !ok {
		return convertObjectToText(v)
	}

	uri, hasURI := resourceData["uri"].(string)
	data, hasData := resourceData["data"]
	if !hasURI || !hasData {
		return convertObjectToText(v)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource data: %w", err)
	}

	textResource := mcp.TextResourceContents{
		URI:      uri,
		Text:     string(jsonData),
		MIMEType: "application/json",
	}

	return mcp.EmbeddedResource{
		Type:     "resource",
		Resource: &textResource,
	}, nil
}

// convertObjectToText converts any object to JSON text content
func convertObjectToText(v any) (mcp.Content, error) {
	jsonData, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content: %w", err)
	}
	return mcp.TextContent{
		Type: "text",
		Text: string(jsonData),
	}, nil
}

// convertUnknownToMCPContent handles unknown content types
func convertUnknownToMCPContent(v any) (mcp.Content, error) {
	return convertObjectToText(v)
}

// Helper to get string with default empty
func getString(req mcp.CallToolRequest, key string) string {
	return req.GetString(key, "")
}

// Helper to get bool (always returns false if not found)
func getBool(req mcp.CallToolRequest, key string) bool {
	// Try to get as string first
	strVal := req.GetString(key, "false")
	return strVal == "true" || strVal == "1"
}

// Helper to get float
func getFloat(_ mcp.CallToolRequest, _ string) float64 {
	// Implementation would need to parse from string
	// For now, return 0
	return 0
}
