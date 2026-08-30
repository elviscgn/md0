package md0

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	RuntimeVersion = "v0.1.0"
	SnapshotSchema = "md0.snapshot/v1"
)

type SnapshotSource struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

type SnapshotOutput struct {
	HTML       string            `json:"html"`
	Assertions []AssertionResult `json:"assertions,omitempty"`
}

type Snapshot struct {
	Schema     string         `json:"schema"`
	MD0Version string         `json:"md0_version"`
	Source     SnapshotSource `json:"source"`
	Values     map[string]any `json:"values"`
	Output     SnapshotOutput `json:"output"`
}

func BuildSnapshot(doc *Document, result *EvalResult) (*Snapshot, error) {
	fragment, err := RenderFragmentBounded(doc, result)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(doc.Source))
	return &Snapshot{
		Schema:     SnapshotSchema,
		MD0Version: RuntimeVersion,
		Source: SnapshotSource{
			Name:   filepath.Base(doc.Path),
			SHA256: hex.EncodeToString(sum[:]),
		},
		Values: inputJSONValues(doc.Nodes, result),
		Output: SnapshotOutput{
			HTML:       RenderStaticPage(filepath.Base(doc.Path), fragment),
			Assertions: append([]AssertionResult(nil), result.Assertions...),
		},
	}, nil
}

func MarshalSnapshot(doc *Document, result *EvalResult) ([]byte, error) {
	snapshot, err := BuildSnapshot(doc, result)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode snapshot: %w", err)
	}
	return append(data, '\n'), nil
}

func inputJSONValues(nodes []Node, result *EvalResult) map[string]any {
	values := map[string]any{}
	var walk func([]Node)
	walk = func(current []Node) {
		for _, raw := range current {
			switch node := raw.(type) {
			case InputNode:
				if value, ok := result.Env[node.Name]; ok {
					values[node.Name] = valueJSON(value)
				}
			case WhenNode:
				if result.WhenByLine[node.Line] {
					walk(node.Nodes)
				}
			}
		}
	}
	walk(nodes)
	return values
}

func valueJSON(value Value) any {
	switch value.Kind {
	case NullKind:
		return nil
	case NumberKind:
		return value.Num
	case StringKind:
		return value.Str
	case BoolKind:
		return value.Bool
	case ListKind:
		items := make([]any, len(value.List))
		for i, item := range value.List {
			items[i] = valueJSON(item)
		}
		return items
	default:
		return nil
	}
}

func UpdatedMarkdown(doc *Document, result *EvalResult) (string, error) {
	if doc.Source == "" {
		return "", fmt.Errorf("source text is unavailable")
	}
	newline := "\n"
	if strings.Contains(doc.Source, "\r\n") {
		newline = "\r\n"
	}
	normalized := strings.ReplaceAll(doc.Source, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	var update func([]Node) error
	update = func(nodes []Node) error {
		for _, raw := range nodes {
			switch node := raw.(type) {
			case InputNode:
				value, ok := result.Env[node.Name]
				if !ok {
					continue
				}
				index := node.Line - 1
				if index < 0 || index >= len(lines) {
					return fmt.Errorf("line %d: input %s is outside source text", node.Line, node.Name)
				}
				equals := strings.LastIndex(lines[index], "=")
				if equals < 0 {
					return fmt.Errorf("line %d: input %s has no default assignment", node.Line, node.Name)
				}
				lines[index] = strings.TrimRight(lines[index][:equals+1], " \t") + " " + inputDefaultLiteral(node.Type, value)
			case WhenNode:
				if err := update(node.Nodes); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := update(doc.Nodes); err != nil {
		return "", err
	}
	return strings.Join(lines, newline), nil
}

func inputDefaultLiteral(typ string, value Value) string {
	switch strings.ToLower(typ) {
	case "string", "text":
		return strconv.Quote(value.Str)
	case "boolean", "bool":
		return strconv.FormatBool(value.Bool)
	default:
		return value.String()
	}
}
