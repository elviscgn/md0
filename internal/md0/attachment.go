package md0

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

const (
	maxDataAttachments = 16
	maxDataFileBytes   = 2 * 1024 * 1024
	maxDataTotalBytes  = 8 * 1024 * 1024
	maxJSONDepth       = 32
	maxDataValues      = 100_000
)

type dataDeclaration struct {
	format string
	line   int
}

func BindDataFiles(doc *Document, specs []string) error {
	declarations := map[string]dataDeclaration{}
	var collect func([]Node) error
	collect = func(nodes []Node) error {
		for _, raw := range nodes {
			switch node := raw.(type) {
			case DataNode:
				if previous, exists := declarations[node.Name]; exists {
					return fmt.Errorf("line %d: data %q is already declared at line %d", node.Line, node.Name, previous.line)
				}
				declarations[node.Name] = dataDeclaration{format: node.Format, line: node.Line}
			case WhenNode:
				if err := collect(node.Nodes); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := collect(doc.Nodes); err != nil {
		return err
	}
	if len(declarations) > maxDataAttachments {
		return fmt.Errorf("document exceeds %d-data-attachment limit", maxDataAttachments)
	}

	paths := map[string]string{}
	for _, spec := range specs {
		equals := strings.IndexByte(spec, '=')
		if equals < 1 || equals == len(spec)-1 {
			return fmt.Errorf("invalid data attachment %q; expected name=FILE", spec)
		}
		name, path := spec[:equals], spec[equals+1:]
		if _, ok := declarations[name]; !ok {
			return fmt.Errorf("data attachment %q is not declared by the document", name)
		}
		if _, exists := paths[name]; exists {
			return fmt.Errorf("data attachment %q was provided more than once", name)
		}
		paths[name] = path
	}

	values := map[string]Value{}
	totalBytes := 0
	for name, declaration := range declarations {
		path, ok := paths[name]
		if !ok {
			return fmt.Errorf("line %d: data %s: provide --data %s=FILE", declaration.line, name, name)
		}
		value, size, err := loadDataFile(path, declaration.format)
		if err != nil {
			return fmt.Errorf("data %s: %w", name, err)
		}
		totalBytes += size
		if totalBytes > maxDataTotalBytes {
			return fmt.Errorf("data attachments exceed 8 MiB combined limit")
		}
		values[name] = value
	}

	var bind func([]Node)
	bind = func(nodes []Node) {
		for i, raw := range nodes {
			switch node := raw.(type) {
			case DataNode:
				node.Value = values[node.Name]
				nodes[i] = node
			case WhenNode:
				bind(node.Nodes)
				nodes[i] = node
			}
		}
	}
	bind(doc.Nodes)
	return nil
}

func loadDataFile(path, format string) (Value, int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Null(), 0, err
	}
	if info.Size() > maxDataFileBytes {
		return Null(), 0, fmt.Errorf("%s exceeds 2 MiB attachment limit", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Null(), 0, err
	}
	switch format {
	case "json":
		value, err := decodeDataJSON(data)
		return value, len(data), err
	case "csv":
		value, err := decodeDataCSV(data)
		return value, len(data), err
	default:
		return Null(), 0, fmt.Errorf("unsupported attachment format %q", format)
	}
}

func decodeDataJSON(data []byte) (Value, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var raw any
	if err := decoder.Decode(&raw); err != nil {
		return Null(), fmt.Errorf("invalid JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Null(), fmt.Errorf("JSON attachment must contain exactly one value")
	}
	count := 0
	value, err := jsonDataValue(raw, 0, &count)
	if err != nil {
		return Null(), err
	}
	return value, nil
}

func jsonDataValue(raw any, depth int, count *int) (Value, error) {
	if depth > maxJSONDepth {
		return Null(), fmt.Errorf("JSON attachment exceeds %d nesting levels", maxJSONDepth)
	}
	(*count)++
	if *count > maxDataValues {
		return Null(), fmt.Errorf("attachment exceeds %d-value limit", maxDataValues)
	}
	switch value := raw.(type) {
	case nil:
		return Null(), nil
	case bool:
		return Boolean(value), nil
	case string:
		return String(value), nil
	case json.Number:
		number, err := strconv.ParseFloat(string(value), 64)
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
			return Null(), fmt.Errorf("JSON number %q is not finite", value)
		}
		return Number(number), nil
	case []any:
		items := make([]Value, len(value))
		for i, item := range value {
			converted, err := jsonDataValue(item, depth+1, count)
			if err != nil {
				return Null(), err
			}
			items[i] = converted
		}
		return List(items), nil
	case map[string]any:
		object := make(map[string]Value, len(value))
		for key, item := range value {
			converted, err := jsonDataValue(item, depth+1, count)
			if err != nil {
				return Null(), err
			}
			object[key] = converted
		}
		return Object(object), nil
	default:
		return Null(), fmt.Errorf("unsupported JSON value %T", raw)
	}
}

func decodeDataCSV(data []byte) (Value, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return Null(), fmt.Errorf("invalid CSV: %w", err)
	}
	if len(records) == 0 {
		return Null(), fmt.Errorf("CSV attachment has no header row")
	}
	columns := records[0]
	if len(columns) == 0 || len(columns) > 64 {
		return Null(), fmt.Errorf("CSV attachment must contain 1 to 64 columns")
	}
	seen := map[string]bool{}
	columnValues := make([]Value, len(columns))
	for i, column := range columns {
		column = strings.TrimSpace(column)
		if column == "" || seen[column] {
			return Null(), fmt.Errorf("CSV column names must be non-empty and unique")
		}
		seen[column] = true
		columnValues[i] = String(column)
	}
	if len(records)-1 > 1000 {
		return Null(), fmt.Errorf("CSV attachment exceeds 1000-row limit")
	}
	rows := make([]Value, len(records)-1)
	for rowIndex, record := range records[1:] {
		if len(record) != len(columns) {
			return Null(), fmt.Errorf("CSV row %d has %d cells; expected %d", rowIndex+2, len(record), len(columns))
		}
		cells := make([]Value, len(record))
		for i, cell := range record {
			cells[i] = csvCellValue(cell)
		}
		rows[rowIndex] = List(cells)
	}
	return Object(map[string]Value{
		"columns": List(columnValues),
		"rows":    List(rows),
	}), nil
}

func csvCellValue(cell string) Value {
	trimmed := strings.TrimSpace(cell)
	if strings.EqualFold(trimmed, "true") {
		return Boolean(true)
	}
	if strings.EqualFold(trimmed, "false") {
		return Boolean(false)
	}
	if number, err := strconv.ParseFloat(trimmed, 64); err == nil && !math.IsInf(number, 0) && !math.IsNaN(number) {
		return Number(number)
	}
	return String(cell)
}
