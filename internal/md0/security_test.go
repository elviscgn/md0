package md0

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityMarkdownAndValuesCannotInjectRawHTML(t *testing.T) {
	src := `# <img src=x onerror=alert(1)>
Payload: @input payload string = "<script>alert(1)</script>"
Result: {{ payload }}

@table hostile
columns = ["<img src=x onerror=alert(2)>"]
rows = [["<script>alert(3)</script>"]]
@end`
	doc, err := ParseString("hostile.md", src)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := RenderFragment(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		`<img src=x onerror=alert(1)>`,
		`<script>alert(1)</script>`,
		`<img src=x onerror=alert(2)>`,
		`<script>alert(3)</script>`,
	} {
		if strings.Contains(fragment, forbidden) {
			t.Fatalf("raw hostile HTML reached output: %q in %s", forbidden, fragment)
		}
	}
	for _, escaped := range []string{
		`&lt;img src=x onerror=alert(1)&gt;`,
		`&lt;script&gt;alert(1)&lt;/script&gt;`,
		`&lt;img src=x onerror=alert(2)&gt;`,
		`&lt;script&gt;alert(3)&lt;/script&gt;`,
	} {
		if !strings.Contains(fragment, escaped) {
			t.Fatalf("escaped hostile value missing: %q in %s", escaped, fragment)
		}
	}
}

func testRuntimeHandler(t *testing.T) http.Handler {
	t.Helper()
	doc, err := ParseString("server-security.md", `A: @input a number = 1
@calc doubled = a * 2
Result: {{ doubled }}`)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newHandler(doc)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func requireHeaderContains(t *testing.T, header http.Header, name, want string) {
	t.Helper()
	if got := header.Get(name); !strings.Contains(got, want) {
		t.Fatalf("%s=%q, want to contain %q", name, got, want)
	}
}

func TestSecurityRuntimeHeadersAndRouteBoundary(t *testing.T) {
	handler := testRuntimeHandler(t)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("GET / status=%d body=%s", res.Code, res.Body.String())
	}
	requireHeaderContains(t, res.Header(), "Cache-Control", "no-store")
	requireHeaderContains(t, res.Header(), "X-Content-Type-Options", "nosniff")
	requireHeaderContains(t, res.Header(), "Referrer-Policy", "no-referrer")
	requireHeaderContains(t, res.Header(), "Cross-Origin-Resource-Policy", "same-origin")
	requireHeaderContains(t, res.Header(), "Content-Security-Policy", "default-src 'none'")

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/not-a-route", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("GET unknown route status=%d, want 404", missing.Code)
	}
	if missing.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers must also cover error responses")
	}
}

func TestSecurityRenderEndpointRequiresJSON(t *testing.T) {
	handler := testRuntimeHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(`{"a":"2"}`))
	req.Header.Set("Content-Type", "text/plain")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status=%d body=%s, want 415", res.Code, res.Body.String())
	}
}

func TestSecurityRenderEndpointRejectsAmbiguousOrOversizedBodies(t *testing.T) {
	handler := testRuntimeHandler(t)
	tests := []struct {
		name string
		body string
	}{
		{name: "null", body: `null`},
		{name: "trailing object", body: `{"a":"2"} {"a":"3"}`},
		{name: "oversized", body: `{"a":"2","padding":"` + strings.Repeat("x", maxRenderBodyBytes) + `"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s, want 400", res.Code, res.Body.String())
			}
		})
	}
}

func TestSecurityBadRequestDoesNotPoisonReactiveSession(t *testing.T) {
	handler := testRuntimeHandler(t)

	bad := httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(`{"a":"2"} {"a":"3"}`))
	bad.Header.Set("Content-Type", "application/json")
	badRes := httptest.NewRecorder()
	handler.ServeHTTP(badRes, bad)
	if badRes.Code != http.StatusBadRequest {
		t.Fatalf("bad request status=%d", badRes.Code)
	}

	good := httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(`{"a":"4"}`))
	good.Header.Set("Content-Type", "application/json; charset=utf-8")
	goodRes := httptest.NewRecorder()
	handler.ServeHTTP(goodRes, good)
	if goodRes.Code != http.StatusOK {
		t.Fatalf("valid recovery status=%d body=%s", goodRes.Code, goodRes.Body.String())
	}
	if got := goodRes.Header().Get("X-MD0-Changed"); got != "a" {
		t.Fatalf("changed=%q, want a", got)
	}
}

func TestSecurityDocumentSizeLimit(t *testing.T) {
	_, err := ParseString("too-big.md", strings.Repeat("x", 2*1024*1024+1))
	if err == nil || !strings.Contains(err.Error(), "2 MiB limit") {
		t.Fatalf("err=%v, want document-size rejection", err)
	}
}

func TestSecurityExpressionTokenLimit(t *testing.T) {
	expr := "1" + strings.Repeat(" + 1", 512)
	_, err := ParseString("token-bomb.md", "@calc x = "+expr)
	if err == nil || !strings.Contains(err.Error(), "512-token limit") {
		t.Fatalf("err=%v, want expression-token rejection", err)
	}
}

func TestSecurityBlockNestingLimit(t *testing.T) {
	var src strings.Builder
	for i := 0; i < 66; i++ {
		src.WriteString("@when true\n")
	}
	src.WriteString("inside\n")
	for i := 0; i < 66; i++ {
		src.WriteString("@end\n")
	}
	_, err := ParseString("nesting-bomb.md", src.String())
	if err == nil || !strings.Contains(err.Error(), "64 levels") {
		t.Fatalf("err=%v, want nesting rejection", err)
	}
}

func TestSecurityChartValueLimit(t *testing.T) {
	labels := make([]string, 129)
	values := make([]string, 129)
	for i := range labels {
		labels[i] = fmt.Sprintf("%q", fmt.Sprintf("v%d", i))
		values[i] = fmt.Sprintf("%d", i)
	}
	src := "@chart hostile\nlabels = [" + strings.Join(labels, ",") + "]\nvalues = [" + strings.Join(values, ",") + "]\n@end"
	doc, err := ParseString("chart-bomb.md", src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(doc, nil)
	if err == nil || !strings.Contains(err.Error(), "128-value limit") {
		t.Fatalf("err=%v, want chart-size rejection", err)
	}
}

func TestSecurityTableColumnLimit(t *testing.T) {
	columns := make([]string, 65)
	cells := make([]string, 65)
	for i := range columns {
		columns[i] = fmt.Sprintf("%q", fmt.Sprintf("c%d", i))
		cells[i] = fmt.Sprintf("%d", i)
	}
	src := "@table hostile\ncolumns = [" + strings.Join(columns, ",") + "]\nrows = [[" + strings.Join(cells, ",") + "]]\n@end"
	doc, err := ParseString("table-bomb.md", src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(doc, nil)
	if err == nil || !strings.Contains(err.Error(), "64-column limit") {
		t.Fatalf("err=%v, want table-column rejection", err)
	}
}
