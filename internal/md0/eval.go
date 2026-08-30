package md0

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type AssertionResult struct {
	Line            int
	Source, Message string
	Passed          bool
}

type EvalResult struct {
	Env             map[string]Value
	Assertions      []AssertionResult
	AssertionByLine map[int]AssertionResult
	WhenByLine      map[int]bool
}

func Evaluate(doc *Document, overrides map[string]string) (*EvalResult, error) {
	if _, err := BuildDependencyGraph(doc); err != nil {
		return nil, err
	}
	return evaluateDocument(doc, overrides)
}

func evaluateDocument(doc *Document, overrides map[string]string) (*EvalResult, error) {
	r := &EvalResult{
		Env:             map[string]Value{},
		AssertionByLine: map[int]AssertionResult{},
		WhenByLine:      map[int]bool{},
	}
	if overrides == nil {
		overrides = map[string]string{}
	}
	if err := evalNodes(doc.Nodes, r, overrides); err != nil {
		return nil, err
	}
	rebuildAssertions(doc.Nodes, r)
	return r, nil
}

func evalNodes(nodes []Node, r *EvalResult, overrides map[string]string) error {
	for _, n := range nodes {
		switch x := n.(type) {
		case MarkdownNode:
		case InputNode:
			var v Value
			var err error
			if raw, ok := overrides[x.Name]; ok {
				v, err = parseInputValue(x.Type, raw)
			} else {
				v, err = x.Default.Eval(r.Env)
			}
			if err != nil {
				return fmt.Errorf("line %d: input %s: %w", x.Line, x.Name, err)
			}
			if err := validateInputType(x.Type, v); err != nil {
				return fmt.Errorf("line %d: input %s: %w", x.Line, x.Name, err)
			}
			r.Env[x.Name] = v
		case CalcNode:
			v, err := x.Expr.Eval(r.Env)
			if err != nil {
				return fmt.Errorf("line %d: calc %s: %w", x.Line, x.Name, err)
			}
			r.Env[x.Name] = v
		case ShowNode:
			if _, err := x.Expr.Eval(r.Env); err != nil {
				return fmt.Errorf("line %d: show: %w", x.Line, err)
			}
		case AssertNode:
			v, err := x.Expr.Eval(r.Env)
			if err != nil {
				return fmt.Errorf("line %d: assert: %w", x.Line, err)
			}
			b, err := v.AsBool()
			if err != nil {
				return fmt.Errorf("line %d: assert must be boolean: %w", x.Line, err)
			}
			r.AssertionByLine[x.Line] = AssertionResult{Line: x.Line, Source: x.Source, Message: x.Message, Passed: b}
		case WhenNode:
			v, err := x.Expr.Eval(r.Env)
			if err != nil {
				return fmt.Errorf("line %d: when: %w", x.Line, err)
			}
			b, err := v.AsBool()
			if err != nil {
				return fmt.Errorf("line %d: when must be boolean: %w", x.Line, err)
			}
			r.WhenByLine[x.Line] = b
			if b {
				if err := evalNodes(x.Nodes, r, overrides); err != nil {
					return err
				}
			}
		case ChartNode:
			if err := validateChart(x, r.Env); err != nil {
				return fmt.Errorf("line %d: chart %s: %w", x.Line, x.Name, err)
			}
		case TableNode:
			if err := validateTable(x, r.Env); err != nil {
				return fmt.Errorf("line %d: table %s: %w", x.Line, x.Name, err)
			}
		default:
			return fmt.Errorf("line %d: unsupported node", n.LineNo())
		}
	}
	return nil
}

func rebuildAssertions(nodes []Node, r *EvalResult) {
	r.Assertions = r.Assertions[:0]
	var walk func([]Node)
	walk = func(current []Node) {
		for _, raw := range current {
			switch x := raw.(type) {
			case AssertNode:
				if assertion, ok := r.AssertionByLine[x.Line]; ok {
					r.Assertions = append(r.Assertions, assertion)
				}
			case WhenNode:
				if r.WhenByLine[x.Line] {
					walk(x.Nodes)
				}
			}
		}
	}
	walk(nodes)
}

func parseInputValue(typ, raw string) (Value, error) {
	raw = strings.TrimSpace(raw)
	switch strings.ToLower(typ) {
	case "number", "percent", "currency":
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return Null(), fmt.Errorf("expected %s", typ)
		}
		return Number(n), nil
	case "integer":
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Null(), fmt.Errorf("expected integer")
		}
		return Number(float64(n)), nil
	case "boolean", "bool":
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return Null(), fmt.Errorf("expected boolean")
		}
		return Boolean(b), nil
	case "string", "text":
		return String(raw), nil
	case "duration":
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Null(), fmt.Errorf("expected Go-style duration such as 180ms or 2s")
		}
		return Number(float64(d.Milliseconds())), nil
	default:
		return Null(), fmt.Errorf("unknown input type %q", typ)
	}
}

func validateInputType(typ string, v Value) error {
	switch strings.ToLower(typ) {
	case "number", "percent", "currency", "integer", "duration":
		if v.Kind != NumberKind {
			return fmt.Errorf("default must evaluate to number")
		}
	case "boolean", "bool":
		if v.Kind != BoolKind {
			return fmt.Errorf("default must evaluate to boolean")
		}
	case "string", "text":
		if v.Kind != StringKind {
			return fmt.Errorf("default must evaluate to string")
		}
	default:
		return fmt.Errorf("unknown input type %q", typ)
	}
	return nil
}

func validateChart(x ChartNode, env map[string]Value) error {
	if strings.ToLower(x.Type) != "bar" {
		return fmt.Errorf("unsupported chart type %q (v1 supports bar)", x.Type)
	}
	lv, err := x.Labels.Eval(env)
	if err != nil {
		return err
	}
	vv, err := x.Values.Eval(env)
	if err != nil {
		return err
	}
	labels, err := lv.AsList()
	if err != nil {
		return fmt.Errorf("labels: %w", err)
	}
	values, err := vv.AsList()
	if err != nil {
		return fmt.Errorf("values: %w", err)
	}
	if len(labels) != len(values) {
		return fmt.Errorf("labels (%d) and values (%d) have different lengths", len(labels), len(values))
	}
	if len(labels) == 0 {
		return fmt.Errorf("chart must contain at least one value")
	}
	if len(labels) > 128 {
		return fmt.Errorf("chart exceeds 128-value limit")
	}
	for _, v := range values {
		if _, err := v.AsNumber(); err != nil {
			return fmt.Errorf("chart values must be numbers")
		}
	}
	return nil
}

func validateTable(x TableNode, env map[string]Value) error {
	cv, err := x.Columns.Eval(env)
	if err != nil {
		return err
	}
	rv, err := x.Rows.Eval(env)
	if err != nil {
		return err
	}
	cols, err := cv.AsList()
	if err != nil {
		return fmt.Errorf("columns: %w", err)
	}
	rows, err := rv.AsList()
	if err != nil {
		return fmt.Errorf("rows: %w", err)
	}
	if len(cols) > 64 {
		return fmt.Errorf("table exceeds 64-column limit")
	}
	if len(rows) > 1000 {
		return fmt.Errorf("table exceeds 1000-row limit")
	}
	for i, row := range rows {
		cells, err := row.AsList()
		if err != nil {
			return fmt.Errorf("row %d must be a list", i+1)
		}
		if len(cells) != len(cols) {
			return fmt.Errorf("row %d has %d cells; expected %d", i+1, len(cells), len(cols))
		}
	}
	return nil
}

func SortedEnv(env map[string]Value) []string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
