package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestParseOCISeedBaseOptions(t *testing.T) {
	base, jsonOutput, err := parseOCISeedBaseOptions([]string{"--json", "--base", "haco/ubuntu-26.04"})
	if err != nil {
		t.Fatal(err)
	}
	if base != "haco/ubuntu-26.04" || !jsonOutput {
		t.Fatalf("base=%q json=%v", base, jsonOutput)
	}
}

func TestParseOCISeedBaseOptionsRejectsAuthorityShapingInputs(t *testing.T) {
	for _, args := range [][]string{
		{"--base"},
		{"--base", "--json"},
		{"--base", "../victim"},
		{"--base", "haco/ubuntu-26.04", "--base", "other"},
		{"--json", "--json"},
		{"--unknown"},
	} {
		t.Run(stringsForTest(args), func(t *testing.T) {
			_, _, err := parseOCISeedBaseOptions(args)
			if !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("args=%#v err=%v want ErrInvalidArgument", args, err)
			}
		})
	}
}

func TestParseOCISeedJSONOnly(t *testing.T) {
	got, err := parseOCISeedJSONOnly([]string{"--json"})
	if err != nil || !got {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if _, err := parseOCISeedJSONOnly([]string{"--base", "x"}); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("err=%v want ErrInvalidArgument", err)
	}
}

func stringsForTest(values []string) string {
	return strings.Join(values, "_")
}
