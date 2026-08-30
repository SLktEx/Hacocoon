package ec2

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func continueWorkspaceRestore(journalPath string, journal workspaceRestoreJournal, hook workspaceRestoreHook) error {
	if err := validateWorkspaceRestoreJournal(journal, journal.Workspace); err != nil {
		return err
	}
	workspaceOriginal, workspaceReplacement, workspaceExists, err := inspectWorkspaceIdentity(journal.Workspace, journal.OriginalID, journal.ReplacementID)
	if err != nil {
		return err
	}
	backupOriginal, _, backupExists, err := inspectWorkspaceIdentity(journal.Backup, journal.OriginalID, workspaceFileIdentity{})
	if err != nil {
		return err
	}
	_, replacementIdentity, replacementExists, err := inspectWorkspaceIdentity(journal.Replacement, workspaceFileIdentity{}, journal.ReplacementID)
	if err != nil {
		return err
	}
	workspaceProof, err := workspaceReplacementProofMatches(journal.Workspace, journal)
	if err != nil && !os.IsNotExist(err) {
		return workspaceRestoreAmbiguous(fmt.Sprintf("inspect installed replacement proof: %v", err))
	}
	replacementProof, err := workspaceReplacementProofMatches(journal.Replacement, journal)
	if err != nil && !os.IsNotExist(err) {
		return workspaceRestoreAmbiguous(fmt.Sprintf("inspect staged replacement proof: %v", err))
	}
	txnExists, txnMatches, err := inspectTransactionDirectory(journal.TransactionDir, journal.TransactionID)
	if err != nil {
		return err
	}

	switch {
	case workspaceExists && workspaceOriginal && !backupExists && replacementExists && replacementIdentity && replacementProof && txnExists && txnMatches:
		return moveOriginalAndInstall(journalPath, journal, hook)
	case !workspaceExists && backupExists && backupOriginal && replacementExists && replacementIdentity && replacementProof && txnExists && txnMatches:
		return installReplacement(journalPath, journal, hook)
	case workspaceExists && workspaceReplacement && workspaceProof && backupExists && backupOriginal && !replacementExists && txnExists && txnMatches:
		return finalizeWorkspaceRestore(journalPath, journal, hook)
	case workspaceExists && workspaceReplacement && !backupExists && !replacementExists:
		if workspaceProof {
			if err := removeWorkspaceReplacementProof(journal.Workspace, journal); err != nil {
				return workspaceRestoreAmbiguous(fmt.Sprintf("remove replacement proof after backup cleanup: %v", err))
			}
		}
		return finishWorkspaceRestoreMetadata(journalPath, journal)
	case workspaceExists && workspaceOriginal && !backupExists && !replacementExists:
		if txnExists && !txnMatches {
			return workspaceRestoreAmbiguous("transaction directory identity changed")
		}
		if txnExists {
			if err := removeOwnedTransactionDir(journal.TransactionDir, journal.TransactionID); err != nil {
				return workspaceRestoreAmbiguous(fmt.Sprintf("remove incomplete restore transaction: %v", err))
			}
		}
		if err := removeWorkspaceRestoreJournal(journalPath, journal); err != nil {
			return workspaceRestoreAmbiguous(fmt.Sprintf("remove incomplete restore journal: %v", err))
		}
		return errors.Join(errors.New("workspace replacement staging disappeared before any original data was moved"), core.ErrRecoveryRequired)
	case !workspaceExists && backupExists && backupOriginal && !replacementExists && txnExists && txnMatches:
		if err := renameWorkspaceNoReplace(journal.Backup, journal.Workspace); err != nil {
			return workspaceRestoreAmbiguous(fmt.Sprintf("rollback original workspace: %v", err))
		}
		if err := syncRenameParents(journal.Backup, journal.Workspace); err != nil {
			return workspaceRestoreAmbiguous(fmt.Sprintf("sync rolled-back workspace: %v", err))
		}
		if err := verifyWorkspaceIdentity(journal.Workspace, journal.OriginalID); err != nil {
			return workspaceRestoreAmbiguous(fmt.Sprintf("verify rolled-back workspace: %v", err))
		}
		if err := removeOwnedTransactionDir(journal.TransactionDir, journal.TransactionID); err != nil {
			return workspaceRestoreAmbiguous(fmt.Sprintf("remove rolled-back restore transaction: %v", err))
		}
		if err := removeWorkspaceRestoreJournal(journalPath, journal); err != nil {
			return workspaceRestoreAmbiguous(fmt.Sprintf("remove rolled-back restore journal: %v", err))
		}
		return errors.Join(errors.New("workspace replacement staging disappeared; original workspace was restored"), core.ErrRecoveryRequired)
	default:
		return workspaceRestoreAmbiguous(fmt.Sprintf("workspace-exists=%t original=%t replacement-id=%t replacement-proof=%t backup-exists=%t backup-original=%t replacement-exists=%t replacement-id=%t staged-proof=%t transaction=%t/%t", workspaceExists, workspaceOriginal, workspaceReplacement, workspaceProof, backupExists, backupOriginal, replacementExists, replacementIdentity, replacementProof, txnExists, txnMatches))
	}
}

func moveOriginalAndInstall(journalPath string, journal workspaceRestoreJournal, hook workspaceRestoreHook) error {
	if err := verifyWorkspaceIdentity(journal.Workspace, journal.OriginalID); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("original workspace identity changed before swap: %v", err))
	}
	if err := verifyWorkspaceIdentity(journal.Replacement, journal.ReplacementID); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("replacement identity changed before swap: %v", err))
	}
	if ok, err := workspaceReplacementProofMatches(journal.Replacement, journal); err != nil || !ok {
		return workspaceRestoreAmbiguous(fmt.Sprintf("replacement proof changed before swap: ok=%t err=%v", ok, err))
	}
	if err := renameWorkspaceNoReplace(journal.Workspace, journal.Backup); err != nil {
		return fmt.Errorf("move original workspace into owned backup: %w", err)
	}
	if err := syncRenameParents(journal.Workspace, journal.Backup); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("sync original workspace backup rename: %v", err))
	}
	if err := runWorkspaceRestoreHook(hook, workspaceRestoreAfterOriginalRename); err != nil {
		return errors.Join(err, core.ErrRecoveryRequired)
	}
	journal.Phase = workspaceRestoreOriginalMoved
	if err := updateWorkspaceRestoreJournal(journalPath, journal); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("persist original-moved phase: %v", err))
	}
	if err := runWorkspaceRestoreHook(hook, workspaceRestoreAfterOriginalMovedJournal); err != nil {
		return errors.Join(err, core.ErrRecoveryRequired)
	}
	return installReplacement(journalPath, journal, hook)
}

func installReplacement(journalPath string, journal workspaceRestoreJournal, hook workspaceRestoreHook) error {
	if _, err := os.Lstat(journal.Workspace); err == nil {
		return workspaceRestoreAmbiguous("workspace path was recreated before replacement install")
	} else if !os.IsNotExist(err) {
		return workspaceRestoreAmbiguous(fmt.Sprintf("inspect workspace before replacement install: %v", err))
	}
	if err := verifyWorkspaceIdentity(journal.Backup, journal.OriginalID); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("backup identity changed before replacement install: %v", err))
	}
	if err := verifyWorkspaceIdentity(journal.Replacement, journal.ReplacementID); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("replacement identity changed before install: %v", err))
	}
	if ok, err := workspaceReplacementProofMatches(journal.Replacement, journal); err != nil || !ok {
		return workspaceRestoreAmbiguous(fmt.Sprintf("replacement proof changed before install: ok=%t err=%v", ok, err))
	}
	if err := renameWorkspaceNoReplace(journal.Replacement, journal.Workspace); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("install replacement workspace: %v", err))
	}
	if err := syncRenameParents(journal.Replacement, journal.Workspace); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("sync replacement workspace rename: %v", err))
	}
	if err := runWorkspaceRestoreHook(hook, workspaceRestoreAfterReplacementRename); err != nil {
		return errors.Join(err, core.ErrRecoveryRequired)
	}
	journal.Phase = workspaceRestoreReplacementInstalled
	if err := updateWorkspaceRestoreJournal(journalPath, journal); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("persist replacement-installed phase: %v", err))
	}
	if err := runWorkspaceRestoreHook(hook, workspaceRestoreAfterReplacementInstalled); err != nil {
		return errors.Join(err, core.ErrRecoveryRequired)
	}
	return finalizeWorkspaceRestore(journalPath, journal, hook)
}

func finalizeWorkspaceRestore(journalPath string, journal workspaceRestoreJournal, hook workspaceRestoreHook) error {
	if err := verifyWorkspaceIdentity(journal.Workspace, journal.ReplacementID); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("replacement workspace identity cannot be proven: %v", err))
	}
	if ok, err := workspaceReplacementProofMatches(journal.Workspace, journal); err != nil || !ok {
		return workspaceRestoreAmbiguous(fmt.Sprintf("replacement workspace proof cannot be proven: ok=%t err=%v", ok, err))
	}
	if err := verifyWorkspaceIdentity(journal.Backup, journal.OriginalID); err != nil {
		if !os.IsNotExist(err) {
			return workspaceRestoreAmbiguous(fmt.Sprintf("original backup identity cannot be proven: %v", err))
		}
	} else {
		if err := os.RemoveAll(journal.Backup); err != nil {
			return workspaceRestoreAmbiguous(fmt.Sprintf("remove proven original backup: %v", err))
		}
		if err := syncDirectory(filepath.Dir(journal.Backup)); err != nil {
			return workspaceRestoreAmbiguous(fmt.Sprintf("sync backup removal: %v", err))
		}
		if err := runWorkspaceRestoreHook(hook, workspaceRestoreAfterBackupRemoval); err != nil {
			return errors.Join(err, core.ErrRecoveryRequired)
		}
	}
	if err := removeWorkspaceReplacementProof(journal.Workspace, journal); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("remove completed replacement proof: %v", err))
	}
	return finishWorkspaceRestoreMetadata(journalPath, journal)
}

func finishWorkspaceRestoreMetadata(journalPath string, journal workspaceRestoreJournal) error {
	if err := verifyWorkspaceIdentity(journal.Workspace, journal.ReplacementID); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("cannot finalize without replacement identity: %v", err))
	}
	if _, err := os.Lstat(journal.Backup); err == nil {
		return workspaceRestoreAmbiguous("refusing to discard restore metadata while original backup still exists")
	} else if !os.IsNotExist(err) {
		return workspaceRestoreAmbiguous(fmt.Sprintf("inspect backup before metadata cleanup: %v", err))
	}
	if _, err := os.Lstat(journal.TransactionDir); err == nil {
		if err := removeOwnedTransactionDir(journal.TransactionDir, journal.TransactionID); err != nil {
			return workspaceRestoreAmbiguous(fmt.Sprintf("remove completed restore transaction: %v", err))
		}
	} else if !os.IsNotExist(err) {
		return workspaceRestoreAmbiguous(fmt.Sprintf("inspect restore transaction before cleanup: %v", err))
	}
	if err := removeWorkspaceRestoreJournal(journalPath, journal); err != nil {
		return workspaceRestoreAmbiguous(fmt.Sprintf("remove completed restore journal: %v", err))
	}
	return nil
}

func inspectWorkspaceIdentity(path string, first, second workspaceFileIdentity) (firstMatch, secondMatch, exists bool, err error) {
	identity, err := identifyWorkspaceDirectory(path)
	if os.IsNotExist(err) {
		return false, false, false, nil
	}
	if err != nil {
		return false, false, true, fmt.Errorf("inspect workspace restore path %s: %w", path, err)
	}
	return first != (workspaceFileIdentity{}) && identity == first, second != (workspaceFileIdentity{}) && identity == second, true, nil
}

func inspectTransactionDirectory(path string, expected workspaceFileIdentity) (bool, bool, error) {
	identity, err := identifyOwnedDirectory(path, 0o700)
	if os.IsNotExist(err) {
		return false, false, nil
	}
	if err != nil {
		return true, false, err
	}
	return true, identity == expected, nil
}
