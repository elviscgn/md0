package md0

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	inputRE = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$`)
	calcRE  = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$`)
)

const maxExpressionTokens = 512

type sourceLine struct {
	no   int
	text string
}

// parseExprBounded performs a complete lexical pass before invoking the
// expression parser. This guarantees malformed characters are rejected before
// parser state can advance, and caps expression complexity independently of
// the document-size limit.
func parseExprBounded(src string) (Expr, error) {
	l := lexer{src: src}
	tokens := 0
	for {
		t, err := l.next()
		if err != nil {
			return nil, err
		}
		if t.typ == tokEOF {
			break
		}
		tokens++
		if tokens > maxExpressionTokens {
			return nil, fmt.Errorf("expression exceeds %d-token limit", maxExpressionTokens)
		}
	}
	return ParseExpr(src)
}

func ParseFile(path string) (*Document, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > 2*1024*1024 {
		return nil, fmt.Errorf("%s: document exceeds 2 MiB limit", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []sourceLine
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), 256*1024)
	n := 0
	for s.Scan() {
		n++
		lines = append(lines, sourceLine{n, s.Text()})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	nodes, idx, err := parseNodes(lines, 0, false, 0)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if idx != len(lines) {
		return nil, fmt.Errorf("%s: parser stopped early at line %d", path, lines[idx].no)
	}
	return &Document{Path: path, Nodes: nodes}, nil
}

func ParseString(name, src string) (*Document, error) {
	if len(src) > 2*1024*1024 {
		return nil, fmt.Errorf("%s: document exceeds 2 MiB limit", name)
	}
	raw := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	lines := make([]sourceLine, len(raw))
	for i, t := range raw {
		lines[i] = sourceLine{i + 1, t}
	}
	nodes, idx, err := parseNodes(lines, 0, false, 0)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if idx != len(lines) {
		return nil, fmt.Errorf("%s: parser stopped early", name)
	}
	return &Document{Path: name, Nodes: nodes}, nil
}

func parseNodes(lines []sourceLine, start int, stopAtEnd bool, depth int) ([]Node, int, error) {
	if depth > 64 {
		return nil, start, fmt.Errorf("block nesting exceeds 64 levels")
	}
	nodes := []Node{}
	var md []sourceLine
	inFence := false
	flush := func() {
		if len(md) == 0 {
			return
		}
		parts := make([]string, len(md))
		for i, l := range md {
			parts[i] = l.text
		}
		nodes = append(nodes, MarkdownNode{Line: md[0].no, Text: strings.Join(parts, "\n")})
		md = nil
	}

	for i := start; i < len(lines); {
		line := lines[i]
		trim := strings.TrimSpace(line.text)
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			md = append(md, line)
			inFence = !inFence
			i++
			continue
		}
		if inFence {
			md = append(md, line)
			i++
			continue
		}
		if trim == "@end" {
			if !stopAtEnd {
				return nil, i, fmt.Errorf("line %d: unexpected @end", line.no)
			}
			flush()
			return nodes, i + 1, nil
		}

		if pos := strings.Index(line.text, "@input "); pos >= 0 {
			prefix := strings.TrimSpace(line.text[:pos])
			rest := strings.TrimSpace(line.text[pos+len("@input "):])
			m := inputRE.FindStringSubmatch(rest)
			if m != nil {
				flush()
				e, err := parseExprBounded(m[3])
				if err != nil {
					return nil, i, fmt.Errorf("line %d: invalid @input default: %w", line.no, err)
				}
				nodes = append(nodes, InputNode{Line: line.no, Prefix: prefix, Name: m[1], Type: m[2], DefaultSource: m[3], Default: e})
				i++
				continue
			}
		}

		if strings.HasPrefix(trim, "@calc ") {
			flush()
			m := calcRE.FindStringSubmatch(strings.TrimSpace(strings.TrimPrefix(trim, "@calc ")))
			if m == nil {
				return nil, i, fmt.Errorf("line %d: expected @calc name = expression", line.no)
			}
			e, err := parseExprBounded(m[2])
			if err != nil {
				return nil, i, fmt.Errorf("line %d: invalid @calc expression: %w", line.no, err)
			}
			nodes = append(nodes, CalcNode{Line: line.no, Name: m[1], Source: m[2], Expr: e})
			i++
			continue
		}
		if strings.HasPrefix(trim, "@show ") {
			flush()
			src := strings.TrimSpace(strings.TrimPrefix(trim, "@show "))
			e, err := parseExprBounded(src)
			if err != nil {
				return nil, i, fmt.Errorf("line %d: invalid @show expression: %w", line.no, err)
			}
			nodes = append(nodes, ShowNode{Line: line.no, Source: src, Expr: e})
			i++
			continue
		}
		if strings.HasPrefix(trim, "@assert ") {
			flush()
			src := strings.TrimSpace(strings.TrimPrefix(trim, "@assert "))
			e, err := parseExprBounded(src)
			if err != nil {
				return nil, i, fmt.Errorf("line %d: invalid @assert expression: %w", line.no, err)
			}
			i++
			var msg []string
			for i < len(lines) && strings.TrimSpace(lines[i].text) != "@end" {
				msg = append(msg, lines[i].text)
				i++
			}
			if i >= len(lines) {
				return nil, i, fmt.Errorf("line %d: @assert missing @end", line.no)
			}
			nodes = append(nodes, AssertNode{Line: line.no, Source: src, Expr: e, Message: strings.TrimSpace(strings.Join(msg, "\n"))})
			i++
			continue
		}
		if strings.HasPrefix(trim, "@when ") {
			flush()
			src := strings.TrimSpace(strings.TrimPrefix(trim, "@when "))
			e, err := parseExprBounded(src)
			if err != nil {
				return nil, i, fmt.Errorf("line %d: invalid @when expression: %w", line.no, err)
			}
			inner, next, err := parseNodes(lines, i+1, true, depth+1)
			if err != nil {
				return nil, i, err
			}
			nodes = append(nodes, WhenNode{Line: line.no, Source: src, Expr: e, Nodes: inner})
			i = next
			continue
		}
		if strings.HasPrefix(trim, "@chart") {
			flush()
			name := strings.TrimSpace(strings.TrimPrefix(trim, "@chart"))
			if name == "" {
				name = "chart"
			}
			cfg, next, err := parseConfigBlock(lines, i+1, line.no, "@chart")
			if err != nil {
				return nil, i, err
			}
			typ := "bar"
			if s, ok := cfg["type"]; ok {
				typ = strings.Trim(strings.TrimSpace(s), "\"'")
			}
			labelsSrc, okL := cfg["labels"]
			valuesSrc, okV := cfg["values"]
			if !okL || !okV {
				return nil, i, fmt.Errorf("line %d: @chart requires labels and values", line.no)
			}
			labels, err := parseExprBounded(labelsSrc)
			if err != nil {
				return nil, i, fmt.Errorf("line %d: invalid chart labels: %w", line.no, err)
			}
			values, err := parseExprBounded(valuesSrc)
			if err != nil {
				return nil, i, fmt.Errorf("line %d: invalid chart values: %w", line.no, err)
			}
			nodes = append(nodes, ChartNode{Line: line.no, Name: name, Type: typ, LabelsSource: labelsSrc, ValuesSource: valuesSrc, Labels: labels, Values: values})
			i = next
			continue
		}
		if strings.HasPrefix(trim, "@table") {
			flush()
			name := strings.TrimSpace(strings.TrimPrefix(trim, "@table"))
			if name == "" {
				name = "table"
			}
			cfg, next, err := parseConfigBlock(lines, i+1, line.no, "@table")
			if err != nil {
				return nil, i, err
			}
			csrc, okC := cfg["columns"]
			rsrc, okR := cfg["rows"]
			if !okC || !okR {
				return nil, i, fmt.Errorf("line %d: @table requires columns and rows", line.no)
			}
			c, err := parseExprBounded(csrc)
			if err != nil {
				return nil, i, fmt.Errorf("line %d: invalid table columns: %w", line.no, err)
			}
			r, err := parseExprBounded(rsrc)
			if err != nil {
				return nil, i, fmt.Errorf("line %d: invalid table rows: %w", line.no, err)
			}
			nodes = append(nodes, TableNode{Line: line.no, Name: name, ColumnsSource: csrc, RowsSource: rsrc, Columns: c, Rows: r})
			i = next
			continue
		}
		if strings.HasPrefix(trim, "@") {
			flush()
			return nil, i, fmt.Errorf("line %d: unknown directive %q", line.no, strings.Fields(trim)[0])
		}
		md = append(md, line)
		i++
	}
	if stopAtEnd {
		return nil, len(lines), fmt.Errorf("unterminated block: expected @end")
	}
	flush()
	return nodes, len(lines), nil
}

func parseConfigBlock(lines []sourceLine, start, parentLine int, kind string) (map[string]string, int, error) {
	cfg := map[string]string{}
	for i := start; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i].text)
		if trim == "@end" {
			return cfg, i + 1, nil
		}
		if trim == "" {
			continue
		}
		eq := strings.Index(trim, "=")
		if eq < 1 {
			return nil, i, fmt.Errorf("line %d: expected key = value inside %s", lines[i].no, kind)
		}
		k := strings.TrimSpace(trim[:eq])
		v := strings.TrimSpace(trim[eq+1:])
		cfg[k] = v
	}
	return nil, len(lines), fmt.Errorf("line %d: %s missing @end", parentLine, kind)
}
