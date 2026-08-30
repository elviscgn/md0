package md0

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

const maxEditorBodyBytes = 3 << 20

const editorCSS = `
body.md0-editing{padding-left:min(46vw,700px)}
.md0-editor-pane{position:fixed;inset:0 auto 0 0;z-index:45;display:flex;flex-direction:column;width:min(46vw,700px);border-right:1px solid var(--line);background:var(--field);color:var(--ink);font-family:var(--md0-font-sans)}
.md0-editor-head{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:11px 14px;border-bottom:1px solid var(--line);background:var(--paper)}
.md0-editor-title{min-width:0}.md0-editor-title strong{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px}.md0-editor-title span{display:block;margin-top:2px;color:var(--muted);font-size:10px}
.md0-editor-actions{display:flex;align-items:center;gap:7px}.md0-editor-save{border:1px solid var(--line);border-radius:8px;padding:7px 10px;background:var(--surface);color:var(--ink);font:700 11px/1 var(--md0-font-sans);cursor:pointer}.md0-editor-save:hover{background:var(--paper)}.md0-editor-save:focus-visible{outline:2px solid var(--focus);outline-offset:2px}
.md0-editor-state{min-width:44px;color:var(--muted);font:600 10px/1 var(--md0-font-sans);text-align:right}.md0-editor-state.error{color:var(--red)}.md0-editor-state.ok{color:var(--green)}
.md0-editor-source{box-sizing:border-box;display:block;flex:1;width:100%;min-height:0;resize:none;border:0;outline:0;padding:18px 20px 40px;background:var(--field);color:var(--ink);font:13px/1.65 var(--md0-font-mono);tab-size:4;white-space:pre;overflow:auto}
.md0-editor-diagnostic{max-height:150px;overflow:auto;padding:10px 14px;border-top:1px solid var(--line);background:var(--paper);color:var(--red);font:11px/1.45 var(--md0-font-mono);white-space:pre-wrap}.md0-editor-diagnostic[hidden]{display:none}
body.md0-editing #md0-document{max-width:var(--page-width);margin-inline:auto}
@media(max-width:900px){body.md0-editing{padding-left:0}.md0-editor-pane{position:relative;width:100%;height:46vh;border-right:0;border-bottom:1px solid var(--line)}.md0-editor-source{min-height:260px}}
`

const editorJS = `
const md0EditorToken=document.querySelector('meta[name="md0-editor-token"]')?.content||'';
const md0Editor=document.getElementById('md0-editor-source');
const md0EditorState=document.getElementById('md0-editor-state');
const md0EditorDiagnostic=document.getElementById('md0-editor-diagnostic');
const md0EditorSave=document.getElementById('md0-editor-save');
let md0EditorTimer;
let md0EditorBusy=false;
let md0EditorQueued=false;
function md0EditorSetState(text,kind=''){md0EditorState.textContent=text;md0EditorState.className='md0-editor-state '+kind}
function md0EditorSetDiagnostic(message){md0EditorDiagnostic.textContent=message||'';md0EditorDiagnostic.hidden=!message}
function md0EditorRequest(){clearTimeout(md0EditorTimer);md0EditorSetState('editing');md0EditorTimer=setTimeout(md0EditorRenderDraft,180)}
async function md0EditorRenderDraft(){md0EditorQueued=true;if(md0EditorBusy)return;md0EditorBusy=true;try{while(md0EditorQueued){md0EditorQueued=false;let response;try{response=await fetch('/editor/draft',{method:'POST',headers:{'content-type':'application/json','x-md0-editor-token':md0EditorToken},body:JSON.stringify({source:md0Editor.value,values:md0InputValues()})})}catch(err){md0EditorSetState('offline','error');md0EditorSetDiagnostic('md0 editor unavailable: '+err.message);continue}let payload;try{payload=await response.json()}catch{payload={error:'md0 editor returned an invalid response'}}if(!response.ok){md0EditorSetState('invalid','error');md0EditorSetDiagnostic(payload.error||'draft is invalid');continue}const root=document.getElementById('md0-document');root.innerHTML=payload.fragment;md0EnhanceInputs(root);md0EditorSetDiagnostic('');md0EditorSetState('live','ok')}}finally{md0EditorBusy=false}}
async function md0EditorCommit(){md0EditorSetState('saving');let response;try{response=await fetch('/editor/source',{method:'POST',headers:{'content-type':'text/plain; charset=utf-8','x-md0-editor-token':md0EditorToken},body:md0Editor.value})}catch(err){md0EditorSetState('error','error');md0EditorSetDiagnostic('save failed: '+err.message);return}let payload;try{payload=await response.json()}catch{payload={error:'invalid save response'}}if(!response.ok){md0EditorSetState('error','error');md0EditorSetDiagnostic(payload.error||'save failed');return}try{sessionStorage.setItem('md0:editor-selection',JSON.stringify({start:md0Editor.selectionStart,end:md0Editor.selectionEnd,scroll:md0Editor.scrollTop}))}catch{}md0EditorSetState('saved','ok');setTimeout(()=>location.reload(),90)}
md0Editor.addEventListener('input',md0EditorRequest);
md0Editor.addEventListener('keydown',event=>{if((event.metaKey||event.ctrlKey)&&event.key.toLowerCase()==='s'){event.preventDefault();md0EditorCommit()}});
md0EditorSave.addEventListener('click',md0EditorCommit);
try{const saved=JSON.parse(sessionStorage.getItem('md0:editor-selection')||'null');sessionStorage.removeItem('md0:editor-selection');if(saved){md0Editor.selectionStart=saved.start||0;md0Editor.selectionEnd=saved.end||saved.start||0;md0Editor.scrollTop=saved.scroll||0;md0Editor.focus()}}catch{}
md0SendLatest=md0EditorRenderDraft;
`

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
	path      string
	port      string
	dataSpecs []string
	token     string
	live      http.Handler
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
	return &editorHandler{path: path, port: port, dataSpecs: append([]string(nil), dataSpecs...), token: token, live: live}, nil
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
		_, _ = w.Write(recorder.body.Bytes())
		return
	}
	source, err := os.ReadFile(h.path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	page := recorder.body.String()
	page = strings.Replace(page, `</head>`, `<meta name="md0-editor-token" content="`+html.EscapeString(h.token)+`"><style>`+editorCSS+`</style></head>`, 1)
	panel := `<aside class="md0-editor-pane" aria-label="md0 source editor"><div class="md0-editor-head"><div class="md0-editor-title"><strong>` + html.EscapeString(h.path) + `</strong><span>live draft · Cmd/Ctrl+S saves</span></div><div class="md0-editor-actions"><span id="md0-editor-state" class="md0-editor-state">ready</span><button id="md0-editor-save" class="md0-editor-save" type="button">Save</button></div></div><textarea id="md0-editor-source" class="md0-editor-source" spellcheck="false" aria-label="Document source">` + html.EscapeString(string(source)) + `</textarea><div id="md0-editor-diagnostic" class="md0-editor-diagnostic" role="status" aria-live="polite" hidden></div></aside>`
	page = strings.Replace(page, `<body>`, `<body class="md0-editing">`+panel, 1)
	page = strings.Replace(page, `</body>`, `<script>`+editorJS+`</script></body>`, 1)
	csp := w.Header().Get("Content-Security-Policy")
	csp = strings.Replace(csp, "; script-src ", " "+cspHash(editorCSS)+"; script-src ", 1)
	csp = strings.Replace(csp, "; connect-src ", " "+cspHash(editorJS)+"; connect-src ", 1)
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("Content-Length", fmt.Sprint(len(page)))
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, page)
}

func (h *editorHandler) serveDraft(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, maxEditorBodyBytes)
	decoder := json.NewDecoder(r.Body)
	var request editorDraftRequest
	if err := decoder.Decode(&request); err != nil {
		writeEditorJSON(w, http.StatusBadRequest, editorDraftResponse{Error: "invalid editor draft payload"})
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
	result, err := Evaluate(doc, request.Values)
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
	header http.Header
	body   strings.Builder
	status int
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{header: make(http.Header), status: http.StatusOK}
}

func (r *responseRecorder) Header() http.Header            { return r.header }
func (r *responseRecorder) WriteHeader(status int)         { r.status = status }
func (r *responseRecorder) Write(data []byte) (int, error) { return r.body.Write(data) }

func ServeFileEditorWithOptions(path, addr string, initialValues map[string]string, dataSpecs []string) error {
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
