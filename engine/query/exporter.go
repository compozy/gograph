package query

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ExportFormat represents the export format
type ExportFormat string

const (
	FormatJSON ExportFormat = "json"
	FormatCSV  ExportFormat = "csv"
	FormatTSV  ExportFormat = "tsv"
)

// ExportOptions contains options for exporting query results
type ExportOptions struct {
	Format      ExportFormat `json:"format"`
	Pretty      bool         `json:"pretty"`       // For JSON: pretty formatting
	Headers     bool         `json:"headers"`      // For CSV/TSV: include headers
	Delimiter   string       `json:"delimiter"`    // For CSV/TSV: custom delimiter
	NullValue   string       `json:"null_value"`   // How to represent null values
	BoolFormat  string       `json:"bool_format"`  // "true/false" or "1/0"
	DateFormat  string       `json:"date_format"`  // Date formatting
	IncludeNull bool         `json:"include_null"` // Whether to include null fields in JSON
}

// DefaultExportOptions returns default export options
func DefaultExportOptions(format ExportFormat) *ExportOptions {
	opts := &ExportOptions{
		Format:      format,
		Headers:     true,
		NullValue:   "",
		BoolFormat:  "true/false",
		DateFormat:  "2006-01-02T15:04:05Z07:00",
		IncludeNull: false,
	}

	switch format {
	case FormatJSON:
		opts.Pretty = true
	case FormatCSV:
		opts.Delimiter = ","
	case FormatTSV:
		opts.Delimiter = "\t"
	}

	return opts
}

// Exporter handles exporting query results to different formats
type Exporter struct {
	options *ExportOptions
}

// NewExporter creates a new exporter with the specified options
func NewExporter(options *ExportOptions) *Exporter {
	if options == nil {
		options = DefaultExportOptions(FormatJSON)
	}
	return &Exporter{
		options: options,
	}
}

// Export exports the query results to the specified writer
func (e *Exporter) Export(writer io.Writer, results []map[string]any) error {
	if len(results) == 0 {
		return e.exportEmpty(writer)
	}

	switch e.options.Format {
	case FormatJSON:
		return e.exportJSON(writer, results)
	case FormatCSV, FormatTSV:
		return e.exportCSV(writer, results)
	default:
		return fmt.Errorf("unsupported export format: %s", e.options.Format)
	}
}

// exportEmpty handles exporting empty result sets
func (e *Exporter) exportEmpty(writer io.Writer) error {
	switch e.options.Format {
	case FormatJSON:
		_, err := writer.Write([]byte("[]"))
		return err
	case FormatCSV, FormatTSV:
		// Write empty CSV/TSV
		return nil
	default:
		return fmt.Errorf("unsupported export format: %s", e.options.Format)
	}
}

// exportJSON exports results as JSON
func (e *Exporter) exportJSON(writer io.Writer, results []map[string]any) error {
	// Process results to handle null values and format consistency
	processedResults := make([]map[string]any, 0, len(results))

	for _, result := range results {
		processed := make(map[string]any)
		for key, value := range result {
			processedValue := e.processValue(value)
			if processedValue != nil || e.options.IncludeNull {
				processed[key] = processedValue
			}
		}
		processedResults = append(processedResults, processed)
	}

	var data []byte
	var err error

	if e.options.Pretty {
		data, err = json.MarshalIndent(processedResults, "", "  ")
	} else {
		data, err = json.Marshal(processedResults)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	_, err = writer.Write(data)
	return err
}

// exportCSV exports results as CSV or TSV
func (e *Exporter) exportCSV(writer io.Writer, results []map[string]any) error {
	if len(results) == 0 {
		return nil
	}

	csvWriter := csv.NewWriter(writer)
	if e.options.Delimiter != "" {
		delimiter, _ := utf8.DecodeRuneInString(e.options.Delimiter)
		csvWriter.Comma = delimiter
	}
	defer csvWriter.Flush()

	// Get all unique column names and sort them for consistent output
	columnSet := make(map[string]bool)
	for _, result := range results {
		for key := range result {
			columnSet[key] = true
		}
	}

	columns := make([]string, 0, len(columnSet))
	for column := range columnSet {
		columns = append(columns, column)
	}
	sort.Strings(columns)

	// Write headers
	if e.options.Headers {
		if err := csvWriter.Write(columns); err != nil {
			return fmt.Errorf("failed to write CSV headers: %w", err)
		}
	}

	// Write data rows
	for _, result := range results {
		row := make([]string, len(columns))
		for i, column := range columns {
			value := result[column]
			row[i] = e.formatValueForCSV(value)
		}
		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// processValue processes a value for JSON export
func (e *Exporter) processValue(value any) any {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case bool:
		if e.options.BoolFormat == "1/0" {
			if v {
				return 1
			}
			return 0
		}
		return v
	case string:
		if v == "" && e.options.NullValue != "" {
			return nil
		}
		return v
	case int64:
		return v
	case float64:
		return v
	default:
		// Convert other types to string
		return fmt.Sprintf("%v", v)
	}
}

// formatValueForCSV formats a value for CSV export
func (e *Exporter) formatValueForCSV(value any) string {
	if value == nil {
		return e.options.NullValue
	}

	switch v := value.(type) {
	case bool:
		if e.options.BoolFormat == "1/0" {
			if v {
				return "1"
			}
			return "0"
		}
		return strconv.FormatBool(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		if v == "" {
			return e.options.NullValue
		}
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ExportResult represents the result of an export operation
type ExportResult struct {
	Format      ExportFormat `json:"format"`
	RowCount    int          `json:"row_count"`
	ColumnCount int          `json:"column_count"`
	Size        int64        `json:"size"`
	Error       string       `json:"error,omitempty"`
}

// ExportWithMetadata exports results and returns metadata about the export
func (e *Exporter) ExportWithMetadata(writer io.Writer, results []map[string]any) (*ExportResult, error) {
	// Create a counting writer to track bytes written
	countingWriter := &countingWriter{writer: writer}

	err := e.Export(countingWriter, results)

	result := &ExportResult{
		Format:   e.options.Format,
		RowCount: len(results),
		Size:     countingWriter.count,
	}

	// Calculate column count
	if len(results) > 0 {
		columnSet := make(map[string]bool)
		for _, row := range results {
			for key := range row {
				columnSet[key] = true
			}
		}
		result.ColumnCount = len(columnSet)
	}

	if err != nil {
		result.Error = err.Error()
	}

	return result, err
}

// countingWriter is a wrapper that counts bytes written
type countingWriter struct {
	writer io.Writer
	count  int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.writer.Write(p)
	cw.count += int64(n)
	return n, err
}

// ResultProcessor provides utilities for processing query results
type ResultProcessor struct{}

// NewResultProcessor creates a new result processor
func NewResultProcessor() *ResultProcessor {
	return &ResultProcessor{}
}

// Flatten flattens nested maps in query results
func (rp *ResultProcessor) Flatten(results []map[string]any, separator string) []map[string]any {
	if separator == "" {
		separator = "."
	}

	flattened := make([]map[string]any, 0, len(results))

	for _, result := range results {
		flat := make(map[string]any)
		rp.flattenMap(result, "", separator, flat)
		flattened = append(flattened, flat)
	}

	return flattened
}

// flattenMap recursively flattens a map
func (rp *ResultProcessor) flattenMap(
	source map[string]any,
	prefix string,
	separator string,
	target map[string]any,
) {
	for key, value := range source {
		newKey := key
		if prefix != "" {
			newKey = prefix + separator + key
		}

		if nestedMap, ok := value.(map[string]any); ok {
			rp.flattenMap(nestedMap, newKey, separator, target)
		} else {
			target[newKey] = value
		}
	}
}

// Filter filters results based on a predicate function
func (rp *ResultProcessor) Filter(
	results []map[string]any,
	predicate func(map[string]any) bool,
) []map[string]any {
	filtered := make([]map[string]any, 0)

	for _, result := range results {
		if predicate(result) {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

// Transform transforms results using a transformer function
func (rp *ResultProcessor) Transform(
	results []map[string]any,
	transformer func(map[string]any) map[string]any,
) []map[string]any {
	transformed := make([]map[string]any, 0, len(results))

	for _, result := range results {
		transformed = append(transformed, transformer(result))
	}

	return transformed
}

// Aggregate aggregates results by grouping key
func (rp *ResultProcessor) Aggregate(
	results []map[string]any,
	groupKey string,
	aggregator func([]map[string]any) map[string]any,
) []map[string]any {
	groups := make(map[string][]map[string]any)

	for _, result := range results {
		key := ""
		if groupValue, exists := result[groupKey]; exists {
			key = fmt.Sprintf("%v", groupValue)
		}
		groups[key] = append(groups[key], result)
	}

	aggregated := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		aggregated = append(aggregated, aggregator(group))
	}

	return aggregated
}

// SortBy sorts results by the specified field
func (rp *ResultProcessor) SortBy(results []map[string]any, field string, ascending bool) []map[string]any {
	sorted := make([]map[string]any, len(results))
	copy(sorted, results)

	sort.Slice(sorted, func(i, j int) bool {
		valueI := sorted[i][field]
		valueJ := sorted[j][field]

		cmp := rp.compareValues(valueI, valueJ)
		if ascending {
			return cmp < 0
		}
		return cmp > 0
	})

	return sorted
}

// compareValues compares two values for sorting
func (rp *ResultProcessor) compareValues(a, b any) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	valueA := reflect.ValueOf(a)
	valueB := reflect.ValueOf(b)

	if valueA.Type() != valueB.Type() {
		return rp.compareAsStrings(a, b)
	}

	switch valueA.Kind() {
	case reflect.String:
		return strings.Compare(valueA.String(), valueB.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rp.compareInts(valueA.Int(), valueB.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rp.compareUints(valueA.Uint(), valueB.Uint())
	case reflect.Float32, reflect.Float64:
		return rp.compareFloats(valueA.Float(), valueB.Float())
	case reflect.Bool:
		return rp.compareBools(valueA.Bool(), valueB.Bool())
	default:
		return rp.compareAsStrings(a, b)
	}
}

// compareAsStrings compares values as strings
func (rp *ResultProcessor) compareAsStrings(a, b any) int {
	strA := fmt.Sprintf("%v", a)
	strB := fmt.Sprintf("%v", b)
	return strings.Compare(strA, strB)
}

// compareInts compares two int64 values
func (rp *ResultProcessor) compareInts(a, b int64) int {
	if a < b {
		return -1
	} else if a > b {
		return 1
	}
	return 0
}

// compareUints compares two uint64 values
func (rp *ResultProcessor) compareUints(a, b uint64) int {
	if a < b {
		return -1
	} else if a > b {
		return 1
	}
	return 0
}

// compareFloats compares two float64 values
func (rp *ResultProcessor) compareFloats(a, b float64) int {
	if a < b {
		return -1
	} else if a > b {
		return 1
	}
	return 0
}

// compareBools compares two bool values
func (rp *ResultProcessor) compareBools(a, b bool) int {
	if !a && b {
		return -1
	} else if a && !b {
		return 1
	}
	return 0
}
