//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestHostEnsureNamespaceIgnoresOtherCommands(t *testing.T) {
	deps := hostEnsureNamespaceDeps{}
	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(),
		[]string{"doctor"},
		bytes.NewReader(nil),
		io.Discard,
		io.Discard,
		deps,
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
	deps.run = func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
		t.Fatal("namespace runner must not be called when mount namespace already matches PID 1")
		return nil
	}

	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(),
		[]string{"host", "ensure"},
		bytes.NewReader(nil),
		io.Discard,
		io.Discard,
		deps,
	)
	if handled || err != nil {
		t.Fatalf("handled=%v err=%v, want false nil", handled, err)
	}
}

func TestHostEnsureNamespaceReexecsThroughPID1MountNamespace(t *testing.T) {
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
		context.Background(),
		[]string{"host", "ensure"},
		bytes.NewReader(nil),
		io.Discard,
		io.Discard,
		deps,
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

func TestHostEnsureNamespaceRejectsDifferentNamespaceWithoutRoot(t *testing.T) {
	deps := testHostEnsureNamespaceDeps()
	deps.readlink = differingMountNamespaces
	deps.geteuid = func() int { return 1000 }

	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(),
		[]string{"host", "ensure"},
		bytes.NewReader(nil),
		io.Discard,
		io.Discard,
		deps,
	)
	if !handled || err == nil || !strings.Contains(err.Error(), "must run as root") {
		t.Fatalf("handled=%v err=%v, want root-required failure", handled, err)
	}
}

func TestHostEnsureNamespaceRejectsNonSystemdPID1(t *testing.T) {
	deps := testHostEnsureNamespaceDeps()
	deps.readlink = differingMountNamespaces
	deps.readFile = func(string) ([]byte, error) { return []byte("bash\n"), nil }

	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(),
		[]string{"host", "ensure"},
		bytes.NewReader(nil),
		io.Discard,
		io.Discard,
		deps,
	)
	if !handled || err == nil || !strings.Contains(err.Error(), "not systemd") {
		t.Fatalf("handled=%v err=%v, want PID1 validation failure", handled, err)
	}
}

func testHostEnsureNamespaceDeps() hostEnsureNamespaceDeps {
	return hostEnsureNamespaceDeps{
		readlink: differingMountNamespaces,
		readFile: func(string) ([]byte, error) { return []byte("systemd\n"), nil },
		geteuid:  func() int { return 0 },
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
