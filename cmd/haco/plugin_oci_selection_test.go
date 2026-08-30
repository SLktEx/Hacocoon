package main

import (
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestParseOCISeedSelectionOptions(t *testing.T) {
	target := "example.invalid/app:latest@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	gotTarget, reenable, jsonOutput, err := parseOCISeedSelectionOptions([]string{target, "--re-enable", "--json"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if gotTarget != target || !reenable || !jsonOutput {
		t.Fatalf("target=%q reenable=%t json=%t", gotTarget, reenable, jsonOutput)
	}
}

func TestParseOCISeedSelectionOptionsRejectsReenableForNonPinCommand(t *testing.T) {
	_, _, _, err := parseOCISeedSelectionOptions([]string{"example.invalid/app:latest@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--re-enable"}, false)
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
}

func TestParseOCISeedSelectionOptionsRejectsDuplicateAndUnknownOptions(t *testing.T) {
	target := "example.invalid/app:latest@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, args := range [][]string{
		{target, "--json", "--json"},
		{target, "--re-enable", "--re-enable"},
		{target, "--wat"},
		{"--json"},
	} {
		_, _, _, err := parseOCISeedSelectionOptions(args, true)
		if !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("args=%#v err=%v", args, err)
		}
	}
}
