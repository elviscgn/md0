package md0

import (
	"fmt"
	"sort"
	"strings"
)

type DependencyNode struct {
	ID        string
	Kind      string
	Label     string
	Line      int
	Defines   string
	DependsOn []string
}

type DependencyGraph struct {
	Nodes      map[string]DependencyNode
	Order      []string
	Producers  map[string]string
	Dependents map[string][]string
	EdgeCount  int
}

type pendingDependency struct {
	symbols []string
	nodes   []string
}

func BuildDependencyGraph(doc *Document) (*DependencyGraph, error) {
	g := &DependencyGraph{
		Nodes:      map[string]DependencyNode{},
		Producers:  map[string]string{},
		Dependents: map[string][]string{},
	}
	pending := map[string]pendingDependency{}

	add := func(node DependencyNode, symbols, nodeDeps []string) error {
		if _, exists := g.Nodes[node.ID]; exists {
			return fmt.Errorf("line %d: duplicate dependency node %q", node.Line, node.ID)
		}
		if node.Defines != "" {
			if previous, exists := g.Producers[node.Defines]; exists {
				prev := g.Nodes[previous]
				return fmt.Errorf("line %d: %q is already defined at line %d", node.Line, node.Defines, prev.Line)
			}
			g.Producers[node.Defines] = node.ID
		}
		g.Nodes[node.ID] = node
		g.Order = append(g.Order, node.ID)
		pending[node.ID] = pendingDependency{symbols: uniqueSorted(symbols), nodes: uniqueSorted(nodeDeps)}
		return nil
	}

	var collect func([]Node, string) error
	collect = func(nodes []Node, parent string) error {
		for _, raw := range nodes {
			id := dependencyNodeID(raw)
			structural := []string{}
			if parent != "" {
				structural = append(structural, parent)
			}

			switch x := raw.(type) {
			case MarkdownNode:
				deps, err := markdownDependencies(x.Text)
				if err != nil {
					return fmt.Errorf("line %d: interpolation dependency: %w", x.Line, err)
				}
				label := fmt.Sprintf("markdown@%d", x.Line)
				if len(deps) > 0 {
					label = fmt.Sprintf("prose@%d", x.Line)
				}
				if err := add(DependencyNode{ID: id, Kind: "markdown", Label: label, Line: x.Line}, deps, structural); err != nil {
					return err
				}
			case InputNode:
				deps, err := ExprDependencies(x.Default)
				if err != nil {
					return fmt.Errorf("line %d: input %s: %w", x.Line, x.Name, err)
				}
				if err := add(DependencyNode{ID: id, Kind: "input", Label: x.Name, Line: x.Line, Defines: x.Name}, deps, structural); err != nil {
					return err
				}
			case CalcNode:
				deps, err := ExprDependencies(x.Expr)
				if err != nil {
					return fmt.Errorf("line %d: calc %s: %w", x.Line, x.Name, err)
				}
				if err := add(DependencyNode{ID: id, Kind: "calc", Label: x.Name, Line: x.Line, Defines: x.Name}, deps, structural); err != nil {
					return err
				}
			case ShowNode:
				deps, err := ExprDependencies(x.Expr)
				if err != nil {
					return fmt.Errorf("line %d: show: %w", x.Line, err)
				}
				if err := add(DependencyNode{ID: id, Kind: "show", Label: fmt.Sprintf("show@%d", x.Line), Line: x.Line}, deps, structural); err != nil {
					return err
				}
			case AssertNode:
				deps, err := ExprDependencies(x.Expr)
				if err != nil {
					return fmt.Errorf("line %d: assert: %w", x.Line, err)
				}
				if err := add(DependencyNode{ID: id, Kind: "assert", Label: fmt.Sprintf("assert@%d", x.Line), Line: x.Line}, deps, structural); err != nil {
					return err
				}
			case WhenNode:
				deps, err := ExprDependencies(x.Expr)
				if err != nil {
					return fmt.Errorf("line %d: when: %w", x.Line, err)
				}
				if err := add(DependencyNode{ID: id, Kind: "when", Label: fmt.Sprintf("when@%d", x.Line), Line: x.Line}, deps, structural); err != nil {
					return err
				}
				if err := collect(x.Nodes, id); err != nil {
					return err
				}
			case ChartNode:
				labels, err := ExprDependencies(x.Labels)
				if err != nil {
					return fmt.Errorf("line %d: chart %s labels: %w", x.Line, x.Name, err)
				}
				values, err := ExprDependencies(x.Values)
				if err != nil {
					return fmt.Errorf("line %d: chart %s values: %w", x.Line, x.Name, err)
				}
				deps := append(labels, values...)
				if err := add(DependencyNode{ID: id, Kind: "chart", Label: fmt.Sprintf("chart:%s@%d", x.Name, x.Line), Line: x.Line}, deps, structural); err != nil {
					return err
				}
			case TableNode:
				columns, err := ExprDependencies(x.Columns)
				if err != nil {
					return fmt.Errorf("line %d: table %s columns: %w", x.Line, x.Name, err)
				}
				rows, err := ExprDependencies(x.Rows)
				if err != nil {
					return fmt.Errorf("line %d: table %s rows: %w", x.Line, x.Name, err)
				}
				deps := append(columns, rows...)
				if err := add(DependencyNode{ID: id, Kind: "table", Label: fmt.Sprintf("table:%s@%d", x.Name, x.Line), Line: x.Line}, deps, structural); err != nil {
					return err
				}
			default:
				return fmt.Errorf("line %d: unsupported node in dependency graph", raw.LineNo())
			}
		}
		return nil
	}

	if err := collect(doc.Nodes, ""); err != nil {
		return nil, err
	}

	for _, id := range g.Order {
		node := g.Nodes[id]
		p := pending[id]
		deps := append([]string(nil), p.nodes...)
		for _, symbol := range p.symbols {
			producer, ok := g.Producers[symbol]
			if !ok {
				return nil, fmt.Errorf("line %d: %s references unknown value %q", node.Line, node.Label, symbol)
			}
			deps = append(deps, producer)
		}
		node.DependsOn = uniqueSorted(deps)
		g.Nodes[id] = node
		for _, dependency := range node.DependsOn {
			g.Dependents[dependency] = append(g.Dependents[dependency], id)
			g.EdgeCount++
		}
	}
	for id := range g.Dependents {
		sort.Strings(g.Dependents[id])
	}

	if cycle := g.findCycle(); len(cycle) > 0 {
		labels := make([]string, len(cycle))
		for i, id := range cycle {
			labels[i] = g.Nodes[id].Label
		}
		return nil, fmt.Errorf("dependency cycle: %s", strings.Join(labels, " -> "))
	}
	return g, nil
}

func dependencyNodeID(n Node) string {
	switch x := n.(type) {
	case InputNode:
		return "input:" + x.Name
	case CalcNode:
		return "calc:" + x.Name
	case MarkdownNode:
		return fmt.Sprintf("markdown:%d", x.Line)
	case ShowNode:
		return fmt.Sprintf("show:%d", x.Line)
	case AssertNode:
		return fmt.Sprintf("assert:%d", x.Line)
	case WhenNode:
		return fmt.Sprintf("when:%d", x.Line)
	case ChartNode:
		return fmt.Sprintf("chart:%s:%d", x.Name, x.Line)
	case TableNode:
		return fmt.Sprintf("table:%s:%d", x.Name, x.Line)
	default:
		return fmt.Sprintf("node:%d", n.LineNo())
	}
}

func ExprDependencies(expr Expr) ([]string, error) {
	set := map[string]struct{}{}
	var walk func(Expr) error
	walk = func(raw Expr) error {
		switch x := raw.(type) {
		case literalExpr:
			return nil
		case identExpr:
			set[x.name] = struct{}{}
			return nil
		case arrayExpr:
			for _, item := range x.items {
				if err := walk(item); err != nil {
					return err
				}
			}
			return nil
		case unaryExpr:
			return walk(x.right)
		case binaryExpr:
			if err := walk(x.left); err != nil {
				return err
			}
			return walk(x.right)
		case ternaryExpr:
			if err := walk(x.cond); err != nil {
				return err
			}
			if err := walk(x.yes); err != nil {
				return err
			}
			return walk(x.no)
		case callExpr:
			for _, arg := range x.args {
				if err := walk(arg); err != nil {
					return err
				}
			}
			return nil
		default:
			return fmt.Errorf("unsupported expression node %T", raw)
		}
	}
	if err := walk(expr); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func markdownDependencies(text string) ([]string, error) {
	set := map[string]struct{}{}
	matches := interpRE.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		expr, err := parseExprBounded(match[1])
		if err != nil {
			return nil, err
		}
		deps, err := ExprDependencies(expr)
		if err != nil {
			return nil, err
		}
		for _, dep := range deps {
			set[dep] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for dep := range set {
		out = append(out, dep)
	}
	sort.Strings(out)
	return out, nil
}

func uniqueSorted(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		if value != "" {
			set[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func (g *DependencyGraph) findCycle() []string {
	state := map[string]uint8{}
	stack := []string{}
	positions := map[string]int{}
	var cycle []string
	var visit func(string) bool
	visit = func(id string) bool {
		state[id] = 1
		positions[id] = len(stack)
		stack = append(stack, id)
		for _, next := range g.Dependents[id] {
			switch state[next] {
			case 0:
				if visit(next) {
					return true
				}
			case 1:
				start := positions[next]
				cycle = append(cycle, stack[start:]...)
				cycle = append(cycle, next)
				return true
			}
		}
		stack = stack[:len(stack)-1]
		delete(positions, id)
		state[id] = 2
		return false
	}
	for _, id := range g.Order {
		if state[id] == 0 && visit(id) {
			return cycle
		}
	}
	return nil
}

func (g *DependencyGraph) AffectedBySymbols(symbols []string) map[string]bool {
	affected := map[string]bool{}
	queue := []string{}
	for _, symbol := range symbols {
		if producer, ok := g.Producers[symbol]; ok && !affected[producer] {
			affected[producer] = true
			queue = append(queue, producer)
		}
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, dependent := range g.Dependents[id] {
			if affected[dependent] {
				continue
			}
			affected[dependent] = true
			queue = append(queue, dependent)
		}
	}
	return affected
}

func (g *DependencyGraph) OrderedAffected(affected map[string]bool) []string {
	out := []string{}
	for _, id := range g.Order {
		if affected[id] {
			out = append(out, id)
		}
	}
	return out
}

func (g *DependencyGraph) IsInputSymbol(name string) bool {
	producer, ok := g.Producers[name]
	if !ok {
		return false
	}
	return g.Nodes[producer].Kind == "input"
}

func (g *DependencyGraph) EdgeLines() []string {
	lines := []string{}
	for _, targetID := range g.Order {
		target := g.Nodes[targetID]
		for _, sourceID := range target.DependsOn {
			source := g.Nodes[sourceID]
			lines = append(lines, fmt.Sprintf("%s -> %s", source.Label, target.Label))
		}
	}
	sort.Strings(lines)
	return lines
}
