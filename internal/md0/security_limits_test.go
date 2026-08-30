package md0

import (
	"strings"
	"testing"
)

func TestSecurityStringInputSizeLimit(t *testing.T) {
	doc, err := ParseString("string-limit.md", `Payload: @input payload string = "ok"
Result: {{ payload }}`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(doc, map[string]string{"payload": strings.Repeat("x", maxInputStringBytes+1)})
	if err == nil || !strings.Contains(err.Error(), "16 KiB limit") {
		t.Fatalf("err=%v, want string-input size rejection", err)
	}
}

func TestSecurityInterpolationOutputLimit(t *testing.T) {
	var src strings.Builder
	src.WriteString(`Payload: @input payload string = "ok"`)
	src.WriteByte('\n')
	for i := 0; i < 270; i++ {
		src.WriteString("{{ payload }} ")
	}
	doc, err := ParseString("amplification.md", src.String())
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, map[string]string{"payload": strings.Repeat("x", maxInputStringBytes)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RenderFragment(doc, result)
	if err == nil || !strings.Contains(err.Error(), "4 MiB limit") {
		t.Fatalf("err=%v, want interpolation-output rejection", err)
	}
}
