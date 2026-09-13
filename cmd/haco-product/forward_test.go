package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestClientForwardValidatesBeforeControllerAccess(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", "/nonexistent/forward.sock")
	for _, args := range [][]string{{"--target-port", "80", "--listen", "0.0.0.0:80", "demo"}, {"--target-port", "80", "--address", "169.254.254.1", "demo"}, {"--target-port", "80", "--duration", "2h", "demo"}, {"--target-port", "0", "demo"}} {
		var out, diag bytes.Buffer
		if code := forwardClientCommand(context.Background(), args, &out, &diag); code != 2 {
			t.Fatal(args, code, diag.String())
		}
	}
}

func TestClientForwardHelpIsBilingualAndControllerIndependent(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Setenv("HACO_UI_LANGUAGE", language)
		t.Setenv("HACO_CONTROL_SOCKET", "/nonexistent/forward.sock")
		var out, diag bytes.Buffer
		if code := environmentCommand(context.Background(), []string{"tunnel", "--help"}, &out, &diag); code != 0 {
			t.Fatal(code, diag.String())
		}
		for _, want := range []string{"--target-port", "--listen", "--address", "--duration", "127.0.0.1"} {
			if !strings.Contains(out.String(), want) {
				t.Fatal(language, want, out.String())
			}
		}
	}
}
