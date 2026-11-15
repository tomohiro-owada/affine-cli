package auth

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestParseOIDCCallback(t *testing.T) {
	payload := url.QueryEscape(`{"state":"state-token","code":"authorization-code"}`)
	parsed, err := parseCallback("affine-beta://authentication?method=oauth&payload=" + payload)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.State != "state-token" || parsed.Code != "authorization-code" {
		t.Fatalf("unexpected payload: %#v", parsed)
	}
}

func TestParseOIDCCallbackRejectsOtherSchemes(t *testing.T) {
	_, err := parseCallback("affine://authentication?method=oauth&payload=%7B%7D")
	if err == nil {
		t.Fatal("expected invalid callback error")
	}
}

func TestOpenBrowserFallsBackToXDGOpen(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "opened")
	t.Setenv("PATH", dir)
	t.Setenv("OUTPUT", output)

	if err := os.WriteFile(filepath.Join(dir, "gio"), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "xdg-open"), []byte("#!/bin/sh\nprintf '%s' \"$1\" > \"$OUTPUT\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := openBrowser("https://example.com/auth"); err != nil {
		t.Fatal(err)
	}
	opened, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(opened) != "https://example.com/auth" {
		t.Fatalf("opened URL = %q", opened)
	}
}
