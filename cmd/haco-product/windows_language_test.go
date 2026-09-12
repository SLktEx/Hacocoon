package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/cliui"
)

func TestInteractiveWindowsLanguagePrecedenceAndFallback(t *testing.T) {
	for _, tc := range []struct {
		name, override, wsl, locale, reply string
		failure                            bool
		want                               cliui.Language
		calls                              int
	}{
		{"Japanese Windows", "", "Hacocoon", "C.UTF-8", "ja", false, cliui.Japanese, 1},
		{"English Windows", "", "Hacocoon", "ja_JP.UTF-8", "en", false, cliui.English, 1},
		{"explicit English", "en", "Hacocoon", "ja_JP.UTF-8", "ja", false, cliui.English, 0},
		{"explicit Japanese", "ja", "Hacocoon", "C", "en", false, cliui.Japanese, 0},
		{"invalid override", "ja;evil", "Hacocoon", "ja_JP.UTF-8", "ja", false, cliui.English, 0},
		{"native Linux", "", "", "ja_JP.UTF-8", "en", false, cliui.Japanese, 0},
		{"missing interop", "", "Hacocoon", "ja_JP.UTF-8", "", true, cliui.Japanese, 1},
		{"malformed reply", "", "Hacocoon", "C", "ja\n", false, cliui.English, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"HACO_UI_LANGUAGE": tc.override, "WSL_DISTRO_NAME": tc.wsl, "LANG": tc.locale}
			calls := 0
			got := hostEntryLanguage(context.Background(), func(k string) string { return env[k] }, func(context.Context) (string, error) {
				calls++
				if tc.failure {
					return "", errors.New("interop failed")
				}
				return tc.reply, nil
			})
			if got != tc.want || calls != tc.calls {
				t.Fatalf("language=%s calls=%d", got, calls)
			}
			if env["LANG"] != tc.locale || env["HACO_UI_LANGUAGE"] != tc.override {
				t.Fatal("selection changed environment")
			}
		})
	}
}

func TestWindowsLanguageReplyBound(t *testing.T) {
	var reply languageReply
	if _, err := reply.Write([]byte("j")); err != nil {
		t.Fatal(err)
	}
	if _, err := reply.Write([]byte("a")); err != nil {
		t.Fatal(err)
	}
	if _, err := reply.Write([]byte("\n")); err == nil {
		t.Fatal("accepted extra output")
	}
	if reply.value.String() != "ja" {
		t.Fatal("changed accepted prefix")
	}
}

func TestNativeWindowsLanguageReadOnly(t *testing.T) {
	if os.Getenv("HACO_E2E_WINDOWS_LANGUAGE") != "1" {
		t.Skip("explicit native Windows/WSL language probe not enabled")
	}
	value, err := detectWindowsLanguage(context.Background())
	if err != nil || (value != "ja" && value != "en") {
		t.Fatalf("Windows UI query failed: %q %v", value, err)
	}
	t.Logf("Windows presentation language: %s; no locale writes", value)
}
