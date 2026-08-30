package main

import (
	"reflect"
	"testing"
)

func TestBrowserCommandUsesArgumentPassingWithoutAShell(t *testing.T) {
	const url = "http://127.0.0.1:8080/?value=a&other=b"
	tests := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{goos: "darwin", wantName: "open", wantArgs: []string{url}},
		{goos: "linux", wantName: "xdg-open", wantArgs: []string{url}},
		{goos: "windows", wantName: "rundll32", wantArgs: []string{"url.dll,FileProtocolHandler", url}},
	}
	for _, test := range tests {
		name, args, err := browserCommand(test.goos, url)
		if err != nil {
			t.Fatalf("browserCommand(%q): %v", test.goos, err)
		}
		if name != test.wantName || !reflect.DeepEqual(args, test.wantArgs) {
			t.Fatalf("browserCommand(%q)=(%q, %#v), want (%q, %#v)", test.goos, name, args, test.wantName, test.wantArgs)
		}
	}
}

func TestBrowserCommandRejectsUnsupportedPlatform(t *testing.T) {
	if _, _, err := browserCommand("plan9", "http://127.0.0.1:8080/"); err == nil {
		t.Fatal("browserCommand accepted an unsupported platform")
	}
}
