package md0

import (
	"strings"
	"testing"
)

func TestCodeInterpolationsStayLiteralAndDependencyFree(t *testing.T) {
	src := "Value: @input value number = 5\nReal: **{{ value }}**\nInline: `{{ missing_inline }}`\n```md\n{{ missing_fence }}\n```\nEscaped: \\{{ missing_escaped }}\n"
	doc, err := ParseString("literal-interpolation.md", src)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := BuildDependencyGraph(doc)
	if err != nil {
		t.Fatalf("literal interpolation created dependency: %v", err)
	}
	if graph.EdgeCount != 1 {
		t.Fatalf("edges=%d, want only the real interpolation edge", graph.EdgeCount)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := RenderFragment(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fragment, "<strong>5</strong>") {
		t.Fatalf("real interpolation was not evaluated: %s", fragment)
	}
	for _, literal := range []string{"{{ missing_inline }}", "{{ missing_fence }}", `\{{ missing_escaped }}`} {
		if !strings.Contains(fragment, literal) {
			t.Fatalf("literal interpolation %q was changed: %s", literal, fragment)
		}
	}
}

func TestInterpolationClosingBracesInsideStringAreLiteral(t *testing.T) {
	doc, err := ParseString("quoted-braces.md", `Result: {{ "a}}b" }}`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildDependencyGraph(doc); err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := RenderFragment(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fragment, "Result: a}}b") {
		t.Fatalf("quoted braces ended interpolation early: %s", fragment)
	}
}

func TestMalformedInterpolationInCodeDoesNotFailGraph(t *testing.T) {
	src := "Inline: `{{ 1 + }}`\n```\n{{ 2 + }}\n```\n"
	doc, err := ParseString("code-malformed.md", src)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildDependencyGraph(doc); err != nil {
		t.Fatalf("malformed literal code interpolation should be ignored: %v", err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RenderFragment(doc, result); err != nil {
		t.Fatalf("malformed literal code interpolation should render literally: %v", err)
	}
}

func TestMalformedActiveInterpolationStillFails(t *testing.T) {
	doc, err := ParseString("bad-interpolation.md", `Bad: {{ 1 + }}`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildDependencyGraph(doc)
	if err == nil || !strings.Contains(err.Error(), "interpolation dependency") {
		t.Fatalf("expected active interpolation error, got %v", err)
	}
}

func TestReactiveUpdatePatchesRealInterpolationButNotLiteralCode(t *testing.T) {
	src := "Value: @input value number = 1\nReal: {{ value }}\nCode: `{{ value }}`\n"
	doc, err := ParseString("reactive-code.md", src)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewReactiveSession(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, stats, err := session.Update(map[string]string{"value": "9"})
	if err != nil {
		t.Fatal(err)
	}
	patches, err := RenderPatches(doc, result, stats)
	if err != nil {
		t.Fatal(err)
	}
	if len(patches) != 1 {
		t.Fatalf("patches=%#v, want one markdown region patch", patches)
	}
	if !strings.Contains(patches[0].HTML, "Real: 9") || !strings.Contains(patches[0].HTML, "{{ value }}") {
		t.Fatalf("patch did not separate prose from literal code: %s", patches[0].HTML)
	}
}
