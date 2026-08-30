package ec2

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var testRefKey = []byte("0123456789abcdef0123456789abcdef")

type authFakeInner struct {
	ref          string
	calls        int
	lastRef      string
	inspectState core.EnvironmentState
}

func (f *authFakeInner) record(ref string) { f.calls++; f.lastRef = ref }

func (f *authFakeInner) CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	f.calls++
	return core.EnvironmentRuntime{Ref: f.ref}, nil
}
func (f *authFakeInner) ExecEnvironment(_ context.Context, ref string, _ core.ExecutionRequest) (core.ExecutionResult, error) {
	f.record(ref)
	return core.ExecutionResult{ExitCode: 0}, nil
}
func (f *authFakeInner) ShellEnvironment(_ context.Context, ref string) error {
	f.record(ref)
	return nil
}
func (f *authFakeInner) DeleteEnvironment(_ context.Context, ref string) error {
	f.record(ref)
	return nil
}
func (f *authFakeInner) InspectEnvironment(_ context.Context, ref string) (core.EnvironmentRuntimeStatus, error) {
	f.record(ref)
	return core.EnvironmentRuntimeStatus{State: f.inspectState}, nil
}

func canonicalInnerRef(t *testing.T, mutate func(*runtimeRef)) string {
	t.Helper()
	ref := runtimeRef{
		InstanceID:    "i-0123456789abcdef0",
		WorkspacePath: "/srv/hacocoon/workspaces/demo",
		Bucket:        "hacocoon-workspaces-example",
		Prefix:        "tests/demo",
		ReadOnly:      false,
	}
	if mutate != nil { mutate(&ref) }
	raw, err := encodeRef(ref)
	if err != nil { t.Fatal(err) }
	return raw
}

func forgeAuthenticatedPayload(t *testing.T, signed, replacementInner string) string {
	t.Helper()
	if !strings.HasPrefix(signed, authenticatedRefPrefix) { t.Fatalf("not authenticated: %q", signed) }
	parts := strings.Split(strings.TrimPrefix(signed, authenticatedRefPrefix), ".")
	if len(parts) != 2 { t.Fatalf("bad signed test ref: %q", signed) }
	return authenticatedRefPrefix + base64.RawURLEncoding.EncodeToString([]byte(replacementInner)) + "." + parts[1]
}

func TestAuthenticatedRuntimeRoundTripPreservesInnerAuthority(t *testing.T) {
	innerRef := canonicalInnerRef(t, nil)
	inner := &authFakeInner{ref: innerRef, inspectState: core.EnvironmentRunning}
	provider, err := NewAuthenticated(inner, testRefKey)
	if err != nil { t.Fatal(err) }

	created, err := provider.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/srv/hacocoon/workspaces/demo"})
	if err != nil { t.Fatal(err) }
	if !strings.HasPrefix(created.Ref, authenticatedRefPrefix) || created.Ref == innerRef {
		t.Fatalf("persisted ref was not authenticated: %q", created.Ref)
	}
	before := inner.calls
	if _, err := provider.ExecEnvironment(context.Background(), created.Ref, core.ExecutionRequest{Argv: []string{"true"}}); err != nil { t.Fatal(err) }
	if inner.calls != before+1 || inner.lastRef != innerRef {
		t.Fatalf("inner received wrong authority: calls=%d ref=%q", inner.calls, inner.lastRef)
	}
}

func TestAuthenticatedRuntimeRejectsEveryForgedAuthoritySelector(t *testing.T) {
	originalInner := canonicalInnerRef(t, nil)
	signed, err := encodeAuthenticatedRef(originalInner, testRefKey)
	if err != nil { t.Fatal(err) }

	cases := map[string]func(*runtimeRef){
		"instance":  func(r *runtimeRef) { r.InstanceID = "i-fedcba98765432100" },
		"workspace": func(r *runtimeRef) { r.WorkspacePath = "/srv/hacocoon/workspaces/victim" },
		"bucket":    func(r *runtimeRef) { r.Bucket = "hacocoon-workspaces-victim" },
		"prefix":    func(r *runtimeRef) { r.Prefix = "tests/victim" },
		"read-only": func(r *runtimeRef) { r.ReadOnly = true },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			inner := &authFakeInner{}
			provider, err := NewAuthenticated(inner, testRefKey)
			if err != nil { t.Fatal(err) }
			forged := forgeAuthenticatedPayload(t, signed, canonicalInnerRef(t, mutate))
			if err := provider.DeleteEnvironment(context.Background(), forged); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatalf("forged %s selector err=%v", name, err)
			}
			if inner.calls != 0 {
				t.Fatalf("forged %s selector reached host-authority provider: calls=%d", name, inner.calls)
			}
		})
	}
}

func TestAuthenticatedRuntimeRejectsLegacyUnsignedRefBeforeAuthority(t *testing.T) {
	inner := &authFakeInner{}
	provider, err := NewAuthenticated(inner, testRefKey)
	if err != nil { t.Fatal(err) }
	err = provider.DeleteEnvironment(context.Background(), canonicalInnerRef(t, nil))
	if !errors.Is(err, core.ErrRecoveryRequired) || !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("legacy unsigned ref err=%v", err)
	}
	if inner.calls != 0 { t.Fatalf("legacy unsigned ref reached inner provider: %d", inner.calls) }
}

func TestAuthenticatedRuntimeRejectsWrongHostKeyAfterRestart(t *testing.T) {
	innerRef := canonicalInnerRef(t, nil)
	signed, err := encodeAuthenticatedRef(innerRef, testRefKey)
	if err != nil { t.Fatal(err) }
	wrongKey := []byte("fedcba9876543210fedcba9876543210")
	inner := &authFakeInner{}
	provider, err := NewAuthenticated(inner, wrongKey)
	if err != nil { t.Fatal(err) }
	if err := provider.DeleteEnvironment(context.Background(), signed); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("wrong-key ref err=%v", err)
	}
	if inner.calls != 0 { t.Fatalf("wrong-key ref reached inner provider") }
}

func TestLoadOrCreateRefKeyPersistsAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "ec2-ref.key")
	first, err := LoadOrCreateRefKey(path)
	if err != nil { t.Fatal(err) }
	second, err := LoadOrCreateRefKey(path)
	if err != nil { t.Fatal(err) }
	if string(first) != string(second) || len(first) != refKeyBytes {
		t.Fatalf("ref key did not persist: first=%d second=%d", len(first), len(second))
	}
	info, err := os.Stat(path)
	if err != nil { t.Fatal(err) }
	if info.Mode().Perm() != 0o600 { t.Fatalf("ref key mode=%o", info.Mode().Perm()) }

	innerRef := canonicalInnerRef(t, nil)
	signed, err := encodeAuthenticatedRef(innerRef, first)
	if err != nil { t.Fatal(err) }
	inner := &authFakeInner{}
	restarted, err := NewAuthenticated(inner, second)
	if err != nil { t.Fatal(err) }
	if err := restarted.DeleteEnvironment(context.Background(), signed); err != nil { t.Fatal(err) }
	if inner.calls != 1 || inner.lastRef != innerRef { t.Fatalf("restart lost ref authority: calls=%d ref=%q", inner.calls, inner.lastRef) }
}

func TestLoadOrCreateRefKeyRejectsExposedOrSymlinkKey(t *testing.T) {
	t.Run("exposed permissions", func(t *testing.T) {
		state := filepath.Join(t.TempDir(), "state")
		if err := os.MkdirAll(state, 0o700); err != nil { t.Fatal(err) }
		path := filepath.Join(state, "ec2-ref.key")
		if err := os.WriteFile(path, testRefKey, 0o644); err != nil { t.Fatal(err) }
		if _, err := LoadOrCreateRefKey(path); !errors.Is(err, core.ErrIncompatibleState) { t.Fatalf("err=%v", err) }
	})
	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		state := filepath.Join(root, "state")
		if err := os.MkdirAll(state, 0o700); err != nil { t.Fatal(err) }
		target := filepath.Join(root, "target")
		if err := os.WriteFile(target, testRefKey, 0o600); err != nil { t.Fatal(err) }
		path := filepath.Join(state, "ec2-ref.key")
		if err := os.Symlink(target, path); err != nil { t.Fatal(err) }
		if _, err := LoadOrCreateRefKey(path); !errors.Is(err, core.ErrIncompatibleState) { t.Fatalf("err=%v", err) }
	})
}
