package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrustedHostNoticeLocaleAndRedirectedOutput(t *testing.T) {
	for _, tc := range []struct{ name, all, messages, lang, want string }{
		{"Japanese LANG", "", "", "ja_JP.UTF-8", trustedHostNoticeJapanese},
		{"messages overrides LANG", "", "ja_JP.UTF-8", "en_US.UTF-8", trustedHostNoticeJapanese},
		{"all overrides messages", "C.UTF-8", "ja_JP.UTF-8", "ja_JP.UTF-8", trustedHostNoticeEnglish},
		{"BCP47", "", "", "JA-jp", trustedHostNoticeJapanese},
		{"default", "", "", "", trustedHostNoticeEnglish},
		{"unknown", "", "", "fr_FR.UTF-8", trustedHostNoticeEnglish},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LC_ALL", tc.all)
			t.Setenv("LC_MESSAGES", tc.messages)
			t.Setenv("LANG", tc.lang)
			t.Setenv("NO_COLOR", "")
			path := filepath.Join(t.TempDir(), "notice")
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			writeTrustedHostNotice(f)
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != tc.want+"\n" || strings.Contains(string(raw), "\x1b") {
				t.Fatalf("notice = %q, want plain %q", raw, tc.want)
			}
		})
	}
}
