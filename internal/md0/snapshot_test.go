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

func TestSnapshotRoundTripsInputValues(t *testing.T) {
	doc, err := ParseString("decision.md", `Count: @input count integer = 2
Name: @input name string = "draft"
Enabled: @input enabled boolean = false
Result: {{ name }}`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, map[string]string{"count": "7", "name": "ship", "enabled": "true"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := MarshalSnapshot(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Schema != SnapshotSchema || snapshot.MD0Version != RuntimeVersion {
		t.Fatalf("snapshot identity=%#v", snapshot)
	}
	if snapshot.Values["count"] != float64(7) || snapshot.Values["name"] != "ship" || snapshot.Values["enabled"] != true {
		t.Fatalf("snapshot values=%#v", snapshot.Values)
	}
	if len(snapshot.Source.SHA256) != 64 || !strings.Contains(snapshot.Output.HTML, "ship") {
		t.Fatalf("snapshot source/output=%#v", snapshot)
	}

	path := filepath.Join(t.TempDir(), "decision.snapshot.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	values, err := LoadValuesFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if values["count"] != "7" || values["name"] != "ship" || values["enabled"] != "true" {
		t.Fatalf("round-tripped values=%#v", values)
	}
}

func TestValuesFileAcceptsOnlyPrimitiveInputValues(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "values.json")
	if err := os.WriteFile(valid, []byte(`{"n":1.5,"ok":true,"name":"test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	values, err := LoadValuesFile(valid)
	if err != nil {
		t.Fatal(err)
	}
	if values["n"] != "1.5" || values["ok"] != "true" || values["name"] != "test" {
		t.Fatalf("values=%#v", values)
	}

	invalid := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"nested":[1,2]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadValuesFile(invalid); err == nil || !strings.Contains(err.Error(), "string, number, or boolean") {
		t.Fatalf("nested value err=%v", err)
	}
}

func TestUpdatedMarkdownWritesCurrentValuesAsDefaults(t *testing.T) {
	doc, err := ParseString("updated.md", "# Decision\n\nCount: @input count integer = 2\nName: @input name string = \"draft\"\nEnabled: @input enabled boolean = false\n")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, map[string]string{"count": "7", "name": `ready "now"`, "enabled": "true"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := UpdatedMarkdown(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`@input count integer = 7`, `@input name string = "ready \"now\""`, `@input enabled boolean = true`} {
		if !strings.Contains(source, want) {
			t.Fatalf("updated Markdown missing %q:\n%s", want, source)
		}
	}
	updated, err := ParseString("updated.md", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Evaluate(updated, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeExportsSnapshotAndUpdatedMarkdown(t *testing.T) {
	doc, err := ParseString("runtime.md", "A: @input a number = 1\nResult: {{ a }}")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newHandler(doc)
	if err != nil {
		t.Fatal(err)
	}
	token := runtimeTokenForHandler(t, handler)
	update := httptest.NewRecorder()
	handler.ServeHTTP(update, runtimeJSONRequest(token, `{"a":"4"}`))
	if update.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", update.Code, update.Body.String())
	}

	for _, tc := range []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/snapshot", contentType: "application/json", contains: `"a": 4`},
		{path: "/markdown", contentType: "text/markdown", contains: `@input a number = 4`},
	} {
		req := runtimeTestRequest(http.MethodPost, tc.path, nil)
		req.Header.Set("X-MD0-Token", token)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK || !strings.HasPrefix(res.Header().Get("Content-Type"), tc.contentType) || !strings.Contains(res.Body.String(), tc.contains) {
			t.Fatalf("%s status=%d type=%q body=%s", tc.path, res.Code, res.Header().Get("Content-Type"), res.Body.String())
		}
	}
}
