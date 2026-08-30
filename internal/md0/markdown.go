package md0

import (
	"html"
	"regexp"
	"strings"
)

var (
	boldRE = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	codeRE = regexp.MustCompile("`([^`]+)`")
)

func renderInline(s string) string {
	s = html.EscapeString(s)
	s = codeRE.ReplaceAllString(s, "<code>$1</code>")
	s = boldRE.ReplaceAllString(s, "<strong>$1</strong>")
	return s
}

func renderMarkdown(s string) string {
	lines := strings.Split(s, "\n")
	var out strings.Builder
	inCode := false
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
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			closePara()
			closeList()
			if !inCode {
				out.WriteString("<pre><code>")
				inCode = true
			} else {
				out.WriteString("</code></pre>\n")
				inCode = false
			}
			continue
		}
		if inCode {
			out.WriteString(html.EscapeString(line))
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
	if inCode {
		out.WriteString("</code></pre>\n")
	}
	return out.String()
}
