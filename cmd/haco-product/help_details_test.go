package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/cliui"
)

func TestDetailedHelpDescribesEveryAdvertisedOption(t *testing.T) {
	flags := regexp.MustCompile(`(?:^|[\s\[,|])(--?[a-z][a-z-]*)`)
	for _, page := range helpPages {
		described := map[string]bool{}
		for _, field := range page.Options {
			for _, name := range flags.FindAllStringSubmatch(field.Syntax, -1) {
				described[name[1]] = true
			}
		}
		for _, name := range flags.FindAllStringSubmatch(page.Syntax, -1) {
			if !described[name[1]] {
				t.Errorf("%s does not describe %s", page.Path, name[1])
			}
		}
		fields := append(append([]cliui.HelpField{}, page.Arguments...), page.Options...)
		for _, language := range []cliui.Language{cliui.English, cliui.Japanese} {
			var output bytes.Buffer
			if !commandHelp(&output, page.Path, language) {
				t.Fatal(page.Path)
			}
			for _, field := range fields {
				if field.Syntax == "" || language.Text(field.Message) == field.Message {
					t.Errorf("missing detail: %s %+v", page.Path, field)
				}
				if !strings.Contains(output.String(), field.Syntax) {
					t.Errorf("missing syntax: %s %+v", page.Path, field)
				}
			}
			if len(page.Options) != 0 && !strings.Contains(output.String(), language.Text("help.options")) {
				t.Errorf("missing options: %s", page.Path)
			}
		}
	}
}

func TestDailyHelpExplainsRequiredInputDefaultsAndAuthorityInBothLanguages(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", "/missing/help-details.sock")
	for _, language := range []string{"C", "ja_JP.UTF-8"} {
		setCLITestLocale(t, language)
		for _, path := range []string{"env create", "workspace prepare", "repo clone", "git approve", "reclaim", "setup", "config"} {
			code, stdout, stderr := captureRun(t, append(strings.Fields(path), "--help")...)
			if code != 0 || stderr != "" {
				t.Fatalf("%s: %d %s", path, code, stderr)
			}
			for _, expected := range map[string][]string{
				"env create":        {"--workspace", "--no-oci"},
				"workspace prepare": {"--path", "--repo", "auto"},
				"repo clone":        {"--branch", "main", "push"},
				"git approve":       {"ask-env", "ask-all", "main"},
				"reclaim":           {"--status", "--review"},
				"setup":             {"--script", "--clear-script"},
				"config":            {"--edit", "--file"},
			}[path] {
				if !strings.Contains(stdout, expected) {
					t.Fatalf("%s missing %s", path, expected)
				}
			}
		}
	}
}
