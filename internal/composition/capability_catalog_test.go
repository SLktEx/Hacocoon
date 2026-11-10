package composition

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestControllerCatalogCoversRegisteredProductionCapabilities(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HACO_ROOT", root)
	t.Setenv("HACO_RUNTIME_PROVIDER", "")
	app, err := Controller(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Networks.Close)
	// Discovery must not read or repair Policy, including malformed private data.
	policyPath := filepath.Join(root, "policy.json")
	private := []byte("private malformed policy; discovery must not inspect this")
	if err := os.WriteFile(policyPath, private, 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := app.Capabilities.CapabilityCatalog(context.Background())
	if err != nil {
		t.Fatal("a registered provider lacks discovery metadata", err)
	}
	got := map[string][]string{}
	for _, descriptor := range catalog.Capabilities {
		for _, action := range descriptor.Actions {
			got[descriptor.Capability] = append(got[descriptor.Capability], action.Action)
		}
	}
	want := map[string][]string{
		"aws.s3":          {"GetObject", "ListObjectsV2"},
		"git.repository":  {"fetch", "push"},
		"network.connect": {"connect"},
		"network.egress":  {"connect"},
		"network.resolve": {"lookup"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("production discovery changed: got %#v, want %#v", got, want)
	}
	if after, err := os.ReadFile(policyPath); err != nil || string(after) != string(private) {
		t.Fatal("discovery changed Policy", err)
	}
	if _, err := os.Stat(filepath.Join(root, "audit", "capabilities.jsonl")); !os.IsNotExist(err) {
		t.Fatal("discovery recorded an operation", err)
	}
}
