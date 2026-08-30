package md0

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxRenderBodyBytes = 1 << 20
	maxRuntimeSessions = 32
	runtimeTokenBytes  = 32
)

type patchResponse struct {
	Changed    []string   `json:"changed"`
	Recomputed []string   `json:"recomputed"`
	Patched    int        `json:"patched"`
	Patches    []DOMPatch `json:"patches"`
}

type patchErrorResponse struct {
	Error string `json:"error"`
	Input string `json:"input,omitempty"`
}

type runtimeSessionStore struct {
	mu            sync.Mutex
	doc           *Document
	initialValues map[string]string
	sessions      map[string]*ReactiveSession
	order         []string
}

var inputErrorNameRE = regexp.MustCompile(`\binput ([A-Za-z_][A-Za-z0-9_]*):`)

func cspHash(source string) string {
	sum := sha256.Sum256([]byte(source))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

func runtimeContentSecurityPolicy() string {
	return "default-src 'none'; style-src " +
		cspHash(pageCSS) + " " + cspHash(runtimeStatusCSS) +
		"; script-src " + cspHash(pageJS) + " " + cspHash(runtimeStatusJS) +
		"; connect-src 'self'; img-src data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
}

func setRuntimeSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", runtimeContentSecurityPolicy())
}

func writePatchError(w http.ResponseWriter, status int, message, input string) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(patchErrorResponse{Error: message, Input: input})
}

func inputNameFromError(err error) string {
	if err == nil {
		return ""
	}
	match := inputErrorNameRE.FindStringSubmatch(err.Error())
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func newRuntimeToken() (string, error) {
	buf := make([]byte, runtimeTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate runtime token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func newRuntimeSessionStore(doc *Document) *runtimeSessionStore {
	return newRuntimeSessionStoreWithValues(doc, nil)

}

func newRuntimeSessionStoreWithValues(doc *Document, values map[string]string) *runtimeSessionStore {
	return &runtimeSessionStore{doc: doc, initialValues: copyStringMap(values), sessions: map[string]*ReactiveSession{}}
}

func (s *runtimeSessionStore) create() (string, *ReactiveSession, error) {
	session, err := NewReactiveSessionWithValues(s.doc, s.initialValues)
	if err != nil {
		return "", nil, err
	}

	for {
		token, err := newRuntimeToken()
		if err != nil {
			return "", nil, err
		}
		s.mu.Lock()
		if _, exists := s.sessions[token]; exists {
			s.mu.Unlock()
			continue
		}
		for len(s.sessions) >= maxRuntimeSessions && len(s.order) > 0 {
			oldest := s.order[0]
			s.order = s.order[1:]
			delete(s.sessions, oldest)
		}
		s.sessions[token] = session
		s.order = append(s.order, token)
		s.mu.Unlock()
		return token, session, nil
	}
}

func (s *runtimeSessionStore) get(token string) (*ReactiveSession, bool) {
	if token == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[token]
	if !ok {
		return nil, false
	}
	for i, current := range s.order {
		if current == token {
			copy(s.order[i:], s.order[i+1:])
			s.order[len(s.order)-1] = token
			break
		}
	}
	return session, true
}

func loopbackHost(host string) bool {
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateLoopbackAddress(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid listen address %q: %w", addr, err)
	}
	if host == "" || !loopbackHost(host) {
		return "", fmt.Errorf("md0 open only listens on loopback addresses; got %q", addr)
	}
	if port == "" {
		return "", fmt.Errorf("listen address %q has no port", addr)
	}
	return port, nil
}

func splitRequestHost(hostport string) (string, string, bool) {
	host, port, err := net.SplitHostPort(hostport)
	if err == nil {
		return host, port, true
	}
	if !strings.Contains(hostport, ":") {
		return hostport, "", true
	}
	return "", "", false
}

func validRuntimeHost(hostport, expectedPort string) bool {
	host, port, ok := splitRequestHost(hostport)
	if !ok || !loopbackHost(host) {
		return false
	}
	if port == "" {
		return expectedPort == "80"
	}
	return port == expectedPort
}

func validRuntimeOrigin(origin, expectedPort string) bool {
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if !loopbackHost(u.Hostname()) {
		return false
	}
	port := u.Port()
	if port == "" {
		port = "80"
	}
	return port == expectedPort
}

func decodeRenderInputs(w http.ResponseWriter, r *http.Request) (map[string]string, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return nil, fmt.Errorf("content type must be application/json")
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRenderBodyBytes)
	decoder := json.NewDecoder(r.Body)
	var values map[string]string
	if err := decoder.Decode(&values); err != nil {
		return nil, fmt.Errorf("invalid input payload")
	}
	if values == nil {
		return nil, fmt.Errorf("input payload must be a JSON object")
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("input payload must contain exactly one JSON object")
	}
	return values, nil
}

func requireEmptyRequestBody(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1)
	read, err := io.Copy(io.Discard, r.Body)
	if err != nil || read != 0 {
		return fmt.Errorf("request body must be empty")
	}
	return nil
}

func newHandler(doc *Document) (http.Handler, error) {
	return newHandlerForAddr(doc, "127.0.0.1:8080")
}

func newHandlerForAddr(doc *Document, addr string) (http.Handler, error) {
	return newHandlerForAddrWithValues(doc, addr, nil)
}

func newHandlerForAddrWithValues(doc *Document, addr string, initialValues map[string]string) (http.Handler, error) {
	port, err := validateLoopbackAddress(addr)
	if err != nil {
		return nil, err
	}
	store := newRuntimeSessionStoreWithValues(doc, initialValues)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		token, session, err := store.create()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		frag, err := RenderFragmentBounded(doc, session.Snapshot())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("content-type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, renderInteractiveRuntimePageRevision(doc.Path, frag, token, sourceSHA256(doc.Source)))
	})
	mux.HandleFunc("POST /render", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		session, ok := store.get(r.Header.Get("X-MD0-Token"))
		if !ok {
			writePatchError(w, http.StatusForbidden, "invalid or expired runtime token", "")
			return
		}
		values, err := decodeRenderInputs(w, r)
		if err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "content type") {
				status = http.StatusUnsupportedMediaType
			}
			writePatchError(w, status, err.Error(), "")
			return
		}
		res, stats, err := session.Update(values)
		if err != nil {
			writePatchError(w, http.StatusBadRequest, err.Error(), inputNameFromError(err))
			return
		}
		patches, err := RenderPatchesBounded(doc, res, stats)
		if err != nil {
			writePatchError(w, http.StatusBadRequest, err.Error(), "")
			return
		}
		w.Header().Set("content-type", "application/json; charset=utf-8")
		w.Header().Set("X-MD0-Changed", strings.Join(stats.Changed, ","))
		w.Header().Set("X-MD0-Recomputed", strconv.Itoa(len(stats.Recomputed)))
		w.Header().Set("X-MD0-Patched", strconv.Itoa(len(patches)))
		if err := json.NewEncoder(w).Encode(patchResponse{
			Changed:    stats.Changed,
			Recomputed: stats.Recomputed,
			Patched:    len(patches),
			Patches:    patches,
		}); err != nil {
			return
		}
	})
	mux.HandleFunc("POST /snapshot", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		session, ok := store.get(r.Header.Get("X-MD0-Token"))
		if !ok {
			writePatchError(w, http.StatusForbidden, "invalid or expired runtime token", "")
			return
		}
		if err := requireEmptyRequestBody(w, r); err != nil {
			writePatchError(w, http.StatusBadRequest, err.Error(), "")
			return
		}
		data, err := MarshalSnapshot(doc, session.Snapshot())
		if err != nil {
			writePatchError(w, http.StatusInternalServerError, err.Error(), "")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, snapshotFilename(doc.Path)))
		_, _ = w.Write(data)
	})
	mux.HandleFunc("POST /markdown", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		session, ok := store.get(r.Header.Get("X-MD0-Token"))
		if !ok {
			writePatchError(w, http.StatusForbidden, "invalid or expired runtime token", "")
			return
		}
		if err := requireEmptyRequestBody(w, r); err != nil {
			writePatchError(w, http.StatusBadRequest, err.Error(), "")
			return
		}
		source, err := UpdatedMarkdown(doc, session.Snapshot())
		if err != nil {
			writePatchError(w, http.StatusInternalServerError, err.Error(), "")
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filepath.Base(doc.Path)))
		_, _ = io.WriteString(w, source)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setRuntimeSecurityHeaders(w)
		if !validRuntimeHost(r.Host, port) {
			http.Error(w, "invalid runtime host", http.StatusForbidden)
			return
		}
		if !validRuntimeOrigin(r.Header.Get("Origin"), port) {
			http.Error(w, "cross-origin runtime request rejected", http.StatusForbidden)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			http.Error(w, "cross-site runtime request rejected", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	}), nil
}

func Serve(doc *Document, addr string) error {
	return ServeWithValues(doc, addr, nil)
}

func ServeWithValues(doc *Document, addr string, initialValues map[string]string) error {
	if _, err := validateLoopbackAddress(addr); err != nil {
		return err
	}
	handler, err := newHandlerForAddrWithValues(doc, addr, initialValues)
	if err != nil {
		return err
	}
	return serveRuntime(doc.Path, addr, handler)
}

func ServeFileWithValues(path, addr string, initialValues map[string]string) error {
	return ServeFileWithOptions(path, addr, initialValues, nil)
}

func ServeFileWithOptions(path, addr string, initialValues map[string]string, dataSpecs []string) error {
	handler, err := newLiveDocumentHandlerWithData(path, addr, initialValues, dataSpecs)
	if err != nil {
		return err
	}
	return serveRuntime(path, addr, handler)
}

func serveRuntime(path, addr string, handler http.Handler) error {
	fmt.Printf("md0 serving %s at http://%s\n", path, addr)
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

func snapshotFilename(path string) string {
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	name = strings.TrimSuffix(name, ext)
	if name == "" || name == "." {
		name = "document"
	}
	return name + ".snapshot.json"
}
