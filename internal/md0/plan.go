package md0

import "fmt"

// EvaluationPlan binds the dependency graph to concrete AST nodes and a
// deterministic dependency-first execution order. Document order remains the
// render order; computation order follows dependencies.
type EvaluationPlan struct {
	Graph  *DependencyGraph
	Nodes  map[string]Node
	Guards map[string][]string
	Order  []string
}

func BuildEvaluationPlan(doc *Document) (*EvaluationPlan, error) {
	graph, err := BuildDependencyGraph(doc)
	if err != nil {
		return nil, err
	}
	plan := &EvaluationPlan{
		Graph:  graph,
		Nodes:  make(map[string]Node, len(graph.Nodes)),
		Guards: make(map[string][]string, len(graph.Nodes)),
	}

	var collect func([]Node, []string)
	collect = func(nodes []Node, guards []string) {
		for _, node := range nodes {
			id := dependencyNodeID(node)
			plan.Nodes[id] = node
			plan.Guards[id] = append([]string(nil), guards...)
			if when, ok := node.(WhenNode); ok {
				next := append(append([]string(nil), guards...), id)
				collect(when.Nodes, next)
			}
		}
	}
	collect(doc.Nodes, nil)

	if len(plan.Nodes) != len(graph.Nodes) {
		return nil, fmt.Errorf("internal evaluation-plan node mismatch")
	}
	if err := plan.validateConditionalScopes(); err != nil {
		return nil, err
	}
	plan.Order = dependencyFirstOrder(graph)
	if len(plan.Order) != len(graph.Nodes) {
		return nil, fmt.Errorf("internal evaluation-plan ordering mismatch")
	}
	return plan, nil
}

func dependencyFirstOrder(graph *DependencyGraph) []string {
	visited := make(map[string]bool, len(graph.Nodes))
	order := make([]string, 0, len(graph.Nodes))
	var visit func(string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		for _, dependency := range graph.Nodes[id].DependsOn {
			visit(dependency)
		}
		order = append(order, id)
	}
	for _, id := range graph.Order {
		visit(id)
	}
	return order
}

func (p *EvaluationPlan) validateConditionalScopes() error {
	for _, targetID := range p.Graph.Order {
		target := p.Graph.Nodes[targetID]
		for _, sourceID := range target.DependsOn {
			missingGuard := firstMissingGuard(p.Guards[sourceID], p.Guards[targetID])
			if missingGuard == "" {
				continue
			}
			source := p.Graph.Nodes[sourceID]
			guard := p.Graph.Nodes[missingGuard]
			return fmt.Errorf(
				"line %d: %s depends on %s from conditional scope opened at line %d",
				target.Line,
				target.Label,
				source.Label,
				guard.Line,
			)
		}
	}
	return nil
}

func firstMissingGuard(sourceGuards, targetGuards []string) string {
	if len(sourceGuards) == 0 {
		return ""
	}
	targetSet := make(map[string]struct{}, len(targetGuards))
	for _, id := range targetGuards {
		targetSet[id] = struct{}{}
	}
	for _, id := range sourceGuards {
		if _, ok := targetSet[id]; !ok {
			return id
		}
	}
	return ""
}

func (p *EvaluationPlan) guardsActive(id string, result *EvalResult) bool {
	for _, guardID := range p.Guards[id] {
		when, ok := p.Nodes[guardID].(WhenNode)
		if !ok || !result.WhenByLine[when.Line] {
			return false
		}
	}
	return true
}

func (p *EvaluationPlan) OrderedAffected(affected map[string]bool) []string {
	out := make([]string, 0, len(affected))
	for _, id := range p.Order {
		if affected[id] {
			out = append(out, id)
		}
	}
	return out
}
