package incus

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func TestHostOCIInventoryUsesOwnedLocalEngine(t *testing.T) {
	for _, driver := range []string{"docker", "nerdctl"} {
		runner := &fakeRunner{run: func(_ context.Context, call int, name string, args []string) (host.Result, error) {
			if call == 0 {
				return host.Result{Stdout: trustedHostRoleValue}, nil
			}
			joined := strings.Join(args, " ")
			for _, part := range []string{"exec haco-host --project hacocoon --cwd / -- /usr/bin/env -i", "DOCKER_CONFIG=/proc/self", "NERDCTL_TOML=/dev/null", "images --no-trunc --digests --format"} {
				if !strings.Contains(joined, part) {
					t.Fatalf("missing confinement: %s", part)
				}
			}
			endpoint := "--host=unix:///var/run/docker.sock"
			if driver == "nerdctl" {
				endpoint = "--address=/run/containerd/containerd.sock --namespace=default"
			}
			if name != "incus" || !strings.Contains(joined, endpoint) {
				t.Fatal("wrong engine endpoint")
			}
			return host.Result{Stdout: "fixture"}, nil
		}}
		r := &Runtime{runner: runner, project: "hacocoon"}
		got, err := r.LocalOCIImages(context.Background(), driver)
		if err != nil || got.Stdout != "fixture" || len(runner.calls) != 2 {
			t.Fatalf("inventory: %+v %v", got, err)
		}
	}
}
func TestHostOCIInventoryRefusesForeignHostAndInvalidDriver(t *testing.T) {
	runner := &fakeRunner{}
	r := &Runtime{runner: runner, project: "hacocoon"}
	if _, err := r.LocalOCIImages(context.Background(), "--evil"); !errors.Is(err, core.ErrInvalidArgument) || len(runner.calls) != 0 {
		t.Fatal("invalid driver executed")
	}
	if _, err := r.LocalOCIImages(context.Background(), "docker"); err == nil || len(runner.calls) != 1 {
		t.Fatal("foreign Host inventory attempted")
	}
}

func TestHostOCIInventoryRejectsIncompleteOwnership(t *testing.T) {
	for _, result := range []host.Result{{Stdout: trustedHostRoleValue, StdoutTruncated: true}, {Stdout: trustedHostRoleValue, ExitCode: 1}} {
		runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) { return result, nil }}
		r := &Runtime{runner: runner, project: "hacocoon"}
		if _, err := r.LocalOCIImages(context.Background(), "docker"); err == nil || len(runner.calls) != 1 {
			t.Fatal("incomplete ownership accepted")
		}
	}
}
