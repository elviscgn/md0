package md0

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderFragmentMarksStableReactiveRegions(t *testing.T) {
	src := `A: @input a number = 1
@calc doubled = a * 2
Result: **{{ doubled }}**`
	doc, err := ParseString("regions.md", src)
	if err != nil {
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
	for _, nodeID := range []string{"input:a", "markdown:3"} {
		if !strings.Contains(fragment, `data-md0-node="`+nodeID+`"`) {
			t.Fatalf("fragment missing reactive region %q: %s", nodeID, fragment)
		}
		if !strings.Contains(fragment, `id="`+domNodeID(nodeID)+`"`) {
			t.Fatalf("fragment missing stable DOM id for %q: %s", nodeID, fragment)
		}
	}
	if strings.Contains(fragment, `data-md0-node="calc:doubled"`) {
		t.Fatal("invisible calculation should not create an empty DOM region")
	}
}

func TestRenderPatchesOnlyAffectedRenderableRegions(t *testing.T) {
	src := `A: @input a number = 1
B: @input b number = 2
@calc sum = a + b
Total: **{{ sum }}**
@calc only_b = b * 10
Unrelated: **{{ only_b }}**`
	doc, err := ParseString("patches.md", src)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewReactiveSession(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, stats, err := session.Update(map[string]string{"a": "4", "b": "2"})
	if err != nil {
		t.Fatal(err)
	}
	patches, err := RenderPatches(doc, result, stats)
	if err != nil {
		t.Fatal(err)
	}
	if len(patches) != 1 {
		t.Fatalf("patches=%#v, want one prose patch", patches)
	}
	if patches[0].NodeID != "markdown:4" {
		t.Fatalf("patched %q, want markdown:4", patches[0].NodeID)
	}
	if !strings.Contains(patches[0].HTML, "6") {
		t.Fatalf("patched prose did not contain recomputed value: %s", patches[0].HTML)
	}
	if containsString(stats.Recomputed, "calc:only_b") || containsString(stats.Recomputed, "markdown:6") {
		t.Fatalf("unrelated branch recomputed: %#v", stats.Recomputed)
	}
}

func TestRenderPatchesCollapseConditionalSubtreeIntoParentPatch(t *testing.T) {
	src := `Enabled: @input enabled boolean = true
@when enabled
@calc hidden = 40 + 2
Hidden: **{{ hidden }}**
@end`
	doc, err := ParseString("conditional-patch.md", src)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewReactiveSession(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, stats, err := session.Update(map[string]string{"enabled": "false"})
	if err != nil {
		t.Fatal(err)
	}
	patches, err := RenderPatches(doc, result, stats)
	if err != nil {
		t.Fatal(err)
	}
	if len(patches) != 1 || patches[0].NodeID != "when:2" {
		t.Fatalf("conditional invalidation should collapse to parent patch: %#v", patches)
	}
	if strings.Contains(patches[0].HTML, "Hidden") {
		t.Fatalf("disabled conditional patch leaked child content: %s", patches[0].HTML)
	}
}

func TestPatchHandlerReturnsJSONPatches(t *testing.T) {
	doc, err := ParseString("handler.md", `A: @input a number = 1
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
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, runtimeJSONRequest(token, `{"a":"3"}`))
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content-type=%q", got)
	}
	if res.Header().Get("X-MD0-Changed") != "a" {
		t.Fatalf("changed header=%q", res.Header().Get("X-MD0-Changed"))
	}
	var payload patchResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Patches) != 1 || payload.Patches[0].NodeID != "markdown:3" {
		t.Fatalf("payload=%#v", payload)
	}
	if payload.Patched != 1 {
		t.Fatalf("patched=%d, want 1", payload.Patched)
	}
}
