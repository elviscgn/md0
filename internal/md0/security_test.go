package md0

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const runtimeTestBaseURL = "http://127.0.0.1:8080"

func runtimeTestRequest(method, path string, body *strings.Reader) *http.Request {
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, runtimeTestBaseURL+path, nil)
	} else {
		req = httptest.NewRequest(method, runtimeTestBaseURL+path, body)
	}
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if method != http.MethodGet && method != http.MethodHead {
		req.Header.Set("Origin", runtimeTestBaseURL)
	}
	return req
}

func runtimeTokenForHandler(t *testing.T, handler http.Handler) string {
	t.Helper()
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, runtimeTestRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("GET / status=%d body=%s", res.Code, res.Body.String())
	}
	const marker = `<meta name="md0-runtime-token" content="`
	page := res.Body.String()
	start := strings.Index(page, marker)
	if start < 0 {
		t.Fatalf("runtime page missing capability token: %s", page)
	}
	start += len(marker)
	end := strings.Index(page[start:], `"`)
	if end < 0 {
		t.Fatal("runtime capability token was not terminated")
	}
	token := page[start : start+end]
	if len(token) < 32 {
		t.Fatalf("runtime capability token unexpectedly short: %q", token)
	}
	return token
}

func runtimeJSONRequest(token, body string) *http.Request {
	req := runtimeTestRequest(http.MethodPost, "/render", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MD0-Token", token)
	return req
}

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
	handler.ServeHTTP(res, runtimeTestRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("GET / status=%d body=%s", res.Code, res.Body.String())
	}
	requireHeaderContains(t, res.Header(), "Cache-Control", "no-store")
	requireHeaderContains(t, res.Header(), "X-Content-Type-Options", "nosniff")
	requireHeaderContains(t, res.Header(), "Referrer-Policy", "no-referrer")
	requireHeaderContains(t, res.Header(), "Cross-Origin-Resource-Policy", "same-origin")
	csp := res.Header().Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'none'", "script-src 'sha256-", "style-src 'sha256-", "connect-src 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, want) {
			t.Fatalf("CSP %q missing %q", csp, want)
		}
	}
	if strings.Contains(csp, "'unsafe-inline'") || strings.Contains(csp, "'unsafe-eval'") {
		t.Fatalf("CSP contains unsafe execution escape hatch: %q", csp)
	}
	for _, source := range []string{pageCSS, runtimeStatusCSS, pageJS, runtimeStatusJS} {
		if !strings.Contains(csp, cspHash(source)) {
			t.Fatalf("CSP missing hash for built-in runtime asset")
		}
	}

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, runtimeTestRequest(http.MethodGet, "/not-a-route", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("GET unknown route status=%d, want 404", missing.Code)
	}
	if missing.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers must also cover error responses")
	}
}

func TestSecurityRuntimeRejectsHostOriginAndCrossSiteRequests(t *testing.T) {
	handler := testRuntimeHandler(t)
	token := runtimeTokenForHandler(t, handler)

	hostileHost := runtimeTestRequest(http.MethodGet, "/", nil)
	hostileHost.Host = "evil.example:8080"
	hostRes := httptest.NewRecorder()
	handler.ServeHTTP(hostRes, hostileHost)
	if hostRes.Code != http.StatusForbidden {
		t.Fatalf("host poisoning status=%d, want 403", hostRes.Code)
	}

	hostileOrigin := runtimeJSONRequest(token, `{"a":"2"}`)
	hostileOrigin.Header.Set("Origin", "https://evil.example")
	originRes := httptest.NewRecorder()
	handler.ServeHTTP(originRes, hostileOrigin)
	if originRes.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status=%d, want 403", originRes.Code)
	}

	crossSite := runtimeTestRequest(http.MethodGet, "/", nil)
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	crossSiteRes := httptest.NewRecorder()
	handler.ServeHTTP(crossSiteRes, crossSite)
	if crossSiteRes.Code != http.StatusForbidden {
		t.Fatalf("cross-site status=%d, want 403", crossSiteRes.Code)
	}
}

func TestSecurityRenderEndpointRequiresCapabilityToken(t *testing.T) {
	handler := testRuntimeHandler(t)

	missing := runtimeJSONRequest("", `{"a":"2"}`)
	missingRes := httptest.NewRecorder()
	handler.ServeHTTP(missingRes, missing)
	if missingRes.Code != http.StatusForbidden {
		t.Fatalf("missing token status=%d, want 403", missingRes.Code)
	}

	unknown := runtimeJSONRequest(strings.Repeat("A", 43), `{"a":"2"}`)
	unknownRes := httptest.NewRecorder()
	handler.ServeHTTP(unknownRes, unknown)
	if unknownRes.Code != http.StatusForbidden {
		t.Fatalf("unknown token status=%d, want 403", unknownRes.Code)
	}
}

func TestSecurityRuntimeSessionsAreIsolated(t *testing.T) {
	handler := testRuntimeHandler(t)
	first := runtimeTokenForHandler(t, handler)
	second := runtimeTokenForHandler(t, handler)
	if first == second {
		t.Fatal("separate page loads must not share a capability token")
	}

	firstUpdate := httptest.NewRecorder()
	handler.ServeHTTP(firstUpdate, runtimeJSONRequest(first, `{"a":"4"}`))
	if firstUpdate.Code != http.StatusOK || firstUpdate.Header().Get("X-MD0-Changed") != "a" {
		t.Fatalf("first session update status=%d changed=%q", firstUpdate.Code, firstUpdate.Header().Get("X-MD0-Changed"))
	}

	firstRepeat := httptest.NewRecorder()
	handler.ServeHTTP(firstRepeat, runtimeJSONRequest(first, `{"a":"4"}`))
	if firstRepeat.Code != http.StatusOK || firstRepeat.Header().Get("X-MD0-Changed") != "" {
		t.Fatalf("first session repeat status=%d changed=%q", firstRepeat.Code, firstRepeat.Header().Get("X-MD0-Changed"))
	}

	secondUpdate := httptest.NewRecorder()
	handler.ServeHTTP(secondUpdate, runtimeJSONRequest(second, `{"a":"4"}`))
	if secondUpdate.Code != http.StatusOK || secondUpdate.Header().Get("X-MD0-Changed") != "a" {
		t.Fatalf("second session inherited first session state: status=%d changed=%q", secondUpdate.Code, secondUpdate.Header().Get("X-MD0-Changed"))
	}
}

func TestSecurityRuntimeSessionStoreIsBounded(t *testing.T) {
	doc, err := ParseString("bounded.md", `A: @input a number = 1`)
	if err != nil {
		t.Fatal(err)
	}
	store := newRuntimeSessionStore(doc)
	tokens := make([]string, 0, maxRuntimeSessions+1)
	for i := 0; i < maxRuntimeSessions+1; i++ {
		token, _, err := store.create()
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, token)
	}
	if _, ok := store.get(tokens[0]); ok {
		t.Fatal("oldest runtime session should be evicted at capacity")
	}
	if _, ok := store.get(tokens[len(tokens)-1]); !ok {
		t.Fatal("newest runtime session should remain available")
	}
}

func TestSecurityOpenOnlyAcceptsLoopbackAddresses(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080"} {
		if _, err := validateLoopbackAddress(addr); err != nil {
			t.Fatalf("loopback address %q rejected: %v", addr, err)
		}
	}
	for _, addr := range []string{"0.0.0.0:8080", ":8080", "192.0.2.1:8080"} {
		if _, err := validateLoopbackAddress(addr); err == nil {
			t.Fatalf("non-loopback address %q accepted", addr)
		}
	}
}

func TestSecurityRenderEndpointRequiresJSON(t *testing.T) {
	handler := testRuntimeHandler(t)
	token := runtimeTokenForHandler(t, handler)
	req := runtimeJSONRequest(token, `{"a":"2"}`)
	req.Header.Set("Content-Type", "text/plain")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status=%d body=%s, want 415", res.Code, res.Body.String())
	}
}

func TestSecurityRenderEndpointRejectsAmbiguousOrOversizedBodies(t *testing.T) {
	handler := testRuntimeHandler(t)
	token := runtimeTokenForHandler(t, handler)
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
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, runtimeJSONRequest(token, tc.body))
			if res.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s, want 400", res.Code, res.Body.String())
			}
		})
	}
}

func TestSecurityBadRequestDoesNotPoisonReactiveSession(t *testing.T) {
	handler := testRuntimeHandler(t)
	token := runtimeTokenForHandler(t, handler)

	badRes := httptest.NewRecorder()
	handler.ServeHTTP(badRes, runtimeJSONRequest(token, `{"a":"2"} {"a":"3"}`))
	if badRes.Code != http.StatusBadRequest {
		t.Fatalf("bad request status=%d", badRes.Code)
	}

	good := runtimeJSONRequest(token, `{"a":"4"}`)
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

func TestSecurityRejectsInvalidUTF8(t *testing.T) {
	invalid := string([]byte{'o', 'k', '\n', 0xff, 'x'})
	if _, err := ParseString("invalid-utf8.md", invalid); err == nil || !strings.Contains(err.Error(), "valid UTF-8") {
		t.Fatalf("ParseString err=%v, want UTF-8 rejection", err)
	}

	path := filepath.Join(t.TempDir(), "invalid.md")
	if err := os.WriteFile(path, []byte{'o', 'k', '\n', 0xff, 'x'}, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseFile(path); err == nil || !strings.Contains(err.Error(), "valid UTF-8") {
		t.Fatalf("ParseFile err=%v, want UTF-8 rejection", err)
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
