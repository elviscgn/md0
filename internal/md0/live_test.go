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

func TestLiveHandlerReportsDiagnosticsAndReloadsValidSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.md")
	if err := os.WriteFile(path, []byte("A: @input a number = 1\nResult: {{ a }}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	handler, err := newLiveDocumentHandler(path, "127.0.0.1:8080", nil)
	if err != nil {
		t.Fatal(err)
	}

	initial := httptest.NewRecorder()
	handler.ServeHTTP(initial, runtimeTestRequest(http.MethodGet, "/", nil))
	if initial.Code != http.StatusOK || !strings.Contains(initial.Body.String(), `value="1"`) {
		t.Fatalf("initial status=%d body=%s", initial.Code, initial.Body.String())
	}

	if err := os.WriteFile(path, []byte("@calc broken = 1 + * 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	broken := readSourceStatus(t, handler)
	if broken.Error == "" || !strings.Contains(broken.Error, "invalid @calc expression") {
		t.Fatalf("broken status=%#v", broken)
	}
	if broken.Revision == "" {
		t.Fatal("last valid revision must remain available during a source error")
	}

	if err := os.WriteFile(path, []byte("A: @input a number = 9\nResult: {{ a }}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	valid := readSourceStatus(t, handler)
	if valid.Error != "" || valid.Revision == broken.Revision {
		t.Fatalf("valid status=%#v broken=%#v", valid, broken)
	}

	refreshed := httptest.NewRecorder()
	handler.ServeHTTP(refreshed, runtimeTestRequest(http.MethodGet, "/", nil))
	if refreshed.Code != http.StatusOK || !strings.Contains(refreshed.Body.String(), `value="9"`) || !strings.Contains(refreshed.Body.String(), valid.Revision) {
		t.Fatalf("refreshed status=%d body=%s", refreshed.Code, refreshed.Body.String())
	}
}

func TestLiveSourceStatusUsesRuntimeRequestBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.md")
	if err := os.WriteFile(path, []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	handler, err := newLiveDocumentHandler(path, "127.0.0.1:8080", nil)
	if err != nil {
		t.Fatal(err)
	}
	request := runtimeTestRequest(http.MethodGet, "/source-status", nil)
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin source status=%d, want 403", response.Code)
	}
}

func readSourceStatus(t *testing.T, handler http.Handler) sourceStatusResponse {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, runtimeTestRequest(http.MethodGet, "/source-status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("source status=%d body=%s", response.Code, response.Body.String())
	}
	var status sourceStatusResponse
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	return status
}
