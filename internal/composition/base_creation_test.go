package composition

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
	"strings"
	"testing"
)

func TestOrdinaryCreateUsesIncusImageWithoutRetainedBase(t *testing.T) {
	fp := strings.Repeat("a", 64)
	initialized := false
	runner := &storageRunnerFunc{run: func(name string, args []string) (host.Result, error) {
		if name != "incus" {
			t.Fatal(name)
		}
		if strings.Contains(strings.Join(args, " "), "haco-base-") {
			t.Fatal("redundant Base storage", args)
		}
		switch args[0] {
		case "image":
			return host.Result{Stdout: `{"fingerprint":"` + fp + `"}`}, nil
		case "profile":
			return host.Result{Stdout: `{"devices":{"root":{"type":"disk","path":"/","pool":"pool"}}}`}, nil
		case "project":
			return host.Result{}, nil
		case "init":
			if args[1] != "images:"+fp || args[2] != "haco-composed" {
				t.Fatal(args)
			}
			initialized = true
			return host.Result{ExitCode: 1}, core.ErrRuntimeUnavailable
		default:
			return host.Result{}, nil
		}
	}}
	provider, err := incus.NewBaseProvider(incus.New(runner))
	if err != nil {
		t.Fatal(err)
	}
	id, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{InstanceID: id, Name: "composed", WorkspacePath: t.TempDir()})
	if !initialized || err == nil {
		t.Fatal("expected fixture init failure", initialized, err)
	}
}
