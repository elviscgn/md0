package md0

import (
	"encoding/base64"
	"fmt"
	"html"
	"math"
	"regexp"
	"strings"
	"time"
)

var interpRE = regexp.MustCompile(`\{\{\s*(.*?)\s*\}\}`)

type DOMPatch struct {
	NodeID string `json:"node"`
	DOMID  string `json:"dom_id"`
	HTML   string `json:"html"`
}

func domNodeID(nodeID string) string {
	return "md0-" + base64.RawURLEncoding.EncodeToString([]byte(nodeID))
}

func interpolate(s string, env map[string]Value) (string, error) {
	var firstErr error
	out := interpRE.ReplaceAllStringFunc(s, func(m string) string {
		if firstErr != nil {
			return m
		}
		sub := interpRE.FindStringSubmatch(m)
		e, err := ParseExpr(sub[1])
		if err != nil {
			firstErr = err
			return m
		}
		v, err := e.Eval(env)
		if err != nil {
			firstErr = err
			return m
		}
		return v.String()
	})
	return out, firstErr
}

func RenderFragment(doc *Document, r *EvalResult) (string, error) {
	var b strings.Builder
	b.WriteString(`<div class="md0-doc">`)
	if err := renderNodes(&b, doc.Nodes, r); err != nil {
		return "", err
	}
	b.WriteString(`</div>`)
	return b.String(), nil
}

func renderNodes(b *strings.Builder, nodes []Node, r *EvalResult) error {
	for _, n := range nodes {
		if err := renderNodeRegion(b, n, r); err != nil {
			return err
		}
	}
	return nil
}

func renderNodeRegion(b *strings.Builder, n Node, r *EvalResult) error {
	if _, invisible := n.(CalcNode); invisible {
		return nil
	}
	id := dependencyNodeID(n)
	writeRegionStart(b, id, regionClass(n))
	defer b.WriteString(`</div>`)

	switch x := n.(type) {
	case MarkdownNode:
		text, err := interpolateMarkdown(x.Text, r.Env)
		if err != nil {
			return fmt.Errorf("line %d: interpolation: %w", x.Line, err)
		}
		b.WriteString(renderMarkdown(text))
	case InputNode:
		v, ok := r.Env[x.Name]
		if !ok {
			return fmt.Errorf("internal input bookkeeping error for %s", x.Name)
		}
		renderInput(b, x, v)
	case ShowNode:
		v, err := x.Expr.Eval(r.Env)
		if err != nil {
			return err
		}
		b.WriteString(`<div class="md0-show"><code>`)
		b.WriteString(html.EscapeString(v.String()))
		b.WriteString(`</code></div>`)
	case AssertNode:
		a, ok := r.AssertionByLine[x.Line]
		if !ok {
			return fmt.Errorf("internal assertion bookkeeping error at line %d", x.Line)
		}
		renderAssertion(b, a)
	case WhenNode:
		active, ok := r.WhenByLine[x.Line]
		if !ok {
			return fmt.Errorf("internal condition bookkeeping error at line %d", x.Line)
		}
		if active {
			if err := renderNodes(b, x.Nodes, r); err != nil {
				return err
			}
		}
	case ChartNode:
		if err := renderChart(b, x, r.Env); err != nil {
			return err
		}
	case TableNode:
		if err := renderTable(b, x, r.Env); err != nil {
			return err
		}
	default:
		return fmt.Errorf("line %d: unsupported render node", n.LineNo())
	}
	return nil
}

func writeRegionStart(b *strings.Builder, nodeID, class string) {
	b.WriteString(`<div id="`)
	b.WriteString(domNodeID(nodeID))
	b.WriteString(`" data-md0-node="`)
	b.WriteString(html.EscapeString(nodeID))
	b.WriteString(`" class="md0-region `)
	b.WriteString(class)
	b.WriteString(`">`)
}

func regionClass(n Node) string {
	switch n.(type) {
	case MarkdownNode:
		return "md0-markdown-region"
	case InputNode:
		return "md0-input-region"
	case ShowNode:
		return "md0-show-region"
	case AssertNode:
		return "md0-assert-region"
	case WhenNode:
		return "md0-when-region"
	case ChartNode:
		return "md0-chart-region"
	case TableNode:
		return "md0-table-region"
	default:
		return "md0-node-region"
	}
}

func isRenderableNode(n Node) bool {
	_, invisible := n.(CalcNode)
	return !invisible
}

func renderNodeRegionString(n Node, r *EvalResult) (string, error) {
	var b strings.Builder
	if err := renderNodeRegion(&b, n, r); err != nil {
		return "", err
	}
	return b.String(), nil
}

func RenderPatches(doc *Document, r *EvalResult, stats IncrementalStats) ([]DOMPatch, error) {
	affected := make(map[string]bool, len(stats.Recomputed))
	for _, id := range stats.Recomputed {
		affected[id] = true
	}
	changedInputs := make(map[string]bool, len(stats.Changed))
	for _, name := range stats.Changed {
		changedInputs[name] = true
	}

	patches := []DOMPatch{}
	var walk func([]Node) error
	walk = func(nodes []Node) error {
		for _, n := range nodes {
			id := dependencyNodeID(n)
			patchThis := affected[id] && isRenderableNode(n)
			if input, ok := n.(InputNode); ok && changedInputs[input.Name] {
				patchThis = false
			}

			if patchThis {
				markup, err := renderNodeRegionString(n, r)
				if err != nil {
					return err
				}
				patches = append(patches, DOMPatch{NodeID: id, DOMID: domNodeID(id), HTML: markup})
				if _, structural := n.(WhenNode); structural {
					continue
				}
			}

			if when, ok := n.(WhenNode); ok {
				active := r.WhenByLine[when.Line]
				if !active {
					continue
				}
				if err := walk(when.Nodes); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(doc.Nodes); err != nil {
		return nil, err
	}
	return patches, nil
}

func renderInput(b *strings.Builder, x InputNode, v Value) {
	label := strings.TrimSpace(strings.TrimSuffix(x.Prefix, ":"))
	if label == "" {
		label = x.Name
	}
	inputID := domNodeID("control:" + x.Name)
	typeID := inputID + "-type"
	b.WriteString(`<label class="md0-input" for="`)
	b.WriteString(inputID)
	b.WriteString(`"><span>`)
	b.WriteString(html.EscapeString(label))
	b.WriteString(`</span>`)
	typ := strings.ToLower(x.Type)
	if typ == "boolean" || typ == "bool" {
		b.WriteString(`<input data-md0-input id="`)
		b.WriteString(inputID)
		b.WriteString(`" aria-describedby="`)
		b.WriteString(typeID)
		b.WriteString(`" name="`)
		b.WriteString(html.EscapeString(x.Name))
		b.WriteString(`" type="checkbox"`)
		if v.Bool {
			b.WriteString(` checked`)
		}
		b.WriteString(`>`)
	} else {
		inputType := "text"
		step := ""
		if typ == "number" || typ == "currency" || typ == "percent" {
			inputType = "number"
			step = ` step="any"`
		}
		if typ == "integer" {
			inputType = "number"
			step = ` step="1"`
		}
		b.WriteString(`<input data-md0-input id="`)
		b.WriteString(inputID)
		b.WriteString(`" aria-describedby="`)
		b.WriteString(typeID)
		b.WriteString(`" autocomplete="off" name="`)
		b.WriteString(html.EscapeString(x.Name))
		b.WriteString(`" type="`)
		b.WriteString(inputType)
		b.WriteString(`" value="`)
		b.WriteString(html.EscapeString(formatInputDisplayValue(x.Type, v)))
		b.WriteString(`"`)
		b.WriteString(step)
		b.WriteString(`>`)
	}
	b.WriteString(`<small id="`)
	b.WriteString(typeID)
	b.WriteString(`">`)
	b.WriteString(html.EscapeString(x.Type))
	b.WriteString(`</small></label>`)
}

func formatInputDisplayValue(typ string, v Value) string {
	if strings.EqualFold(typ, "duration") && v.Kind == NumberKind {
		return (time.Duration(v.Num) * time.Millisecond).String()
	}
	return v.String()
}

func renderAssertion(b *strings.Builder, a AssertionResult) {
	class := "pass"
	title := "assertion passed"
	if !a.Passed {
		class = "fail"
		title = "assertion failed"
	}
	b.WriteString(`<div class="md0-assert ` + class + `" role="status"><div class="md0-assert-title">` + title + `</div><code>`)
	b.WriteString(html.EscapeString(a.Source))
	b.WriteString(`</code>`)
	if !a.Passed && a.Message != "" {
		b.WriteString(`<div class="md0-assert-message">`)
		b.WriteString(renderMarkdown(a.Message))
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
}

func renderChart(b *strings.Builder, x ChartNode, env map[string]Value) error {
	lv, err := x.Labels.Eval(env)
	if err != nil {
		return err
	}
	vv, err := x.Values.Eval(env)
	if err != nil {
		return err
	}
	labels, _ := lv.AsList()
	values, _ := vv.AsList()
	nums := make([]float64, len(values))
	minValue, maxValue := 0.0, 0.0
	for i, v := range values {
		n, _ := v.AsNumber()
		nums[i] = n
		if n < minValue {
			minValue = n
		}
		if n > maxValue {
			maxValue = n
		}
	}
	if minValue == maxValue {
		maxValue = minValue + 1
	}

	const plotTop = 34.0
	const plotBottom = 210.0
	const plotHeight = plotBottom - plotTop
	scaleY := func(value float64) float64 {
		return plotBottom - ((value-minValue)/(maxValue-minValue))*plotHeight
	}
	zeroY := scaleY(0)
	chartTitleID := domNodeID(fmt.Sprintf("chart-title:%s:%d", x.Name, x.Line))

	b.WriteString(`<section class="md0-chart" aria-labelledby="`)
	b.WriteString(chartTitleID)
	b.WriteString(`"><div id="`)
	b.WriteString(chartTitleID)
	b.WriteString(`" class="md0-chart-title">`)
	b.WriteString(html.EscapeString(x.Name))
	b.WriteString(`</div><svg viewBox="0 0 640 280" role="img" aria-labelledby="`)
	b.WriteString(chartTitleID)
	b.WriteString(`"><title>`)
	b.WriteString(html.EscapeString(x.Name))
	b.WriteString(` chart</title>`)

	b.WriteString(fmt.Sprintf(`<line x1="80" y1="%.1f" x2="590" y2="%.1f" class="axis zero-axis"/>`, zeroY, zeroY))
	width := 420.0 / float64(len(nums))
	for i, n := range nums {
		valueY := scaleY(n)
		y := math.Min(valueY, zeroY)
		h := math.Abs(valueY - zeroY)
		xpos := 110 + float64(i)*width + width*.2
		barW := width * .6
		class := fmt.Sprintf("bar bar-%d", i%4)
		if n < 0 {
			class += " negative"
		}
		b.WriteString(fmt.Sprintf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="6" class="%s"/>`, xpos, y, barW, h, class))
		valueLabelY := y - 10
		if n < 0 {
			valueLabelY = y + h + 20
		}
		if valueLabelY < 18 {
			valueLabelY = 18
		}
		if valueLabelY > 232 {
			valueLabelY = 232
		}
		b.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" text-anchor="middle" class="value">%s</text>`, xpos+barW/2, valueLabelY, html.EscapeString(values[i].String())))
		b.WriteString(fmt.Sprintf(`<text x="%.1f" y="258" text-anchor="middle" class="label">%s</text>`, xpos+barW/2, html.EscapeString(labels[i].String())))
	}
	b.WriteString(`</svg></section>`)
	return nil
}

func renderTable(b *strings.Builder, x TableNode, env map[string]Value) error {
	cv, err := x.Columns.Eval(env)
	if err != nil {
		return err
	}
	rv, err := x.Rows.Eval(env)
	if err != nil {
		return err
	}
	cols, _ := cv.AsList()
	rows, _ := rv.AsList()
	b.WriteString(`<section class="md0-table"><div class="md0-table-title">` + html.EscapeString(x.Name) + `</div><div class="md0-table-scroll"><table><caption class="md0-sr-only">` + html.EscapeString(x.Name) + `</caption><thead><tr>`)
	for _, c := range cols {
		b.WriteString(`<th scope="col">` + html.EscapeString(c.String()) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, row := range rows {
		cells, _ := row.AsList()
		b.WriteString(`<tr>`)
		for _, cell := range cells {
			b.WriteString(`<td>` + html.EscapeString(cell.String()) + `</td>`)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table></div></section>`)
	return nil
}

func RenderStaticPage(title, fragment string) string {
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + html.EscapeString(title) + ` · md0</title><style>` + pageCSS + `</style></head><body><main id="md0-document">` + fragment + `</main></body></html>`
}

func RenderInteractivePage(title, fragment string) string {
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + html.EscapeString(title) + ` · md0</title><style>` + pageCSS + `</style></head><body><main id="md0-document">` + fragment + `</main><script>` + pageJS + `</script></body></html>`
}

const pageCSS = `
:root{color-scheme:light;--paper:#fff8f0;--ink:#231f20;--muted:#7a6e67;--line:#e8dcd0;--orange:#c25a2b;--green:#2e7d4f;--red:#a53838;--card:#fffcf8;--focus:#9a4620}*{box-sizing:border-box}body{margin:0;background:var(--paper);color:var(--ink);font:17px/1.65 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}#md0-document{max-width:900px;margin:64px auto;padding:0 24px 80px}.md0-region{display:block}.md0-doc h1{font-size:2.7rem;line-height:1.08;letter-spacing:-.04em;margin-bottom:1.1rem}.md0-doc h2{margin-top:2.2rem}.md0-doc code,.md0-chart-title,.md0-table-title{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}.md0-input{display:grid;grid-template-columns:minmax(150px,1fr) minmax(160px,260px) 70px;gap:14px;align-items:center;margin:12px 0;padding:14px 16px;border:1px solid var(--line);border-radius:14px;background:var(--card);transition:border-color .15s,box-shadow .15s}.md0-input:focus-within{border-color:#d4a181;box-shadow:0 0 0 3px rgba(194,90,43,.08)}.md0-input input{width:100%;padding:9px 11px;border:1px solid #d7cabd;border-radius:9px;background:white;color:var(--ink);font:inherit}.md0-input input[type=checkbox]{width:20px;height:20px;accent-color:var(--orange)}.md0-input input:focus-visible{outline:3px solid rgba(154,70,32,.2);outline-offset:2px;border-color:var(--focus)}.md0-input small{color:var(--muted);font-family:ui-monospace,monospace}.md0-show{margin:14px 0;padding:11px 14px;border-left:3px solid var(--orange);background:#f7ece3;border-radius:0 10px 10px 0}.md0-assert{margin:22px 0;padding:16px 18px;border-radius:14px;border:1px solid}.md0-assert.pass{background:#eaf5ee;border-color:#cde4d6;color:var(--green)}.md0-assert.fail{background:#fff0ed;border-color:#ebc7c0;color:var(--red)}.md0-assert-title{font-weight:750;text-transform:uppercase;letter-spacing:.06em;font-size:.78rem}.md0-assert code{display:block;margin-top:5px;color:inherit}.md0-assert-message p{margin:.55rem 0 0}.md0-chart,.md0-table{margin:28px 0;padding:18px;border:1px solid var(--line);border-radius:16px;background:var(--card);box-shadow:0 8px 24px rgba(35,31,32,.035)}.md0-chart-title,.md0-table-title{font-size:.82rem;color:var(--muted);margin-bottom:6px}.md0-chart svg{display:block;width:100%;height:auto}.md0-chart .axis{stroke:#d7cabd;stroke-width:2}.md0-chart .zero-axis{stroke:#bbaa9c}.md0-chart .bar-0{fill:#c25a2b}.md0-chart .bar-1{fill:#2e7d4f}.md0-chart .bar-2{fill:#e27b45}.md0-chart .bar-3{fill:#53a56f}.md0-chart .bar.negative{opacity:.78}.md0-chart text{font-family:ui-monospace,monospace;fill:var(--muted);font-size:16px}.md0-chart .value{fill:var(--ink);font-weight:700}.md0-table-scroll{overflow-x:auto}.md0-table table{width:100%;border-collapse:collapse}.md0-table th,.md0-table td{text-align:left;padding:10px 12px;border-bottom:1px solid var(--line);white-space:nowrap}.md0-table th{font-weight:700;color:var(--muted);font-size:.88rem}.md0-table tbody tr:last-child td{border-bottom:0}.md0-sr-only{position:absolute!important;width:1px!important;height:1px!important;padding:0!important;margin:-1px!important;overflow:hidden!important;clip:rect(0,0,0,0)!important;white-space:nowrap!important;border:0!important}pre{overflow:auto;padding:16px;border:1px solid var(--line);border-radius:12px;background:#f6eee6}pre code{white-space:pre}code{font-size:.92em}strong{font-weight:780}@media(max-width:640px){#md0-document{margin-top:28px}.md0-input{grid-template-columns:1fr}.md0-doc h1{font-size:2.1rem}.md0-chart,.md0-table{padding:13px}}
`

const pageJS = `
let md0Timer;
let md0Busy=false;
let md0Queued=false;
function md0CurrentInputs(){const values={};document.querySelectorAll('[data-md0-input]').forEach(el=>{values[el.name]=el.type==='checkbox'?String(el.checked):el.value});return values}
function md0RememberValues(){try{sessionStorage.setItem('md0:values',JSON.stringify(md0CurrentInputs()))}catch{}}
function md0RestoreValues(){let values;try{values=JSON.parse(sessionStorage.getItem('md0:values')||'null')}catch{}if(!values||typeof values!=='object')return;let restored=false;document.querySelectorAll('[data-md0-input]').forEach(el=>{if(!Object.prototype.hasOwnProperty.call(values,el.name))return;if(el.type==='checkbox')el.checked=values[el.name]==='true';else el.value=values[el.name];restored=true});if(restored)md0Refresh()}
document.addEventListener('input',e=>{if(!e.target.matches('[data-md0-input]'))return;md0RememberValues();clearTimeout(md0Timer);md0Timer=setTimeout(md0Refresh,120)});
document.addEventListener('change',e=>{if(e.target.matches('[data-md0-input]')){md0RememberValues();md0Refresh()}});
async function md0Refresh(){md0Queued=true;if(md0Busy)return;md0Busy=true;try{while(md0Queued){md0Queued=false;await md0SendLatest()}}finally{md0Busy=false}}
async function md0SendLatest(){const values={};document.querySelectorAll('[data-md0-input]').forEach(el=>{values[el.name]=el.type==='checkbox'?String(el.checked):el.value});const r=await fetch('/render',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(values)});if(!r.ok){console.error(await r.text());return}const payload=await r.json();for(const patch of payload.patches){const node=document.getElementById(patch.dom_id);if(!node){console.warn('md0 patch target missing',patch.node);continue}node.outerHTML=patch.html}}
setTimeout(md0RestoreValues,0);
`
