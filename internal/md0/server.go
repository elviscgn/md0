package md0

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const maxRenderBodyBytes = 1 << 20

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

func newHandler(doc *Document) (http.Handler, error) {
	session, err := NewReactiveSession(doc)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		res, err := session.Reset()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		frag, err := RenderFragment(doc, res)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("content-type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, renderInteractiveRuntimePage(doc.Path, frag))
	})
	mux.HandleFunc("POST /render", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
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
		patches, err := RenderPatches(doc, res, stats)
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setRuntimeSecurityHeaders(w)
		mux.ServeHTTP(w, r)
	}), nil
}

func Serve(doc *Document, addr string) error {
	handler, err := newHandler(doc)
	if err != nil {
		return err
	}
	fmt.Printf("md0 serving %s at http://%s\n", doc.Path, addr)
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
