package md0

import (
	"strings"
	"testing"
)

func TestMalformedExpressionDoesNotCrashParser(t *testing.T) {
	src := "00000000\n@calc aA= A%0+{ y }}**\n"

	_, err := ParseString("fuzz-regression.md", src)
	if err == nil {
		t.Fatal("expected malformed expression to be rejected")
	}
	if !strings.Contains(err.Error(), "unexpected character") {
		t.Fatalf("expected lexical error, got: %v", err)
	}
}

func TestExpressionTokenLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("@calc x = 1")
	for i := 0; i < maxExpressionTokens; i++ {
		b.WriteString(" + 1")
	}
	b.WriteByte('\n')

	_, err := ParseString("token-limit.md", b.String())
	if err == nil {
		t.Fatal("expected expression token limit error")
	}
	if !strings.Contains(err.Error(), "token limit") {
		t.Fatalf("expected token limit error, got: %v", err)
	}
}
