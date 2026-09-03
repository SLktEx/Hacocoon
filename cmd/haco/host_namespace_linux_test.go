//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestHostEnsureNamespaceIgnoresOtherCommands(t *testing.T) {
	deps := hostEnsureNamespaceDeps{}
	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(), []string{"doctor"}, bytes.NewReader(nil), io.Discard, io.Discard, deps,
	)
	if handled || err != nil {
		t.Fatalf("handled=%v err=%v, want false nil", handled, err)
	}
}

func TestHostEnsureNamespaceContinuesWhenAlreadyWithPID1(t *testing.T) {
	deps := testHostEnsureNamespaceDeps()
	deps.readlink = func(path string) (string, error) {
		switch path {
		case selfMountNamespace, initMountNamespace:
			return "mnt:[42]", nil
		default:
			return "", errors.New("unexpected path")
		}
	}
	deps.incusMainPID = func(context.Context) (int, error) {
		t.Fatal("ordinary PID1 namespace path must not inspect incus.service")
		return 0, nil
	}
	deps.run = func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
		t.Fatal("namespace runner must not be called when mount namespace already matches PID 1")
		return nil
	}

	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(), []string{"host", "ensure"}, bytes.NewReader(nil), io.Discard, io.Discard, deps,
	)
	if handled || err != nil {
		t.Fatalf("handled=%v err=%v, want false nil", handled, err)
	}
}

func TestHostEnsureNamespaceReexecsThroughPID1MountNamespaceWhenIncusInactive(t *testing.T) {
	deps := testHostEnsureNamespaceDeps()
	deps.readlink = func(path string) (string, error) {
		switch path {
		case selfMountNamespace:
			return "mnt:[11]", nil
		case initMountNamespace:
			return "mnt:[22]", nil
		default:
			return "", errors.New("unexpected path")
		}
	}

	var gotName string
	var gotArgs []string
	deps.run = func(_ context.Context, name string, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	}

	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(), []string{"host", "ensure"}, bytes.NewReader(nil), io.Discard, io.Discard, deps,
	)
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v, want true nil", handled, err)
	}
	if gotName != nsenterBinary {
		t.Fatalf("runner name=%q, want %q", gotName, nsenterBinary)
	}
	wantArgs := []string{"--mount=" + initMountNamespace, "--", "/usr/local/bin/haco", "host", "ensure"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("runner args=%q, want %q", gotArgs, wantArgs)
	}
}

func TestHostEnsureNamespaceReexecsThroughRunningIncusMountNamespace(t *testing.T) {
	const incusPID = 4242
	incusNamespace := fmt.Sprintf("/proc/%d/ns/mnt", incusPID)
	deps := testHostEnsureNamespaceDeps()
	deps.incusMainPID = func(context.Context) (int, error) { return incusPID, nil }
	deps.readFile = func(path string) ([]byte, error) {
		switch path {
		case initCommPath:
			return []byte("systemd\n"), nil
		case fmt.Sprintf("/proc/%d/comm", incusPID):
			return []byte("incusd\n"), nil
		default:
			return nil, errors.New("unexpected path")
		}
	}
	deps.readlink = func(path string) (string, error) {
		switch path {
		case selfMountNamespace:
			return "mnt:[11]", nil
		case initMountNamespace:
			return "mnt:[22]", nil
		case incusNamespace:
			return "mnt:[33]", nil
		default:
			return "", errors.New("unexpected path")
		}
	}

	var gotArgs []string
	deps.run = func(_ context.Context, _ string, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		gotArgs = append([]string(nil), args...)
		return nil
	}

	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(), []string{"host", "ensure"}, bytes.NewReader(nil), io.Discard, io.Discard, deps,
	)
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v, want true nil", handled, err)
	}
	wantArgs := []string{"--mount=" + incusNamespace, "--", "/usr/local/bin/haco", "host", "ensure"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("runner args=%q, want %q", gotArgs, wantArgs)
	}
}

func TestHostEnsureNamespaceContinuesWhenAlreadyWithRunningIncus(t *testing.T) {
	const incusPID = 4242
	incusNamespace := fmt.Sprintf("/proc/%d/ns/mnt", incusPID)
	deps := testHostEnsureNamespaceDeps()
	deps.incusMainPID = func(context.Context) (int, error) { return incusPID, nil }
	deps.readFile = func(path string) ([]byte, error) {
		switch path {
		case initCommPath:
			return []byte("systemd\n"), nil
		case fmt.Sprintf("/proc/%d/comm", incusPID):
			return []byte("incusd\n"), nil
		default:
			return nil, errors.New("unexpected path")
		}
	}
	deps.readlink = func(path string) (string, error) {
		switch path {
		case selfMountNamespace, incusNamespace:
			return "mnt:[33]", nil
		case initMountNamespace:
			return "mnt:[22]", nil
		default:
			return "", errors.New("unexpected path")
		}
	}
	deps.run = func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
		t.Fatal("namespace runner must not be called when already in incusd mount namespace")
		return nil
	}

	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(), []string{"host", "ensure"}, bytes.NewReader(nil), io.Discard, io.Discard, deps,
	)
	if handled || err != nil {
		t.Fatalf("handled=%v err=%v, want false nil", handled, err)
	}
}

func TestHostEnsureNamespaceLeavesNonRootPathUnchanged(t *testing.T) {
	deps := hostEnsureNamespaceDeps{
		geteuid: func() int { return 1000 },
		readlink: func(string) (string, error) {
			t.Fatal("non-root host ensure must not inspect system namespace handles")
			return "", nil
		},
	}

	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(), []string{"host", "ensure"}, bytes.NewReader(nil), io.Discard, io.Discard, deps,
	)
	if handled || err != nil {
		t.Fatalf("handled=%v err=%v, want false nil", handled, err)
	}
}

func TestHostEnsureNamespaceRejectsNonSystemdPID1(t *testing.T) {
	deps := testHostEnsureNamespaceDeps()
	deps.readlink = differingMountNamespaces
	deps.readFile = func(string) ([]byte, error) { return []byte("bash\n"), nil }

	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(), []string{"host", "ensure"}, bytes.NewReader(nil), io.Discard, io.Discard, deps,
	)
	if !handled || err == nil || !strings.Contains(err.Error(), "not systemd") {
		t.Fatalf("handled=%v err=%v, want PID1 validation failure", handled, err)
	}
}

func TestHostEnsureNamespaceRejectsUnexpectedIncusMainPID(t *testing.T) {
	const incusPID = 4242
	deps := testHostEnsureNamespaceDeps()
	deps.incusMainPID = func(context.Context) (int, error) { return incusPID, nil }
	deps.readFile = func(path string) ([]byte, error) {
		switch path {
		case initCommPath:
			return []byte("systemd\n"), nil
		case fmt.Sprintf("/proc/%d/comm", incusPID):
			return []byte("not-incusd\n"), nil
		default:
			return nil, errors.New("unexpected path")
		}
	}

	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(), []string{"host", "ensure"}, bytes.NewReader(nil), io.Discard, io.Discard, deps,
	)
	if !handled || err == nil || !strings.Contains(err.Error(), "not incusd") {
		t.Fatalf("handled=%v err=%v, want incusd identity validation failure", handled, err)
	}
}

func testHostEnsureNamespaceDeps() hostEnsureNamespaceDeps {
	return hostEnsureNamespaceDeps{
		readlink: differingMountNamespaces,
		readFile: func(path string) ([]byte, error) {
			if path == initCommPath {
				return []byte("systemd\n"), nil
			}
			return nil, errors.New("unexpected path")
		},
		geteuid:      func() int { return 0 },
		incusMainPID: func(context.Context) (int, error) { return 0, nil },
		executable: func() (string, error) {
			return "/usr/local/bin/haco", nil
		},
		evalSymlinks: func(path string) (string, error) { return path, nil },
		stat:         func(string) (os.FileInfo, error) { return nil, nil },
		run:          func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error { return nil },
	}
}

func differingMountNamespaces(path string) (string, error) {
	switch path {
	case selfMountNamespace:
		return "mnt:[11]", nil
	case initMountNamespace:
		return "mnt:[22]", nil
	default:
		return "", errors.New("unexpected path")
	}
}
