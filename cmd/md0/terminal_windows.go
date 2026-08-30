//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

const (
	enableEchoInput            = 0x0004
	enableLineInput            = 0x0002
	enableProcessedInput       = 0x0001
	enableVirtualTerminalInput = 0x0200
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode             = kernel32.NewProc("GetConsoleMode")
	setConsoleMode             = kernel32.NewProc("SetConsoleMode")
	getConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

type consoleScreenBufferInfo struct {
	Size              struct{ X, Y int16 }
	CursorPosition    struct{ X, Y int16 }
	Attributes        uint16
	Window            struct{ Left, Top, Right, Bottom int16 }
	MaximumWindowSize struct{ X, Y int16 }
}

func enableRawTerminal(file *os.File) (func() error, error) {
	handle := syscall.Handle(file.Fd())
	var original uint32
	if err := callConsoleMode(getConsoleMode, handle, &original); err != nil {
		return nil, err
	}
	raw := original &^ (enableEchoInput | enableLineInput | enableProcessedInput)
	raw |= enableVirtualTerminalInput
	if err := setConsoleInputMode(handle, raw); err != nil {
		return nil, err
	}
	return func() error { return setConsoleInputMode(handle, original) }, nil
}

func callConsoleMode(proc *syscall.LazyProc, handle syscall.Handle, mode *uint32) error {
	result, _, callErr := proc.Call(uintptr(handle), uintptr(unsafe.Pointer(mode)))
	if result == 0 {
		return callErr
	}
	return nil
}

func setConsoleInputMode(handle syscall.Handle, mode uint32) error {
	result, _, callErr := setConsoleMode.Call(uintptr(handle), uintptr(mode))
	if result == 0 {
		return callErr
	}
	return nil
}

func terminalSize(file *os.File) (int, int) {
	var info consoleScreenBufferInfo
	result, _, _ := getConsoleScreenBufferInfo.Call(file.Fd(), uintptr(unsafe.Pointer(&info)))
	if result == 0 {
		return fallbackTerminalSize()
	}
	width := int(info.Window.Right-info.Window.Left) + 1
	height := int(info.Window.Bottom-info.Window.Top) + 1
	if width <= 0 || height <= 0 {
		return fallbackTerminalSize()
	}
	return width, height
}
