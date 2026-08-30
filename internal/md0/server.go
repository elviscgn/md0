package md0

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
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

var inputErrorNameRE = regexp.MustCompile(`\binput ([A-Za-z_][A-Za-z0-9_]*):`)

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

func newHandler(doc *Document) (http.Handler, error) {
	session, err := NewReactiveSession(doc)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
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
		var values map[string]string
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if err := decoder.Decode(&values); err != nil {
			writePatchError(w, http.StatusBadRequest, "invalid input payload", "")
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
	return mux, nil
}

func Serve(doc *Document, addr string) error {
	handler, err := newHandler(doc)
	if err != nil {
		return err
	}
	fmt.Printf("md0 serving %s at http://%s\n", doc.Path, addr)
	return http.ListenAndServe(addr, handler)
}
