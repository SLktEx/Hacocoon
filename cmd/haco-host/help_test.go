package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/cliui"
)

func TestHostHelpEveryPublicPathWithoutController(t *testing.T) {
	for _, locale := range []string{"C", "ja_JP.UTF-8"} {
		t.Setenv("LC_ALL", locale)
		t.Setenv("HACO_CONTROL_SOCKET", "invalid socket")
		for _, page := range hostHelpPages {
			for _, flag := range []string{"--help", "-h"} {
				var out bytes.Buffer
				args := append(strings.Fields(page.Path), flag)
				if !requestedHostHelp(args, &out) || !strings.Contains(out.String(), page.Example) || !strings.Contains(out.String(), cliui.Resolve(func(string) string { return locale }).Text("help.example")) {
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

func TestHostHelpLeavesExecutionAndInvalidPathsToDispatch(t *testing.T) {
	for _, args := range [][]string{{"env", "exec", "dev", "--", "tool", "--help"}, {"env", "missing", "--help"}, {"env", "delete", "dev", "--help"}} {
		var out bytes.Buffer
		if requestedHostHelp(args, &out) || out.Len() != 0 {
			t.Fatalf("intercepted %v", args)
		}
	}
}
