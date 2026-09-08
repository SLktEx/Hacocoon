//go:build linux

package sshclient

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type selectionFixture struct {
	*fakeController
	environment core.Environment
}

func (f *selectionFixture) EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error) {
	return core.EnvironmentStatus{Environment: f.environment, State: f.state}, nil
}
func selectedFixtureEnvironment() core.Environment {
	return core.Environment{Name: "dev", RuntimeRef: "runtime-owned", CreatedAt: time.Unix(100, 0), Workspace: core.Workspace{ID: "work"}}
}
func TestSelectedSetupRejectsChangedIdentityBeforeLocalOrRemoteMutation(t *testing.T) {
	for _, kind := range []string{"runtime", "creation", "workspace", "mode"} {
		t.Run(kind, func(t *testing.T) {
			original := selectedFixtureEnvironment()
			c := &selectionFixture{fakeController: &fakeController{state: core.EnvironmentStopped}, environment: original}
			switch kind {
			case "runtime":
				c.environment.RuntimeRef = "replacement"
			case "creation":
				c.environment.CreatedAt = time.Unix(101, 0)
			case "workspace":
				c.environment.Workspace.ID = "other"
			case "mode":
				c.environment.AccessMode = core.WorkspaceAccessMode("changed")
			}
			home := t.TempDir()
			_, err := SetupSelected(context.Background(), c, Desktop{Home: home}, original)
			if !errors.Is(err, core.ErrIncompatibleState) || c.count != 0 || c.starts != 0 {
				t.Fatalf("%v %+v", err, c)
			}
			if _, err := os.Stat(filepath.Join(home, ".ssh")); !os.IsNotExist(err) {
				t.Fatal("stale selection changed client files")
			}
		})
	}
}
func TestSelectedSetupResumesAndRechecksAfterConnectionPreparation(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen unavailable")
	}
	for _, replace := range []bool{false, true} {
		original := selectedFixtureEnvironment()
		c := &selectionFixture{fakeController: &fakeController{state: core.EnvironmentStopped}, environment: original}
		if replace {
			c.prepare = func() { c.environment.CreatedAt = time.Unix(102, 0) }
		}
		alias, err := SetupSelected(context.Background(), c, Desktop{Home: t.TempDir()}, original)
		if replace {
			if !errors.Is(err, core.ErrRecoveryRequired) || alias != "" {
				t.Fatalf("replacement connected: %s %v", alias, err)
			}
		} else if err != nil || alias != "haco-dev" {
			t.Fatalf("%s %v", alias, err)
		}
		if c.starts != 1 || c.count != 1 {
			t.Fatal("ordinary setup was not used")
		}
	}
}
