package main

import (
	"bufio"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLauncherKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  launcherKey
	}{
		{name: "up arrow", input: "\x1b[A", want: launcherKeyUp},
		{name: "down arrow", input: "\x1b[B", want: launcherKeyDown},
		{name: "application down arrow", input: "\x1bOB", want: launcherKeyDown},
		{name: "j", input: "j", want: launcherKeyDown},
		{name: "k", input: "k", want: launcherKeyUp},
		{name: "enter", input: "\r", want: launcherKeySelect},
		{name: "edit", input: "e", want: launcherKeyEdit},
		{name: "quit", input: "q", want: launcherKeyQuit},
		{name: "control c", input: "\x03", want: launcherKeyQuit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := readLauncherKey(bufio.NewReader(strings.NewReader(test.input)))
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("key=%v, want %v", got, test.want)
			}
		})
	}
}

func TestLauncherViewIsCompactAndMarksSelection(t *testing.T) {
	u := terminalUI{color: false}
	path := filepath.Join("examples", "math-playground.md")
	view := u.launcherView(path, 2, true)
	for _, want := range []string{
		"███╗   ███╗  ██████╗    ██████╗",
		"md0/PURE 0.1",
		"document  " + path,
		"› r  Render standalone HTML",
		"↑↓ / jk navigate   enter select",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("launcher view missing %q:\n%s", want, view)
		}
	}
}

func TestLauncherViewUsesColorPalette(t *testing.T) {
	view := (terminalUI{color: true}).launcherView("report.md", 2, true)
	for _, color := range []string{ansiIvory, ansiCoral, ansiSand, ansiRed} {
		if !strings.Contains(view, color) {
			t.Fatalf("launcher view missing ANSI color %q", color)
		}
	}
	for _, color := range []string{ansiCyan, ansiMagenta} {
		if strings.Contains(view, color) {
			t.Fatalf("launcher view unexpectedly uses extra ANSI color %q", color)
		}
	}
}

func TestTerminalColorAcceptsNamesIndexesAndHexWithoutRawANSI(t *testing.T) {
	const fallback = "fallback"
	tests := []struct {
		value string
		want  string
	}{
		{value: "sage", want: "\x1b[38;5;71m"},
		{value: "209", want: "\x1b[38;5;209m"},
		{value: "#12aBcF", want: "\x1b[38;2;18;171;207m"},
		{value: "\x1b[31m", want: fallback},
		{value: "#nope00", want: fallback},
		{value: "999", want: fallback},
	}
	for _, test := range tests {
		if got := terminalColor(test.value, fallback); got != test.want {
			t.Fatalf("terminalColor(%q)=%q, want %q", test.value, got, test.want)
		}
	}
}

func TestLauncherViewUsesConfiguredPalette(t *testing.T) {
	palette := defaultTerminalPalette()
	palette.md = terminalColor("#abcdef", palette.md)
	palette.zero = terminalColor("sage", palette.zero)
	view := (terminalUI{color: true, palette: palette}).launcherView("report.md", 0, true)
	for _, want := range []string{"\x1b[38;2;171;205;239m", "\x1b[38;5;71m"} {
		if !strings.Contains(view, want) {
			t.Fatalf("launcher view missing configured color %q", want)
		}
	}
}

func TestSafeTerminalTextRemovesControlCharacters(t *testing.T) {
	got := safeTerminalText("report\n\x1b[31m.md")
	if got != "report��[31m.md" {
		t.Fatalf("safe text=%q", got)
	}
}

func TestCompactTerminalTextKeepsBothEnds(t *testing.T) {
	got := compactTerminalText("examples/a/very/long/path/to/math-playground.md", 24)
	if got != "examples/a/…layground.md" {
		t.Fatalf("compact text=%q", got)
	}
}
