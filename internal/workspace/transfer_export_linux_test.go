//go:build linux

package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type transferCaptureRuntime struct{ *captureRuntime }

func (r *transferCaptureRuntime) PlanSnapshot(ctx context.Context, source core.SnapshotSource, id string) ([]core.SnapshotComponent, error) {
	components, err := r.captureRuntime.PlanSnapshot(ctx, source, id)
	for i := range components {
		components[i].Binding = "protected-binding-" + components[i].Role
		components[i].NativeRef = id + "-" + components[i].Role
	}
	return components, err
}
func (r *transferCaptureRuntime) VerifySnapshotComponent(ctx context.Context, c core.SnapshotComponent) error {
	// Capture verifies created bytes; read verifies the committed saved bytes.
	want := "created"
	if c.State == "verified" {
		want = "verified"
	}
	r.checkReceipt(c, want)
	return r.trace.step("verify:" + c.Role)
}

type transferTestArchive struct{ payload []byte }

func (a *transferTestArchive) Size() int64 { return int64(len(a.payload)) }
func (a *transferTestArchive) Digest() string {
	h := sha256.Sum256(a.payload)
	return hex.EncodeToString(h[:])
}
func (a *transferTestArchive) Reader() io.Reader { return bytes.NewReader(a.payload) }
func (a *transferTestArchive) Close() error      { return nil }

func TestTransferExportUsesCanonicalLifecycle(t *testing.T) {
	for _, which := range []string{"success", "running", "producer-fails", "cleanup-fails"} {
		t.Run(which, func(t *testing.T) {
			ctx := context.Background()
			svc, catalog, base := captureFixture(t)
			runtime := &transferCaptureRuntime{base}
			svc.runtime = runtime
			// A previously saved snapshot must survive every outcome of a later export.
			retained, err := svc.CaptureStoppedSnapshot(ctx, "resume")
			if err != nil {
				t.Fatal(err)
			}
			envBefore, err := catalog.GetEnvironment(ctx, "resume")
			if err != nil {
				t.Fatal(err)
			}
			leaseBefore, err := catalog.GetWorkspaceLease(ctx, "resume")
			if err != nil {
				t.Fatal(err)
			}
			if which == "running" {
				runtime.inspect = func(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
					return core.EnvironmentRuntimeStatus{State: core.EnvironmentRunning}, nil
				}
			}
			if which == "cleanup-fails" {
				runtime.trace.fail = "delete:workspace:main"
			}
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			other := New(runtime, state.NewEnvironmentJSONStore(catalog.path))
			called := false
			exporter := environmenttransfer.Exporter{Root: root, Snapshots: svc, Component: func(ctx context.Context, c core.SnapshotComponent, _ string, _ int64) (environmenttransfer.Archive, error) {
				called = true
				// Independent service instances must respect the same reservation.
				limited, cancel := context.WithTimeout(ctx, 60*time.Millisecond)
				defer cancel()
				if err := other.DeleteSnapshot(limited, runtime.id); !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("source deletion bypassed export", err)
				}
				if which == "producer-fails" {
					return nil, errors.New("native export failed")
				}
				return &transferTestArchive{[]byte("native data: " + c.Role)}, nil
			}}
			result, err := exporter.ExportStopped(ctx, "resume", 1<<20)
			if which == "success" {
				if err != nil || result.Bundle == nil {
					t.Fatal(result, err)
				}
				defer result.Bundle.Close()
				if _, err := environmenttransfer.Inspect(result.Bundle.Reader(), 1<<20); err != nil {
					t.Fatal(err)
				}
			} else if err == nil || result.Bundle != nil {
				t.Fatal("failed export published", result, err)
			}
			if which == "running" && called {
				t.Fatal("running source reached producer")
			}
			current, readErr := catalog.GetSnapshot(ctx, retained.ID)
			if readErr != nil || !reflect.DeepEqual(current, retained) {
				t.Fatal("retained snapshot changed", readErr)
			}
			envAfter, _ := catalog.GetEnvironment(ctx, "resume")
			leaseAfter, _ := catalog.GetWorkspaceLease(ctx, "resume")
			if !reflect.DeepEqual(envBefore, envAfter) || !reflect.DeepEqual(leaseBefore, leaseAfter) {
				t.Fatal("source identity or lease changed")
			}
			if which == "cleanup-fails" {
				if result.TemporarySnapshot == "" || !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal(result, err)
				}
				pending, err := catalog.GetSnapshot(ctx, result.TemporarySnapshot)
				if err != nil || pending.State != "deleting" {
					t.Fatal("uncertain cleanup receipt lost", pending, err)
				}
				runtime.trace.fail = ""
				if err := svc.DeleteSnapshot(ctx, pending.ID); err != nil {
					t.Fatal("explicit owned cleanup failed", err)
				}
			} else if result.TemporarySnapshot != "" {
				t.Fatal(result)
			}
			if which != "running" {
				if _, err := catalog.GetSnapshot(ctx, runtime.id); !errors.Is(err, core.ErrNotFound) {
					t.Fatal("temporary snapshot survived confirmed cleanup", err)
				}
			}
		})
	}
}
