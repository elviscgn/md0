package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"time"
)

const browserLaunchTimeout = 5 * time.Second

func scheduleBrowserOpen(addr string, disabled bool) {
	if disabled {
		return
	}
	url := "http://" + addr
	go func() {
		if !waitForRuntime(addr, browserLaunchTimeout) {
			return
		}
		if err := openBrowserURL(url); err != nil {
			u := newTerminalUI(os.Stderr)
			fmt.Fprintln(u.out)
			u.warning("could not open the browser automatically")
			u.action(url)
		}
	}()
}

func waitForRuntime(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", addr, 120*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return true
		}
		time.Sleep(40 * time.Millisecond)
	}
	return false
}

func openBrowserURL(url string) error {
	name, args, err := browserCommand(runtime.GOOS, url)
	if err != nil {
		return err
	}
	return exec.Command(name, args...).Run()
}

func browserCommand(goos, url string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "open", []string{url}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}, nil
	case "linux", "dragonfly", "freebsd", "netbsd", "openbsd":
		return "xdg-open", []string{url}, nil
	default:
		return "", nil, errors.New("automatic browser launch is unavailable on this platform")
	}
}
