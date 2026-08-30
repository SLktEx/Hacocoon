package ec2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

var errInjectedWorkspaceRestoreCrash = errors.New("injected workspace restore crash")

type restoreArchiveRunner struct {
	extractions int
	content     string
}

func (r *restoreArchiveRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	if name != "tar" || len(args) != 4 || args[0] != "-xzf" || args[2] != "-C" {
		return host.Result{}, errors.New("unexpected restore command")
	}
	r.extractions++
	if err := os.WriteFile(filepath.Join(args[3], "remote.txt"), []byte(r.content), 0o600); err != nil {
		return host.Result{}, err
	}
	return host.Result{}, nil
}

func TestWorkspaceRestoreRecoversEveryCrashBoundary(t *testing.T) {
	points := []workspaceRestoreCrashPoint{
		workspaceRestoreAfterJournalPrepared,
		workspaceRestoreAfterOriginalRename,
		workspaceRestoreAfterOriginalMovedJournal,
		workspaceRestoreAfterReplacementRename,
		workspaceRestoreAfterReplacementInstalled,
		workspaceRestoreAfterBackupRemoval,
	}
	for _, point := range points {
		t.Run(string(point), func(t *testing.T) {
			workspace, archive := newRestoreFixture(t)
			runner := &restoreArchiveRunner{content: "remote\n"}
			hook := func(got workspaceRestoreCrashPoint) error {
				if got == point {
					return errInjectedWorkspaceRestoreCrash
				}
				return nil
			}

			err := restoreWorkspaceArchiveWithHook(context.Background(), runner, archive, workspace, hook)
			if !errors.Is(err, errInjectedWorkspaceRestoreCrash) || !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatalf("first restore err=%v", err)
			}

			journalPath, _ := workspaceRestoreControlPaths(workspace)
			journal, found, err := loadWorkspaceRestoreJournal(journalPath, workspace)
			if err != nil || !found {
				t.Fatalf("durable journal missing after injected crash: found=%t err=%v", found, err)
			}
			if point == workspaceRestoreAfterOriginalRename || point == workspaceRestoreAfterOriginalMovedJournal || point == workspaceRestoreAfterReplacementRename || point == workspaceRestoreAfterReplacementInstalled {
				if data, err := os.ReadFile(filepath.Join(journal.Backup, "old.txt")); err != nil || string(data) != "old\n" {
					t.Fatalf("original backup not preserved after %s: data=%q err=%v", point, data, err)
				}
			}

			if err := restoreWorkspaceArchive(context.Background(), runner, archive, workspace); err != nil {
				t.Fatalf("retry failed after %s: %v", point, err)
			}
			if runner.extractions != 1 {
				t.Fatalf("recovery re-extracted archive: extractions=%d", runner.extractions)
			}
			if data, err := os.ReadFile(filepath.Join(workspace, "remote.txt")); err != nil || string(data) != "remote\n" {
				t.Fatalf("replacement not installed after retry: data=%q err=%v", data, err)
			}
			if _, err := os.Stat(filepath.Join(workspace, "old.txt")); !os.IsNotExist(err) {
				t.Fatalf("old file survived completed replacement: %v", err)
			}
			if _, err := os.Lstat(journalPath); !os.IsNotExist(err) {
				t.Fatalf("journal survived completed recovery: %v", err)
			}
			if _, err := os.Lstat(journal.TransactionDir); !os.IsNotExist(err) {
				t.Fatalf("transaction directory survived completed recovery: %v", err)
			}
		})
	}
}

func TestWorkspaceRestoreNeverDeletesBackupWhenReplacementIdentityIsUnproven(t *testing.T) {
	workspace, archive := newRestoreFixture(t)
	runner := &restoreArchiveRunner{content: "remote\n"}
	err := restoreWorkspaceArchiveWithHook(context.Background(), runner, archive, workspace, func(point workspaceRestoreCrashPoint) error {
		if point == workspaceRestoreAfterReplacementInstalled {
			return errInjectedWorkspaceRestoreCrash
		}
		return nil
	})
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("injected crash err=%v", err)
	}
	journalPath, _ := workspaceRestoreControlPaths(workspace)
	journal, found, err := loadWorkspaceRestoreJournal(journalPath, workspace)
	if err != nil || !found {
		t.Fatalf("journal found=%t err=%v", found, err)
	}
	if err := os.RemoveAll(workspace); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "intruder.txt"), []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err = restoreWorkspaceArchive(context.Background(), runner, archive, workspace)
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("unproven replacement err=%v", err)
	}
	if data, err := os.ReadFile(filepath.Join(journal.Backup, "old.txt")); err != nil || string(data) != "old\n" {
		t.Fatalf("proven original backup was deleted: data=%q err=%v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "intruder.txt")); err != nil || string(data) != "keep\n" {
		t.Fatalf("unproven workspace was overwritten: data=%q err=%v", data, err)
	}
}

func TestWorkspaceRestoreRollsBackOriginalWhenStagedReplacementDisappears(t *testing.T) {
	workspace, archive := newRestoreFixture(t)
	runner := &restoreArchiveRunner{content: "remote\n"}
	err := restoreWorkspaceArchiveWithHook(context.Background(), runner, archive, workspace, func(point workspaceRestoreCrashPoint) error {
		if point == workspaceRestoreAfterOriginalRename {
			return errInjectedWorkspaceRestoreCrash
		}
		return nil
	})
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("injected crash err=%v", err)
	}
	journalPath, _ := workspaceRestoreControlPaths(workspace)
	journal, found, err := loadWorkspaceRestoreJournal(journalPath, workspace)
	if err != nil || !found {
		t.Fatalf("journal found=%t err=%v", found, err)
	}
	if err := os.RemoveAll(journal.Replacement); err != nil {
		t.Fatal(err)
	}

	err = restoreWorkspaceArchive(context.Background(), runner, archive, workspace)
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("missing replacement retry err=%v", err)
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "old.txt")); err != nil || string(data) != "old\n" {
		t.Fatalf("original workspace was not rolled back: data=%q err=%v", data, err)
	}
	if _, err := os.Lstat(journalPath); !os.IsNotExist(err) {
		t.Fatalf("rolled-back journal survived: %v", err)
	}

	if err := restoreWorkspaceArchive(context.Background(), runner, archive, workspace); err != nil {
		t.Fatalf("fresh retry after rollback failed: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "remote.txt")); err != nil || string(data) != "remote\n" {
		t.Fatalf("fresh retry did not install remote workspace: data=%q err=%v", data, err)
	}
}

func TestWorkspaceRestoreRejectsSymlinkJournalWithoutTouchingVictim(t *testing.T) {
	workspace, archive := newRestoreFixture(t)
	journalPath, _ := workspaceRestoreControlPaths(workspace)
	victim := filepath.Join(filepath.Dir(workspace), "victim.txt")
	if err := os.WriteFile(victim, []byte("sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, journalPath); err != nil {
		t.Fatal(err)
	}

	runner := &restoreArchiveRunner{content: "remote\n"}
	err := restoreWorkspaceArchive(context.Background(), runner, archive, workspace)
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("symlink journal err=%v", err)
	}
	if runner.extractions != 0 {
		t.Fatalf("archive extraction began despite unsafe journal: %d", runner.extractions)
	}
	if data, err := os.ReadFile(victim); err != nil || string(data) != "sentinel\n" {
		t.Fatalf("journal symlink victim changed: data=%q err=%v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "old.txt")); err != nil || string(data) != "old\n" {
		t.Fatalf("workspace changed despite unsafe journal: data=%q err=%v", data, err)
	}
}

func TestWorkspaceRestoreLockRejectsConcurrentRestore(t *testing.T) {
	workspace, archive := newRestoreFixture(t)
	_, lockPath := workspaceRestoreControlPaths(workspace)
	lock, err := acquireWorkspaceRestoreLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()

	runner := &restoreArchiveRunner{content: "remote\n"}
	err = restoreWorkspaceArchive(context.Background(), runner, archive, workspace)
	if !errors.Is(err, core.ErrWorkspaceBusy) {
		t.Fatalf("concurrent restore err=%v", err)
	}
	if runner.extractions != 0 {
		t.Fatalf("concurrent restore extracted archive: %d", runner.extractions)
	}
}

func TestWorkspaceRestorePreservesUnrelatedLegacyBackupPath(t *testing.T) {
	workspace, archive := newRestoreFixture(t)
	legacy := workspace + ".haco-backup"
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "keep.txt"), []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := restoreWorkspaceArchive(context.Background(), &restoreArchiveRunner{content: "remote\n"}, archive, workspace); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(legacy, "keep.txt")); err != nil || string(data) != "keep\n" {
		t.Fatalf("unrelated legacy backup changed: data=%q err=%v", data, err)
	}
}

func newRestoreFixture(t *testing.T) (workspace, archive string) {
	t.Helper()
	parent := t.TempDir()
	workspace = filepath.Join(parent, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "old.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive = filepath.Join(parent, "remote.tgz")
	if err := os.WriteFile(archive, []byte(strings.Repeat("archive", 4)), 0o600); err != nil {
		t.Fatal(err)
	}
	return workspace, archive
}

func TestWorkspaceRestoreRejectsTrailingJournalData(t *testing.T) {
	workspace, archive := newRestoreFixture(t)
	journalPath, _ := workspaceRestoreControlPaths(workspace)
	if err := os.WriteFile(journalPath, []byte(`{} {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &restoreArchiveRunner{content: "remote\n"}
	err := restoreWorkspaceArchive(context.Background(), runner, archive, workspace)
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("trailing journal err=%v", err)
	}
	if runner.extractions != 0 {
		t.Fatalf("unsafe journal reached extraction: %d", runner.extractions)
	}
}
