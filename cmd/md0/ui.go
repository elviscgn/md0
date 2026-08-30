package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type terminalUI struct {
	out   io.Writer
	color bool
}

const (
	ansiReset   = "\x1b[0m"
	ansiBold    = "\x1b[1m"
	ansiDim     = "\x1b[2m"
	ansiCyan    = "\x1b[36m"
	ansiGreen   = "\x1b[32m"
	ansiYellow  = "\x1b[33m"
	ansiRed     = "\x1b[31m"
	ansiMagenta = "\x1b[35m"
)

var cliUI = newTerminalUI(os.Stdout)

func newTerminalUI(out *os.File) terminalUI {
	color := false
	if os.Getenv("NO_COLOR") == "" && !strings.EqualFold(os.Getenv("TERM"), "dumb") {
		if info, err := out.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			color = true
		}
	}
	return terminalUI{out: out, color: color}
}

func (u terminalUI) paint(code, text string) string {
	if !u.color {
		return text
	}
	return code + text + ansiReset
}

func (u terminalUI) logo() {
	logo := "█▀▄▀█  █▀▄  █▀█\n█ ▀ █  █▄▀  █▄█"
	fmt.Fprintln(u.out, u.paint(ansiBold+ansiCyan, logo))
	fmt.Fprintln(u.out, u.paint(ansiDim, "reactive Markdown · zero dependencies"))
}

func (u terminalUI) command(name string) {
	fmt.Fprintf(u.out, "%s  %s\n\n", u.paint(ansiBold+ansiCyan, "md0"), u.paint(ansiBold, name))
}

func (u terminalUI) meta(label, value string) {
	fmt.Fprintf(u.out, "%s  %-11s %s\n", u.paint(ansiMagenta, "◆"), u.paint(ansiDim, label), value)
}

func (u terminalUI) success(message string) {
	fmt.Fprintf(u.out, "%s %s\n", u.paint(ansiGreen, "✓"), message)
}

func (u terminalUI) warning(message string) {
	fmt.Fprintf(u.out, "%s %s\n", u.paint(ansiYellow, "!"), message)
}

func (u terminalUI) action(message string) {
	fmt.Fprintf(u.out, "%s %s\n", u.paint(ansiCyan, "→"), message)
}

func (u terminalUI) fail(message string) {
	fmt.Fprintf(u.out, "%s %s\n", u.paint(ansiRed, "×"), message)
}

func (u terminalUI) choice(key, label, hint string, primary bool) {
	marker := " "
	if primary {
		marker = u.paint(ansiCyan, "›")
	}
	shortcut := u.paint(ansiBold+ansiCyan, "["+key+"]")
	if hint != "" {
		fmt.Fprintf(u.out, "%s %s %-24s %s\n", marker, shortcut, label, u.paint(ansiDim, hint))
		return
	}
	fmt.Fprintf(u.out, "%s %s %s\n", marker, shortcut, label)
}

func cliError(message string) {
	u := newTerminalUI(os.Stderr)
	u.fail(message)
}
