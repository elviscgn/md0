package md0

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	lineDiagnosticRE = regexp.MustCompile(`^line ([0-9]+): (.+)$`)
	trailingPosRE    = regexp.MustCompile(`^(.*) at ([0-9]+)$`)
)

// FormatDiagnostic turns md0's line-oriented parser/evaluator errors into a
// compiler-style source diagnostic when the original document is available.
// It deliberately leaves non-source errors (I/O, listen failures, etc.) alone.
func FormatDiagnostic(path string, err error) string {
	if err == nil {
		return ""
	}
	raw := err.Error()
	if path != "" {
		raw = strings.TrimPrefix(raw, path+": ")
	}
	firstLine := raw
	if i := strings.IndexByte(firstLine, '\n'); i >= 0 {
		firstLine = firstLine[:i]
	}
	match := lineDiagnosticRE.FindStringSubmatch(firstLine)
	if match == nil {
		return err.Error()
	}
	lineNo, convErr := strconv.Atoi(match[1])
	if convErr != nil || lineNo < 1 {
		return err.Error()
	}
	source, ok := readSourceLine(path, lineNo)
	if !ok {
		return err.Error()
	}

	message := match[2]
	column := firstNonSpaceColumn(source)
	if posMatch := trailingPosRE.FindStringSubmatch(message); posMatch != nil {
		if exprPos, posErr := strconv.Atoi(posMatch[2]); posErr == nil && exprPos > 0 {
			message = posMatch[1]
			column = inferExpressionColumn(source, message, exprPos)
		}
	}
	if column < 1 {
		column = 1
	}

	location := fmt.Sprintf("%s:%d:%d", path, lineNo, column)
	lineLabel := strconv.Itoa(lineNo)
	caret := visualCaretPrefix(source, column)
	return fmt.Sprintf("%s: %s\n  %s | %s\n  %s | %s^", location, message, lineLabel, source, strings.Repeat(" ", len(lineLabel)), caret)
}

func readSourceLine(path string, target int) (string, bool) {
	if path == "" {
		return "", false
	}
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 256*1024)
	line := 0
	for scanner.Scan() {
		line++
		if line == target {
			return scanner.Text(), true
		}
	}
	return "", false
}

func firstNonSpaceColumn(source string) int {
	for i, r := range source {
		if r != ' ' && r != '\t' {
			return i + 1
		}
	}
	return 1
}

func inferExpressionColumn(source, message string, exprPos int) int {
	start := -1
	if strings.Contains(message, "@calc") || strings.Contains(message, "@input") {
		if eq := strings.Index(source, "="); eq >= 0 {
			start = eq + 1
		}
	} else {
		for _, marker := range []string{"@show", "@assert", "@when"} {
			if strings.Contains(message, marker) {
				if idx := strings.Index(source, marker); idx >= 0 {
					start = idx + len(marker)
				}
				break
			}
		}
		if start < 0 && strings.Contains(message, "interpolation") {
			if idx := strings.Index(source, "{{"); idx >= 0 {
				start = idx + 2
			}
		}
	}
	if start < 0 {
		return firstNonSpaceColumn(source)
	}
	for start < len(source) && (source[start] == ' ' || source[start] == '\t') {
		start++
	}
	column := start + exprPos
	if column > len(source)+1 {
		column = len(source) + 1
	}
	return column
}

func visualCaretPrefix(source string, column int) string {
	byteCount := column - 1
	if byteCount < 0 {
		byteCount = 0
	}
	if byteCount > len(source) {
		byteCount = len(source)
	}
	prefix := source[:byteCount]
	var b strings.Builder
	for _, r := range prefix {
		if r == '\t' {
			b.WriteString("    ")
		} else {
			b.WriteByte(' ')
		}
	}
	return b.String()
}
