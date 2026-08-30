package state

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestListEnvironmentsReturnsStableNameOrder(t *testing.T) {
	store := NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "environments.json"))
	for _, name := range []string{"zeta", "alpha", "middle"} {
		if err := store.PutEnvironment(context.Background(), core.Environment{
			Name:       name,
			Workspace:  core.Workspace{ID: core.WorkspaceID("/work/" + name), Path: "/work/" + name},
			AccessMode: core.WorkspaceReadWrite,
			RuntimeRef: "haco-" + name,
		}); err != nil {
			t.Fatal(err)
		}
	}

	environments, err := store.ListEnvironments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(environments) != 3 {
		t.Fatalf("len = %d, want 3", len(environments))
	}
	want := []string{"alpha", "middle", "zeta"}
	for i, name := range want {
		if environments[i].Name != name {
			t.Fatalf("environment[%d] = %q, want %q", i, environments[i].Name, name)
		}
	}
}
