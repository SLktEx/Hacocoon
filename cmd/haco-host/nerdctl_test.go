package main

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (fakeControllerClient) CreateWorkload(context.Context, core.WorkloadSpec) (core.Workload, error) {
	return core.Workload{}, nil
}

func (fakeControllerClient) ListWorkloads(context.Context, string) ([]core.Workload, error) {
	return nil, nil
}

func (fakeControllerClient) ExecWorkload(context.Context, string, string, []string) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}

func (fakeControllerClient) StopWorkload(context.Context, string, string) error { return nil }
func (fakeControllerClient) DeleteWorkload(context.Context, string, string) error { return nil }
func (fakeControllerClient) PullWorkloadImage(context.Context, string) error { return nil }

func TestNerdctlImageToIncusDefaultsDockerHub(t *testing.T) {
	for input, want := range map[string]string{
		"postgres:18":        "oci-docker:library/postgres:18",
		"library/redis:8":    "oci-docker:library/redis:8",
		"docker.io/a/b:tag":  "oci-docker:a/b:tag",
		"ecr::team/app:prod": "ecr:team/app:prod",
	} {
		got, err := nerdctlImageToIncus(input)
		if err != nil {
			t.Fatalf("nerdctlImageToIncus(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("nerdctlImageToIncus(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNerdctlImageToIncusRequiresRemoteForPrivateRegistry(t *testing.T) {
	_, err := nerdctlImageToIncus("123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/app:latest")
	if !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("error = %v", err)
	}
}

func TestParseNerdctlNamespace(t *testing.T) {
	namespace, rest, err := parseNerdctlNamespace([]string{"--namespace", "demo", "ps", "-a"})
	if err != nil {
		t.Fatal(err)
	}
	if namespace != "demo" || len(rest) != 2 || rest[0] != "ps" || rest[1] != "-a" {
		t.Fatalf("namespace=%q rest=%#v", namespace, rest)
	}
}

func TestNerdctlBuildFailsClosed(t *testing.T) {
	err := nerdctlCommand(context.Background(), fakeControllerClient{}, []string{"build", "."})
	if !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("error = %v", err)
	}
}
