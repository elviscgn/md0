//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !windows

package main

import (
	"errors"
	"os"
)

func enableRawTerminal(_ *os.File) (func() error, error) {
	return nil, errors.New("interactive terminal mode is unavailable")
}

func terminalSize(_ *os.File) (int, int) {
	return fallbackTerminalSize()
}
