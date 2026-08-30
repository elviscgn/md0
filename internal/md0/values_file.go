package md0

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
)

const maxValuesFileBytes = 1 << 20

func LoadValuesFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) > maxValuesFileBytes {
		return nil, fmt.Errorf("%s: values file exceeds 1 MiB limit", path)
	}

	object, err := decodeJSONObject(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if schema, ok := object["schema"]; ok {
		var name string
		if err := json.Unmarshal(schema, &name); err != nil || name != SnapshotSchema {
			return nil, fmt.Errorf("unsupported snapshot schema")
		}
		raw, ok := object["values"]
		if !ok {
			return nil, fmt.Errorf("snapshot has no values object")
		}
		object, err = decodeJSONObject(raw)
		if err != nil {
			return nil, fmt.Errorf("snapshot values: %w", err)
		}
	}

	values := make(map[string]string, len(object))
	for name, raw := range object {
		var primitive any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&primitive); err != nil {
			return nil, fmt.Errorf("value %q is invalid: %w", name, err)
		}
		switch value := primitive.(type) {
		case string:
			values[name] = value
		case bool:
			values[name] = strconv.FormatBool(value)
		case json.Number:
			if _, err := strconv.ParseFloat(string(value), 64); err != nil {
				return nil, fmt.Errorf("value %q is not a finite JSON number", name)
			}
			values[name] = string(value)
		default:
			return nil, fmt.Errorf("value %q must be a string, number, or boolean", name)
		}
	}
	return values, nil
}

func decodeJSONObject(data []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var object map[string]json.RawMessage
	if err := decoder.Decode(&object); err != nil {
		return nil, fmt.Errorf("expected one JSON object: %w", err)
	}
	if object == nil {
		return nil, fmt.Errorf("expected one JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("expected exactly one JSON object")
	}
	return object, nil
}
