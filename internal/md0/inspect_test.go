package md0

import (
	"strings"
	"testing"
)

func TestInspectShowsDependencyFirstEvaluationPlan(t *testing.T) {
	src := `@calc doubled = value * 2
@when enabled
@calc guarded = doubled + 1
@end
Value: @input value number = 4
Enabled: @input enabled boolean = true
`
	doc, err := ParseString("inspect-plan.md", src)
	if err != nil {
		t.Fatal(err)
	}
	got := Inspect(doc)

	for _, want := range []string{
		"Evaluation plan",
		"Mode              dependency-first",
		"Render order      document order",
		"Forward edges     2",
		"Guarded nodes     1",
		"guarded by when@2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, got)
		}
	}

	inputPos := strings.Index(got, "input:value")
	calcPos := strings.Index(got, "calc:doubled")
	if inputPos < 0 || calcPos < 0 || inputPos > calcPos {
		t.Fatalf("dependency-first order did not put value before doubled:\n%s", got)
	}

	enabledPos := strings.Index(got, "input:enabled")
	whenPos := strings.Index(got, "when:2")
	if enabledPos < 0 || whenPos < 0 || enabledPos > whenPos {
		t.Fatalf("dependency-first order did not put enabled before its guard:\n%s", got)
	}
}

func TestInspectPlanIsDeterministic(t *testing.T) {
	doc, err := ParseString("stable.md", "@calc total = a + b\nA: @input a number = 1\nB: @input b number = 2\n")
	if err != nil {
		t.Fatal(err)
	}
	first := Inspect(doc)
	for i := 0; i < 20; i++ {
		if got := Inspect(doc); got != first {
			t.Fatalf("inspect output changed between runs\nfirst:\n%s\nlater:\n%s", first, got)
		}
	}
}
