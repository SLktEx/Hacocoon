package main

import (
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestParseGitFetchSpec(t *testing.T) {
	spec, err := parseGitFetchSpec([]string{"demo", "--remote", "upstream"})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Environment != "demo" || spec.Remote != "upstream" {
		t.Fatalf("spec=%#v", spec)
	}

	defaults, err := parseGitFetchSpec([]string{"demo"})
	if err != nil || defaults.Remote != "origin" {
		t.Fatalf("defaults=%#v err=%v", defaults, err)
	}

	for _, args := range [][]string{
		{},
		{""},
		{"demo", "--remote"},
		{"demo", "--remote", "origin", "--remote", "upstream"},
		{"demo", "--bogus"},
	} {
		if _, err := parseGitFetchSpec(args); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}
