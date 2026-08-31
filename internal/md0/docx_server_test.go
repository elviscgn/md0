package md0

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDOCXExportRequiresCapabilityAndEmptyBody(t *testing.T) {
	handler := testRuntimeHandler(t)

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, runtimeTestRequest(http.MethodPost, "/docx", nil))
	if missing.Code != http.StatusForbidden {
		t.Fatalf("POST /docx without token status=%d, want 403", missing.Code)
	}

	token := runtimeTokenForHandler(t, handler)
	withBody := runtimeTestRequest(http.MethodPost, "/docx", strings.NewReader("unexpected"))
	withBody.Header.Set("X-MD0-Token", token)
	bodyRes := httptest.NewRecorder()
	handler.ServeHTTP(bodyRes, withBody)
	if bodyRes.Code != http.StatusBadRequest {
		t.Fatalf("POST /docx with body status=%d, want 400", bodyRes.Code)
	}
}

func TestDOCXExportReturnsOfficeDocument(t *testing.T) {
	handler := testRuntimeHandler(t)
	token := runtimeTokenForHandler(t, handler)
	request := runtimeTestRequest(http.MethodPost, "/docx", nil)
	request.Header.Set("X-MD0-Token", token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST /docx status=%d body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Fatalf("Content-Type=%q", got)
	}
	if !strings.Contains(response.Header().Get("Content-Disposition"), ".docx") {
		t.Fatalf("Content-Disposition=%q", response.Header().Get("Content-Disposition"))
	}
	data := response.Body.Bytes()
	if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("response is not a valid DOCX zip: %v", err)
	}
}
