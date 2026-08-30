package md0

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEditorPageEscapesSourceAndExtendsCSP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.md")
	source := "md0: 0.1\n# <script>alert(1)</script>\nValue: @input x number = 2\n"
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	handler, err := newEditorHandler(path, "127.0.0.1:8080", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/", nil)
	req.Host = "127.0.0.1:8080"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{"md0-editor-source", "md0-editor-token", "&lt;script&gt;alert(1)&lt;/script&gt;"} {
		if !strings.Contains(body, want) {
			t.Fatalf("editor page missing %q", want)
		}
	}
	if !strings.Contains(rr.Header().Get("Content-Security-Policy"), cspHash(editorJS)) {
		t.Fatal("editor script hash missing from CSP")
	}
	if !strings.Contains(rr.Header().Get("Content-Security-Policy"), cspHash(editorCSS)) {
		t.Fatal("editor style hash missing from CSP")
	}
}

func TestEditorDraftIsInMemoryAndReactive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.md")
	original := "md0: 0.1\n# Original\nValue: @input x number = 2\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	handler, err := newEditorHandler(path, "127.0.0.1:8080", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	editor := handler.(*editorHandler)
	payload, _ := json.Marshal(editorDraftRequest{
		Source: "md0: 0.1\n# Draft\nValue: @input x number = 2\n\nCurrent **{{ x }}**.\n",
		Values: map[string]string{"x": "7"},
	})
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/editor/draft", bytes.NewReader(payload))
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MD0-Editor-Token", editor.token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Draft") || !strings.Contains(rr.Body.String(), "7") {
		t.Fatalf("draft response did not render updated source/value: %s", rr.Body.String())
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != original {
		t.Fatal("draft rendering mutated the source file")
	}
}

func TestEditorRejectsWrongContentTypesAndTrailingDraftJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.md")
	if err := os.WriteFile(path, []byte("md0: 0.1\n# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	handler, err := newEditorHandler(path, "127.0.0.1:8080", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	editor := handler.(*editorHandler)

	wrongDraft := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/editor/draft", strings.NewReader(`{"source":"md0: 0.1"}`))
	wrongDraft.Host = "127.0.0.1:8080"
	wrongDraft.Header.Set("Content-Type", "text/plain")
	wrongDraft.Header.Set("X-MD0-Editor-Token", editor.token)
	wrongDraftRR := httptest.NewRecorder()
	handler.ServeHTTP(wrongDraftRR, wrongDraft)
	if wrongDraftRR.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wrong draft content type status=%d", wrongDraftRR.Code)
	}

	trailing := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/editor/draft", strings.NewReader(`{"source":"md0: 0.1\n# A\n","values":{}} {}`))
	trailing.Host = "127.0.0.1:8080"
	trailing.Header.Set("Content-Type", "application/json")
	trailing.Header.Set("X-MD0-Editor-Token", editor.token)
	trailingRR := httptest.NewRecorder()
	handler.ServeHTTP(trailingRR, trailing)
	if trailingRR.Code != http.StatusBadRequest {
		t.Fatalf("trailing draft JSON status=%d", trailingRR.Code)
	}

	wrongSave := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/editor/source", strings.NewReader("md0: 0.1\n"))
	wrongSave.Host = "127.0.0.1:8080"
	wrongSave.Header.Set("Content-Type", "application/json")
	wrongSave.Header.Set("X-MD0-Editor-Token", editor.token)
	wrongSaveRR := httptest.NewRecorder()
	handler.ServeHTTP(wrongSaveRR, wrongSave)
	if wrongSaveRR.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wrong save content type status=%d", wrongSaveRR.Code)
	}
}

func TestEditorSaveRequiresCapabilityAndWritesOpenedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.md")
	if err := os.WriteFile(path, []byte("md0: 0.1\n# Old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	handler, err := newEditorHandler(path, "127.0.0.1:8080", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	editor := handler.(*editorHandler)
	newSource := "md0: 0.1\n# Saved\n"

	bad := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/editor/source", strings.NewReader(newSource))
	bad.Host = "127.0.0.1:8080"
	bad.Header.Set("Content-Type", "text/plain; charset=utf-8")
	bad.Header.Set("X-MD0-Editor-Token", "wrong")
	badRR := httptest.NewRecorder()
	handler.ServeHTTP(badRR, bad)
	if badRR.Code != http.StatusForbidden {
		t.Fatalf("bad token status=%d", badRR.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/editor/source", strings.NewReader(newSource))
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	req.Header.Set("X-MD0-Editor-Token", editor.token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		body, _ := io.ReadAll(rr.Body)
		t.Fatalf("status=%d body=%s", rr.Code, body)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != newSource {
		t.Fatalf("saved source=%q", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("permissions changed to %o", info.Mode().Perm())
		}
	}
}
