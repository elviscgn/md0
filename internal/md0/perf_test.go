package md0

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

type benchmarkWorkload struct {
	name      string
	inputName string
	source    string
}

func benchmarkChain(nodes int) benchmarkWorkload {
	var b strings.Builder
	b.Grow(nodes * 32)
	b.WriteString("@input x0 number = 1\n")
	for i := 1; i <= nodes; i++ {
		fmt.Fprintf(&b, "@calc x%d = x%d + 1\n", i, i-1)
	}
	fmt.Fprintf(&b, "\nFinal: {{ x%d }}\n", nodes)
	return benchmarkWorkload{name: "chain", inputName: "x0", source: b.String()}
}

func benchmarkFanout(nodes int) benchmarkWorkload {
	var b strings.Builder
	b.Grow(nodes * 42)
	b.WriteString("@input root number = 1\n\n")
	for i := 1; i <= nodes; i++ {
		fmt.Fprintf(&b, "@calc y%d = root + %d\n", i, i)
	}
	b.WriteString("\n## Values\n")
	for i := 1; i <= nodes; i++ {
		fmt.Fprintf(&b, "y%d = {{ y%d }}\n", i, i)
	}
	return benchmarkWorkload{name: "fanout", inputName: "root", source: b.String()}
}

func BenchmarkRuntime(b *testing.B) {
	for _, size := range []int{100, 1000, 5000} {
		for _, workload := range []benchmarkWorkload{benchmarkChain(size), benchmarkFanout(size)} {
			name := workload.name + "/" + strconv.Itoa(size)
			b.Run(name+"/parse", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, err := ParseString("bench.md", workload.source); err != nil {
						b.Fatal(err)
					}
				}
			})

			doc, err := ParseString("bench.md", workload.source)
			if err != nil {
				b.Fatalf("prepare %s: %v", name, err)
			}

			b.Run(name+"/plan", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, err := BuildEvaluationPlan(doc); err != nil {
						b.Fatal(err)
					}
				}
			})

			plan, err := BuildEvaluationPlan(doc)
			if err != nil {
				b.Fatalf("plan %s: %v", name, err)
			}

			b.Run(name+"/eval", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, err := evaluateWithPlan(doc, plan, nil); err != nil {
						b.Fatal(err)
					}
				}
			})

			result, err := evaluateWithPlan(doc, plan, nil)
			if err != nil {
				b.Fatalf("evaluate %s: %v", name, err)
			}

			b.Run(name+"/render", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, err := RenderFragment(doc, result); err != nil {
						b.Fatal(err)
					}
				}
			})

			session, err := NewReactiveSession(doc)
			if err != nil {
				b.Fatalf("session %s: %v", name, err)
			}
			b.Run(name+"/incremental", func(b *testing.B) {
				b.ReportAllocs()
				if _, err := session.Reset(); err != nil {
					b.Fatal(err)
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					value := "2"
					if i%2 == 1 {
						value = "3"
					}
					if _, stats, err := session.Update(map[string]string{workload.inputName: value}); err != nil {
						b.Fatal(err)
					} else if len(stats.Recomputed) == 0 {
						b.Fatal("expected incremental recomputation")
					}
				}
			})
		}
	}
}

func TestScaleHarness(t *testing.T) {
	workload := benchmarkChain(1000)
	doc, err := ParseString("scale-smoke.md", workload.source)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildEvaluationPlan(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := evaluateWithPlan(doc, plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Env["x1000"].String(); got != "1001" {
		t.Fatalf("x1000 = %s, want 1001", got)
	}
	if _, err := RenderFragment(doc, result); err != nil {
		t.Fatal(err)
	}
	session, err := NewReactiveSession(doc)
	if err != nil {
		t.Fatal(err)
	}
	next, stats, err := session.Update(map[string]string{"x0": "2"})
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Env["x1000"].String(); got != "1002" {
		t.Fatalf("updated x1000 = %s, want 1002", got)
	}
	if len(stats.Recomputed) < 1000 {
		t.Fatalf("recomputed %d nodes, want dependency chain invalidation", len(stats.Recomputed))
	}
}
