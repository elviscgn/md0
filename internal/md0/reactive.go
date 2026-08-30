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
	plan      *EvaluationPlan
	result    *EvalResult
	overrides map[string]string
}

func NewReactiveSession(doc *Document) (*ReactiveSession, error) {
	return NewReactiveSessionWithValues(doc, nil)
}

func NewReactiveSessionWithValues(doc *Document, overrides map[string]string) (*ReactiveSession, error) {
	plan, err := BuildEvaluationPlan(doc)
	if err != nil {
		return nil, err
	}
	result, err := evaluateWithPlan(doc, plan, overrides)
	if err != nil {
		return nil, err
	}
	return &ReactiveSession{
		doc:       doc,
		plan:      plan,
		result:    result,
		overrides: renderedInputSnapshot(doc.Nodes, result),
	}, nil
}

func (s *ReactiveSession) Values() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return copyStringMap(s.overrides)
}

func (s *ReactiveSession) Reset() (*EvalResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := evaluateWithPlan(s.doc, s.plan, nil)
	if err != nil {
		return nil, err
	}
	s.result = result
	s.overrides = renderedInputSnapshot(s.doc.Nodes, result)
	return cloneEvalResult(result), nil
}

func (s *ReactiveSession) Snapshot() *EvalResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneEvalResult(s.result)
}

func (s *ReactiveSession) Graph() *DependencyGraph {
	return s.plan.Graph
}

func (s *ReactiveSession) Update(overrides map[string]string) (*EvalResult, IncrementalStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if overrides == nil {
		overrides = map[string]string{}
	}
	for name := range overrides {
		if !s.plan.Graph.IsInputSymbol(name) {
			return nil, IncrementalStats{}, fmt.Errorf("unknown input override %q", name)
		}
	}

	changed := changedOverrides(s.overrides, overrides)
	stats := IncrementalStats{Changed: changed}
	if len(changed) == 0 {
		return cloneEvalResult(s.result), stats, nil
	}

	affected := s.plan.Graph.AffectedBySymbols(changed)
	stats.Recomputed = s.plan.OrderedAffected(affected)
	next := cloneEvalResult(s.result)
	if err := evalPlanIncremental(s.plan, next, overrides, affected); err != nil {
		return nil, IncrementalStats{}, err
	}
	rebuildAssertions(s.doc.Nodes, next)

	s.result = next
	s.overrides = copyStringMap(overrides)
	return cloneEvalResult(next), stats, nil
}

func evalPlanIncremental(plan *EvaluationPlan, result *EvalResult, overrides map[string]string, affected map[string]bool) error {
	for _, id := range plan.Order {
		if !affected[id] {
			continue
		}
		node := plan.Nodes[id]
		if !plan.guardsActive(id, result) {
			clearNodeState(node, result)
			continue
		}
		if err := evalPlannedNode(node, result, overrides); err != nil {
			return err
		}
	}
	return nil
}

func renderedInputSnapshot(nodes []Node, result *EvalResult) map[string]string {
	out := map[string]string{}
	var walk func([]Node)
	walk = func(current []Node) {
		for _, raw := range current {
			switch x := raw.(type) {
			case InputNode:
				if value, ok := result.Env[x.Name]; ok {
					out[x.Name] = formatInputDisplayValue(x.Type, value)
				}
			case WhenNode:
				if result.WhenByLine[x.Line] {
					walk(x.Nodes)
				}
			}
		}
	}
	walk(nodes)
	return out
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
