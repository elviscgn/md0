package md0

import (
	"html"
	"strings"
)

func renderInline(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '`' {
			run := 1
			for i+run < len(s) && s[i+run] == '`' {
				run++
			}
			if closeAt := matchingBacktickRun(s, i+run, run); closeAt >= 0 {
				content := normalizeCodeSpan(s[i+run : closeAt])
				out.WriteString("<code>")
				out.WriteString(html.EscapeString(content))
				out.WriteString("</code>")
				i = closeAt + run
				continue
			}
			out.WriteString(html.EscapeString(s[i : i+run]))
			i += run
			continue
		}

		if i+1 < len(s) && s[i] == '*' && s[i+1] == '*' {
			if rel := strings.Index(s[i+2:], "**"); rel >= 0 {
				closeAt := i + 2 + rel
				if closeAt > i+2 {
					out.WriteString("<strong>")
					out.WriteString(renderInline(s[i+2 : closeAt]))
					out.WriteString("</strong>")
					i = closeAt + 2
					continue
				}
			}
		}

		start := i
		for i < len(s) {
			if s[i] == '`' || i+1 < len(s) && s[i] == '*' && s[i+1] == '*' {
				break
			}
			i++
		}
		if start == i {
			out.WriteString(html.EscapeString(s[i : i+1]))
			i++
			continue
		}
		out.WriteString(html.EscapeString(s[start:i]))
	}
	return out.String()
}

func normalizeCodeSpan(s string) string {
	// CommonMark code spans collapse line endings to spaces. renderInline is
	// currently line-oriented, but keeping the normalization here makes the
	// rule explicit and correct if multi-line spans are introduced later.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) >= 2 && s[0] == ' ' && s[len(s)-1] == ' ' && strings.Trim(s, " ") != "" {
		s = s[1 : len(s)-1]
	}
	return s
}

func renderMarkdown(s string) string {
	lines := strings.Split(s, "\n")
	var out strings.Builder
	var fence *markdownFence
	inList := false
	inPara := false
	closePara := func() {
		if inPara {
			out.WriteString("</p>\n")
			inPara = false
		}
	}
	closeList := func() {
		if inList {
			out.WriteString("</ul>\n")
			inList = false
		}
	}
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if fence != nil {
			if isFenceClose(line, *fence) {
				out.WriteString("</code></pre>\n")
				fence = nil
			} else {
				out.WriteString(html.EscapeString(line))
				out.WriteByte('\n')
			}
			continue
		}
		if marker, run, _, ok := fenceMarker(line); ok {
			closePara()
			closeList()
			out.WriteString("<pre><code>")
			fence = &markdownFence{marker: marker, length: run}
			continue
		}
		if trim == "" {
			closePara()
			closeList()
			continue
		}
		if strings.HasPrefix(trim, "#") {
			n := 0
			for n < len(trim) && trim[n] == '#' {
				n++
			}
			if n <= 6 && n < len(trim) && trim[n] == ' ' {
				closePara()
				closeList()
				out.WriteString("<h")
				out.WriteByte(byte('0' + n))
				out.WriteString(">")
				out.WriteString(renderInline(strings.TrimSpace(trim[n:])))
				out.WriteString("</h")
				out.WriteByte(byte('0' + n))
				out.WriteString(">\n")
				continue
			}
		}
		if strings.HasPrefix(trim, "- ") {
			closePara()
			if !inList {
				out.WriteString("<ul>\n")
				inList = true
			}
			out.WriteString("<li>")
			out.WriteString(renderInline(strings.TrimSpace(trim[2:])))
			out.WriteString("</li>\n")
			continue
		}
		closeList()
		if !inPara {
			out.WriteString("<p>")
			inPara = true
		} else {
			out.WriteByte(' ')
		}
		out.WriteString(renderInline(trim))
	}
	closePara()
	closeList()
	if fence != nil {
		out.WriteString("</code></pre>\n")
	}
	return out.String()
}
