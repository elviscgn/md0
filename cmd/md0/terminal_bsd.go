//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package main

import (
	"os"
	"syscall"
	"unsafe"
)

func enableRawTerminal(file *os.File) (func() error, error) {
	fd := file.Fd()
	var original syscall.Termios
	if err := ioctlTermios(fd, syscall.TIOCGETA, &original); err != nil {
		return nil, err
	}
	raw := original
	raw.Iflag &^= syscall.BRKINT | syscall.ICRNL | syscall.INPCK | syscall.ISTRIP | syscall.IXON
	raw.Cflag |= syscall.CS8
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := ioctlTermios(fd, syscall.TIOCSETA, &raw); err != nil {
		return nil, err
	}
	return func() error { return ioctlTermios(fd, syscall.TIOCSETA, &original) }, nil
}

func terminalSize(file *os.File) (int, int) {
	var size struct {
		Rows   uint16
		Cols   uint16
		Xpixel uint16
		Ypixel uint16
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&size)))
	if errno != 0 || size.Cols == 0 || size.Rows == 0 {
		return fallbackTerminalSize()
	}
	return int(size.Cols), int(size.Rows)
}

func ioctlTermios(fd, request uintptr, state *syscall.Termios) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(unsafe.Pointer(state)))
	if errno != 0 {
		return errno
	}
	return nil
}
