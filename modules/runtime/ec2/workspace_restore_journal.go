package ec2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func validateWorkspaceRestoreJournal(journal workspaceRestoreJournal, workspace string) error {
	if journal.Version != workspaceRestoreJournalVersion || journal.Workspace != workspace {
		return workspaceRestoreAmbiguous("journal version/workspace mismatch")
	}
	if journal.OriginalID == (workspaceFileIdentity{}) || journal.ReplacementID == (workspaceFileIdentity{}) || journal.TransactionID == (workspaceFileIdentity{}) {
		return workspaceRestoreAmbiguous("journal is missing filesystem identity")
	}
	if len(journal.ProofName) < len(".haco-restore-proof-")+16 || !strings.HasPrefix(journal.ProofName, ".haco-restore-proof-") || filepath.Base(journal.ProofName) != journal.ProofName {
		return workspaceRestoreAmbiguous("journal replacement proof name is invalid")
	}
	if len(journal.ProofToken) != 64 {
		return workspaceRestoreAmbiguous("journal replacement proof token is invalid")
	}
	if _, err := hex.DecodeString(journal.ProofToken); err != nil {
		return workspaceRestoreAmbiguous("journal replacement proof token is invalid")
	}
	if len(journal.ArchiveSHA256) != sha256.Size*2 {
		return workspaceRestoreAmbiguous("journal archive digest is invalid")
	}
	if _, err := hex.DecodeString(journal.ArchiveSHA256); err != nil {
		return workspaceRestoreAmbiguous("journal archive digest is invalid")
	}
	switch journal.Phase {
	case workspaceRestorePrepared, workspaceRestoreOriginalMoved, workspaceRestoreReplacementInstalled:
	default:
		return workspaceRestoreAmbiguous(fmt.Sprintf("journal phase %q is invalid", journal.Phase))
	}
	parent := filepath.Dir(workspace)
	if filepath.Dir(journal.TransactionDir) != parent || !strings.HasPrefix(filepath.Base(journal.TransactionDir), ".haco-restore-txn-") {
		return workspaceRestoreAmbiguous("journal transaction directory escapes workspace parent")
	}
	if journal.Backup != filepath.Join(journal.TransactionDir, "original") || journal.Replacement != filepath.Join(journal.TransactionDir, "replacement") {
		return workspaceRestoreAmbiguous("journal transaction paths are not canonical")
	}
	return nil
}

func loadWorkspaceRestoreJournal(path, workspace string) (workspaceRestoreJournal, bool, error) {
	file, err := openOwnedRegularFile(path)
	if os.IsNotExist(err) {
		return workspaceRestoreJournal{}, false, nil
	}
	if err != nil {
		return workspaceRestoreJournal{}, false, workspaceRestoreAmbiguous(fmt.Sprintf("open restore journal: %v", err))
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, workspaceRestoreJournalMaxSize+1))
	if err != nil {
		return workspaceRestoreJournal{}, false, err
	}
	if len(payload) > workspaceRestoreJournalMaxSize {
		return workspaceRestoreJournal{}, false, workspaceRestoreAmbiguous("restore journal is oversized")
	}
	var journal workspaceRestoreJournal
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&journal); err != nil {
		return workspaceRestoreJournal{}, false, workspaceRestoreAmbiguous(fmt.Sprintf("decode restore journal: %v", err))
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return workspaceRestoreJournal{}, false, workspaceRestoreAmbiguous("restore journal contains trailing data")
	}
	if err := validateWorkspaceRestoreJournal(journal, workspace); err != nil {
		return workspaceRestoreJournal{}, false, err
	}
	return journal, true, nil
}

func createWorkspaceRestoreJournal(path string, journal workspaceRestoreJournal) error {
	payload, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	file, err := createExclusiveOwnedFile(path)
	if err != nil {
		if os.IsExist(err) {
			return workspaceRestoreAmbiguous("restore journal path already exists")
		}
		return err
	}
	removeOnFailure := true
	defer func() {
		_ = file.Close()
		if removeOnFailure {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(payload); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	removeOnFailure = false
	return nil
}

func updateWorkspaceRestoreJournal(path string, journal workspaceRestoreJournal) error {
	current, found, err := loadWorkspaceRestoreJournal(path, journal.Workspace)
	if err != nil {
		return err
	}
	if !found || current.TransactionDir != journal.TransactionDir || current.TransactionID != journal.TransactionID || current.OriginalID != journal.OriginalID || current.ReplacementID != journal.ReplacementID {
		return workspaceRestoreAmbiguous("restore journal ownership changed during transaction")
	}
	payload, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".haco-restore-journal-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keep := true
	defer func() {
		_ = tmp.Close()
		if keep {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(payload); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	keep = false
	return syncDirectory(filepath.Dir(path))
}

func removeWorkspaceRestoreJournal(path string, expected workspaceRestoreJournal) error {
	current, found, err := loadWorkspaceRestoreJournal(path, expected.Workspace)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if current.TransactionDir != expected.TransactionDir || current.TransactionID != expected.TransactionID || current.OriginalID != expected.OriginalID || current.ReplacementID != expected.ReplacementID {
		return workspaceRestoreAmbiguous("refusing to remove a restore journal with changed ownership identity")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
