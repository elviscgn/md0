package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDocumentAppFixture(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDocumentAppRenderReturnsStatusAndWritesHTML(t *testing.T) {
	path := writeDocumentAppFixture(t, "md0: 0.1\n\n# Report\n\n@calc total = 2 + 3\n\nTotal: {{ total }}\n")
	app := &documentApp{path: path}
	if err := app.renderHTML(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(app.status, "rendered · ") {
		t.Fatalf("status=%q", app.status)
	}
	out := defaultHTMLPath(path)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Total: 5") {
		t.Fatalf("rendered HTML missing evaluated content: %s", data)
	}
}

func TestDocumentAppValidateStaysInAppOnSuccess(t *testing.T) {
	path := writeDocumentAppFixture(t, "md0: 0.1\n\n@calc total = 2 + 3\n")
	app := &documentApp{path: path}
	if err := app.validateDocument(); err != nil {
		t.Fatal(err)
	}
	if app.status != "valid md0/PURE document" {
		t.Fatalf("status=%q", app.status)
	}
}

func TestDocumentAppQuitActionOnlyQuitsTopLevel(t *testing.T) {
	app := &documentApp{}
	quit, err := app.activate(launcherQuit)
	if err != nil {
		t.Fatal(err)
	}
	if !quit {
		t.Fatal("quit action did not request app exit")
	}
}
