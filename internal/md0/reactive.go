package md0

import (
	"fmt"
	"sort"
	"sync"
)

type IncrementalStats struct {
	Changed    []string
	Recomputed []string
}

type ReactiveSession struct {
	mu        sync.Mutex
	doc       *Document
	graph     *DependencyGraph
	result    *EvalResult
	overrides map[string]string
}

func NewReactiveSession(doc *Document) (*ReactiveSession, error) {
	graph, err := BuildDependencyGraph(doc)
	if err != nil {
		return nil, err
	}
	result, err := evaluateDocument(doc, nil)
	if err != nil {
		return nil, err
	}
	return &ReactiveSession{doc: doc, graph: graph, result: result, overrides: map[string]string{}}, nil
}

func (s *ReactiveSession) Reset() (*EvalResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := evaluateDocument(s.doc, nil)
	if err != nil {
		return nil, err
	}
	s.result = result
	s.overrides = map[string]string{}
	return cloneEvalResult(result), nil
}

func (s *ReactiveSession) Snapshot() *EvalResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneEvalResult(s.result)
}

func (s *ReactiveSession) Graph() *DependencyGraph {
	return s.graph
}

func (s *ReactiveSession) Update(overrides map[string]string) (*EvalResult, IncrementalStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if overrides == nil {
		overrides = map[string]string{}
	}
	for name := range overrides {
		if !s.graph.IsInputSymbol(name) {
			return nil, IncrementalStats{}, fmt.Errorf("unknown input override %q", name)
		}
	}

	changed := changedOverrides(s.overrides, overrides)
	stats := IncrementalStats{Changed: changed}
	if len(changed) == 0 {
		return cloneEvalResult(s.result), stats, nil
	}

	affected := s.graph.AffectedBySymbols(changed)
	stats.Recomputed = s.graph.OrderedAffected(affected)
	next := cloneEvalResult(s.result)
	if err := evalNodesIncremental(s.doc.Nodes, next, s.result, overrides, affected, false); err != nil {
		return nil, IncrementalStats{}, err
	}
	rebuildAssertions(s.doc.Nodes, next)

	s.result = next
	s.overrides = copyStringMap(overrides)
	return cloneEvalResult(next), stats, nil
}

func changedOverrides(previous, next map[string]string) []string {
	keys := map[string]struct{}{}
	for key := range previous {
		keys[key] = struct{}{}
	}
	for key := range next {
		keys[key] = struct{}{}
	}
	changed := []string{}
	for key := range keys {
		pv, pok := previous[key]
		nv, nok := next[key]
		if pok != nok || pv != nv {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return changed
}

func evalNodesIncremental(nodes []Node, r, previous *EvalResult, overrides map[string]string, affected map[string]bool, force bool) error {
	for _, raw := range nodes {
		id := dependencyNodeID(raw)
		recompute := force || affected[id]
		switch x := raw.(type) {
		case MarkdownNode:
			// Markdown is rendered from the current environment. Its dependency
			// node still participates in invalidation accounting.
		case InputNode:
			if !recompute {
				continue
			}
			var value Value
			var err error
			if source, ok := overrides[x.Name]; ok {
				value, err = parseInputValue(x.Type, source)
			} else {
				value, err = x.Default.Eval(r.Env)
			}
			if err != nil {
				return fmt.Errorf("line %d: input %s: %w", x.Line, x.Name, err)
			}
			if err := validateInputType(x.Type, value); err != nil {
				return fmt.Errorf("line %d: input %s: %w", x.Line, x.Name, err)
			}
			r.Env[x.Name] = value
		case CalcNode:
			if !recompute {
				continue
			}
			value, err := x.Expr.Eval(r.Env)
			if err != nil {
				return fmt.Errorf("line %d: calc %s: %w", x.Line, x.Name, err)
			}
			r.Env[x.Name] = value
		case ShowNode:
			if recompute {
				if _, err := x.Expr.Eval(r.Env); err != nil {
					return fmt.Errorf("line %d: show: %w", x.Line, err)
				}
			}
		case AssertNode:
			if !recompute {
				continue
			}
			value, err := x.Expr.Eval(r.Env)
			if err != nil {
				return fmt.Errorf("line %d: assert: %w", x.Line, err)
			}
			passed, err := value.AsBool()
			if err != nil {
				return fmt.Errorf("line %d: assert must be boolean: %w", x.Line, err)
			}
			r.AssertionByLine[x.Line] = AssertionResult{Line: x.Line, Source: x.Source, Message: x.Message, Passed: passed}
		case WhenNode:
			previousActive, hadPrevious := previous.WhenByLine[x.Line]
			active := previousActive
			if recompute || !hadPrevious {
				value, err := x.Expr.Eval(r.Env)
				if err != nil {
					return fmt.Errorf("line %d: when: %w", x.Line, err)
				}
				boolValue, boolErr := value.AsBool()
				if boolErr != nil {
					return fmt.Errorf("line %d: when must be boolean: %w", x.Line, boolErr)
				}
				active = boolValue
			}
			r.WhenByLine[x.Line] = active
			if !active {
				clearSubtreeState(x.Nodes, r)
				continue
			}
			childForce := force || !previousActive
			if err := evalNodesIncremental(x.Nodes, r, previous, overrides, affected, childForce); err != nil {
				return err
			}
		case ChartNode:
			if recompute {
				if err := validateChart(x, r.Env); err != nil {
					return fmt.Errorf("line %d: chart %s: %w", x.Line, x.Name, err)
				}
			}
		case TableNode:
			if recompute {
				if err := validateTable(x, r.Env); err != nil {
					return fmt.Errorf("line %d: table %s: %w", x.Line, x.Name, err)
				}
			}
		default:
			return fmt.Errorf("line %d: unsupported node", raw.LineNo())
		}
	}
	return nil
}

func clearSubtreeState(nodes []Node, r *EvalResult) {
	for _, raw := range nodes {
		switch x := raw.(type) {
		case InputNode:
			delete(r.Env, x.Name)
		case CalcNode:
			delete(r.Env, x.Name)
		case AssertNode:
			delete(r.AssertionByLine, x.Line)
		case WhenNode:
			delete(r.WhenByLine, x.Line)
			clearSubtreeState(x.Nodes, r)
		}
	}
}

func cloneEvalResult(source *EvalResult) *EvalResult {
	if source == nil {
		return nil
	}
	out := &EvalResult{
		Env:             make(map[string]Value, len(source.Env)),
		Assertions:      append([]AssertionResult(nil), source.Assertions...),
		AssertionByLine: make(map[int]AssertionResult, len(source.AssertionByLine)),
		WhenByLine:      make(map[int]bool, len(source.WhenByLine)),
	}
	for key, value := range source.Env {
		out.Env[key] = value
	}
	for line, assertion := range source.AssertionByLine {
		out.AssertionByLine[line] = assertion
	}
	for line, active := range source.WhenByLine {
		out.WhenByLine[line] = active
	}
	return out
}

func copyStringMap(source map[string]string) map[string]string {
	out := make(map[string]string, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}
