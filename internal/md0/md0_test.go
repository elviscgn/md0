package md0

import (
	"strings"
	"testing"
)

func TestExpressionPrecedenceAndFunctions(t *testing.T) {
	expr, err := ParseExpr(`round((10 + 2) / 5 * 10) / 10`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := expr.Eval(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != NumberKind || got.Num != 2.4 {
		t.Fatalf("got %v, want 2.4", got)
	}
}

func TestExpressionTernaryAndShortCircuit(t *testing.T) {
	expr, err := ParseExpr(`true ? "ship" : (1 / 0)`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := expr.Eval(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "ship" {
		t.Fatalf("got %q", got.String())
	}

	expr, err = ParseExpr(`false && (1 / 0 > 1)`)
	if err != nil {
		t.Fatal(err)
	}
	got, err = expr.Eval(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != BoolKind || got.Bool {
		t.Fatalf("expected false")
	}
}

func TestParserAndBenchmarkEvaluation(t *testing.T) {
	src := `# Parser Benchmark

Baseline: @input baseline_ms number = 1.82
Candidate: @input candidate_ms number = 1.31
@calc speedup = (baseline_ms - candidate_ms) / baseline_ms * 100

Parse time improved by **{{ round(speedup * 10) / 10 }}%**.

@assert candidate_ms <= baseline_ms * 1.05
Candidate regressed by more than 5%.
@end

@chart latency
labels = ["Baseline", "Candidate"]
values = [baseline_ms, candidate_ms]
@end

@table comparison
columns = ["Metric", "Baseline", "Candidate"]
rows = [["Parse latency (ms)", baseline_ms, candidate_ms]]
@end`
	doc, err := ParseString("benchmark.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assertions) != 1 || !result.Assertions[0].Passed {
		t.Fatalf("assertion should pass: %#v", result.Assertions)
	}
	frag, err := RenderFragment(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"28%", "Baseline", "Candidate", "assertion passed", "Parse latency (ms)"} {
		if !strings.Contains(frag, want) {
			t.Fatalf("render missing %q\n%s", want, frag)
		}
	}
}

func TestOverrideFlipsAssertion(t *testing.T) {
	src := `Candidate: @input candidate number = 1.31
Baseline: @input baseline number = 1.82
@assert candidate <= baseline * 1.05
Regression.
@end`
	doc, err := ParseString("x.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, map[string]string{"candidate": "2.10", "baseline": "1.82"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assertions) != 1 || result.Assertions[0].Passed {
		t.Fatalf("assertion should fail")
	}
}

func TestWhenCondition(t *testing.T) {
	src := `Enabled: @input enabled boolean = true
@when enabled
@calc answer = 40 + 2
Answer: **{{ answer }}**
@end`
	doc, err := ParseString("when.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, map[string]string{"enabled": "true"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Env["answer"].String() != "42" {
		t.Fatalf("answer=%s", result.Env["answer"].String())
	}
	frag, err := RenderFragment(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(frag, "42") {
		t.Fatalf("render missing conditional result")
	}
}

func TestUnknownDirectiveRejected(t *testing.T) {
	_, err := ParseString("bad.md", "@shell rm -rf /\n")
	if err == nil || !strings.Contains(err.Error(), "unknown directive") {
		t.Fatalf("expected unknown directive error, got %v", err)
	}
}

func TestMalformedExpressionHasPosition(t *testing.T) {
	_, err := ParseString("bad.md", "@calc x = 1 + * 2\n")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "line 1") || !strings.Contains(err.Error(), "at") {
		t.Fatalf("error lacks useful position: %v", err)
	}
}

func TestStaticRenderContainsNoServerFetch(t *testing.T) {
	doc, err := ParseString("x.md", "Value: @input x number = 2\n\n**{{ x }}**\n")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	frag, err := RenderFragment(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	page := RenderStaticPage("x.md", frag)
	if strings.Contains(page, "fetch('/render'") {
		t.Fatal("static render must not contain interactive server fetch")
	}
}

func FuzzParseAndEvaluateNeverPanics(f *testing.F) {
	seeds := []string{
		"# hello\n",
		"Value: @input x number = 1\n@calc y = x * 2\n**{{ y }}**\n",
		"@assert true\nok\n@end\n",
		"@chart x\nlabels = [\"a\"]\nvalues = [1]\n@end\n",
		"```\n@calc this_is_code = 1\n```\n",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 128*1024 {
			t.Skip()
		}
		doc, err := ParseString("fuzz.md", src)
		if err != nil {
			return
		}
		_, _ = Evaluate(doc, nil)
	})
}
