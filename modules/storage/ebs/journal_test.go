package ebs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileJournalSaveDoesNotFollowPredictableTempSymlink(t *testing.T) {
	root := t.TempDir()
	victim := filepath.Join(root, "victim")
	if err := os.WriteFile(victim, []byte("safe\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	predictable := filepath.Join(root, "op.json.tmp")
	if err := os.Symlink(victim, predictable); err != nil {
		t.Fatal(err)
	}

	journal := NewFileJournal(root)
	if err := journal.Save(context.Background(), Operation{Version: 1, ID: "op", Phase: PhasePlanning}); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "safe\n" {
		t.Fatalf("predictable temp symlink target was modified: %q", got)
	}
	if info, err := os.Lstat(predictable); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("predictable attacker symlink should be untouched: info=%v err=%v", info, err)
	}
	if info, err := os.Stat(filepath.Join(root, "op.json")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("journal file mode=%v err=%v", info, err)
	}
}
