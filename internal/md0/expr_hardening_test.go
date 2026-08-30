package md0

import (
	"strings"
	"testing"
)

func TestParseExprPropagatesLexerErrors(t *testing.T) {
	cases := []string{
		"1 + {",
		"-{",
		"true ? 1 : {",
		"max(1, {",
		"[1, {",
		"(1 + {",
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			_, err := ParseExpr(src)
			if err == nil {
				t.Fatal("expected lexical error")
			}
			if !strings.Contains(err.Error(), "unexpected character") {
				t.Fatalf("err=%v, want unexpected-character error", err)
			}
		})
	}
}

func TestEvaluatedStringResultLimit(t *testing.T) {
	expr, err := ParseExpr("a + b")
	if err != nil {
		t.Fatal(err)
	}
	left := strings.Repeat("a", maxEvaluatedStringBytes/2+1)
	right := strings.Repeat("b", maxEvaluatedStringBytes/2+1)
	_, err = expr.Eval(map[string]Value{"a": String(left), "b": String(right)})
	if err == nil || !strings.Contains(err.Error(), "1 MiB limit") {
		t.Fatalf("err=%v, want computed-string size rejection", err)
	}
}

func TestDocumentStringGrowthIsBounded(t *testing.T) {
	src := `Seed: @input seed string = "x"
@calc a = seed + seed
@calc b = a + a
@calc c = b + b
@calc d = c + c
@calc e = d + d
@calc f = e + e
@calc g = f + f
@calc h = g + g`
	doc, err := ParseString("string-growth.md", src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(doc, map[string]string{"seed": strings.Repeat("x", maxInputStringBytes)})
	if err == nil || !strings.Contains(err.Error(), "1 MiB limit") {
		t.Fatalf("err=%v, want evaluator amplification rejection", err)
	}
}
