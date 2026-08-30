package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestParseGitPushSpec(t *testing.T) {
	spec, err := parseGitPushSpec([]string{"demo", "--branch", "feature/x", "--source", "HEAD~1", "--remote", "upstream", "--force"})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Environment != "demo" || spec.Branch != "feature/x" || spec.Source != "HEAD~1" || spec.Remote != "upstream" || !spec.Force {
		t.Fatalf("spec=%#v", spec)
	}
	defaults, err := parseGitPushSpec([]string{"demo", "--branch", "feature/x"})
	if err != nil || defaults.Remote != "origin" || defaults.Source != "HEAD" || defaults.Force {
		t.Fatalf("defaults=%#v err=%v", defaults, err)
	}
	for _, args := range [][]string{
		{"demo"},
		{"demo", "--branch"},
		{"demo", "--branch", "x", "--branch", "y"},
		{"demo", "--branch", "x", "--source", "A", "--source", "B"},
		{"demo", "--branch", "x", "--bogus"},
	} {
		if _, err := parseGitPushSpec(args); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}

func TestGitPushLivesUnderPluginNamespace(t *testing.T) {
	ctx := context.Background()

	err := dispatch(ctx, nil, []string{"git", "push", "demo", "--branch", "feature/x"})
	if !errors.Is(err, core.ErrInvalidArgument) || !strings.Contains(err.Error(), `unknown command "git"`) {
		t.Fatalf("top-level git err=%v", err)
	}

	err = pluginCommand(ctx, nil, []string{"git", "push", "demo"})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("plugin git push err=%v", err)
	}

	err = pluginCommand(ctx, nil, []string{"unknown"})
	if !errors.Is(err, core.ErrInvalidArgument) || !strings.Contains(err.Error(), `unknown plugin "unknown"`) {
		t.Fatalf("unknown plugin err=%v", err)
	}
}
