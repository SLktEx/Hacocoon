package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestHistoricalHacoCommandsAreClassifiedExactlyOnce(t *testing.T) {
	seen := map[string]bool{}
	for _, command := range historicalHacoCommands {
		if seen[command] {
			t.Fatalf("historical command %q listed more than once", command)
		}
		seen[command] = true
		classification, ok := hacoCommandClassification(command)
		if !ok {
			t.Fatalf("historical command %q has no classification", command)
		}
		if classification.Name != command {
			t.Fatalf("classification name for %q = %q", command, classification.Name)
		}
	}
	if len(seen) != len(hacoCommandClassifications) {
		for command := range hacoCommandClassifications {
			if !seen[command] {
				t.Fatalf("classification %q is not represented in historicalHacoCommands", command)
			}
		}
	}
}

func TestLegacyEnvironmentAliasesPointAtFirstClassNamespace(t *testing.T) {
	for command, replacement := range map[string]string{
		"create": "haco env create",
		"status": "haco env status",
		"exec":   "haco env exec",
		"shell":  "haco env shell",
		"delete": "haco env delete",
	} {
		classification, ok := hacoCommandClassification(command)
		if !ok {
			t.Fatalf("missing classification for %q", command)
		}
		if classification.Domain != commandDomainCompatibility || classification.Replacement != replacement {
			t.Fatalf("classification for %q = %+v", command, classification)
		}
	}
}

func TestHandleHelpArgsIsPreRuntimeAndRoleAware(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		var out bytes.Buffer
		handled, err := handleHelpArgs(args, &out)
		if err != nil || !handled {
			t.Fatalf("args=%v handled=%t err=%v", args, handled, err)
		}
		text := out.String()
		for _, required := range []string{
			"Hacocoon CLI roles",
			"general Hacocoon client",
			"trusted logical Host-local tooling",
			"General controller-client operations",
			"Physical Host bootstrap/recovery/service operations",
			"Trusted haco-host-local migration targets",
			"Temporary compatibility aliases",
			"haco env create",
			"migration pending",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("help for %v missing %q:\n%s", args, required, text)
			}
		}
	}
}

func TestHandleHelpArgsRejectsExtraArguments(t *testing.T) {
	handled, err := handleHelpArgs([]string{"help", "env"}, &bytes.Buffer{})
	if !handled || !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
}

func TestHandleHelpArgsIgnoresOtherCommands(t *testing.T) {
	handled, err := handleHelpArgs([]string{"env", "list"}, &bytes.Buffer{})
	if handled || err != nil {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
}

func TestControllerClientModeErrorsExplainClassification(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"base", "general haco client operation"},
		{"host", "Physical Host-local"},
		{"plugin", "trusted haco-host-local"},
	}
	for _, test := range cases {
		err := controllerClientModeCommandError(test.command)
		if !errors.Is(err, core.ErrUnsupported) || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("command=%q err=%v", test.command, err)
		}
	}
}
