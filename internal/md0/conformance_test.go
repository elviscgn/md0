package md0

import (
	"strings"
	"testing"
)

func TestParserConformanceCorpus(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		errContain string
	}{
		{"plain markdown", "hello world\n", ""},
		{"inline input", "Value: @input x number = 3\n", ""},
		{"forward calculation", "@calc doubled = value * 2\nValue: @input value number = 4\n", ""},
		{"escaped inline input", "\\@input fake number = 9\n", ""},
		{"single backtick input literal", "`@input fake number = 9`\n", ""},
		{"double backtick input literal", "``@input fake number = `9` ``\n", ""},
		{"backtick fenced directives literal", "```md\n@calc fake = 1\n@end\n```\n", ""},
		{"tilde fenced directives literal", "~~~md\n@input fake number = 1\n~~~\n", ""},
		{"mismatched fence stays open", "```\n@calc fake = 1\n~~~\n```\n", ""},
		{"longer matching fence closes", "```\n@calc fake = 1\n````\n", ""},
		{"nested conditional", "Enabled: @input enabled boolean = true\n@when enabled\n@when true\n@calc inside = 42\n@end\n@end\n", ""},
		{"assert message fenced terminator", "@assert true\n```text\n@end\n```\nstill message\n@end\n", ""},
		{"chart", "@chart c\nlabels = [\"a\", \"b\"]\nvalues = [1, 2]\n@end\n", ""},
		{"table", "@table t\ncolumns = [\"a\"]\nrows = [[1]]\n@end\n", ""},
		{"interpolation in prose", "Value: @input x number = 3\nResult {{ x + 1 }}\n", ""},
		{"interpolation in code literal", "`{{ missing }}`\n", ""},
		{"interpolation in double code literal", "``{{ missing }} and ` tick``\n", ""},
		{"escaped interpolation literal", "\\{{ missing }}\n", ""},
		{"quoted interpolation terminator", "Text: @input text string = \"x\"\n{{ text == \"}}\" ? 1 : 0 }}\n", ""},
		{"unexpected end", "@end\n", "unexpected @end"},
		{"unknown directive", "@wat nope\n", "unknown directive"},
		{"chart keyword boundary", "@chartreuse\n", "unknown directive"},
		{"table keyword boundary", "@tabletop\n", "unknown directive"},
		{"calc missing expression", "@calc x =\n", "expected @calc"},
		{"show missing expression", "@show\n", "requires an expression"},
		{"assert missing expression", "@assert\n", "requires an expression"},
		{"when missing expression", "@when\n@end\n", "requires an expression"},
		{"unterminated when", "@when true\nhello\n", "unterminated block"},
		{"unterminated assert", "@assert true\nmessage\n", "missing @end"},
		{"chart missing values", "@chart c\nlabels = [\"a\"]\n@end\n", "requires labels and values"},
		{"table missing rows", "@table t\ncolumns = [\"a\"]\n@end\n", "requires columns and rows"},
		{"duplicate chart key", "@chart c\nlabels = [\"a\"]\nlabels = [\"b\"]\nvalues = [1]\n@end\n", "duplicate @chart key"},
		{"duplicate table key", "@table t\ncolumns = [\"a\"]\ncolumns = [\"b\"]\nrows = [[1]]\n@end\n", "duplicate @table key"},
		{"bad config line", "@chart c\nlabels [\"a\"]\nvalues = [1]\n@end\n", "expected key = value"},
		{"invalid calc expression", "@calc x = 1 + * 2\n", "invalid @calc expression"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := ParseString(tc.name+".md", tc.source)
			if tc.errContain != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errContain) {
					t.Fatalf("expected error containing %q, got %v", tc.errContain, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Evaluate(doc, nil); err != nil {
				t.Fatalf("valid corpus case failed evaluation: %v", err)
			}
		})
	}
}

func TestInlineCodeSpanConformance(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"single ticks", "Use `code` now", "Use <code>code</code> now"},
		{"double ticks contain single tick", "Use ``code `tick` here`` now", "Use <code>code `tick` here</code> now"},
		{"triple ticks contain double ticks", "```a `` b```", "<code>a `` b</code>"},
		{"code protects bold markers", "`**literal**`", "<code>**literal**</code>"},
		{"bold can contain code", "**use `x`**", "<strong>use <code>x</code></strong>"},
		{"html escaped inside code", "`<tag>&`", "<code>&lt;tag&gt;&amp;</code>"},
		{"html escaped in prose", "<tag>&", "&lt;tag&gt;&amp;"},
		{"trim one code padding space", "` code `", "<code>code</code>"},
		{"preserve all-space code", "`   `", "<code>   </code>"},
		{"unmatched backtick literal", "before `` after", "before `` after"},
		{"longer run does not close shorter", "`a``b`", "<code>a``b</code>"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderInline(tc.in); got != tc.want {
				t.Fatalf("renderInline(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCodeExamplesStayNonReactiveAcrossParserAndRenderer(t *testing.T) {
	src := "Value: @input real number = 4\n\n``md0 {{ missing }} @input fake number = 9 `tick` ``\n\nReal result: **{{ real * 2 }}**\n"
	doc, err := ParseString("code-example.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := result.Env["fake"]; exists {
		t.Fatal("inline code example became executable")
	}
	fragment, err := RenderFragment(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fragment, "<code>md0 {{ missing }} @input fake number = 9 `tick` </code>") {
		t.Fatalf("code example rendered incorrectly: %s", fragment)
	}
	if !strings.Contains(fragment, "<strong>8</strong>") {
		t.Fatalf("real interpolation did not render: %s", fragment)
	}
}
