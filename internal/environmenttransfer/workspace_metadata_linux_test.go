//go:build linux

package environmenttransfer

import (
	"bytes"
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"reflect"
	"testing"
)

func metadataFixture() []Workspace {
	return []Workspace{{Role: "workspace", Name: "repo-000", Remote: "https://github.com/SLktEx/Hacocoon-test.git", Branch: "main"}, {Role: "workspace-002", Name: "repo-001", Remote: "file:///source-only/repo.git", Branch: "dev"}}
}
func TestWorkspaceMetadataVersionedRoundTripAndIsolation(t *testing.T) {
	for _, version := range []int{1, 2} {
		e, s, _ := exportFixture(t, 2, true)
		if version == 2 {
			e.Workspaces = func(_ context.Context, saved core.Snapshot) ([]Workspace, error) {
				if !s.locked || saved.ID != s.saved.ID {
					t.Fatal("metadata outside protected source")
				}
				return metadataFixture(), nil
			}
		}
		result, err := e.ExportStopped(context.Background(), "dev", 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		defer result.Bundle.Close()
		m := result.Bundle.Manifest()
		if m.Version != version {
			t.Fatal(m)
		}
		if version == 2 {
			if !reflect.DeepEqual(m.Workspaces, metadataFixture()) {
				t.Fatal(m)
			}
			m.Workspaces[0].Remote = "https://github.com/other/other.git"
			if !reflect.DeepEqual(result.Bundle.Manifest().Workspaces, metadataFixture()) {
				t.Fatal("mutable routing exposed")
			}
		} else if len(m.Workspaces) != 0 {
			t.Fatal("legacy metadata invented")
		}
		r, err := result.Bundle.ComponentReader("workspace-002")
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		if err != nil || string(data) != "archive workspace:repo-001" {
			t.Fatal("component mapping changed", err)
		}
		inspected, err := Inspect(result.Bundle.Reader(), 1<<20)
		if err != nil || !reflect.DeepEqual(inspected, result.Bundle.Manifest()) {
			t.Fatal(inspected, err)
		}
	}
}
func TestWorkspaceMetadataRejectsInvalidRoutingBeforeNativeExport(t *testing.T) {
	for _, mode := range []string{"missing", "duplicate-name", "wrong-order", "credential", "unknown-host", "traversal-name", "bad-branch", "partial-routing", "oversized", "callback-error"} {
		t.Run(mode, func(t *testing.T) {
			e, s, opened := exportFixture(t, 2, false)
			e.Workspaces = func(context.Context, core.Snapshot) ([]Workspace, error) {
				w := metadataFixture()
				switch mode {
				case "missing":
					return nil, nil
				case "duplicate-name":
					w[1].Name = w[0].Name
				case "wrong-order":
					w[0], w[1] = w[1], w[0]
				case "credential":
					w[0].Remote = "https://secret@github.com/org/repo.git"
				case "unknown-host":
					w[0].Remote = "https://internal.local/repo"
				case "traversal-name":
					w[0].Name = "../repo"
				case "bad-branch":
					w[0].Branch = "--upload-pack=bad"
				case "partial-routing":
					w[0].Branch = ""
				case "oversized":
					w[0].Remote = "file:///" + string(bytes.Repeat([]byte("a"), 4096))
				case "callback-error":
					return nil, core.ErrCapabilityStale
				}
				return w, nil
			}
			result, err := e.ExportStopped(context.Background(), "dev", 1<<20)
			if err == nil || result.Bundle != nil || len(*opened) != 0 || s.deletes != 1 || result.TemporarySnapshot != "" {
				t.Fatal("invalid mapping published or exported", err)
			}
		})
	}
}
func TestWorkspaceMetadataLegacyRules(t *testing.T) {
	s, a := snapshotFixture(2, false)
	var out bytes.Buffer
	if err := writeSnapshot(&out, s, a, 1<<20, []Workspace{{Role: "workspace", Name: "repo-000"}, {Role: "workspace-002", Name: "repo-001"}}); err != nil {
		t.Fatal(err)
	}
	m, err := Inspect(bytes.NewReader(out.Bytes()), 1<<20)
	if err != nil || m.Version != 2 || m.Workspaces[0].Remote != "" {
		t.Fatal(m, err)
	}
	m.Version = 1
	if !errors.Is(m.validate(1<<20), ErrInvalidBundle) {
		t.Fatal("v1 accepted routing extension")
	}
}
