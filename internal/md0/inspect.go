package md0

import (
	"fmt"
	"strings"
)

func Inspect(doc *Document) string {
	counts := map[string]int{"inputs": 0, "data": 0, "calculations": 0, "assertions": 0, "charts": 0, "tables": 0, "conditions": 0}
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			switch x := n.(type) {
			case InputNode:
				counts["inputs"]++
			case DataNode:
				counts["data"]++
			case CalcNode:
				counts["calculations"]++
			case AssertNode:
				counts["assertions"]++
			case ChartNode:
				counts["charts"]++
			case TableNode:
				counts["tables"]++
			case WhenNode:
				counts["conditions"]++
				walk(x.Nodes)
			}
		}
	}
	walk(doc.Nodes)

	var b strings.Builder
	fmt.Fprintf(&b, "Profile             md0/PURE\n")
	languageMode := "implicit"
	if doc.LanguageDeclared {
		languageMode = "declared"
	}
	fmt.Fprintf(&b, "Language            %s (%s)\n", doc.LanguageVersion, languageMode)
	fmt.Fprintf(&b, "Inputs              %d\nData attachments    %d\nCalculations        %d\nAssertions          %d\nCharts              %d\nTables              %d\nConditions          %d\n\n", counts["inputs"], counts["data"], counts["calculations"], counts["assertions"], counts["charts"], counts["tables"], counts["conditions"])

	graph, err := BuildDependencyGraph(doc)
	b.WriteString("Dependency graph\n")
	if err != nil {
		fmt.Fprintf(&b, "  error             %s\n\n", err)
	} else {
		fmt.Fprintf(&b, "  Values            %d\n", len(graph.Producers))
		fmt.Fprintf(&b, "  Nodes             %d\n", len(graph.Nodes))
		fmt.Fprintf(&b, "  Edges             %d\n", graph.EdgeCount)
		fmt.Fprintf(&b, "  Cycles            0\n")
		for _, edge := range graph.EdgeLines() {
			fmt.Fprintf(&b, "  %s\n", edge)
		}
		b.WriteByte('\n')

		renderEvaluationPlan(&b, doc, graph)
	}

	b.WriteString("Document authority\n")
	b.WriteString("  Filesystem        no\n  Network           no\n  Shell/processes   no\n  Environment       no\n  Package imports   no\n  Native code       no\n  Dynamic eval      no\n")
	return b.String()
}

func renderEvaluationPlan(b *strings.Builder, doc *Document, graph *DependencyGraph) {
	b.WriteString("Evaluation plan\n")
	plan, err := BuildEvaluationPlan(doc)
	if err != nil {
		fmt.Fprintf(b, "  error             %s\n\n", err)
		return
	}

	guarded := 0
	for _, id := range plan.Order {
		if len(plan.Guards[id]) > 0 {
			guarded++
		}
	}

	forward := forwardEdgeCount(graph)
	fmt.Fprintf(b, "  Mode              dependency-first\n")
	fmt.Fprintf(b, "  Steps             %d\n", len(plan.Order))
	fmt.Fprintf(b, "  Guarded nodes     %d\n", guarded)
	fmt.Fprintf(b, "  Forward edges     %d\n", forward)
	fmt.Fprintf(b, "  Render order      document order\n")

	for i, id := range plan.Order {
		node := graph.Nodes[id]
		guard := guardDescription(plan, id, graph)
		fmt.Fprintf(b, "  %02d  %-28s line %-4d%s\n", i+1, id, node.Line, guard)
	}
	b.WriteByte('\n')
}

func forwardEdgeCount(graph *DependencyGraph) int {
	count := 0
	for _, targetID := range graph.Order {
		target := graph.Nodes[targetID]
		for _, sourceID := range target.DependsOn {
			source, ok := graph.Nodes[sourceID]
			if ok && source.Line > target.Line {
				count++
			}
		}
	}
	return count
}

func guardDescription(plan *EvaluationPlan, id string, graph *DependencyGraph) string {
	guards := plan.Guards[id]
	if len(guards) == 0 {
		return ""
	}
	parts := make([]string, 0, len(guards))
	for _, guardID := range guards {
		guard, ok := graph.Nodes[guardID]
		if !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("when@%d", guard.Line))
	}
	if len(parts) == 0 {
		return ""
	}
	return "  guarded by " + strings.Join(parts, ", ")
}
