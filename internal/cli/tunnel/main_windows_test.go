package tunnelcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeHelpAndInvalidTargetsDoNotStartWSL(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			path := filepath.Join(t.TempDir(), "help.txt")
			out, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			diagnostic, err := os.Create(filepath.Join(t.TempDir(), "diagnostic.txt"))
			if err != nil {
				t.Fatal(err)
			}
			previous := os.Stderr
			previousOut := os.Stdout
			os.Stderr = diagnostic
			os.Stdout = out
			defer func() { os.Stderr = previous; os.Stdout = previousOut; out.Close(); diagnostic.Close() }()
			if code := run(context.Background(), []string{"--help"}); code != 0 {
				t.Fatal(code)
			}
			if err = out.Sync(); err != nil {
				t.Fatal(err)
			}
			text, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			info, err := diagnostic.Stat()
			if err != nil || info.Size() != 0 {
				t.Fatal("help wrote diagnostics", err)
			}
			for _, want := range []string{"--distribution", "--target-port", "--listen", "--address", "--duration", map[string]string{"en": "1h", "ja": "1時間"}[language]} {
				if !strings.Contains(string(text), want) {
					t.Fatal(want, string(text))
				}
			}
			if strings.Contains(string(text), "%!") {
				t.Fatal(string(text))
			}
			for _, args := range [][]string{{}, {"--distribution", "-u root", "--help"}, {"--distribution", "hacocoon", "--target-port", "0", "demo"}} {
				if code := run(context.Background(), args); code != 2 {
					t.Fatal(args, code)
				}
			}
		})
	}
}
