//go:build linux

package controller

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func controllerFixture(t *testing.T) (string, string) {
	t.Helper()
	// Keep Unix socket paths independent of long test names.
	dir, err := os.MkdirTemp("", "haco-controller-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	root, socket := filepath.Join(dir, "data"), filepath.Join(dir, "control.sock")
	t.Setenv("HACO_ROOT", root)
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	t.Setenv("HACO_RUNTIME_PROVIDER", "")
	// Bare management startup and these catalog/Policy operations need no
	// provider executable. An accidental subprocess call must fail this test.
	t.Setenv("PATH", "")
	return root, socket
}

func startController(t *testing.T, socket string) (*controlapi.Client, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	done := make(chan error, 1)
	go func() { done <- run(ctx, nil) }()
	stopped := false
	stop := func() {
		t.Helper()
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("controller shutdown: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("controller did not finish shutdown")
		}
	}
	t.Cleanup(stop)
	client, err := controlapi.NewClient(socket)
	if err != nil {
		t.Fatal(err)
	}
	for {
		if pong, err := client.Ping(ctx); err == nil {
			if pong.ProtocolVersion != control.ProtocolVersion {
				t.Fatal("unexpected protocol", pong)
			}
			return client, stop
		}
		select {
		case err := <-done:
			stopped = true
			cancel()
			t.Fatal("controller failed before readiness", err)
		case <-ctx.Done():
			t.Fatal("controller never became ready")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestControllerRestartPreservesManagementPolicy(t *testing.T) {
	root, socket := controllerFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, stop := startController(t, socket)
	info, err := os.Stat(socket)
	if err != nil || info.Mode().Perm() != 0600 || info.Mode()&os.ModeSocket == 0 {
		t.Fatal("management endpoint is not a private socket", info, err)
	}
	if envs, err := client.ListEnvironments(ctx); err != nil || len(envs) != 0 {
		t.Fatal("new catalog", envs, err)
	}
	if pending, err := client.PendingApprovals(ctx); err != nil || len(pending) != 0 {
		t.Fatal("new approval queue", pending, err)
	}
	if pending, err := client.PendingGit(ctx); err != nil || len(pending) != 0 {
		t.Fatal("new Git proposal queue", pending, err)
	}
	before, err := client.ReadConfiguration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	edit := before
	edit.Policy = []byte(`{"default":"deny","rules":[{"capability":"local.echo","action":"echo","resource":"startup-test","environment":"*","decision":"deny"}]}`)
	after, err := client.ReplaceConfiguration(ctx, edit)
	if err != nil || after.Revision == before.Revision {
		t.Fatal("policy replacement", after, err)
	}
	if _, err := client.ReplaceConfiguration(ctx, edit); err == nil {
		t.Fatal("stale policy replaced newer state")
	}
	// Refusing a second controller must leave the existing owner usable.
	if err := run(ctx, nil); !errors.Is(err, control.ErrAlreadyRunning) {
		t.Fatal("duplicate controller was not refused", err)
	}
	if _, err := client.Ping(ctx); err != nil {
		t.Fatal("duplicate startup disturbed the owner", err)
	}
	stop()
	if _, err := os.Lstat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("shutdown retained the management socket", err)
	}
	client, stop = startController(t, socket)
	current, err := client.ReadConfiguration(ctx)
	if err != nil || current.Revision != after.Revision || string(current.Policy) != string(after.Policy) {
		t.Fatal("restart changed persisted Policy", current, err)
	}
	stop()
	if info, err := os.Stat(filepath.Join(root, "policy.json")); err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("Policy was not persisted privately", info, err)
	}
}

func TestControllerStartupFailureReleasesPublishedSocket(t *testing.T) {
	for _, obstruction := range []string{"socket-directory", "malformed-binding"} {
		t.Run(obstruction, func(t *testing.T) {
			root, socket := controllerFixture(t)
			var path string
			if obstruction == "socket-directory" {
				path = filepath.Join(root, "run")
			} else {
				path = filepath.Join(root, "state", "repositories", "bindings", "broken.json")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("retain this invalid state"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := run(ctx, nil)
			if err == nil || errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("startup did not refuse the Git broker failure", err)
			}
			if obstruction == "malformed-binding" && !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal("binding error was lost", err)
			}
			if _, err := os.Lstat(socket); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed startup left its management socket", err)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "retain this invalid state" {
				t.Fatal("failed startup rewrote existing state", string(data), err)
			}
		})
	}
}

func TestControllerRejectsConfigurationBeforePublishing(t *testing.T) {
	for _, invalid := range []string{"arguments", "provider", "socket-file"} {
		t.Run(invalid, func(t *testing.T) {
			_, socket := controllerFixture(t)
			var args []string
			switch invalid {
			case "arguments":
				args = []string{"--standard-egress=false"}
			case "provider":
				t.Setenv("HACO_RUNTIME_PROVIDER", "unavailable-provider")
			case "socket-file":
				if err := os.WriteFile(socket, []byte("existing user file"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := run(context.Background(), args); err == nil {
				t.Fatal("invalid startup succeeded")
			}
			if invalid == "socket-file" {
				if data, err := os.ReadFile(socket); err != nil || string(data) != "existing user file" {
					t.Fatal("startup replaced existing socket-path file", string(data), err)
				}
			} else if _, err := os.Lstat(socket); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid startup published a socket", err)
			}
		})
	}
}

func TestControllerProductionListenerRefusesUnexpectedAuthority(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", "")
	path := filepath.Join(t.TempDir(), "control.sock")
	if listener, err := controllerListener(path); err == nil || listener != nil {
		t.Fatal("accepted a noncanonical production socket", listener, err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("refused endpoint was created", err)
	}
	if os.Geteuid() != 0 {
		if listener, err := controllerListener(control.DefaultSocketPath); err == nil || listener != nil || !strings.Contains(err.Error(), "root authority") {
			t.Fatal("accepted production socket without root", listener, err)
		}
	}
}
