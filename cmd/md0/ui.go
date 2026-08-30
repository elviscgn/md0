package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type terminalUI struct {
	out     io.Writer
	color   bool
	palette terminalPalette
}

type terminalPalette struct {
	md        string
	zero      string
	accent    string
	secondary string
	success   string
	error     string
}

const (
	ansiReset   = "\x1b[0m"
	ansiBold    = "\x1b[1m"
	ansiDim     = "\x1b[2m"
	ansiBlack   = "\x1b[30m"
	ansiBlue    = "\x1b[34m"
	ansiCyan    = "\x1b[36m"
	ansiGreen   = "\x1b[32m"
	ansiYellow  = "\x1b[33m"
	ansiRed     = "\x1b[31m"
	ansiMagenta = "\x1b[35m"
	ansiWhite   = "\x1b[37m"
	ansiGray    = "\x1b[90m"
	ansiIvory   = "\x1b[38;5;230m"
	ansiCoral   = "\x1b[38;5;209m"
	ansiSand    = "\x1b[38;5;180m"
)

var cliUI = newTerminalUI(os.Stdout)

type md0LogoRow struct {
	m    string
	d    string
	zero string
}

var md0LogoRows = []md0LogoRow{
	{m: "███╗   ███╗", d: "██████╗ ", zero: " ██████╗ "},
	{m: "████╗ ████║", d: "██╔══██╗", zero: "██╔═████╗"},
	{m: "██╔████╔██║", d: "██║  ██║", zero: "██║██╔██║"},
	{m: "██║╚██╔╝██║", d: "██║  ██║", zero: "████╔╝██║"},
	{m: "██║ ╚═╝ ██║", d: "██████╔╝", zero: "╚██████╔╝"},
	{m: "╚═╝     ╚═╝", d: "╚═════╝ ", zero: " ╚═════╝ "},
}

func newTerminalUI(out *os.File) terminalUI {
	color := false
	if os.Getenv("NO_COLOR") == "" && !strings.EqualFold(os.Getenv("TERM"), "dumb") {
		if info, err := out.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			color = true
		}
	}
	return terminalUI{out: out, color: color, palette: terminalPaletteFromEnv()}
}

func defaultTerminalPalette() terminalPalette {
	return terminalPalette{
		md:        ansiIvory,
		zero:      ansiCoral,
		accent:    ansiCoral,
		secondary: ansiSand,
		success:   ansiGreen,
		error:     ansiRed,
	}
}

func terminalPaletteFromEnv() terminalPalette {
	palette := defaultTerminalPalette()
	palette.md = terminalColor(os.Getenv("MD0_COLOR_MD"), palette.md)
	palette.zero = terminalColor(os.Getenv("MD0_COLOR_ZERO"), palette.zero)
	palette.accent = terminalColor(os.Getenv("MD0_COLOR_ACCENT"), palette.accent)
	palette.secondary = terminalColor(os.Getenv("MD0_COLOR_SECONDARY"), palette.secondary)
	palette.success = terminalColor(os.Getenv("MD0_COLOR_SUCCESS"), palette.success)
	palette.error = terminalColor(os.Getenv("MD0_COLOR_ERROR"), palette.error)
	return palette
}

func terminalColor(value, fallback string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "default" {
		return fallback
	}
	named := map[string]string{
		"black": ansiBlack, "red": ansiRed, "green": ansiGreen,
		"yellow": ansiYellow, "blue": ansiBlue, "magenta": ansiMagenta,
		"cyan": ansiCyan, "white": ansiWhite, "gray": ansiGray,
		"grey": ansiGray, "ivory": ansiIvory, "coral": ansiCoral,
		"sand": ansiSand, "sage": "\x1b[38;5;71m", "orange": "\x1b[38;5;208m",
	}
	if color, ok := named[value]; ok {
		return color
	}
	if strings.HasPrefix(value, "#") && len(value) == 7 {
		if rgb, err := strconv.ParseUint(value[1:], 16, 24); err == nil {
			return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", rgb>>16, rgb>>8&0xff, rgb&0xff)
		}
	}
	if index, err := strconv.Atoi(value); err == nil && index >= 0 && index <= 255 {
		return fmt.Sprintf("\x1b[38;5;%dm", index)
	}
	return fallback
}

func (u terminalUI) colors() terminalPalette {
	if u.palette.md == "" {
		return defaultTerminalPalette()
	}
	return u.palette
}

func (u terminalUI) paint(code, text string) string {
	if !u.color {
		return text
	}
	return code + text + ansiReset
}

func (u terminalUI) logo() {
	fmt.Fprintln(u.out, u.asciiLogo())
	u.writeIdentity(u.out)
}

func (u terminalUI) command(name string) {
	fmt.Fprintf(u.out, "%s  %s\n\n", u.paint(ansiBold+u.colors().accent, "md0"), u.paint(ansiBold, name))
}

func (u terminalUI) meta(label, value string) {
	fmt.Fprintf(u.out, "%s  %-11s %s\n", u.paint(u.colors().secondary, "◆"), u.paint(ansiDim, label), value)
}

func (u terminalUI) success(message string) {
	fmt.Fprintf(u.out, "%s %s\n", u.paint(u.colors().success, "✓"), message)
}

func (u terminalUI) warning(message string) {
	fmt.Fprintf(u.out, "%s %s\n", u.paint(u.colors().secondary, "!"), message)
}

func (u terminalUI) action(message string) {
	fmt.Fprintf(u.out, "%s %s\n", u.paint(u.colors().accent, "→"), message)
}

func (u terminalUI) fail(message string) {
	fmt.Fprintf(u.out, "%s %s\n", u.paint(u.colors().error, "×"), message)
}

func (u terminalUI) launcherView(path string, selected int, interactive bool) string {
	var b strings.Builder
	palette := u.colors()
	path = compactTerminalText(safeTerminalText(filepath.Clean(path)), 56)

	fmt.Fprintln(&b, u.asciiLogo())
	u.writeIdentity(&b)
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "%s  %s\n", u.paint(ansiBold+palette.secondary, "document"), u.paint(ansiBold, path))
	fmt.Fprintln(&b, u.paint(ansiDim+palette.accent, "────────────────────────────────────────────────────────"))

	for index, option := range launcherOptions {
		marker := "  "
		label := option.label
		key := u.paint(ansiDim, option.key)
		if index == selected {
			color := palette.accent
			if option.action == launcherQuit {
				color = palette.error
			}
			marker = u.paint(ansiBold+color, "› ")
			label = u.paint(ansiBold+color, label)
			key = u.paint(ansiBold+color, option.key)
		}
		padding := 24 - len(option.label)
		if padding < 1 {
			padding = 1
		}
		fmt.Fprintf(&b, "%s%s  %s%s%s\n", marker, key, label, strings.Repeat(" ", padding), u.paint(ansiDim, option.hint))
	}

	fmt.Fprintln(&b)
	if interactive {
		fmt.Fprintf(&b, "%s %s   %s %s   %s %s   %s %s\n",
			u.paint(ansiBold+palette.accent, "↑↓ / jk"), u.paint(ansiDim, "navigate"),
			u.paint(ansiBold+palette.secondary, "enter"), u.paint(ansiDim, "select"),
			u.paint(ansiBold+palette.secondary, "?"), u.paint(ansiDim, "help"),
			u.paint(ansiBold+palette.error, "q"), u.paint(ansiDim, "quit"))
	} else {
		fmt.Fprintln(&b, u.paint(ansiDim, "Type a shortcut and press Enter"))
	}
	return b.String()
}

func (u terminalUI) asciiLogo() string {
	var b strings.Builder
	palette := u.colors()
	for index, row := range md0LogoRows {
		if index > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(u.paint(ansiBold+palette.md, row.m))
		b.WriteString("  ")
		b.WriteString(u.paint(ansiBold+palette.md, row.d))
		b.WriteString("  ")
		b.WriteString(u.paint(ansiBold+palette.zero, row.zero))
	}
	return b.String()
}

func (u terminalUI) writeIdentity(out io.Writer) {
	palette := u.colors()
	fmt.Fprintf(out, "%s %s %s %s %s\n",
		u.paint(ansiBold+palette.md, "md0/PURE 0.1"),
		u.paint(ansiDim, "·"),
		u.paint(palette.secondary, "reactive Markdown"),
		u.paint(ansiDim, "·"),
		u.paint(palette.zero, "zero dependencies"))
}

func safeTerminalText(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r >= 0x80 && r <= 0x9f {
			return '�'
		}
		return r
	}, value)
}

func compactTerminalText(value string, limit int) string {
	runes := []rune(value)
	if limit < 2 || len(runes) <= limit {
		return value
	}
	left := (limit - 1) / 2
	right := limit - left - 1
	return string(runes[:left]) + "…" + string(runes[len(runes)-right:])
}

func cliError(message string) {
	u := newTerminalUI(os.Stderr)
	u.fail(message)
}
