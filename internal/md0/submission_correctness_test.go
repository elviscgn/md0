package md0

import (
	"strings"
	"testing"
)

func TestUpdatedMarkdownHandlesEqualsInsideDefaultExpression(t *testing.T) {
	doc, err := ParseString("equals.md", `Flag: @input flag boolean = 1 == 1
Text: @input text string = "a=b"`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, map[string]string{"flag": "false", "text": "x=y"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := UpdatedMarkdown(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`Flag: @input flag boolean = false`,
		`Text: @input text string = "x=y"`,
	} {
		if !strings.Contains(updated, want) {
			t.Fatalf("updated Markdown missing %q:\n%s", want, updated)
		}
	}
	if _, err := ParseString("equals.md", updated); err != nil {
		t.Fatalf("updated Markdown no longer parses: %v\n%s", err, updated)
	}
}

func TestLargeWholeNumberStringDoesNotOverflowInt64(t *testing.T) {
	got := Number(1e20).String()
	if got != "100000000000000000000" {
		t.Fatalf("large whole number rendered as %q", got)
	}
	if strings.HasPrefix(got, "-") {
		t.Fatalf("large positive number overflowed to negative: %q", got)
	}
}

func TestIntegerDefaultRejectsFractionalNumber(t *testing.T) {
	doc, err := ParseString("integer.md", `Count: @input count integer = 1.5`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(doc, nil)
	if err == nil || !strings.Contains(err.Error(), "default must evaluate to integer") {
		t.Fatalf("fractional integer default error=%v", err)
	}
}
