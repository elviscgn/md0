package md0

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatDiagnosticAddsSourceFrame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.md")
	source := "@calc total = 1 + * 2\n"
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
	formatted := FormatDiagnostic(path, err)
	for _, want := range []string{path + ":1:", "invalid @calc expression", "@calc total = 1 + * 2", "|", "^"} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("diagnostic missing %q:\n%s", want, formatted)
		}
	}
	if strings.Contains(formatted, " at 5") {
		t.Fatalf("raw expression offset should be converted into a source column:\n%s", formatted)
	}
}

func TestPatchHandlerReturnsStructuredInputErrorAndRecovers(t *testing.T) {
	doc, err := ParseString("recover.md", `A: @input a number = 1
@calc doubled = a * 2
Result: **{{ doubled }}**`)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newHandler(doc)
	if err != nil {
		t.Fatal(err)
	}
	token := runtimeTokenForHandler(t, handler)

	badRes := httptest.NewRecorder()
	handler.ServeHTTP(badRes, runtimeJSONRequest(token, `{"a":"nope"}`))
	if badRes.Code != http.StatusBadRequest {
		t.Fatalf("bad input status=%d body=%s", badRes.Code, badRes.Body.String())
	}
	if got := badRes.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("bad input content-type=%q", got)
	}
	var problem patchErrorResponse
	if err := json.NewDecoder(badRes.Body).Decode(&problem); err != nil {
		t.Fatal(err)
	}
	if problem.Input != "a" || !strings.Contains(problem.Error, "expected number") {
		t.Fatalf("problem=%#v", problem)
	}

	goodRes := httptest.NewRecorder()
	handler.ServeHTTP(goodRes, runtimeJSONRequest(token, `{"a":"3"}`))
	if goodRes.Code != http.StatusOK {
		t.Fatalf("recovery status=%d body=%s", goodRes.Code, goodRes.Body.String())
	}
	var payload patchResponse
	if err := json.NewDecoder(goodRes.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Patches) != 1 || !strings.Contains(payload.Patches[0].HTML, "6") {
		t.Fatalf("recovery payload=%#v", payload)
	}
}

func TestInteractiveRuntimePageHasVisibleErrorChannel(t *testing.T) {
	page := renderInteractiveRuntimePage("demo.md", `<div class="md0-doc"></div>`, "test-capability-token")
	for _, want := range []string{`id="md0-status"`, `aria-live="polite"`, `name="md0-runtime-token"`, `name="md0-source-revision"`, "x-md0-token", "payload.error", "aria-invalid", "node.outerHTML=patch.html", "/source-status", "Export snapshot", "Save values to Markdown"} {
		if !strings.Contains(page, want) {
			t.Fatalf("interactive runtime page missing %q", want)
		}
	}
	if strings.Contains(page, "document.getElementById('md0-document').innerHTML=") {
		t.Fatal("interactive runtime must patch regions rather than replace the whole document")
	}
}
