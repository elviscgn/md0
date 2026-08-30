package md0

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
)

type sourceStatusResponse struct {
	Revision string `json:"revision"`
	Error    string `json:"error,omitempty"`
}

type liveDocumentHandler struct {
	mu            sync.RWMutex
	path          string
	addr          string
	port          string
	initialValues map[string]string
	dataSpecs     []string
	handler       http.Handler
	seenRevision  string
	revision      string
	problem       string
}

func newLiveDocumentHandler(path, addr string, initialValues map[string]string) (*liveDocumentHandler, error) {
	return newLiveDocumentHandlerWithData(path, addr, initialValues, nil)
}

func newLiveDocumentHandlerWithData(path, addr string, initialValues map[string]string, dataSpecs []string) (*liveDocumentHandler, error) {
	port, err := validateLoopbackAddress(addr)
	if err != nil {
		return nil, err
	}
	doc, err := ParseFile(path)
	if err != nil {
		return nil, err
	}
	if err := BindDataFiles(doc, dataSpecs); err != nil {
		return nil, err
	}
	handler, err := newHandlerForAddrWithValues(doc, addr, initialValues)
	if err != nil {
		return nil, err
	}
	revision := sourceSHA256(doc.Source)
	return &liveDocumentHandler{
		path:          path,
		addr:          addr,
		port:          port,
		initialValues: copyStringMap(initialValues),
		dataSpecs:     append([]string(nil), dataSpecs...),
		handler:       handler,
		seenRevision:  revision,
		revision:      revision,
	}, nil
}

func (h *liveDocumentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/source-status" {
		h.serveSourceStatus(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/" {
		h.refresh()
	}
	h.mu.RLock()
	handler := h.handler
	h.mu.RUnlock()
	handler.ServeHTTP(w, r)
}

func (h *liveDocumentHandler) serveSourceStatus(w http.ResponseWriter, r *http.Request) {
	setRuntimeSecurityHeaders(w)
	if !validRuntimeHost(r.Host, h.port) {
		http.Error(w, "invalid runtime host", http.StatusForbidden)
		return
	}
	if !validRuntimeOrigin(r.Header.Get("Origin"), h.port) {
		http.Error(w, "cross-origin runtime request rejected", http.StatusForbidden)
		return
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		http.Error(w, "cross-site runtime request rejected", http.StatusForbidden)
		return
	}
	h.refresh()
	h.mu.RLock()
	status := sourceStatusResponse{Revision: h.revision, Error: h.problem}
	h.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(status)
}

func (h *liveDocumentHandler) refresh() {
	revision, probeErr := sourceFileRevision(h.path)
	h.mu.RLock()
	unchanged := revision == h.seenRevision
	h.mu.RUnlock()
	if unchanged {
		return
	}

	if probeErr != nil {
		h.mu.Lock()
		h.seenRevision = revision
		h.problem = probeErr.Error()
		h.mu.Unlock()
		return
	}
	doc, err := ParseFile(h.path)
	if err != nil {
		h.mu.Lock()
		h.seenRevision = revision
		h.problem = FormatDiagnostic(h.path, err)
		h.mu.Unlock()
		return
	}
	if err := BindDataFiles(doc, h.dataSpecs); err != nil {
		h.mu.Lock()
		h.seenRevision = revision
		h.problem = FormatDiagnostic(h.path, err)
		h.mu.Unlock()
		return
	}
	handler, err := newHandlerForAddrWithValues(doc, h.addr, h.initialValues)
	if err != nil {
		h.mu.Lock()
		h.seenRevision = revision
		h.problem = err.Error()
		h.mu.Unlock()
		return
	}

	h.mu.Lock()
	h.handler = handler
	h.seenRevision = revision
	h.revision = revision
	h.problem = ""
	h.mu.Unlock()
}

func sourceFileRevision(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "error:" + err.Error(), err
	}
	if info.Size() > 2*1024*1024 {
		err := fmt.Errorf("%s: document exceeds 2 MiB limit", path)
		return fmt.Sprintf("oversized:%d:%d", info.Size(), info.ModTime().UnixNano()), err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "error:" + err.Error(), err
	}
	return sourceSHA256(string(data)), nil
}
