package md0

import (
	"strings"
	"testing"
)

func TestFencedCodeKeepsDirectivesLiteral(t *testing.T) {
	src := "```md\n@calc hidden = 40 + 2\n@end\n~~~\n@input fake number = 99\n```\nReal: @input real number = 3\n"
	doc, err := ParseString("fences.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Env["hidden"]; ok {
		t.Fatal("directive inside fenced code was evaluated")
	}
	if _, ok := result.Env["fake"]; ok {
		t.Fatal("inline input inside fenced code was evaluated")
	}
	if got := result.Env["real"].String(); got != "3" {
		t.Fatalf("real=%s, want 3", got)
	}
	fragment, err := RenderFragment(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fragment, "@calc hidden = 40 + 2") || !strings.Contains(fragment, "~~~") {
		t.Fatalf("fenced source was not rendered literally: %s", fragment)
	}
}

func TestTildeFenceIgnoresBacktickFenceInside(t *testing.T) {
	src := "~~~md\n```\n@calc hidden = 1\n```\n~~~\nVisible: @input visible number = 2\n"
	doc, err := ParseString("tilde.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Env["hidden"]; ok {
		t.Fatal("mismatched backtick fence closed tilde fence")
	}
	if result.Env["visible"].String() != "2" {
		t.Fatal("visible input was not parsed after closing tilde fence")
	}
}

func TestAssertEndInsideFenceDoesNotTerminateBlock(t *testing.T) {
	src := "@assert true\nThe literal terminator is:\n```text\n@end\n```\nand this is still the assertion message.\n@end\n"
	doc, err := ParseString("assert-fence.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assertions) != 1 || !result.Assertions[0].Passed {
		t.Fatalf("assertion did not survive fenced @end: %#v", result.Assertions)
	}
	message := result.Assertions[0].Message
	if !strings.Contains(message, "@end") || !strings.Contains(message, "still the assertion message") {
		t.Fatalf("assertion message truncated: %q", message)
	}
}

func TestInlineCodeAndEscapedInputStayLiteral(t *testing.T) {
	src := "Example: `@input fake number = 99`\nEscaped: \\@input also_fake number = 12\nReal: @input real number = 7\n"
	doc, err := ParseString("literal-input.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fake", "also_fake"} {
		if _, ok := result.Env[name]; ok {
			t.Fatalf("literal input %q was evaluated", name)
		}
	}
	if got := result.Env["real"].String(); got != "7" {
		t.Fatalf("real=%s, want 7", got)
	}
}

func TestDirectiveKeywordRequiresBoundary(t *testing.T) {
	for _, src := range []string{"@chartreuse\n", "@tabletop\n"} {
		_, err := ParseString("keyword.md", src)
		if err == nil || !strings.Contains(err.Error(), "unknown directive") {
			t.Fatalf("%q should be an unknown directive, got %v", strings.TrimSpace(src), err)
		}
	}
}

func TestDuplicateConfigKeyRejected(t *testing.T) {
	src := "@chart latency\nlabels = [\"a\"]\nlabels = [\"b\"]\nvalues = [1]\n@end\n"
	_, err := ParseString("duplicate.md", src)
	if err == nil || !strings.Contains(err.Error(), "duplicate @chart key \"labels\"") {
		t.Fatalf("expected duplicate config-key error, got %v", err)
	}
}

func TestLongerClosingFenceClosesShorterFence(t *testing.T) {
	src := "```\n@calc hidden = 1\n````\nReal: @input real number = 4\n"
	doc, err := ParseString("long-close.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Env["hidden"]; ok {
		t.Fatal("fenced calc was evaluated")
	}
	if result.Env["real"].String() != "4" {
		t.Fatal("longer matching closing fence did not close block")
	}
}
