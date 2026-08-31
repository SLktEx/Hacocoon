package incus

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestEnsureOCIImageRemoteCreatesDockerHubRemoteWhenMissing(t *testing.T) {
	configured := false
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "remote" && args[1] == "list" {
			if configured {
				return host.Result{Stdout: "oci-docker,oci\nimages,simplestreams\n"}, nil
			}
			return host.Result{Stdout: "images,simplestreams\n"}, nil
		}
		if len(args) >= 2 && args[0] == "remote" && args[1] == "add" {
			configured = true
			return host.Result{}, nil
		}
		return host.Result{}, nil
	}}
	if err := New(runner).ensureOCIImageRemote(context.Background(), "oci-docker:library/postgres:18"); err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("Docker Hub OCI remote was not configured")
	}
}

func TestEnsureOCIImageRemoteRejectsNonOCIRemote(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "remote" && args[1] == "list" {
			return host.Result{Stdout: "images,simplestreams\n"}, nil
		}
		return host.Result{}, nil
	}}
	err := New(runner).ensureOCIImageRemote(context.Background(), "images:ubuntu/26.04")
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureOCIImageRemoteRequiresConfiguredPrivateRemote(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "remote" && args[1] == "list" {
			return host.Result{Stdout: "oci-docker,oci\n"}, nil
		}
		return host.Result{}, nil
	}}
	err := New(runner).ensureOCIImageRemote(context.Background(), "ecr:team/app:latest")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}
