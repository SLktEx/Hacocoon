package sshconfig_test

import (
	"errors"
	"strings"
	"testing"

	sshconfig "github.com/SLktEx/Hacocoon/internal/client/ssh/config"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestStreamCommandPinsTypedTargetAndDistribution(t *testing.T) {
	target := core.StreamTarget{Environment: "dev", Instance: "env-" + strings.Repeat("a", 32), Workspace: "work", AccessMode: core.WorkspaceReadWrite, Service: "ssh", Grant: "grant"}
	token, err := core.EncodeStreamTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, distro := range []string{"", "Hacocoon", "haco.dev-24_04", strings.Repeat("a", 128)} {
		got, err := sshconfig.StreamCommand(&target, distro)
		want := "/usr/local/bin/haco stream " + token
		if distro != "" {
			want = "C:/Windows/System32/wsl.exe --distribution " + distro + " --exec " + want
		}
		if err != nil || got != want {
			t.Fatalf("distro=%q: command=%q, err=%v", distro, got, err)
		}
	}
}

func TestStreamCommandRejectsMissingIdentityAndShellExpansions(t *testing.T) {
	if command, err := sshconfig.StreamCommand(nil, ""); command != "" || !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("%q: %v", command, err)
	}
	invalid := core.StreamTarget{}
	if command, err := sshconfig.StreamCommand(&invalid, ""); command != "" || err == nil {
		t.Fatalf("%q: %v", command, err)
	}
	target := core.StreamTarget{Environment: "dev", Instance: "env-" + strings.Repeat("a", 32), Workspace: "work", AccessMode: core.WorkspaceReadWrite, Service: "ssh", Grant: "grant"}
	for _, distro := range []string{"-option", "a b", "a\nb", "a%b", "a;exit", "$(id)", "a\"b", "../distro", "a\\b", strings.Repeat("a", 129)} {
		if command, err := sshconfig.StreamCommand(&target, distro); command != "" || !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("distro=%q: %q, %v", distro, command, err)
		}
	}
}
