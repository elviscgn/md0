package md0

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

const maxEvaluatedStringBytes = 1 << 20

type Expr interface {
	Eval(env map[string]Value) (Value, error)
}

type literalExpr struct{ value Value }

func (e literalExpr) Eval(_ map[string]Value) (Value, error) { return e.value, nil }

type identExpr struct{ name string }

func (e identExpr) Eval(env map[string]Value) (Value, error) {
	v, ok := env[e.name]
	if !ok {
		return Null(), fmt.Errorf("unknown variable %q", e.name)
	}
	return v, nil
}

type arrayExpr struct{ items []Expr }

func (e arrayExpr) Eval(env map[string]Value) (Value, error) {
	out := make([]Value, 0, len(e.items))
	for _, item := range e.items {
		v, err := item.Eval(env)
		if err != nil {
			return Null(), err
		}
		out = append(out, v)
	}
	return List(out), nil
}

type unaryExpr struct {
	op    string
	right Expr
}

func (e unaryExpr) Eval(env map[string]Value) (Value, error) {
	v, err := e.right.Eval(env)
	if err != nil {
		return Null(), err
	}
	switch e.op {
	case "-":
		n, err := v.AsNumber()
		if err != nil {
			return Null(), err
		}
		return Number(-n), nil
	case "+":
		n, err := v.AsNumber()
		if err != nil {
			return Null(), err
		}
		return Number(n), nil
	case "!":
		b, err := v.AsBool()
		if err != nil {
			return Null(), err
		}
		return Boolean(!b), nil
	default:
		return Null(), fmt.Errorf("unknown unary operator %q", e.op)
	}
}

type binaryExpr struct {
	left  Expr
	op    string
	right Expr
}

func (e binaryExpr) Eval(env map[string]Value) (Value, error) {
	if e.op == "&&" || e.op == "||" {
		lv, err := e.left.Eval(env)
		if err != nil {
			return Null(), err
		}
		lb, err := lv.AsBool()
		if err != nil {
			return Null(), err
		}
		if e.op == "&&" && !lb {
			return Boolean(false), nil
		}
		if e.op == "||" && lb {
			return Boolean(true), nil
		}
		rv, err := e.right.Eval(env)
		if err != nil {
			return Null(), err
		}
		rb, err := rv.AsBool()
		if err != nil {
			return Null(), err
		}
		return Boolean(rb), nil
	}

	l, err := e.left.Eval(env)
	if err != nil {
		return Null(), err
	}
	r, err := e.right.Eval(env)
	if err != nil {
		return Null(), err
	}

	switch e.op {
	case "==":
		return Boolean(ValuesEqual(l, r)), nil
	case "!=":
		return Boolean(!ValuesEqual(l, r)), nil
	case "+":
		if l.Kind == StringKind || r.Kind == StringKind {
			left, right := l.String(), r.String()
			if len(left) > maxEvaluatedStringBytes || len(right) > maxEvaluatedStringBytes-len(left) {
				return Null(), fmt.Errorf("string result exceeds 1 MiB limit")
			}
			return String(left + right), nil
		}
		a, err := l.AsNumber()
		if err != nil {
			return Null(), err
		}
		b, err := r.AsNumber()
		if err != nil {
			return Null(), err
		}
		return Number(a + b), nil
	case "-", "*", "/", "%", "<", "<=", ">", ">=":
		a, err := l.AsNumber()
		if err != nil {
			return Null(), err
		}
		b, err := r.AsNumber()
		if err != nil {
			return Null(), err
		}
		switch e.op {
		case "-":
			return Number(a - b), nil
		case "*":
			return Number(a * b), nil
		case "/":
			if b == 0 {
				return Null(), fmt.Errorf("division by zero")
			}
			return Number(a / b), nil
		case "%":
			if b == 0 {
				return Null(), fmt.Errorf("modulo by zero")
			}
			return Number(math.Mod(a, b)), nil
		case "<":
			return Boolean(a < b), nil
		case "<=":
			return Boolean(a <= b), nil
		case ">":
			return Boolean(a > b), nil
		case ">=":
			return Boolean(a >= b), nil
		}
	}
	return Null(), fmt.Errorf("unknown operator %q", e.op)
}

type ternaryExpr struct{ cond, yes, no Expr }

func (e ternaryExpr) Eval(env map[string]Value) (Value, error) {
	c, err := e.cond.Eval(env)
	if err != nil {
		return Null(), err
	}
	b, err := c.AsBool()
	if err != nil {
		return Null(), err
	}
	if b {
		return e.yes.Eval(env)
	}
	return e.no.Eval(env)
}

type callExpr struct {
	name string
	args []Expr
}

func (e callExpr) Eval(env map[string]Value) (Value, error) {
	vals := make([]Value, len(e.args))
	for i, arg := range e.args {
		v, err := arg.Eval(env)
		if err != nil {
			return Null(), err
		}
		vals[i] = v
	}
	oneNumber := func() (float64, error) {
		if len(vals) != 1 {
			return 0, fmt.Errorf("%s expects 1 argument", e.name)
		}
		return vals[0].AsNumber()
	}
	switch e.name {
	case "ceil":
		n, err := oneNumber()
		if err != nil {
			return Null(), err
		}
		return Number(math.Ceil(n)), nil
	case "floor":
		n, err := oneNumber()
		if err != nil {
			return Null(), err
		}
		return Number(math.Floor(n)), nil
	case "round":
		n, err := oneNumber()
		if err != nil {
			return Null(), err
		}
		return Number(math.Round(n)), nil
	case "abs":
		n, err := oneNumber()
		if err != nil {
			return Null(), err
		}
		return Number(math.Abs(n)), nil
	case "sqrt":
		n, err := oneNumber()
		if err != nil {
			return Null(), err
		}
		if n < 0 {
			return Null(), fmt.Errorf("sqrt of negative number")
		}
		return Number(math.Sqrt(n)), nil
	case "min", "max":
		if len(vals) < 1 {
			return Null(), fmt.Errorf("%s expects at least 1 argument", e.name)
		}
		best, err := vals[0].AsNumber()
		if err != nil {
			return Null(), err
		}
		for _, v := range vals[1:] {
			n, err := v.AsNumber()
			if err != nil {
				return Null(), err
			}
			if e.name == "min" && n < best || e.name == "max" && n > best {
				best = n
			}
		}
		return Number(best), nil
	case "len":
		if len(vals) != 1 {
			return Null(), fmt.Errorf("len expects 1 argument")
		}
		if vals[0].Kind == StringKind {
			return Number(float64(len([]rune(vals[0].Str)))), nil
		}
		if vals[0].Kind == ListKind {
			return Number(float64(len(vals[0].List))), nil
		}
		return Null(), fmt.Errorf("len expects string or list")
	case "sum", "avg":
		if len(vals) != 1 || vals[0].Kind != ListKind {
			return Null(), fmt.Errorf("%s expects one list", e.name)
		}
		if len(vals[0].List) == 0 {
			return Number(0), nil
		}
		total := 0.0
		for _, v := range vals[0].List {
			n, err := v.AsNumber()
			if err != nil {
				return Null(), err
			}
			total += n
		}
		if e.name == "avg" {
			total /= float64(len(vals[0].List))
		}
		return Number(total), nil
	case "get":
		if len(vals) != 2 || vals[1].Kind != StringKind {
			return Null(), fmt.Errorf("get expects an object and string key")
		}
		object, err := vals[0].AsObject()
		if err != nil {
			return Null(), err
		}
		value, ok := object[vals[1].Str]
		if !ok {
			return Null(), fmt.Errorf("object has no key %q", vals[1].Str)
		}
		return value, nil
	case "columns", "rows":
		if len(vals) != 1 {
			return Null(), fmt.Errorf("%s expects one CSV attachment", e.name)
		}
		object, err := vals[0].AsObject()
		if err != nil {
			return Null(), err
		}
		value, ok := object[e.name]
		if !ok || value.Kind != ListKind {
			return Null(), fmt.Errorf("%s expects a CSV attachment", e.name)
		}
		return value, nil
	case "column":
		if len(vals) != 2 || vals[1].Kind != StringKind {
			return Null(), fmt.Errorf("column expects a CSV attachment and column name")
		}
		object, err := vals[0].AsObject()
		if err != nil {
			return Null(), err
		}
		columns, columnsOK := object["columns"]
		rows, rowsOK := object["rows"]
		if !columnsOK || !rowsOK || columns.Kind != ListKind || rows.Kind != ListKind {
			return Null(), fmt.Errorf("column expects a CSV attachment")
		}
		index := -1
		for i, name := range columns.List {
			if name.Kind == StringKind && name.Str == vals[1].Str {
				index = i
				break
			}
		}
		if index < 0 {
			return Null(), fmt.Errorf("CSV attachment has no column %q", vals[1].Str)
		}
		result := make([]Value, len(rows.List))
		for i, row := range rows.List {
			if row.Kind != ListKind || index >= len(row.List) {
				return Null(), fmt.Errorf("CSV attachment row %d is malformed", i+1)
			}
			result[i] = row.List[index]
		}
		return List(result), nil
	default:
		return Null(), fmt.Errorf("unknown function %q", e.name)
	}
}

type tokenType uint8

const (
	tokEOF tokenType = iota
	tokNumber
	tokString
	tokIdent
	tokLParen
	tokRParen
	tokLBracket
	tokRBracket
	tokComma
	tokQuestion
	tokColon
	tokOp
)

type token struct {
	typ  tokenType
	text string
	pos  int
}

type lexer struct {
	src string
	pos int
}

func (l *lexer) next() (token, error) {
	for l.pos < len(l.src) && unicode.IsSpace(rune(l.src[l.pos])) {
		l.pos++
	}
	if l.pos >= len(l.src) {
		return token{typ: tokEOF, pos: l.pos}, nil
	}
	start := l.pos
	c := l.src[l.pos]
	switch c {
	case '(':
		l.pos++
		return token{tokLParen, "(", start}, nil
	case ')':
		l.pos++
		return token{tokRParen, ")", start}, nil
	case '[':
		l.pos++
		return token{tokLBracket, "[", start}, nil
	case ']':
		l.pos++
		return token{tokRBracket, "]", start}, nil
	case ',':
		l.pos++
		return token{tokComma, ",", start}, nil
	case '?':
		l.pos++
		return token{tokQuestion, "?", start}, nil
	case ':':
		l.pos++
		return token{tokColon, ":", start}, nil
	case '"', '\'':
		quote := c
		l.pos++
		var b strings.Builder
		for l.pos < len(l.src) {
			ch := l.src[l.pos]
			l.pos++
			if ch == quote {
				return token{tokString, b.String(), start}, nil
			}
			if ch == '\\' {
				if l.pos >= len(l.src) {
					return token{}, fmt.Errorf("unterminated escape at %d", start+1)
				}
				esc := l.src[l.pos]
				l.pos++
				switch esc {
				case 'n':
					b.WriteByte('\n')
				case 'r':
					b.WriteByte('\r')
				case 't':
					b.WriteByte('\t')
				case '\\':
					b.WriteByte('\\')
				case '"':
					b.WriteByte('"')
				case '\'':
					b.WriteByte('\'')
				default:
					return token{}, fmt.Errorf("unsupported escape \\%c at %d", esc, l.pos)
				}
			} else {
				b.WriteByte(ch)
			}
		}
		return token{}, fmt.Errorf("unterminated string at %d", start+1)
	}
	if c >= '0' && c <= '9' || c == '.' && l.pos+1 < len(l.src) && l.src[l.pos+1] >= '0' && l.src[l.pos+1] <= '9' {
		l.pos++
		for l.pos < len(l.src) && ((l.src[l.pos] >= '0' && l.src[l.pos] <= '9') || l.src[l.pos] == '.') {
			l.pos++
		}
		if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
			l.pos++
			if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
				l.pos++
			}
			for l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
				l.pos++
			}
		}
		return token{tokNumber, l.src[start:l.pos], start}, nil
	}
	if unicode.IsLetter(rune(c)) || c == '_' {
		l.pos++
		for l.pos < len(l.src) {
			r := rune(l.src[l.pos])
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				break
			}
			l.pos++
		}
		return token{tokIdent, l.src[start:l.pos], start}, nil
	}
	if strings.ContainsRune("+-*/%!<>=&|", rune(c)) {
		l.pos++
		if l.pos < len(l.src) {
			pair := l.src[start : l.pos+1]
			if pair == "<=" || pair == ">=" || pair == "==" || pair == "!=" || pair == "&&" || pair == "||" {
				l.pos++
				return token{tokOp, pair, start}, nil
			}
		}
		if strings.ContainsRune("+-*/%!<>", rune(c)) {
			return token{tokOp, string(c), start}, nil
		}
	}
	return token{}, fmt.Errorf("unexpected character %q at %d", c, start+1)
}

type exprParser struct {
	lex   lexer
	cur   token
	depth int
}

func ParseExpr(src string) (Expr, error) {
	if len(src) > 16*1024 {
		return nil, fmt.Errorf("expression exceeds 16 KiB limit")
	}
	p := &exprParser{lex: lexer{src: src}}
	if err := p.advance(); err != nil {
		return nil, err
	}
	e, err := p.parseTernary()
	if err != nil {
		return nil, err
	}
	if p.cur.typ != tokEOF {
		return nil, fmt.Errorf("unexpected token %q at %d", p.cur.text, p.cur.pos+1)
	}
	return e, nil
}

func (p *exprParser) advance() error {
	t, err := p.lex.next()
	if err != nil {
		return err
	}
	p.cur = t
	return nil
}

func (p *exprParser) consume(t tokenType, text string) error {
	if p.cur.typ != t || text != "" && p.cur.text != text {
		return fmt.Errorf("expected %q at %d", text, p.cur.pos+1)
	}
	return p.advance()
}

func (p *exprParser) parseTernary() (Expr, error) {
	e, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.cur.typ == tokQuestion {
		if err := p.advance(); err != nil {
			return nil, err
		}
		yes, err := p.parseTernary()
		if err != nil {
			return nil, err
		}
		if err := p.consume(tokColon, ":"); err != nil {
			return nil, err
		}
		no, err := p.parseTernary()
		if err != nil {
			return nil, err
		}
		e = ternaryExpr{e, yes, no}
	}
	return e, nil
}

func (p *exprParser) parseOr() (Expr, error) {
	return p.parseBinary(p.parseAnd, map[string]bool{"||": true})
}
func (p *exprParser) parseAnd() (Expr, error) {
	return p.parseBinary(p.parseEquality, map[string]bool{"&&": true})
}
func (p *exprParser) parseEquality() (Expr, error) {
	return p.parseBinary(p.parseComparison, map[string]bool{"==": true, "!=": true})
}
func (p *exprParser) parseComparison() (Expr, error) {
	return p.parseBinary(p.parseTerm, map[string]bool{"<": true, "<=": true, ">": true, ">=": true})
}
func (p *exprParser) parseTerm() (Expr, error) {
	return p.parseBinary(p.parseFactor, map[string]bool{"+": true, "-": true})
}
func (p *exprParser) parseFactor() (Expr, error) {
	return p.parseBinary(p.parseUnary, map[string]bool{"*": true, "/": true, "%": true})
}

func (p *exprParser) parseBinary(next func() (Expr, error), ops map[string]bool) (Expr, error) {
	e, err := next()
	if err != nil {
		return nil, err
	}
	for p.cur.typ == tokOp && ops[p.cur.text] {
		op := p.cur.text
		if err := p.advance(); err != nil {
			return nil, err
		}
		r, err := next()
		if err != nil {
			return nil, err
		}
		e = binaryExpr{e, op, r}
	}
	return e, nil
}

func (p *exprParser) parseUnary() (Expr, error) {
	if p.cur.typ == tokOp && (p.cur.text == "!" || p.cur.text == "-" || p.cur.text == "+") {
		op := p.cur.text
		if err := p.advance(); err != nil {
			return nil, err
		}
		r, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return unaryExpr{op, r}, nil
	}
	return p.parsePrimary()
}

func (p *exprParser) enter() error {
	p.depth++
	if p.depth > 128 {
		return fmt.Errorf("expression nesting exceeds 128 levels")
	}
	return nil
}
func (p *exprParser) leave() { p.depth-- }

func (p *exprParser) parsePrimary() (Expr, error) {
	t := p.cur
	switch t.typ {
	case tokNumber:
		if err := p.advance(); err != nil {
			return nil, err
		}
		n, err := strconv.ParseFloat(t.text, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q", t.text)
		}
		return literalExpr{Number(n)}, nil
	case tokString:
		if err := p.advance(); err != nil {
			return nil, err
		}
		return literalExpr{String(t.text)}, nil
	case tokIdent:
		if err := p.advance(); err != nil {
			return nil, err
		}
		if t.text == "true" {
			return literalExpr{Boolean(true)}, nil
		}
		if t.text == "false" {
			return literalExpr{Boolean(false)}, nil
		}
		if t.text == "null" {
			return literalExpr{Null()}, nil
		}
		if p.cur.typ == tokLParen {
			if err := p.enter(); err != nil {
				return nil, err
			}
			defer p.leave()
			if err := p.advance(); err != nil {
				return nil, err
			}
			args := []Expr{}
			if p.cur.typ != tokRParen {
				for {
					a, err := p.parseTernary()
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					if len(args) > 64 {
						return nil, fmt.Errorf("function call exceeds 64 arguments")
					}
					if p.cur.typ != tokComma {
						break
					}
					if err := p.advance(); err != nil {
						return nil, err
					}
				}
			}
			if err := p.consume(tokRParen, ")"); err != nil {
				return nil, err
			}
			return callExpr{t.text, args}, nil
		}
		return identExpr{t.text}, nil
	case tokLParen:
		if err := p.enter(); err != nil {
			return nil, err
		}
		defer p.leave()
		if err := p.advance(); err != nil {
			return nil, err
		}
		e, err := p.parseTernary()
		if err != nil {
			return nil, err
		}
		if err := p.consume(tokRParen, ")"); err != nil {
			return nil, err
		}
		return e, nil
	case tokLBracket:
		if err := p.enter(); err != nil {
			return nil, err
		}
		defer p.leave()
		if err := p.advance(); err != nil {
			return nil, err
		}
		items := []Expr{}
		if p.cur.typ != tokRBracket {
			for {
				e, err := p.parseTernary()
				if err != nil {
					return nil, err
				}
				items = append(items, e)
				if len(items) > 1024 {
					return nil, fmt.Errorf("list literal exceeds 1024 items")
				}
				if p.cur.typ != tokComma {
					break
				}
				if err := p.advance(); err != nil {
					return nil, err
				}
			}
		}
		if err := p.consume(tokRBracket, "]"); err != nil {
			return nil, err
		}
		return arrayExpr{items}, nil
	default:
		return nil, fmt.Errorf("expected expression at %d", t.pos+1)
	}
}
