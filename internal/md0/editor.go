package md0

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

const maxEditorBodyBytes = 3 << 20

type editorDraftRequest struct {
	Source string            `json:"source"`
	Values map[string]string `json:"values"`
}

type editorDraftResponse struct {
	Fragment string `json:"fragment,omitempty"`
	Error    string `json:"error,omitempty"`
}

type editorSaveResponse struct {
	Revision string `json:"revision"`
}

type editorHandler struct {
	path          string
	port          string
	initialValues map[string]string
	dataSpecs     []string
	token         string
	live          http.Handler
}

func newEditorHandler(path, addr string, initialValues map[string]string, dataSpecs []string) (http.Handler, error) {
	port, err := validateLoopbackAddress(addr)
	if err != nil {
		return nil, err
	}
	live, err := newLiveDocumentHandlerWithData(path, addr, initialValues, dataSpecs)
	if err != nil {
		return nil, err
	}
	token, err := newRuntimeToken()
	if err != nil {
		return nil, err
	}
	return &editorHandler{
		path:          path,
		port:          port,
		initialValues: copyStringMap(initialValues),
		dataSpecs:     append([]string(nil), dataSpecs...),
		token:         token,
		live:          live,
	}, nil
}

func (h *editorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	if r.URL.Path == "/" && r.Method == http.MethodGet {
		h.serveEditorPage(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/editor/") {
		setRuntimeSecurityHeaders(w)
		if r.Header.Get("X-MD0-Editor-Token") != h.token {
			writePatchError(w, http.StatusForbidden, "invalid editor token", "")
			return
		}
		switch {
		case r.URL.Path == "/editor/draft" && r.Method == http.MethodPost:
			h.serveDraft(w, r)
		case r.URL.Path == "/editor/source" && r.Method == http.MethodPost:
			h.serveSave(w, r)
		default:
			http.NotFound(w, r)
		}
		return
	}
	h.live.ServeHTTP(w, r)
}

func (h *editorHandler) serveEditorPage(w http.ResponseWriter, r *http.Request) {
	recorder := newResponseRecorder()
	h.live.ServeHTTP(recorder, r)
	for key, values := range recorder.header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if recorder.status >= 400 {
		w.WriteHeader(recorder.status)
		_, _ = io.WriteString(w, recorder.body.String())
		return
	}
	source, err := os.ReadFile(h.path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	page := recorder.body.String()
	editorID := sourceSHA256(h.path)[:16]
	meta := `<meta name="md0-editor-token" content="` + html.EscapeString(h.token) + `"><meta name="md0-editor-id" content="` + editorID + `">`
	page = strings.Replace(page, `</head>`, meta+`<style>`+editorCSS+`</style></head>`, 1)
	panel := `<aside id="md0-editor-pane" class="md0-editor-pane" aria-label="md0 source editor" hidden>` +
		`<div class="md0-editor-head"><div class="md0-editor-title"><strong>` + html.EscapeString(h.path) + `</strong><span>live draft · explicit save</span></div><div class="md0-editor-actions"><span id="md0-editor-state" class="md0-editor-state">ready</span><button id="md0-editor-save" class="md0-editor-save" type="button">Save</button></div></div>` +
		`<div id="md0-editor-code" class="md0-editor-code"><div id="md0-editor-current-line" class="md0-editor-current-line" aria-hidden="true"></div><div class="md0-editor-gutter" aria-hidden="true"><div id="md0-editor-gutter-lines" class="md0-editor-gutter-lines"></div></div><pre id="md0-editor-highlight" class="md0-editor-highlight" aria-hidden="true"><code id="md0-editor-highlight-code"></code></pre><textarea id="md0-editor-source" class="md0-editor-source" wrap="off" spellcheck="false" autocomplete="off" autocorrect="off" autocapitalize="off" aria-label="Document source" aria-autocomplete="list" aria-controls="md0-editor-completions" aria-expanded="false">` + html.EscapeString(string(source)) + `</textarea><div id="md0-editor-completions" class="md0-editor-completions" role="listbox" aria-label="md0 syntax suggestions" hidden></div><span id="md0-editor-measure" class="md0-editor-measure" aria-hidden="true"></span></div>` +
		`<div class="md0-editor-foot"><span><span class="md0-editor-language">md0/PURE</span> · <kbd>Ctrl Space</kbd> complete · <kbd>Tab</kbd> insert</span><span id="md0-editor-position" class="md0-editor-position">Ln 1, Col 1</span></div><div id="md0-editor-diagnostic" class="md0-editor-diagnostic" role="status" aria-live="polite" hidden></div></aside>`
	page = strings.Replace(page, `<body>`, `<body>`+panel, 1)
	page = strings.Replace(page, `</body>`, `<script>`+editorJS+`</script></body>`, 1)
	csp := w.Header().Get("Content-Security-Policy")
	csp = strings.Replace(csp, "style-src ", "style-src "+cspHash(editorCSS)+" ", 1)
	csp = strings.Replace(csp, "script-src ", "script-src "+cspHash(editorJS)+" ", 1)
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("Content-Length", fmt.Sprint(len(page)))
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, page)
}

func (h *editorHandler) serveDraft(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeEditorJSON(w, http.StatusUnsupportedMediaType, editorDraftResponse{Error: "content type must be application/json"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxEditorBodyBytes)
	decoder := json.NewDecoder(r.Body)
	var request editorDraftRequest
	if err := decoder.Decode(&request); err != nil {
		writeEditorJSON(w, http.StatusBadRequest, editorDraftResponse{Error: "invalid editor draft payload"})
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeEditorJSON(w, http.StatusBadRequest, editorDraftResponse{Error: "editor draft payload must contain exactly one JSON object"})
		return
	}
	if len(request.Source) > 2*1024*1024 {
		writeEditorJSON(w, http.StatusRequestEntityTooLarge, editorDraftResponse{Error: "document exceeds 2 MiB limit"})
		return
	}
	if !utf8.ValidString(request.Source) {
		writeEditorJSON(w, http.StatusBadRequest, editorDraftResponse{Error: "document must be valid UTF-8"})
		return
	}
	doc, err := ParseString(h.path, request.Source)
	if err != nil {
		writeEditorJSON(w, http.StatusBadRequest, editorDraftResponse{Error: FormatDiagnostic(h.path, err)})
		return
	}
	if err := BindDataFiles(doc, h.dataSpecs); err != nil {
		writeEditorJSON(w, http.StatusBadRequest, editorDraftResponse{Error: FormatDiagnostic(h.path, err)})
		return
	}
	values := copyStringMap(h.initialValues)
	for key, value := range request.Values {
		values[key] = value
	}
	result, err := Evaluate(doc, values)
	if err != nil {
		writeEditorJSON(w, http.StatusBadRequest, editorDraftResponse{Error: FormatDiagnostic(h.path, err)})
		return
	}
	fragment, err := RenderFragmentBounded(doc, result)
	if err != nil {
		writeEditorJSON(w, http.StatusBadRequest, editorDraftResponse{Error: FormatDiagnostic(h.path, err)})
		return
	}
	writeEditorJSON(w, http.StatusOK, editorDraftResponse{Fragment: fragment})
}

func (h *editorHandler) serveSave(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "text/plain" {
		writePatchError(w, http.StatusUnsupportedMediaType, "content type must be text/plain", "")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2*1024*1024+1)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writePatchError(w, http.StatusBadRequest, "could not read editor source", "")
		return
	}
	if len(data) > 2*1024*1024 {
		writePatchError(w, http.StatusRequestEntityTooLarge, "document exceeds 2 MiB limit", "")
		return
	}
	if !utf8.Valid(data) {
		writePatchError(w, http.StatusBadRequest, "document must be valid UTF-8", "")
		return
	}
	if expected := r.Header.Get("X-MD0-Source-Revision"); expected != "" {
		current, err := sourceFileRevision(h.path)
		if err != nil {
			writePatchError(w, http.StatusInternalServerError, err.Error(), "")
			return
		}
		if current != expected {
			writePatchError(w, http.StatusConflict, "source changed on disk; reload or copy your draft before saving", "")
			return
		}
	}
	info, err := os.Stat(h.path)
	if err != nil {
		writePatchError(w, http.StatusInternalServerError, err.Error(), "")
		return
	}
	if err := os.WriteFile(h.path, data, info.Mode().Perm()); err != nil {
		writePatchError(w, http.StatusInternalServerError, err.Error(), "")
		return
	}
	writeEditorJSON(w, http.StatusOK, editorSaveResponse{Revision: sourceSHA256(string(data))})
}

func writeEditorJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// responseRecorder is deliberately tiny: it captures the standard live viewer
// response so authoring mode can add its source pane without duplicating the
// viewer's runtime/session implementation.
type responseRecorder struct {
	header      http.Header
	body        strings.Builder
	status      int
	wroteHeader bool
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{header: make(http.Header), status: http.StatusOK}
}

func (r *responseRecorder) Header() http.Header { return r.header }

func (r *responseRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.body.Write(data)
}

func ServeFileWorkspaceWithOptions(path, addr string, initialValues map[string]string, dataSpecs []string) error {
	handler, err := newEditorHandler(path, addr, initialValues, dataSpecs)
	if err != nil {
		return err
	}
	return serveRuntimeWithoutBanner(addr, handler)
}

func ServeFileWithOptionsWithoutBanner(path, addr string, initialValues map[string]string, dataSpecs []string) error {
	handler, err := newLiveDocumentHandlerWithData(path, addr, initialValues, dataSpecs)
	if err != nil {
		return err
	}
	return serveRuntimeWithoutBanner(addr, handler)
}

func serveRuntimeWithoutBanner(addr string, handler http.Handler) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return server.ListenAndServe()
}
