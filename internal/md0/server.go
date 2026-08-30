package md0

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func Serve(doc *Document, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		res, err := Evaluate(doc, nil)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		frag, err := RenderFragment(doc, res)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("content-type", "text/html; charset=utf-8")
		io.WriteString(w, RenderInteractivePage(doc.Path, frag))
	})
	mux.HandleFunc("POST /render", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var values map[string]string
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&values); err != nil {
			http.Error(w, "invalid input payload", 400)
			return
		}
		res, err := Evaluate(doc, values)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		frag, err := RenderFragment(doc, res)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.Header().Set("content-type", "text/html; charset=utf-8")
		io.WriteString(w, frag)
	})
	fmt.Printf("md0 serving %s at http://%s\n", doc.Path, addr)
	return http.ListenAndServe(addr, mux)
}
