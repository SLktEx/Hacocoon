package incus

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
)

// Explicit teardown of a retained, inspected test fixture, not product recovery.
// Ambiguous copy journals or ownership still require recovery and are refused.
func TestCleanupRealIncusHostAreaFixture(t *testing.T) {
	project := os.Getenv("HACO_E2E_CLEANUP_AREA_PROJECT")
	if project == "" {
		t.Skip("explicit retained fixture cleanup only")
	}
	root := os.Getenv("HACO_E2E_CLEANUP_AREA_STATE")
	if os.Geteuid() != 0 || !regexp.MustCompile(`^haco-area-[a-f0-9]{16}$`).MatchString(project) || !regexp.MustCompile(`^/tmp/haco-host-area-state-[0-9]+$`).MatchString(root) {
		t.Fatal("invalid fixture scope")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("invalid state directory")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	runner := host.ExecRunner{}
	runtime := New(runner)
	runtime.project = project
	backend := &PersistentResourceBackend{Runtime: runtime}
	store := state.NewEnvironmentJSONStore(filepath.Join(root, "state.json"))
	service := &persistentresource.Service{Store: store, Backend: backend}
	resources, err := store.ListPersistentResources(ctx)
	if err != nil || len(resources) != 2 {
		t.Fatal("unexpected fixture catalog")
	}
	command := func(args ...string) string {
		t.Helper()
		r, e := runner.Run(ctx, "incus", args...)
		if e != nil || r.ExitCode != 0 || r.StdoutTruncated {
			t.Fatal("fixture cleanup command failed", args)
		}
		return r.Stdout
	}
	var instances []hostOCICopyInstance
	if json.Unmarshal([]byte(command("query", "/1.0/instances?recursion=1&project="+project)), &instances) != nil || len(instances) != 2 {
		t.Fatal("unexpected fixture instances")
	}
	images := map[string]bool{}
	for _, i := range instances {
		if i.Type != "container" || i.Profiles == nil || len(i.Profiles) != 0 || i.StatusCode != 103 || i.Config[hostOCICopyKey] != "" {
			t.Fatal("ambiguous fixture state")
		}
		marker, key := "runtime-copy-fixture", "user.hacocoon.kind"
		if i.Name == trustedHostName {
			marker, key = trustedHostRoleValue, trustedHostRoleKey
		} else if i.Name != "haco-area-runtime-copy" {
			t.Fatal("foreign fixture instance")
		}
		if i.LocalConfig[key] != marker {
			t.Fatal("foreign fixture ownership")
		}
		image := i.Config["volatile.base_image"]
		decoded, e := hex.DecodeString(image)
		if e != nil || len(decoded) != 32 {
			t.Fatal("invalid fixture image")
		}
		images[image] = true
	}
	for _, resource := range resources {
		pool, volume, e := persistentVolume(resource)
		if e != nil || pool != project || resource.State != "ready" {
			t.Fatal("unexpected fixture resource")
		}
		expected := "haco-area-runtime-copy"
		if resource.SourceOnly {
			expected = trustedHostName
			if e := backend.VerifyHostSource(ctx, resource); e != nil {
				t.Fatal(e)
			}
		} else if resource.WorkspaceID != "area-copy-work" {
			t.Fatal("foreign fixture Workspace")
		}
		observed, e := backend.observe(ctx, resource)
		if e != nil || observed == nil || len(observed.UsedBy) != 1 || observed.UsedBy[0] != "/1.0/instances/"+expected+"?project="+project {
			t.Fatal("foreign fixture consumer")
		}
		matches := 0
		for _, i := range instances {
			if i.Name != expected {
				continue
			}
			for _, d := range i.Devices {
				if d["type"] == "disk" && d["pool"] == pool && d["source"] == volume && d["path"] == OCIStorePath {
					matches++
				}
			}
		}
		if matches != 1 {
			t.Fatal("fixture attachment differs")
		}
	}
	for _, i := range instances {
		command("delete", i.Name, "--force", "--project", project)
	}
	for _, r := range resources {
		if err := service.Delete(ctx, r.ID); err != nil {
			t.Fatal(err)
		}
	}
	for image := range images {
		command("image", "delete", image, "--project", project)
	}
	command("project", "delete", project)
	command("storage", "delete", project)
	for _, name := range []string{"state.json", "state.json.lock"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	t.Log("PASS exact retained fixture cleanup via canonical resource deletion:", strings.TrimSpace(project))
}
