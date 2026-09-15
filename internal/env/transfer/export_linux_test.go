//go:build linux

package environmenttransfer

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

	"github.com/SLktEx/Hacocoon/internal/core"
)

type exportSnapshotsStub struct {
	saved                          core.Snapshot
	captureErr, readErr, deleteErr error
	locked                         bool
	captures, deletes              int
	deleted                        string
	beforeRead                     func(*core.Snapshot)
	cleanupContextErr              error
}

func (s *exportSnapshotsStub) CaptureStoppedSnapshot(ctx context.Context, name string) (core.Snapshot, error) {
	s.captures++
	return s.saved, s.captureErr
}
func (s *exportSnapshotsStub) ReadSnapshot(ctx context.Context, id string, read func(context.Context, core.Snapshot) error) error {
	if s.readErr != nil {
		return s.readErr
	}
	current := s.saved
	current.Components = append([]core.SnapshotComponent(nil), current.Components...)
	if s.beforeRead != nil {
		s.beforeRead(&current)
	}
	s.locked = true
	defer func() { s.locked = false }()
	return read(ctx, current)
}
func (s *exportSnapshotsStub) DeleteSnapshot(ctx context.Context, id string) error {
	if s.locked {
		panic("lifecycle re-entry inside source reservation")
	}
	s.cleanupContextErr = ctx.Err()
	s.deletes++
	s.deleted = id
	return s.deleteErr
}

type exportArchiveStub struct {
	payload  []byte
	digest   string
	reader   io.Reader
	size     int64
	closed   bool
	closeErr error
	onClose  func()
}

func (a *exportArchiveStub) Size() int64    { return a.size }
func (a *exportArchiveStub) Digest() string { return a.digest }
func (a *exportArchiveStub) Reader() io.Reader {
	if a.reader != nil {
		return a.reader
	}
	return bytes.NewReader(a.payload)
}
func (a *exportArchiveStub) Close() error {
	a.closed = true
	if a.onClose != nil {
		a.onClose()
	}
	return a.closeErr
}
func exportFixture(t *testing.T, count int, oci bool) (*Exporter, *exportSnapshotsStub, *[]*exportArchiveStub) {
	t.Helper()
	saved, inputs := snapshotFixture(count, oci)
	snapshots := &exportSnapshotsStub{saved: saved}
	byRole := map[string]SnapshotArchive{}
	for _, a := range inputs {
		byRole[a.Component.Role] = a
	}
	opened := []*exportArchiveStub{}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	e := &Exporter{Snapshots: snapshots, Root: root}
	e.Component = func(ctx context.Context, c core.SnapshotComponent, dir string, limit int64) (Archive, error) {
		if !snapshots.locked || dir != root {
			t.Fatal("producer outside protected source/private output")
		}
		input, ok := byRole[c.Role]
		if !ok {
			t.Fatal(c)
		}
		data, err := io.ReadAll(input.Data)
		if err != nil {
			t.Fatal(err)
		}
		a := &exportArchiveStub{payload: data, size: int64(len(data)), digest: input.SHA256, onClose: func() {
			if !snapshots.locked {
				t.Error("archive closed after reservation release")
			}
		}}
		opened = append(opened, a)
		return a, nil
	}
	return e, snapshots, &opened
}
func assertExportClosed(t *testing.T, opened []*exportArchiveStub) {
	t.Helper()
	for _, a := range opened {
		if !a.closed {
			t.Error("component descriptor leaked")
		}
	}
}
func TestExportStoppedCompletePrivateBundle(t *testing.T) {
	for _, count := range []int{1, 2, maxWorkspaces} {
		for _, oci := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d-%v", count, oci), func(t *testing.T) {
				e, s, opened := exportFixture(t, count, oci)
				// The source catalog may contain historical Base provenance, never export it.
				s.saved.Components = append(s.saved.Components, core.SnapshotComponent{Role: "base", State: "verified", Owner: "base-owner", Binding: "base-binding", NativeRef: "base-native"})
				original := append([]core.SnapshotComponent(nil), s.saved.Components...)
				result, err := e.ExportStopped(context.Background(), "dev", 1<<20)
				if err != nil {
					t.Fatal(err)
				}
				defer result.Bundle.Close()
				if result.TemporarySnapshot != "" || s.captures != 1 || s.deletes != 1 || s.deleted != s.saved.ID {
					t.Fatal(result, s)
				}
				if !reflect.DeepEqual(original, s.saved.Components) {
					t.Fatal("source catalog modified")
				}
				m, err := Inspect(result.Bundle.Reader(), 1<<20)
				want := 1 + count
				if oci {
					want++
				}
				if err != nil || len(m.Components) != want || m.HasOCI != oci {
					t.Fatal(m, err)
				}
				files, err := os.ReadDir(e.Root)
				if err != nil || len(files) != 0 {
					t.Fatal(files, err)
				}
				assertExportClosed(t, *opened)
			})
		}
	}
}
func TestExportStoppedFailuresNeverPublish(t *testing.T) {
	for _, which := range []string{"running", "partial-capture", "read", "identity", "generation", "unknown-role", "missing-oci", "producer", "nil-archive", "oversize", "hash", "read-bytes", "close", "delete", "cancel"} {
		t.Run(which, func(t *testing.T) {
			e, s, opened := exportFixture(t, 2, true)
			failure := errors.New("injected failure")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			original := e.Component
			switch which {
			case "running":
				s.saved = core.Snapshot{}
				s.captureErr = core.ErrIncompatibleState
			case "partial-capture":
				s.captureErr = failure
				s.deleteErr = failure
			case "read":
				s.readErr = failure
			case "identity":
				s.beforeRead = func(s *core.Snapshot) { s.ID = "snap-" + strings.Repeat("f", 32) }
			case "generation":
				s.beforeRead = func(s *core.Snapshot) { s.Source.InstanceID = "env-" + strings.Repeat("f", 32) }
			case "unknown-role":
				s.beforeRead = func(s *core.Snapshot) { s.Components[1].Role = "external" }
			case "missing-oci":
				s.beforeRead = func(s *core.Snapshot) { s.Components = s.Components[:len(s.Components)-1] }
			case "delete":
				s.deleteErr = failure
			default:
				e.Component = func(ctx context.Context, c core.SnapshotComponent, root string, limit int64) (Archive, error) {
					a, err := original(ctx, c, root, limit)
					if err != nil {
						return a, err
					}
					stub := a.(*exportArchiveStub)
					switch which {
					case "producer":
						return a, failure
					case "nil-archive":
						stub.Close()
						return nil, nil
					case "oversize":
						stub.size = limit + 1
					case "hash":
						stub.digest = strings.Repeat("0", 64)
					case "read-bytes":
						stub.reader = io.MultiReader(bytes.NewReader(stub.payload), strings.NewReader("extra"))
					case "close":
						stub.closeErr = failure
					case "cancel":
						cancel()
					}
					return a, nil
				}
			}
			result, err := e.ExportStopped(ctx, "dev", 1<<20)
			if err == nil || result.Bundle != nil {
				t.Fatal("failed operation published bytes", result, err)
			}
			if which == "delete" || which == "partial-capture" {
				if result.TemporarySnapshot != s.saved.ID || !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal(result, err)
				}
			} else if result.TemporarySnapshot != "" {
				t.Fatal(result)
			}
			if which == "running" {
				if s.deletes != 0 {
					t.Fatal("guessed deletion")
				}
			} else if s.deletes != 1 || s.deleted != s.saved.ID {
				t.Fatal(s)
			}
			if s.cleanupContextErr != nil {
				t.Fatal("cleanup inherited cancellation")
			}
			if (which == "identity" || which == "generation" || which == "unknown-role" || which == "missing-oci") && len(*opened) != 0 {
				t.Fatal("invalid inventory exported")
			}
			assertExportClosed(t, *opened)
			files, _ := os.ReadDir(e.Root)
			if len(files) != 0 {
				t.Fatal(files)
			}
		})
	}
}
func TestExportStoppedBudgetIsAggregate(t *testing.T) {
	e, _, opened := exportFixture(t, 2, true)
	original := e.Component
	remaining := int64(70)
	e.Component = func(ctx context.Context, c core.SnapshotComponent, root string, limit int64) (Archive, error) {
		if limit != remaining {
			t.Fatalf("budget reset: %d != %d", limit, remaining)
		}
		a, err := original(ctx, c, root, limit)
		if a != nil {
			remaining -= a.Size()
		}
		return a, err
	}
	result, err := e.ExportStopped(context.Background(), "dev", 70)
	if err == nil || result.Bundle != nil {
		t.Fatal(result, err)
	}
	assertExportClosed(t, *opened)
}
func TestExportStoppedRejectsBeforeCapture(t *testing.T) {
	for _, which := range []string{"name", "budget", "directory", "cancel"} {
		t.Run(which, func(t *testing.T) {
			e, s, _ := exportFixture(t, 1, false)
			name := "dev"
			limit := int64(1 << 20)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch which {
			case "name":
				name = "--all"
			case "budget":
				limit = 0
			case "directory":
				e.Root = "relative"
			case "cancel":
				cancel()
			}
			r, err := e.ExportStopped(ctx, name, limit)
			if err == nil || r.Bundle != nil || s.captures != 0 || s.deletes != 0 {
				t.Fatal(r, err, s)
			}
		})
	}
}
func TestStageProducedRejectsLateFailure(t *testing.T) {
	_, archives := snapshotFixture(1, false)
	saved, _ := snapshotFixture(1, false)
	root := t.TempDir()
	os.Chmod(root, 0700)
	failure := errors.New("cleanup failed after complete bytes")
	result, err := stageProduced(context.Background(), root, 1<<20, func(w io.Writer) error {
		if err := WriteSnapshot(w, saved, archives, 1<<20); err != nil {
			return err
		}
		return failure
	})
	if result != nil || !errors.Is(err, failure) {
		t.Fatal(result, err)
	}
}
