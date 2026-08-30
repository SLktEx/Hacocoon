package ec2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const (
	workspaceRestoreJournalVersion = 1
	workspaceRestoreJournalMaxSize = 64 << 10
)

type workspaceRestorePhase string

const (
	workspaceRestorePrepared             workspaceRestorePhase = "prepared"
	workspaceRestoreOriginalMoved        workspaceRestorePhase = "original-moved"
	workspaceRestoreReplacementInstalled workspaceRestorePhase = "replacement-installed"
)

type workspaceFileIdentity struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

type workspaceRestoreJournal struct {
	Version        int                   `json:"version"`
	Workspace      string                `json:"workspace"`
	TransactionDir string                `json:"transaction_dir"`
	Backup         string                `json:"backup"`
	Replacement    string                `json:"replacement"`
	ArchiveSHA256  string                `json:"archive_sha256"`
	ProofName      string                `json:"proof_name"`
	ProofToken     string                `json:"proof_token"`
	OriginalID     workspaceFileIdentity `json:"original_id"`
	ReplacementID  workspaceFileIdentity `json:"replacement_id"`
	TransactionID  workspaceFileIdentity `json:"transaction_id"`
	Phase          workspaceRestorePhase `json:"phase"`
}

type workspaceRestoreCrashPoint string

const (
	workspaceRestoreAfterJournalPrepared      workspaceRestoreCrashPoint = "after-journal-prepared"
	workspaceRestoreAfterOriginalRename       workspaceRestoreCrashPoint = "after-original-rename"
	workspaceRestoreAfterOriginalMovedJournal workspaceRestoreCrashPoint = "after-original-moved-journal"
	workspaceRestoreAfterReplacementRename    workspaceRestoreCrashPoint = "after-replacement-rename"
	workspaceRestoreAfterReplacementInstalled workspaceRestoreCrashPoint = "after-replacement-installed-journal"
	workspaceRestoreAfterBackupRemoval        workspaceRestoreCrashPoint = "after-backup-removal"
)

type workspaceRestoreHook func(workspaceRestoreCrashPoint) error

func restoreWorkspaceArchive(ctx context.Context, runner host.Runner, archive, workspace string) error {
	return restoreWorkspaceArchiveWithHook(ctx, runner, archive, workspace, nil)
}

func restoreWorkspaceArchiveWithHook(ctx context.Context, runner host.Runner, archive, workspace string, hook workspaceRestoreHook) error {
	if runner == nil || strings.TrimSpace(archive) == "" || strings.TrimSpace(workspace) == "" {
		return core.ErrInvalidArgument
	}
	workspace, err := canonicalWorkspaceRestorePath(workspace)
	if err != nil {
		return err
	}
	journalPath, lockPath := workspaceRestoreControlPaths(workspace)
	lock, err := acquireWorkspaceRestoreLock(lockPath)
	if err != nil {
		return err
	}
	defer lock.Release()

	journal, found, err := loadWorkspaceRestoreJournal(journalPath, workspace)
	if err != nil {
		return err
	}
	if found {
		if err := reconcileWorkspaceRestore(journalPath, journal, hook); err != nil {
			return err
		}
		return nil
	}

	return beginWorkspaceRestore(ctx, runner, archive, workspace, journalPath, hook)
}

func beginWorkspaceRestore(ctx context.Context, runner host.Runner, archive, workspace, journalPath string, hook workspaceRestoreHook) error {
	originalID, err := identifyWorkspaceDirectory(workspace)
	if err != nil {
		return fmt.Errorf("identify original workspace: %w", err)
	}
	parent := filepath.Dir(workspace)
	txnDir, err := os.MkdirTemp(parent, ".haco-restore-txn-*")
	if err != nil {
		return fmt.Errorf("create workspace restore transaction: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = removeOwnedTransactionDir(txnDir, workspaceFileIdentity{})
		}
	}()
	if err := os.Chmod(txnDir, 0o700); err != nil {
		return fmt.Errorf("secure workspace restore transaction: %w", err)
	}
	txnID, err := identifyOwnedDirectory(txnDir, 0o700)
	if err != nil {
		return fmt.Errorf("identify workspace restore transaction: %w", err)
	}
	replacement := filepath.Join(txnDir, "replacement")
	if err := os.Mkdir(replacement, 0o700); err != nil {
		return fmt.Errorf("create workspace replacement directory: %w", err)
	}
	if _, err := runner.Run(ctx, "tar", "-xzf", archive, "-C", replacement); err != nil {
		return fmt.Errorf("extract remote workspace: %w", err)
	}
	proofName, proofToken, err := createWorkspaceReplacementProof(replacement)
	if err != nil {
		return fmt.Errorf("create workspace replacement proof: %w", err)
	}
	if err := syncWorkspaceTree(replacement); err != nil {
		return fmt.Errorf("sync extracted workspace: %w", err)
	}
	replacementID, err := identifyWorkspaceDirectory(replacement)
	if err != nil {
		return fmt.Errorf("identify workspace replacement: %w", err)
	}
	archiveDigest, err := sha256File(archive)
	if err != nil {
		return fmt.Errorf("hash remote workspace archive: %w", err)
	}

	journal := workspaceRestoreJournal{
		Version:        workspaceRestoreJournalVersion,
		Workspace:      workspace,
		TransactionDir: txnDir,
		Backup:         filepath.Join(txnDir, "original"),
		Replacement:    replacement,
		ArchiveSHA256:  archiveDigest,
		ProofName:      proofName,
		ProofToken:     proofToken,
		OriginalID:     originalID,
		ReplacementID:  replacementID,
		TransactionID:  txnID,
		Phase:          workspaceRestorePrepared,
	}
	if err := validateWorkspaceRestoreJournal(journal, workspace); err != nil {
		return err
	}
	if err := createWorkspaceRestoreJournal(journalPath, journal); err != nil {
		return fmt.Errorf("persist workspace restore journal: %w", err)
	}
	published = true
	if err := runWorkspaceRestoreHook(hook, workspaceRestoreAfterJournalPrepared); err != nil {
		return errors.Join(err, core.ErrRecoveryRequired)
	}
	return continueWorkspaceRestore(journalPath, journal, hook)
}

func reconcileWorkspaceRestore(journalPath string, journal workspaceRestoreJournal, hook workspaceRestoreHook) error {
	return continueWorkspaceRestore(journalPath, journal, hook)
}
