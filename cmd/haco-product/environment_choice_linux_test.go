//go:build linux

package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestDesktopEnvironmentChoiceUsesStableNamesAndShowsWorkspace(t *testing.T) {
	envs := []core.Environment{{Name: "zeta", Workspace: core.Workspace{ID: "work-z"}}, {Name: "alpha", Workspace: core.Workspace{ID: "work-a"}}}
	var out bytes.Buffer
	selected, err := chooseDesktopEnvironment(envs, true, strings.NewReader("2\n"), &out)
	if err != nil || selected.Name != "zeta" || !strings.Contains(out.String(), `1. "alpha"  Workspace: "work-a"`) || envs[0].Name != "zeta" {
		t.Fatalf("%+v %v %s", selected, err, &out)
	}
}
func TestDesktopEnvironmentChoiceCancellationAndInvalidInput(t *testing.T) {
	envs := []core.Environment{{Name: "a"}, {Name: "b"}}
	for _, answer := range []string{"\n", "", "0\n", "3\n", "1;echo bad\n", "1", "1" + strings.Repeat(" ", 128) + "\n"} {
		_, err := chooseDesktopEnvironment(envs, true, strings.NewReader(answer), io.Discard)
		if answer == "" || answer == "\n" {
			if !errors.Is(err, errEnvironmentChoiceCanceled) {
				t.Fatal(err)
			}
		} else if err == nil {
			t.Fatalf("accepted %q", answer)
		}
	}
}
func TestDesktopEnvironmentChoiceDoesNotReadNoninteractiveInput(t *testing.T) {
	reader := strings.NewReader("2\n")
	_, err := chooseDesktopEnvironment([]core.Environment{{Name: "a"}, {Name: "b"}}, false, reader, io.Discard)
	if err == nil || reader.Len() != 2 {
		t.Fatal("noninteractive input consumed")
	}
	selected, err := chooseDesktopEnvironment([]core.Environment{{Name: "only"}}, false, reader, io.Discard)
	if err != nil || selected.Name != "only" || reader.Len() != 2 {
		t.Fatal("single Environment needs no input")
	}
	if _, err := chooseDesktopEnvironment(nil, true, reader, io.Discard); err == nil {
		t.Fatal("empty list accepted")
	}
}
func TestDesktopEnvironmentChoiceEscapesLabelsAndRejectsDuplicates(t *testing.T) {
	var out bytes.Buffer
	_, err := chooseDesktopEnvironment([]core.Environment{{Name: "a", Workspace: core.Workspace{ID: "\x1b[2J\nwrong"}}, {Name: "b"}}, true, strings.NewReader("1\n"), &out)
	if err != nil || strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "\nwrong") {
		t.Fatalf("unsafe labels: %q %v", out.String(), err)
	}
	if _, err := chooseDesktopEnvironment([]core.Environment{{Name: "a"}, {Name: "a"}}, true, strings.NewReader("1\n"), io.Discard); err == nil {
		t.Fatal("duplicate names accepted")
	}
}
