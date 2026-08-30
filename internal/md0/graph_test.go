package md0

import (
	"strings"
	"testing"
)

func TestDependencyGraphTracksBenchmarkFlow(t *testing.T) {
	src := `Baseline: @input baseline_ms number = 1.82
Candidate: @input candidate_ms number = 1.31
@calc speedup = (baseline_ms - candidate_ms) / baseline_ms * 100
Result: **{{ speedup }}%**
@assert candidate_ms <= baseline_ms * 1.05
Regression.
@end
@chart latency
labels = ["Baseline", "Candidate"]
values = [baseline_ms, candidate_ms]
@end`
	doc, err := ParseString("graph.md", src)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := BuildDependencyGraph(doc)
	if err != nil {
		t.Fatal(err)
	}

	calc := graph.Nodes["calc:speedup"]
	if !containsString(calc.DependsOn, "input:baseline_ms") || !containsString(calc.DependsOn, "input:candidate_ms") {
		t.Fatalf("speedup dependencies = %#v", calc.DependsOn)
	}

	affected := graph.AffectedBySymbols([]string{"candidate_ms"})
	for _, id := range []string{"input:candidate_ms", "calc:speedup", "markdown:4", "assert:5", "chart:latency:8"} {
		if !affected[id] {
			t.Fatalf("candidate_ms should invalidate %s; affected=%#v", id, graph.OrderedAffected(affected))
		}
	}
	if affected["input:baseline_ms"] {
		t.Fatal("changing candidate_ms must not invalidate baseline_ms")
	}
}

func TestDependencyCycleRejectedBeforeEvaluation(t *testing.T) {
	doc, err := ParseString("cycle.md", "@calc a = b + 1\n@calc b = a + 1\n")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(doc, nil)
	if err == nil || !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("expected dependency cycle, got %v", err)
	}
}

func TestUnknownDependencyRejected(t *testing.T) {
	doc, err := ParseString("unknown.md", "@calc answer = missing + 1\n")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(doc, nil)
	if err == nil || !strings.Contains(err.Error(), `unknown value "missing"`) {
		t.Fatalf("expected unknown dependency error, got %v", err)
	}
}

func TestReactiveSessionRecomputesOnlyAffectedBranch(t *testing.T) {
	src := `A: @input a number = 1
B: @input b number = 2
@calc sum = a + b
@calc doubled = sum * 2
@calc only_b = b * 10`
	doc, err := ParseString("incremental.md", src)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewReactiveSession(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := session.Update(map[string]string{"a": "1", "b": "2"}); err != nil {
		t.Fatal(err)
	}
	result, stats, err := session.Update(map[string]string{"a": "4", "b": "2"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Env["doubled"].Num != 12 {
		t.Fatalf("doubled=%s, want 12", result.Env["doubled"].String())
	}
	if result.Env["only_b"].Num != 20 {
		t.Fatalf("only_b=%s, want 20", result.Env["only_b"].String())
	}
	for _, id := range []string{"input:a", "calc:sum", "calc:doubled"} {
		if !containsString(stats.Recomputed, id) {
			t.Fatalf("expected %s to recompute: %#v", id, stats.Recomputed)
		}
	}
	for _, id := range []string{"input:b", "calc:only_b"} {
		if containsString(stats.Recomputed, id) {
			t.Fatalf("did not expect %s to recompute: %#v", id, stats.Recomputed)
		}
	}
}

func TestReactiveSessionClearsDisabledSubtree(t *testing.T) {
	src := `Enabled: @input enabled boolean = true
@when enabled
@calc hidden = 40 + 2
@end`
	doc, err := ParseString("when-reactive.md", src)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewReactiveSession(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := session.Snapshot().Env["hidden"]; !ok {
		t.Fatal("hidden value should exist while condition is true")
	}
	result, stats, err := session.Update(map[string]string{"enabled": "false"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Env["hidden"]; ok {
		t.Fatal("hidden value leaked after condition became false")
	}
	if !containsString(stats.Recomputed, "when:2") || !containsString(stats.Recomputed, "calc:hidden") {
		t.Fatalf("conditional subtree was not invalidated: %#v", stats.Recomputed)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
