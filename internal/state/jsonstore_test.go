package state

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestJSONStoreRoundTrip(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	session := core.Session{ID: "abc", Name: "demo"}
	if err := store.Put(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "demo" {
		t.Fatalf("got %q", got.Name)
	}
	if err := store.Delete(context.Background(), "abc"); err != nil {
		t.Fatal(err)
	}
	_, err = store.Get(context.Background(), "abc")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
