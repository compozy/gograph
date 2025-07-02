package query

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExporter_ExportJSON(t *testing.T) {
	results := []map[string]any{
		{"name": "function1", "count": int64(5), "exported": true},
		{"name": "function2", "count": int64(3), "exported": false},
	}

	t.Run("Should_export_JSON_with_pretty_formatting", func(t *testing.T) {
		options := DefaultExportOptions(FormatJSON)
		options.Pretty = true
		exporter := NewExporter(options)

		var buf bytes.Buffer
		err := exporter.Export(&buf, results)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "function1")
		assert.Contains(t, output, "function2")
		assert.Contains(t, output, "  ") // Should have indentation
	})

	t.Run("Should_export_JSON_without_pretty_formatting", func(t *testing.T) {
		options := DefaultExportOptions(FormatJSON)
		options.Pretty = false
		exporter := NewExporter(options)

		var buf bytes.Buffer
		err := exporter.Export(&buf, results)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "function1")
		assert.NotContains(t, output, "  ") // Should not have indentation
	})
}

func TestExporter_ExportCSV(t *testing.T) {
	results := []map[string]any{
		{"name": "function1", "count": int64(5), "exported": true},
		{"name": "function2", "count": int64(3), "exported": false},
	}

	t.Run("Should_export_CSV_with_headers", func(t *testing.T) {
		options := DefaultExportOptions(FormatCSV)
		options.Headers = true
		exporter := NewExporter(options)

		var buf bytes.Buffer
		err := exporter.Export(&buf, results)
		require.NoError(t, err)

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		assert.Len(t, lines, 3) // header + 2 data rows

		// Check header line contains expected columns
		header := lines[0]
		assert.Contains(t, header, "count")
		assert.Contains(t, header, "exported")
		assert.Contains(t, header, "name")
	})

	t.Run("Should_export_CSV_without_headers", func(t *testing.T) {
		options := DefaultExportOptions(FormatCSV)
		options.Headers = false
		exporter := NewExporter(options)

		var buf bytes.Buffer
		err := exporter.Export(&buf, results)
		require.NoError(t, err)

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		assert.Len(t, lines, 2) // only 2 data rows, no header
	})
}

func TestExporter_ExportTSV(t *testing.T) {
	results := []map[string]any{
		{"name": "function1", "count": int64(5)},
		{"name": "function2", "count": int64(3)},
	}

	t.Run("Should_export_TSV_format", func(t *testing.T) {
		options := DefaultExportOptions(FormatTSV)
		exporter := NewExporter(options)

		var buf bytes.Buffer
		err := exporter.Export(&buf, results)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "\t") // Should contain tab separators
		assert.Contains(t, output, "function1")
		assert.Contains(t, output, "5")
	})
}

func TestExporter_ExportEmpty(t *testing.T) {
	results := []map[string]any{}

	t.Run("Should_export_empty_JSON", func(t *testing.T) {
		options := DefaultExportOptions(FormatJSON)
		exporter := NewExporter(options)

		var buf bytes.Buffer
		err := exporter.Export(&buf, results)
		require.NoError(t, err)
		assert.Equal(t, "[]", buf.String())
	})

	t.Run("Should_export_empty_CSV", func(t *testing.T) {
		options := DefaultExportOptions(FormatCSV)
		exporter := NewExporter(options)

		var buf bytes.Buffer
		err := exporter.Export(&buf, results)
		require.NoError(t, err)
		assert.Empty(t, buf.String())
	})
}

func TestResultProcessor(t *testing.T) {
	rp := NewResultProcessor()
	results := []map[string]any{
		{"name": "z", "count": int64(1)},
		{"name": "a", "count": int64(3)},
		{"name": "m", "count": int64(2)},
	}

	t.Run("Should_sort_by_field_ascending", func(t *testing.T) {
		sorted := rp.SortBy(results, "name", true)
		assert.Equal(t, "a", sorted[0]["name"])
		assert.Equal(t, "m", sorted[1]["name"])
		assert.Equal(t, "z", sorted[2]["name"])
	})

	t.Run("Should_sort_by_field_descending", func(t *testing.T) {
		sorted := rp.SortBy(results, "count", false)
		assert.Equal(t, int64(3), sorted[0]["count"])
		assert.Equal(t, int64(2), sorted[1]["count"])
		assert.Equal(t, int64(1), sorted[2]["count"])
	})

	t.Run("Should_filter_results", func(t *testing.T) {
		filtered := rp.Filter(results, func(row map[string]any) bool {
			count := row["count"].(int64)
			return count > 1
		})
		assert.Len(t, filtered, 2)
	})
}
