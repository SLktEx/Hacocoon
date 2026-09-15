//go:build linux

package cache

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSettingsApplyOnceAndRefuseStaleEditor(t *testing.T) {
	ctx := context.Background()
	s := Settings{Path: filepath.Join(t.TempDir(), "private", "cache.json")}
	initial, err := s.Read(ctx)
	if err != nil || len(initial.Configuration.Areas) != 0 {
		t.Fatal(initial, err)
	}
	edit := initial
	edit.Configuration = Configuration{Areas: []Area{{Name: "compiler", Path: "/root/.cache/go-build", Compatibility: "go-linux-amd64"}}}
	saved, err := s.Replace(ctx, edit)
	if err != nil || saved.Revision == initial.Revision {
		t.Fatal(saved, err)
	}
	if _, err := s.Replace(ctx, edit); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("stale edit replaced settings", err)
	}
	reopened, err := (Settings{Path: s.Path}).Read(ctx)
	if err != nil || reopened.Revision != saved.Revision {
		t.Fatal(reopened, err)
	}
}
func TestSettingsRejectUnsafeFileAndLock(t *testing.T) {
	for _, mode := range []string{"symlink", "hardlink", "public", "fifo", "lock-link"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			s := Settings{Path: filepath.Join(dir, "cache.json")}
			ctx := context.Background()
			initial, err := s.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			other := filepath.Join(t.TempDir(), "private.json")
			if err := os.WriteFile(other, []byte(`{"areas":[]}`), 0600); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "symlink":
				err = os.Symlink(other, s.Path)
			case "hardlink":
				err = os.Link(other, s.Path)
			case "public":
				err = os.WriteFile(s.Path, []byte(`{"areas":[]}`), 0644)
			case "fifo":
				err = syscall.Mkfifo(s.Path, 0600)
			case "lock-link":
				err = os.Symlink(other, s.Path+".lock")
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Replace(ctx, initial); err == nil {
				t.Fatal("unsafe state accepted")
			}
			content, err := os.ReadFile(other)
			if err != nil || string(content) != `{"areas":[]}` {
				t.Fatal("outside file changed", err)
			}
		})
	}
}
