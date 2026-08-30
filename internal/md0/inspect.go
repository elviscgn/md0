package md0

import (
	"fmt"
	"strings"
)

func Inspect(doc *Document) string {
	counts := map[string]int{"inputs": 0, "calculations": 0, "assertions": 0, "charts": 0, "tables": 0, "conditions": 0}
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			switch x := n.(type) {
			case InputNode:
				counts["inputs"]++
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
	fmt.Fprintf(&b, "Inputs              %d\nCalculations        %d\nAssertions          %d\nCharts              %d\nTables              %d\nConditions          %d\n\n", counts["inputs"], counts["calculations"], counts["assertions"], counts["charts"], counts["tables"], counts["conditions"])
	b.WriteString("Document authority\n")
	b.WriteString("  Filesystem        no\n  Network           no\n  Shell/processes   no\n  Environment       no\n  Package imports   no\n  Native code       no\n  Dynamic eval      no\n")
	return b.String()
}
