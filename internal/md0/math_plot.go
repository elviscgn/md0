package md0

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"html"
	"math"
	"strconv"
	"strings"
	"unicode"
)

const (
	defaultPlotSamples = 320
	minPlotSamples     = 32
	maxPlotSamples     = 1024
	maxPlotCurves      = 4
)

func renderInlineMath(src string) string {
	return renderMathML(src, false)
}

func renderDisplayMath(src string) string {
	return `<div class="md0-math-display" style="overflow-x:auto;margin:1rem 0;text-align:center">` + renderMathML(src, true) + `</div>`
}

func renderMathML(src string, display bool) string {
	p := mathMLParser{src: strings.TrimSpace(src)}
	body, ok := p.parseSequence(0)
	if !ok || strings.TrimSpace(p.src[p.pos:]) != "" {
		return `<code class="md0-math-fallback">` + html.EscapeString(src) + `</code>`
	}
	displayAttr := "inline"
	if display {
		displayAttr = "block"
	}
	return `<math xmlns="http://www.w3.org/1998/Math/MathML" display="` + displayAttr + `" style="font-family:serif;font-size:1.08em">` + body + `</math>`
}

type mathMLParser struct {
	src string
	pos int
}

func (p *mathMLParser) parseSequence(stop byte) (string, bool) {
	var out strings.Builder
	for p.pos < len(p.src) {
		p.skipSpaces()
		if p.pos >= len(p.src) {
			break
		}
		if stop != 0 && p.src[p.pos] == stop {
			p.pos++
			return `<mrow>` + out.String() + `</mrow>`, true
		}
		atom, ok := p.parseAtom()
		if !ok {
			return "", false
		}
		atom = p.parseScripts(atom)
		out.WriteString(atom)
	}
	if stop != 0 {
		return "", false
	}
	return `<mrow>` + out.String() + `</mrow>`, true
}

func (p *mathMLParser) parseAtom() (string, bool) {
	p.skipSpaces()
	if p.pos >= len(p.src) {
		return "", false
	}
	c := p.src[p.pos]
	if c == '{' {
		p.pos++
		return p.parseSequence('}')
	}
	if c == '\\' {
		return p.parseCommand()
	}
	if c >= '0' && c <= '9' || c == '.' && p.pos+1 < len(p.src) && p.src[p.pos+1] >= '0' && p.src[p.pos+1] <= '9' {
		start := p.pos
		p.pos++
		for p.pos < len(p.src) {
			ch := p.src[p.pos]
			if ch >= '0' && ch <= '9' || ch == '.' {
				p.pos++
				continue
			}
			break
		}
		return `<mn>` + html.EscapeString(p.src[start:p.pos]) + `</mn>`, true
	}
	if unicode.IsLetter(rune(c)) {
		start := p.pos
		p.pos++
		for p.pos < len(p.src) && unicode.IsLetter(rune(p.src[p.pos])) {
			p.pos++
		}
		return `<mi>` + html.EscapeString(p.src[start:p.pos]) + `</mi>`, true
	}
	p.pos++
	switch c {
	case '+', '-', '=', '<', '>', '/', '*', '(', ')', '[', ']', ',', ':', '|':
		return `<mo>` + html.EscapeString(string(c)) + `</mo>`, true
	default:
		return `<mtext>` + html.EscapeString(string(c)) + `</mtext>`, true
	}
}

func (p *mathMLParser) parseCommand() (string, bool) {
	p.pos++
	start := p.pos
	for p.pos < len(p.src) && unicode.IsLetter(rune(p.src[p.pos])) {
		p.pos++
	}
	name := p.src[start:p.pos]
	if name == "" && p.pos < len(p.src) {
		name = string(p.src[p.pos])
		p.pos++
	}
	switch name {
	case "frac":
		num, ok := p.parseRequiredGroup()
		if !ok {
			return "", false
		}
		den, ok := p.parseRequiredGroup()
		if !ok {
			return "", false
		}
		return `<mfrac>` + num + den + `</mfrac>`, true
	case "sqrt":
		body, ok := p.parseRequiredGroup()
		if !ok {
			return "", false
		}
		return `<msqrt>` + body + `</msqrt>`, true
	case "text":
		p.skipSpaces()
		if p.pos >= len(p.src) || p.src[p.pos] != '{' {
			return "", false
		}
		p.pos++
		start := p.pos
		depth := 1
		for p.pos < len(p.src) && depth > 0 {
			switch p.src[p.pos] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					text := p.src[start:p.pos]
					p.pos++
					return `<mtext>` + html.EscapeString(text) + `</mtext>`, true
				}
			}
			p.pos++
		}
		return "", false
	case "left", "right":
		p.skipSpaces()
		if p.pos >= len(p.src) {
			return "", true
		}
		ch := p.src[p.pos]
		p.pos++
		return `<mo stretchy="true">` + html.EscapeString(string(ch)) + `</mo>`, true
	}
	if symbol, ok := mathGreek[name]; ok {
		return `<mi>` + symbol + `</mi>`, true
	}
	if op, ok := mathOperators[name]; ok {
		return `<mo>` + op + `</mo>`, true
	}
	if _, ok := mathFunctions[name]; ok {
		return `<mi mathvariant="normal">` + html.EscapeString(name) + `</mi>`, true
	}
	return `<mtext>\` + html.EscapeString(name) + `</mtext>`, true
}

func (p *mathMLParser) parseRequiredGroup() (string, bool) {
	p.skipSpaces()
	if p.pos >= len(p.src) || p.src[p.pos] != '{' {
		return "", false
	}
	p.pos++
	return p.parseSequence('}')
}

func (p *mathMLParser) parseScripts(base string) string {
	var sub, sup string
	for {
		p.skipSpaces()
		if p.pos >= len(p.src) || p.src[p.pos] != '^' && p.src[p.pos] != '_' {
			break
		}
		kind := p.src[p.pos]
		p.pos++
		arg, ok := p.parseScriptArg()
		if !ok {
			return base
		}
		if kind == '^' {
			sup = arg
		} else {
			sub = arg
		}
	}
	switch {
	case sub != "" && sup != "":
		return `<msubsup>` + base + sub + sup + `</msubsup>`
	case sub != "":
		return `<msub>` + base + sub + `</msub>`
	case sup != "":
		return `<msup>` + base + sup + `</msup>`
	default:
		return base
	}
}

func (p *mathMLParser) parseScriptArg() (string, bool) {
	p.skipSpaces()
	if p.pos >= len(p.src) {
		return "", false
	}
	if p.src[p.pos] == '{' {
		p.pos++
		return p.parseSequence('}')
	}
	return p.parseAtom()
}

func (p *mathMLParser) skipSpaces() {
	for p.pos < len(p.src) && unicode.IsSpace(rune(p.src[p.pos])) {
		p.pos++
	}
}

var mathGreek = map[string]string{
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ε",
	"theta": "θ", "lambda": "λ", "mu": "μ", "pi": "π", "rho": "ρ",
	"sigma": "σ", "phi": "φ", "omega": "ω", "Delta": "Δ", "Sigma": "Σ", "Omega": "Ω",
}

var mathOperators = map[string]string{
	"cdot": "·", "times": "×", "div": "÷", "pm": "±", "le": "≤", "leq": "≤",
	"ge": "≥", "geq": "≥", "ne": "≠", "neq": "≠", "approx": "≈", "infty": "∞",
	"sum": "∑", "prod": "∏", "int": "∫", "to": "→",
}

var mathFunctions = map[string]struct{}{
	"sin": {}, "cos": {}, "tan": {}, "asin": {}, "acos": {}, "atan": {}, "log": {}, "ln": {}, "exp": {},
}

func renderPlotFence(src string) string {
	plot, err := parsePlotFence(src)
	if err != nil {
		return `<div class="md0-plot-error" role="alert" style="margin:1rem 0;padding:.7rem .8rem;border-left:2px solid var(--red);color:var(--red);font-family:var(--md0-font-sans)">plot: ` + html.EscapeString(err.Error()) + `</div>`
	}
	return plot.render()
}

type plotSpec struct {
	title   string
	xmin    float64
	xmax    float64
	samples int
	curves  []plotCurve
}

type plotCurve struct {
	label string
	src   string
	expr  ast.Expr
}

func parsePlotFence(src string) (*plotSpec, error) {
	cfg := map[string]string{}
	for i, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		eq := strings.Index(trim, "=")
		if eq <= 0 {
			return nil, fmt.Errorf("line %d: expected key = value", i+1)
		}
		key := strings.TrimSpace(trim[:eq])
		value := strings.TrimSpace(trim[eq+1:])
		if _, exists := cfg[key]; exists {
			return nil, fmt.Errorf("line %d: duplicate key %q", i+1, key)
		}
		cfg[key] = value
	}
	spec := &plotSpec{xmin: -10, xmax: 10, samples: defaultPlotSamples}
	if raw := cfg["title"]; raw != "" {
		spec.title = strings.Trim(strings.TrimSpace(raw), `"'`)
	}
	if raw := cfg["x"]; raw != "" {
		lo, hi, err := parsePlotRange(raw)
		if err != nil {
			return nil, fmt.Errorf("x range: %w", err)
		}
		spec.xmin, spec.xmax = lo, hi
	}
	if !isFinite(spec.xmin) || !isFinite(spec.xmax) || spec.xmin >= spec.xmax {
		return nil, fmt.Errorf("x range must contain two finite increasing numbers")
	}
	if raw := cfg["samples"]; raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < minPlotSamples || n > maxPlotSamples {
			return nil, fmt.Errorf("samples must be an integer from %d to %d", minPlotSamples, maxPlotSamples)
		}
		spec.samples = n
	}
	keys := []string{"y", "y2", "y3", "y4"}
	for i, key := range keys {
		raw := strings.TrimSpace(cfg[key])
		if raw == "" {
			continue
		}
		expr, err := goparser.ParseExpr(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		labelKey := "label"
		if i > 0 {
			labelKey = fmt.Sprintf("label%d", i+1)
		}
		label := strings.Trim(strings.TrimSpace(cfg[labelKey]), `"'`)
		if label == "" {
			label = raw
		}
		spec.curves = append(spec.curves, plotCurve{label: label, src: raw, expr: expr})
	}
	if len(spec.curves) == 0 {
		return nil, fmt.Errorf("requires y = expression")
	}
	if len(spec.curves) > maxPlotCurves {
		return nil, fmt.Errorf("supports at most %d curves", maxPlotCurves)
	}
	return spec, nil
}

func parsePlotRange(raw string) (float64, float64, error) {
	trim := strings.TrimSpace(raw)
	if len(trim) < 5 || trim[0] != '[' || trim[len(trim)-1] != ']' {
		return 0, 0, fmt.Errorf("expected [min, max]")
	}
	inside := trim[1 : len(trim)-1]
	parts := strings.Split(inside, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected exactly two bounds")
	}
	loExpr, err := goparser.ParseExpr(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, err
	}
	hiExpr, err := goparser.ParseExpr(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, err
	}
	lo, err := evalPlotAST(loExpr, 0)
	if err != nil {
		return 0, 0, err
	}
	hi, err := evalPlotAST(hiExpr, 0)
	if err != nil {
		return 0, 0, err
	}
	return lo, hi, nil
}

func (p *plotSpec) render() string {
	type sampledCurve struct {
		label  string
		points []plotPoint
	}
	curves := make([]sampledCurve, len(p.curves))
	ymin, ymax := math.Inf(1), math.Inf(-1)
	for ci, curve := range p.curves {
		points := make([]plotPoint, p.samples)
		for i := 0; i < p.samples; i++ {
			x := p.xmin + (p.xmax-p.xmin)*float64(i)/float64(p.samples-1)
			y, err := evalPlotAST(curve.expr, x)
			valid := err == nil && isFinite(y)
			points[i] = plotPoint{x: x, y: y, valid: valid}
			if valid {
				if y < ymin {
					ymin = y
				}
				if y > ymax {
					ymax = y
				}
			}
		}
		curves[ci] = sampledCurve{label: curve.label, points: points}
	}
	if math.IsInf(ymin, 0) || math.IsInf(ymax, 0) {
		return `<div class="md0-plot-error" role="alert" style="margin:1rem 0;padding:.7rem .8rem;border-left:2px solid var(--red);color:var(--red);font-family:var(--md0-font-sans)">plot: function is undefined across the requested range</div>`
	}
	if ymin == ymax {
		delta := math.Max(1, math.Abs(ymin)*0.1)
		ymin -= delta
		ymax += delta
	} else {
		pad := (ymax - ymin) * 0.08
		ymin -= pad
		ymax += pad
	}

	const width, height = 720.0, 360.0
	const left, right, top, bottom = 62.0, 704.0, 24.0, 318.0
	sx := func(x float64) float64 { return left + (x-p.xmin)/(p.xmax-p.xmin)*(right-left) }
	sy := func(y float64) float64 { return bottom - (y-ymin)/(ymax-ymin)*(bottom-top) }

	var b strings.Builder
	b.WriteString(`<section class="md0-plot" style="margin:1.2rem 0;padding:.75rem 0;border-top:1px solid var(--line);border-bottom:1px solid var(--line)">`)
	if p.title != "" {
		b.WriteString(`<div style="margin:0 0 .55rem;font:600 .78rem/1.2 var(--md0-font-sans);color:var(--muted)">` + html.EscapeString(p.title) + `</div>`)
	}
	b.WriteString(fmt.Sprintf(`<svg viewBox="0 0 %.0f %.0f" role="img" aria-label="function plot" style="display:block;width:100%%;height:auto;overflow:visible">`, width, height))
	for i := 0; i <= 5; i++ {
		t := float64(i) / 5
		x := left + t*(right-left)
		y := top + t*(bottom-top)
		b.WriteString(fmt.Sprintf(`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="var(--line)" stroke-width="1" stroke-dasharray="2 5"/>`, x, top, x, bottom))
		b.WriteString(fmt.Sprintf(`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="var(--line)" stroke-width="1" stroke-dasharray="2 5"/>`, left, y, right, y))
		xv := p.xmin + t*(p.xmax-p.xmin)
		yv := ymax - t*(ymax-ymin)
		b.WriteString(fmt.Sprintf(`<text x="%.2f" y="341" text-anchor="middle" fill="var(--muted)" style="font:11px var(--md0-font-sans)">%s</text>`, x, html.EscapeString(formatPlotNumber(xv))))
		b.WriteString(fmt.Sprintf(`<text x="52" y="%.2f" text-anchor="end" dominant-baseline="middle" fill="var(--muted)" style="font:11px var(--md0-font-sans)">%s</text>`, y, html.EscapeString(formatPlotNumber(yv))))
	}
	if p.xmin <= 0 && p.xmax >= 0 {
		x0 := sx(0)
		b.WriteString(fmt.Sprintf(`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="var(--muted)" stroke-width="1.2"/>`, x0, top, x0, bottom))
	}
	if ymin <= 0 && ymax >= 0 {
		y0 := sy(0)
		b.WriteString(fmt.Sprintf(`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="var(--muted)" stroke-width="1.2"/>`, left, y0, right, y0))
	}
	for ci, curve := range curves {
		var path strings.Builder
		penDown := false
		for _, point := range curve.points {
			if !point.valid {
				penDown = false
				continue
			}
			px, py := sx(point.x), sy(point.y)
			if py < top-2000 || py > bottom+2000 {
				penDown = false
				continue
			}
			if !penDown {
				fmt.Fprintf(&path, "M %.2f %.2f ", px, py)
				penDown = true
			} else {
				fmt.Fprintf(&path, "L %.2f %.2f ", px, py)
			}
		}
		b.WriteString(fmt.Sprintf(`<path d="%s" fill="none" stroke="var(--chart-%d)" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`, path.String(), ci%4))
	}
	b.WriteString(`</svg>`)
	if len(curves) > 1 {
		b.WriteString(`<div style="display:flex;gap:1rem;flex-wrap:wrap;margin:.4rem 0 0;font:11px/1.3 var(--md0-font-sans);color:var(--muted)">`)
		for ci, curve := range curves {
			b.WriteString(fmt.Sprintf(`<span><span aria-hidden="true" style="display:inline-block;width:14px;height:2px;vertical-align:middle;margin-right:5px;background:var(--chart-%d)"></span>%s</span>`, ci%4, html.EscapeString(curve.label)))
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</section>`)
	return b.String()
}

type plotPoint struct {
	x, y  float64
	valid bool
}

func evalPlotAST(expr ast.Expr, x float64) (float64, error) {
	var eval func(ast.Expr) (float64, error)
	eval = func(node ast.Expr) (float64, error) {
		switch n := node.(type) {
		case *ast.BasicLit:
			if n.Kind != token.INT && n.Kind != token.FLOAT {
				return 0, fmt.Errorf("only numeric literals are allowed")
			}
			v, err := strconv.ParseFloat(n.Value, 64)
			if err != nil || !isFinite(v) {
				return 0, fmt.Errorf("invalid numeric literal %q", n.Value)
			}
			return v, nil
		case *ast.Ident:
			switch n.Name {
			case "x":
				return x, nil
			case "pi":
				return math.Pi, nil
			case "e":
				return math.E, nil
			default:
				return 0, fmt.Errorf("unknown plot symbol %q; use {{ %s }} for md0 values", n.Name, n.Name)
			}
		case *ast.ParenExpr:
			return eval(n.X)
		case *ast.UnaryExpr:
			v, err := eval(n.X)
			if err != nil {
				return 0, err
			}
			switch n.Op {
			case token.ADD:
				return v, nil
			case token.SUB:
				return -v, nil
			default:
				return 0, fmt.Errorf("unsupported unary operator %s", n.Op)
			}
		case *ast.BinaryExpr:
			a, err := eval(n.X)
			if err != nil {
				return 0, err
			}
			b, err := eval(n.Y)
			if err != nil {
				return 0, err
			}
			var v float64
			switch n.Op {
			case token.ADD:
				v = a + b
			case token.SUB:
				v = a - b
			case token.MUL:
				v = a * b
			case token.QUO:
				if b == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				v = a / b
			case token.REM:
				if b == 0 {
					return 0, fmt.Errorf("modulo by zero")
				}
				v = math.Mod(a, b)
			case token.XOR:
				v = math.Pow(a, b)
			default:
				return 0, fmt.Errorf("unsupported operator %s", n.Op)
			}
			if !isFinite(v) {
				return 0, fmt.Errorf("non-finite result")
			}
			return v, nil
		case *ast.CallExpr:
			ident, ok := n.Fun.(*ast.Ident)
			if !ok {
				return 0, fmt.Errorf("only named math functions are allowed")
			}
			args := make([]float64, len(n.Args))
			for i, arg := range n.Args {
				v, err := eval(arg)
				if err != nil {
					return 0, err
				}
				args[i] = v
			}
			return evalPlotFunction(ident.Name, args)
		default:
			return 0, fmt.Errorf("unsupported plot expression %T", node)
		}
	}
	return eval(expr)
}

func evalPlotFunction(name string, args []float64) (float64, error) {
	one := func(fn func(float64) float64) (float64, error) {
		if len(args) != 1 {
			return 0, fmt.Errorf("%s expects 1 argument", name)
		}
		v := fn(args[0])
		if !isFinite(v) {
			return 0, fmt.Errorf("%s produced a non-finite result", name)
		}
		return v, nil
	}
	switch name {
	case "sin":
		return one(math.Sin)
	case "cos":
		return one(math.Cos)
	case "tan":
		return one(math.Tan)
	case "asin":
		return one(math.Asin)
	case "acos":
		return one(math.Acos)
	case "atan":
		return one(math.Atan)
	case "sqrt":
		if len(args) != 1 || args[0] < 0 {
			return 0, fmt.Errorf("sqrt expects one non-negative argument")
		}
		return one(math.Sqrt)
	case "abs":
		return one(math.Abs)
	case "exp":
		return one(math.Exp)
	case "log", "ln":
		if len(args) != 1 || args[0] <= 0 {
			return 0, fmt.Errorf("%s expects one positive argument", name)
		}
		return one(math.Log)
	case "log10":
		if len(args) != 1 || args[0] <= 0 {
			return 0, fmt.Errorf("log10 expects one positive argument")
		}
		return one(math.Log10)
	case "floor":
		return one(math.Floor)
	case "ceil":
		return one(math.Ceil)
	case "round":
		return one(math.Round)
	case "pow":
		if len(args) != 2 {
			return 0, fmt.Errorf("pow expects 2 arguments")
		}
		v := math.Pow(args[0], args[1])
		if !isFinite(v) {
			return 0, fmt.Errorf("pow produced a non-finite result")
		}
		return v, nil
	case "min", "max":
		if len(args) < 1 {
			return 0, fmt.Errorf("%s expects at least 1 argument", name)
		}
		best := args[0]
		for _, v := range args[1:] {
			if name == "min" && v < best || name == "max" && v > best {
				best = v
			}
		}
		return best, nil
	default:
		return 0, fmt.Errorf("unknown plot function %q", name)
	}
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func formatPlotNumber(v float64) string {
	if math.Abs(v) < 1e-12 {
		v = 0
	}
	return strconv.FormatFloat(v, 'g', 4, 64)
}
