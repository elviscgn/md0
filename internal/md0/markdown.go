package md0

import (
	"html"
	"strings"
	"unicode"
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

		if s[i] == '$' && (i+1 >= len(s) || s[i+1] != '$') && inlineMathCanOpen(s, i) {
			if closeAt := inlineMathClose(s, i+1); closeAt >= 0 {
				out.WriteString(renderInlineMath(s[i+1 : closeAt]))
				i = closeAt + 1
				continue
			}
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
			if s[i] == '`' || s[i] == '$' || i+1 < len(s) && s[i] == '*' && s[i+1] == '*' {
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

func inlineMathCanOpen(s string, at int) bool {
	if at+1 >= len(s) || unicode.IsSpace(rune(s[at+1])) {
		return false
	}
	if at > 0 && s[at-1] == '\\' {
		return false
	}
	return true
}

func inlineMathClose(s string, start int) int {
	for i := start; i < len(s); i++ {
		if s[i] != '$' || i > 0 && s[i-1] == '\\' {
			continue
		}
		if i == start || unicode.IsSpace(rune(s[i-1])) {
			continue
		}
		return i
	}
	return -1
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
	fenceKind := ""
	var semanticFence strings.Builder
	inDisplayMath := false
	var displayMath strings.Builder
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

		if inDisplayMath {
			if trim == "$$" {
				out.WriteString(renderDisplayMath(strings.TrimSpace(displayMath.String())))
				out.WriteByte('\n')
				displayMath.Reset()
				inDisplayMath = false
			} else {
				if displayMath.Len() > 0 {
					displayMath.WriteByte('\n')
				}
				displayMath.WriteString(line)
			}
			continue
		}

		if fence != nil {
			if isFenceClose(line, *fence) {
				if fenceKind == "plot" {
					out.WriteString(renderPlotFence(semanticFence.String()))
					out.WriteByte('\n')
					semanticFence.Reset()
				} else {
					out.WriteString("</code></pre>\n")
				}
				fence = nil
				fenceKind = ""
			} else if fenceKind == "plot" {
				if semanticFence.Len() > 0 {
					semanticFence.WriteByte('\n')
				}
				semanticFence.WriteString(line)
			} else {
				out.WriteString(html.EscapeString(line))
				out.WriteByte('\n')
			}
			continue
		}
		if marker, run, info, ok := fenceMarker(line); ok {
			closePara()
			closeList()
			kind := strings.ToLower(strings.TrimSpace(info))
			if kind == "plot" || kind == "md0-plot" {
				fenceKind = "plot"
				semanticFence.Reset()
			} else {
				out.WriteString("<pre><code>")
			}
			fence = &markdownFence{marker: marker, length: run}
			continue
		}

		if trim == "$$" {
			closePara()
			closeList()
			inDisplayMath = true
			displayMath.Reset()
			continue
		}
		if strings.HasPrefix(trim, "$$") && strings.HasSuffix(trim, "$$") && len(trim) > 4 {
			closePara()
			closeList()
			out.WriteString(renderDisplayMath(strings.TrimSpace(trim[2 : len(trim)-2])))
			out.WriteByte('\n')
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
	if inDisplayMath {
		out.WriteString(renderDisplayMath(strings.TrimSpace(displayMath.String())))
		out.WriteByte('\n')
	}
	if fence != nil {
		if fenceKind == "plot" {
			out.WriteString(renderPlotFence(semanticFence.String()))
		} else {
			out.WriteString("</code></pre>\n")
		}
	}
	return out.String()
}
