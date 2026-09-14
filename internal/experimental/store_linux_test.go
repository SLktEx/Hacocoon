//go:build linux

package experimental

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func testStore(t *testing.T) Store {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "hacocoon")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return Store{Path: filepath.Join(dir, "config.yaml")}
}

func TestStorePreservesOtherConfigurationAndRejectsStaleEdit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := os.WriteFile(s.Path, []byte("unrelated:\n  list: [one, two]\nexperimental:\n  other: {keep: true}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	object, _ := DecodeObject([]byte("settings: {editor.formatOnSave: true}"))
	if err := s.Replace(ctx, snap.Revision, object); err != nil {
		t.Fatal(err)
	}
	if err := s.Replace(ctx, snap.Revision, map[string]any{}); err == nil {
		t.Fatal("stale edit accepted")
	}
	b, _ := os.ReadFile(s.Path)
	m, err := DecodeObject(b)
	if err != nil {
		t.Fatal(err)
	}
	if m["unrelated"] == nil || m["experimental"].(map[string]any)["other"] == nil {
		t.Fatal("lost sibling")
	}
	if _, err := ParseVSCode([]byte("force: true")); err == nil {
		t.Fatal("schema")
	}
	before := string(b)
	snap, _ = s.Read(ctx)
	if err := s.Replace(ctx, snap.Revision, map[string]any{"force": true}); err == nil {
		t.Fatal("invalid persisted")
	}
	b, _ = os.ReadFile(s.Path)
	if string(b) != before {
		t.Fatal("failed write changed YAML")
	}
}

func TestStoreConcurrentSingleWinner(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	snap, _ := s.Read(ctx)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- s.Replace(ctx, snap.Revision, map[string]any{}) }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("winners: %d", success)
	}
}

func TestStoreUnsafeAndMalformedFiles(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "directory", "malformed", "empty", "writable", "null"} {
		t.Run(kind, func(t *testing.T) {
			s := testStore(t)
			target := filepath.Join(t.TempDir(), "target")
			if err := os.WriteFile(target, []byte("{}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(target, s.Path)
			case "hardlink":
				err = os.Link(target, s.Path)
			case "directory":
				err = os.Mkdir(s.Path, 0700)
			case "malformed":
				err = os.WriteFile(s.Path, []byte("experimental: ["), 0600)
			case "empty":
				err = os.WriteFile(s.Path, nil, 0600)
			case "null":
				err = os.WriteFile(s.Path, []byte("experimental: {vscode: null}"), 0600)
			case "writable":
				err = os.WriteFile(s.Path, []byte("{}"), 0600)
				if err == nil {
					err = os.Chmod(s.Path, 0666)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Read(context.Background()); err == nil {
				t.Fatal("unsafe configuration read")
			}
			if err := s.Replace(context.Background(), "stale", map[string]any{}); err == nil {
				t.Fatal("unsafe configuration replaced")
			}
			b, _ := os.ReadFile(target)
			if string(b) != "{}\n" {
				t.Fatal("outside file changed")
			}
		})
	}
}
