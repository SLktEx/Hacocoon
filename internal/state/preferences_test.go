package state

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultImagePersistsAndInitialSetupPreservesChoice(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "environments.json")
	s := NewEnvironmentJSONStore(path)
	if err := s.SetDefaultImage(ctx, "standard", true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDefaultImage(ctx, "tools", false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDefaultImage(ctx, "standard", true); err != nil {
		t.Fatal(err)
	}
	got, err := NewEnvironmentJSONStore(path).DefaultImage(ctx)
	if err != nil || got != "tools" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestVersion16PreferencesUpgradePreservesExistingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "environments.json")
	old := newEnvironmentFileState()
	old.Version = 16
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	s := NewEnvironmentJSONStore(path)
	if err = s.SetDefaultImage(context.Background(), "standard", true); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got environmentFileState
	if json.Unmarshal(after, &got) != nil || got.Version != 17 || got.DefaultImage != "standard" {
		t.Fatal(string(after))
	}
}
