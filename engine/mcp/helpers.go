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

// Helper function to create JSON tool results
func newToolResultJSON(data any) (*mcp.CallToolResult, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return mcp.NewToolResultText(string(jsonData)), nil
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
