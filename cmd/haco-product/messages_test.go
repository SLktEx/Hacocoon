package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/cliui"
)

func TestCLIMessageSelection(t *testing.T) {
	for _, tc := range []struct{ all, messages, lang, want string }{
		{"", "", "ja_JP.UTF-8", "使い方:"},
		{"C", "ja", "ja", "Usage:"},
		{"", "ja", "en_US.UTF-8", "使い方:"},
		{"fr_FR.UTF-8", "ja", "ja", "Usage:"},
		{"", "", "", "Usage:"},
	} {
		t.Run(tc.all+"/"+tc.messages+"/"+tc.lang, func(t *testing.T) {
			t.Setenv("LC_ALL", tc.all)
			t.Setenv("LC_MESSAGES", tc.messages)
			t.Setenv("LANG", tc.lang)
			if got := cliMessage("help.usage"); got != tc.want {
				t.Fatalf("got %q", got)
			}
		})
	}
}

func TestLocalizedHelpKeepsCommands(t *testing.T) {
	var en, ja bytes.Buffer
	writeLocalizedHelp(&en, cliui.English)
	writeLocalizedHelp(&ja, cliui.Japanese)
	if !strings.Contains(ja.String(), "開発環境を作る") || !strings.Contains(ja.String(), "設定や接続の問題") {
		t.Fatal(ja.String())
	}
	english, japanese := strings.Split(en.String(), "\n"), strings.Split(ja.String(), "\n")
	if len(english) != len(japanese) {
		t.Fatal("language changed command count")
	}
	for i, line := range english {
		if strings.HasPrefix(line, "  ") && len(line) >= 13 && line[:13] != japanese[i][:13] {
			t.Fatalf("command changed: %q -> %q", line, japanese[i])
		}
	}
	for _, output := range []string{en.String(), ja.String()} {
		if strings.Contains(output, "\x1b") || strings.Contains(output, "help.") || strings.Contains(output, "%!") {
			t.Fatal(output)
		}
	}
}

func TestCLIFlagHelpHeading(t *testing.T) {
	for _, locale := range []string{"C", "ja_JP.UTF-8"} {
		t.Run(locale, func(t *testing.T) {
			t.Setenv("LC_ALL", locale)
			var output bytes.Buffer
			flags := flag.NewFlagSet("haco approve", flag.ContinueOnError)
			configureCLIFlags(flags, &output)
			flags.Bool("json", false, cliMessage("approval.flag_json"))
			if err := flags.Parse([]string{"--help"}); !errors.Is(err, flag.ErrHelp) {
				t.Fatal(err)
			}
			want := "Usage of haco approve:"
			if locale != "C" {
				want = "haco approve の使い方:"
			}
			if !strings.Contains(output.String(), want) || !strings.Contains(output.String(), "-json") {
				t.Fatal(output.String())
			}
		})
	}
}
