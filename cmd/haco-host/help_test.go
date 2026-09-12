package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/cliui"
)

func TestHostHelpEveryPublicPathWithoutController(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "")
	for _, locale := range []string{"C", "ja_JP.UTF-8"} {
		t.Setenv("LC_ALL", locale)
		t.Setenv("HACO_CONTROL_SOCKET", "invalid socket")
		for _, page := range hostHelpPages {
			for _, flag := range []string{"--help", "-h"} {
				var out bytes.Buffer
				args := append(strings.Fields(page.Path), flag)
				if !requestedHostHelp(args, &out) || !strings.Contains(out.String(), page.Example) || !strings.Contains(out.String(), cliui.ParseLocale(locale).Text("help.example")) {
					t.Fatalf("%v: %q", args, out.String())
				}
				// nil client demonstrates dispatch cannot contact any service.
				if err := dispatch(context.Background(), nil, args); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestHostHelpUsesForwardedPresentationBeforeLocalLocale(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	t.Setenv("HACO_UI_LANGUAGE", "ja")
	var output bytes.Buffer
	if !requestedHostHelp([]string{"--help"}, &output) || !strings.Contains(output.String(), cliui.Japanese.Text("help.example")) {
		t.Fatalf("Host lost forwarded presentation: %q", output.String())
	}
}

func TestHostHelpLeavesExecutionAndInvalidPathsToDispatch(t *testing.T) {
	for _, args := range [][]string{{"env", "exec", "dev", "--", "tool", "--help"}, {"env", "missing", "--help"}, {"env", "delete", "dev", "--help"}} {
		var out bytes.Buffer
		if requestedHostHelp(args, &out) || out.Len() != 0 {
			t.Fatalf("intercepted %v", args)
		}
	}
}
