//go:build linux

package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func TestExportWorkspaceMetadataUsesSavedBindingOnly(t *testing.T) {
	runtime := New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		t.Fatal("metadata read accessed mutable guest/provider state")
		return host.Result{}, nil
	}})
	saved := core.Snapshot{State: "ready"}
	for i, name := range []string{"zeta", "alpha"} {
		p := snapshotVolumeFixture("work")
		p.SourceID = name
		p.Role = "workspace:" + name
		p.Owner = strings.Repeat(string(rune('c'+i)), 32)
		p.Remote = "https://github.com/SLktEx/" + name + ".git"
		p.Branch = "main"
		c, err := runtime.snapshotComponent(snapshotBinding{Version: 1, Project: "hacocoon", Volume: &p})
		if err != nil {
			t.Fatal(err)
		}
		c.State = "verified"
		saved.Components = append(saved.Components, c)
	}
	got, err := runtime.ExportSnapshotWorkspaces(context.Background(), saved)
	if err != nil || len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "zeta" || got[0].Role != "workspace" || got[1].Role != "workspace-002" || got[0].Remote != "https://github.com/SLktEx/alpha.git" {
		t.Fatal(got, err)
	}
	saved.Components[0].Owner = strings.Repeat("f", 32)
	if _, err := runtime.ExportSnapshotWorkspaces(context.Background(), saved); err == nil {
		t.Fatal("changed binding owner accepted")
	}
}
