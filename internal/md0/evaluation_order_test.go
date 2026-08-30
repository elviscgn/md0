package md0

import (
	"strings"
	"testing"
)

func TestEvaluationFollowsDependenciesNotDocumentOrder(t *testing.T) {
	src := `Result first: **{{ doubled }}**
@calc doubled = value * 2
Value: @input value number = 3`
	doc, err := ParseString("forward.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Env["doubled"].String(); got != "6" {
		t.Fatalf("doubled=%s, want 6", got)
	}
	fragment, err := RenderFragment(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fragment, "<strong>6</strong>") {
		t.Fatalf("forward prose dependency did not render: %s", fragment)
	}
}

func TestInputDefaultCanDependOnLaterValue(t *testing.T) {
	src := `Scaled: @input scaled number = base * 2
Base: @input base number = 4`
	doc, err := ParseString("forward-default.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Env["scaled"].String(); got != "8" {
		t.Fatalf("scaled=%s, want 8", got)
	}
}

func TestForwardDependencyReactsInTopologicalOrder(t *testing.T) {
	src := `@calc doubled = value * 2
Result: {{ doubled }}
Value: @input value number = 2`
	doc, err := ParseString("forward-reactive.md", src)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewReactiveSession(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, stats, err := session.Update(map[string]string{"value": "7"})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Env["doubled"].String(); got != "14" {
		t.Fatalf("doubled=%s, want 14", got)
	}
	wantOrder := []string{"input:value", "calc:doubled", "markdown:2"}
	last := -1
	for _, want := range wantOrder {
		idx := indexString(stats.Recomputed, want)
		if idx < 0 {
			t.Fatalf("missing %s from recomputation: %#v", want, stats.Recomputed)
		}
		if idx <= last {
			t.Fatalf("recomputation is not dependency ordered: %#v", stats.Recomputed)
		}
		last = idx
	}
}

func TestConditionalCanDependOnLaterOuterValue(t *testing.T) {
	src := `@when enabled
@calc local = base + 1
Local: {{ local }}
@end
Base: @input base number = 9
Enabled: @input enabled boolean = true`
	doc, err := ParseString("guard-forward.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Env["local"].String(); got != "10" {
		t.Fatalf("local=%s, want 10", got)
	}
}

func TestConditionalValueCannotLeakToOuterScope(t *testing.T) {
	src := `Enabled: @input enabled boolean = true
@when enabled
@calc local = 41 + 1
@end
@calc leaked = local + 1`
	doc, err := ParseString("guard-leak.md", src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(doc, nil)
	if err == nil || !strings.Contains(err.Error(), "conditional scope opened at line 2") {
		t.Fatalf("expected conditional-scope error, got %v", err)
	}
}

func TestSiblingConditionalCannotConsumeGuardedValue(t *testing.T) {
	src := `A: @input a boolean = true
B: @input b boolean = true
@when a
@calc secret = 5
@end
@when b
@calc copy = secret
@end`
	doc, err := ParseString("sibling-guard.md", src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildEvaluationPlan(doc)
	if err == nil || !strings.Contains(err.Error(), "conditional scope opened at line 3") {
		t.Fatalf("expected sibling conditional-scope error, got %v", err)
	}
}

func TestNestedConditionalMayUseOuterGuardedValue(t *testing.T) {
	src := `Outer: @input outer boolean = true
Inner: @input inner boolean = true
@when outer
@calc guarded = 20 + 1
@when inner
@calc nested = guarded * 2
@end
@end`
	doc, err := ParseString("nested-guard.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Env["nested"].String(); got != "42" {
		t.Fatalf("nested=%s, want 42", got)
	}
}

func indexString(values []string, want string) int {
	for i, value := range values {
		if value == want {
			return i
		}
	}
	return -1
}
