package md0

import (
	"fmt"
	"strings"
)

const maxInterpolatedMarkdownBytes = 4 * 1024 * 1024

func writeInterpolationBounded(out *strings.Builder, value string) error {
	if len(value) > maxInterpolatedMarkdownBytes-out.Len() {
		return fmt.Errorf("interpolated Markdown exceeds 4 MiB limit")
	}
	out.WriteString(value)
	return nil
}

// transformMarkdownInterpolations visits {{ ... }} expressions where md0
// treats them as reactive prose. Ordinary fenced code, inline code, and escaped
// opening braces remain literal. A plot fence is intentionally different: its
// configuration is declarative md0 content, so {{ ... }} values inside it are
// tracked and interpolated before the bounded plot renderer sees the formula.
func transformMarkdownInterpolations(text string, replace func(expr, raw string) (string, error)) (string, error) {
	lines := strings.SplitAfter(text, "\n")
	var out strings.Builder
	var fence *markdownFence
	plotFence := false

	for _, part := range lines {
		line := strings.TrimSuffix(part, "\n")
		hasNewline := len(part) > len(line)

		if fence != nil {
			if isFenceClose(line, *fence) {
				if err := writeInterpolationBounded(&out, line); err != nil {
					return "", err
				}
				fence = nil
				plotFence = false
			} else if plotFence {
				transformed, err := transformInlineInterpolations(line, replace)
				if err != nil {
					return "", err
				}
				if err := writeInterpolationBounded(&out, transformed); err != nil {
					return "", err
				}
			} else {
				if err := writeInterpolationBounded(&out, line); err != nil {
					return "", err
				}
			}
		} else if marker, run, info, ok := fenceMarker(line); ok {
			if err := writeInterpolationBounded(&out, line); err != nil {
				return "", err
			}
			fence = &markdownFence{marker: marker, length: run}
			kind := strings.ToLower(strings.TrimSpace(info))
			plotFence = kind == "plot" || kind == "md0-plot"
		} else {
			transformed, err := transformInlineInterpolations(line, replace)
			if err != nil {
				return "", err
			}
			if err := writeInterpolationBounded(&out, transformed); err != nil {
				return "", err
			}
		}
		if hasNewline {
			if out.Len() >= maxInterpolatedMarkdownBytes {
				return "", fmt.Errorf("interpolated Markdown exceeds 4 MiB limit")
			}
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
			if err := writeInterpolationBounded(&out, value); err != nil {
				return "", err
			}
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
