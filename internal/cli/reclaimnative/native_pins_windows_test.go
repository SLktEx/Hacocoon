//go:build windows && (amd64 || arm64)

package reclaimnative

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestNativeFrontendPinnedFiles(t *testing.T) {
	requireNativeFixture(t)
	root := t.TempDir()
	file := filepath.Join(root, "fixture.exe")
	if err := os.WriteFile(file, []byte("synthetic ordinary data, not an executable"), 0600); err != nil {
		t.Fatal("could not create owned file fixture")
	}
	if !t.Run("ordinary-and-type-refusal", func(t *testing.T) {
		for _, item := range []struct {
			path      string
			directory bool
		}{{root, true}, {file, false}} {
			held, err := pinComponent(item.path, item.directory)
			if err != nil {
				t.Fatal("ordinary owned component refused")
			}
			if err := windows.CloseHandle(held); err != nil {
				t.Fatal("ordinary pin close failed")
			}
		}
		for _, item := range []struct {
			path      string
			directory bool
		}{{root, false}, {file, true}} {
			held, err := pinComponent(item.path, item.directory)
			if err == nil {
				_ = windows.CloseHandle(held)
				t.Fatal("wrong component type accepted")
			}
		}
	}) {
		return
	}
	if !t.Run("hardlink-refusal", func(t *testing.T) {
		alias := filepath.Join(root, "alias.exe")
		if err := os.Link(file, alias); err != nil {
			t.Fatal("same-volume hardlink fixture unavailable")
		}
		defer func() {
			if err := os.Remove(alias); err != nil {
				t.Error("owned hardlink removal failed")
			}
		}()
		held, err := pinComponent(file, false)
		if err == nil {
			_ = windows.CloseHandle(held)
			t.Fatal("multiply linked file accepted")
		}
	}) {
		return
	}
	if !t.Run("held-file-sharing", func(t *testing.T) {
		held, err := pinComponent(file, false)
		if err != nil {
			t.Fatal("ordinary pin refused")
		}
		defer func() {
			if held != 0 {
				if err := windows.CloseHandle(held); err != nil {
					t.Error("held pin close failed")
				}
			}
		}()
		if writable, err := os.OpenFile(file, os.O_WRONLY, 0); err == nil {
			_ = writable.Close()
			t.Fatal("held file permitted a write handle")
		}
		moved := filepath.Join(root, "renamed.exe")
		if err := os.Rename(file, moved); err == nil {
			t.Fatal("held file permitted rename")
		}
		if err := windows.CloseHandle(held); err != nil {
			t.Fatal("pin release failed")
		}
		held = 0
		// The positive control distinguishes sharing exclusion from an unrelated
		// directory permission failure. Only this fixture's own ordinary file moves.
		if err := os.Rename(file, moved); err != nil {
			t.Fatal("rename remained blocked after pin release")
		}
		if err := os.Rename(moved, file); err != nil {
			t.Fatal("owned fixture rename restoration failed")
		}
		writable, err := os.OpenFile(file, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal("write handle remained blocked after pin release")
		}
		if err := writable.Close(); err != nil {
			t.Fatal("owned write handle close failed")
		}
	}) {
		return
	}
	// Reparse-point coverage deliberately remains pending: this fixture never
	// enables symlink privileges, Developer Mode or changes security settings.
}
