//go:build linux

package recipes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecipePersistsBeforeFailureAndReplaysAcrossServiceRecreation(t *testing.T) {
	root := privateDir(t)
	script := "\ufeffecho example\r\n"
	failure := errors.New("script failed")
	s := &Service{Root: root, Execute: func(_ context.Context, b []byte) error {
		if string(b) != "echo example\n" {
			t.Fatalf("unexpected normalization %q", b)
		}
		return failure
	}}
	if err := s.Apply(context.Background(), Update{Script: &script}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, recipeFile))
	if err != nil || string(data) != "echo example\n" {
		t.Fatalf("saved recipe: %q %v", data, err)
	}
	called := 0
	s = &Service{Root: root, Execute: func(_ context.Context, b []byte) error { called++; return nil }}
	if err := s.Apply(context.Background(), Update{}); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatal("saved recipe not replayed")
	}
	if err := s.Apply(context.Background(), Update{Clear: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(context.Background(), Update{}); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatal("cleared recipe executed")
	}
}
func TestRecipeRejectsUnsafeFilesWithoutExecuting(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "public", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			root := privateDir(t)
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(outside, []byte("unchanged"), 0600); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, recipeFile)
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(outside, target)
			case "hardlink":
				err = os.Link(outside, target)
			case "public":
				err = os.WriteFile(target, []byte("content"), 0644)
			case "fifo":
				err = makeFIFO(target)
			}
			if err != nil {
				t.Fatal(err)
			}
			s := &Service{Root: root, Execute: func(context.Context, []byte) error { t.Fatal("unsafe recipe executed"); return nil }}
			replacement := "echo replacement"
			for _, u := range []Update{{}, {Script: &replacement}, {Clear: true}} {
				if err := s.Apply(context.Background(), u); err == nil {
					t.Fatal("unsafe file accepted")
				}
			}
			data, _ := os.ReadFile(outside)
			if string(data) != "unchanged" {
				t.Fatal("outside file modified")
			}
		})
	}
}
func TestRecipeSerializesReplayAndRejectsInvalidInput(t *testing.T) {
	root := privateDir(t)
	s := &Service{Root: root, Execute: func(context.Context, []byte) error { return nil }}
	f, err := openStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(context.Background(), Update{}); err == nil {
		t.Fatal("concurrent replay accepted")
	}
	f.close()
	for _, script := range []string{"bad\x00input", string([]byte{0xff}), strings.Repeat("x", MaxScriptBytes+1)} {
		if err := s.Apply(context.Background(), Update{Script: &script}); err == nil {
			t.Fatal("invalid script accepted")
		}
	}
	script := "true"
	if err := s.Apply(context.Background(), Update{Script: &script, Clear: true}); err == nil {
		t.Fatal("ambiguous update accepted")
	}
}

func privateDir(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	return root
}
