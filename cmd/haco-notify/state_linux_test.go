//go:build linux

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestNotifyStateDoesNotTouchPredictableTemporaryLink(t *testing.T) {
	directory := t.TempDir()
	state := filepath.Join(directory, "state.json")
	victim := filepath.Join(directory, "unrelated")
	if err := os.WriteFile(victim, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, state+".tmp"); err != nil {
		t.Fatal(err)
	}
	if err := saveState(state, notifyState{Offset: 42}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(victim)
	if err != nil || string(data) != "preserve" {
		t.Fatalf("victim modified: %q %v", data, err)
	}
	if _, err := os.Readlink(state + ".tmp"); err != nil {
		t.Fatal("foreign temporary link removed")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".native-notify-") {
			t.Fatal("owned temporary file leaked")
		}
	}
}

func TestNotifyStateRejectsLinkedSpecialAndOversizedFiles(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "fifo", "oversized", "permissions"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			state := filepath.Join(directory, "state.json")
			victim := filepath.Join(directory, "unrelated")
			if err := os.WriteFile(victim, []byte("preserve"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(victim, state)
			case "hardlink":
				err = os.Link(victim, state)
			case "fifo":
				err = syscall.Mkfifo(state, 0600)
			case "oversized":
				err = os.WriteFile(state, []byte(strings.Repeat("x", maxNotifyStateBytes+1)), 0600)
			case "permissions":
				err = os.WriteFile(state, []byte("{}"), 0666)
				if err == nil {
					err = os.Chmod(state, 0666)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = loadState(state); err == nil {
				t.Fatal("unsafe read accepted")
			}
			if kind != "oversized" {
				if err = saveState(state, notifyState{Offset: 42}); err == nil {
					t.Fatal("unsafe replacement accepted")
				}
			}
			data, _ := os.ReadFile(victim)
			if string(data) != "preserve" {
				t.Fatal("unrelated file changed")
			}
		})
	}
}

func TestNotifyStateLockProcess(t *testing.T) {
	if path := os.Getenv("HACO_NOTIFY_LOCK_PROBE"); path != "" {
		store, err := openNotifyStore(path)
		if err == nil {
			store.close()
			t.Fatal("second process acquired live state")
		}
		if !strings.Contains(err.Error(), "another notification client") {
			t.Fatalf("unexpected refusal: %v", err)
		}
		return
	}
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := openNotifyStore(path)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		store.close()
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestNotifyStateLockProcess$")
	child.Env = append(os.Environ(), "HACO_NOTIFY_LOCK_PROBE="+path)
	output, err := child.CombinedOutput()
	store.close()
	if err != nil {
		t.Fatalf("child: %v %s", err, output)
	}
	next, err := openNotifyStore(path)
	if err != nil {
		t.Fatalf("lock survived owner close: %v", err)
	}
	next.close()
}

func TestNotifyStateRemainsInPinnedDirectory(t *testing.T) {
	parent := t.TempDir()
	original := filepath.Join(parent, "original")
	store, err := openNotifyStore(filepath.Join(original, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	moved := filepath.Join(parent, "moved")
	if err = os.Rename(original, moved); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	if err = store.save(notifyState{Offset: 99}); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(original, "state.json")); !os.IsNotExist(err) {
		t.Fatal("replacement directory was modified")
	}
	state, err := loadState(filepath.Join(moved, "state.json"))
	if err != nil || state.Offset != 99 {
		t.Fatalf("pinned state: %+v %v", state, err)
	}
}

func TestNotifyDuplicateStopsBeforeReadingOrDelivering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	owner, err := openNotifyStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	reader := &scriptedReader{}
	presenter := &recordingNotifier{}
	if err = runNative(context.Background(), reader, presenter, path, time.Second, true, false); err == nil {
		t.Fatal("duplicate accepted")
	}
	if reader.calledCount != 0 || len(presenter.titles) != 0 {
		t.Fatal("duplicate observed or delivered events")
	}
}

func TestNotifyStateRejectsUnsafeParentAndLock(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0777); err != nil {
		t.Fatal(err)
	}
	if store, err := openNotifyStore(filepath.Join(directory, "state.json")); err == nil {
		store.close()
		t.Fatal("writable parent accepted")
	}
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(directory, "unrelated")
	if err := os.WriteFile(victim, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(directory, "state.json")
	if err := os.Symlink(victim, state+".lock"); err != nil {
		t.Fatal(err)
	}
	if store, err := openNotifyStore(state); err == nil {
		store.close()
		t.Fatal("linked lock accepted")
	}
	data, _ := os.ReadFile(victim)
	if string(data) != "preserve" {
		t.Fatal("lock opening changed unrelated file")
	}
}
