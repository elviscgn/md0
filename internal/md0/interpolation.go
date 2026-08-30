package md0

import (
	"fmt"
	"strings"
)

// transformMarkdownInterpolations visits {{ ... }} expressions only where
// Markdown treats them as prose. Fenced code, inline code, and escaped opening
// braces remain literal document text.
func transformMarkdownInterpolations(text string, replace func(expr, raw string) (string, error)) (string, error) {
	lines := strings.SplitAfter(text, "\n")
	var out strings.Builder
	var fence *markdownFence

	for _, part := range lines {
		line := strings.TrimSuffix(part, "\n")
		hasNewline := len(part) > len(line)

		if fence != nil {
			out.WriteString(line)
			if isFenceClose(line, *fence) {
				fence = nil
			}
		} else if marker, run, _, ok := fenceMarker(line); ok {
			out.WriteString(line)
			fence = &markdownFence{marker: marker, length: run}
		} else {
			transformed, err := transformInlineInterpolations(line, replace)
			if err != nil {
				return "", err
			}
			out.WriteString(transformed)
		}
		if hasNewline {
			out.WriteByte('\n')
		}
	}
	return out.String(), nil
}

func transformInlineInterpolations(line string, replace func(expr, raw string) (string, error)) (string, error) {
	var out strings.Builder
	for i := 0; i < len(line); {
		if line[i] == '\\' {
			out.WriteByte(line[i])
			i++
			if i < len(line) {
				out.WriteByte(line[i])
				i++
			}
			continue
		}
		if line[i] == '`' {
			run := 1
			for i+run < len(line) && line[i+run] == '`' {
				run++
			}
			if closeAt := matchingBacktickRun(line, i+run, run); closeAt >= 0 {
				end := closeAt + run
				out.WriteString(line[i:end])
				i = end
				continue
			}
			out.WriteString(line[i : i+run])
			i += run
			continue
		}
		if i+1 < len(line) && line[i] == '{' && line[i+1] == '{' {
			end, ok := interpolationEnd(line, i+2)
			if !ok {
				out.WriteString(line[i:])
				break
			}
			raw := line[i : end+2]
			expr := strings.TrimSpace(line[i+2 : end])
			if expr == "" {
				return "", fmt.Errorf("empty interpolation")
			}
			value, err := replace(expr, raw)
			if err != nil {
				return "", err
			}
			out.WriteString(value)
			i = end + 2
			continue
		}
		out.WriteByte(line[i])
		i++
	}
	return out.String(), nil
}

func interpolationEnd(line string, start int) (int, bool) {
	var quote byte
	escaped := false
	for i := start; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == '}' && i+1 < len(line) && line[i+1] == '}' {
			return i, true
		}
	}
	return 0, false
}

func markdownInterpolationDependencies(text string) ([]string, error) {
	set := map[string]struct{}{}
	_, err := transformMarkdownInterpolations(text, func(expr, raw string) (string, error) {
		parsed, err := parseExprBounded(expr)
		if err != nil {
			return "", err
		}
		deps, err := ExprDependencies(parsed)
		if err != nil {
			return "", err
		}
		for _, dep := range deps {
			set[dep] = struct{}{}
		}
		return raw, nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(set))
	for dep := range set {
		out = append(out, dep)
	}
	return uniqueSorted(out), nil
}

func interpolateMarkdown(text string, env map[string]Value) (string, error) {
	return transformMarkdownInterpolations(text, func(expr, _ string) (string, error) {
		parsed, err := parseExprBounded(expr)
		if err != nil {
			return "", err
		}
		value, err := parsed.Eval(env)
		if err != nil {
			return "", err
		}
		return value.String(), nil
	})
}
