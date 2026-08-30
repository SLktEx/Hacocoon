package ec2

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func canonicalWorkspaceRestorePath(workspace string) (string, error) {
	if strings.TrimSpace(workspace) != workspace || !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		return "", fmt.Errorf("workspace restore path %q must be canonical and absolute: %w", workspace, core.ErrInvalidArgument)
	}
	return workspace, nil
}

func workspaceRestoreControlPaths(workspace string) (journal, lock string) {
	digest := sha256.Sum256([]byte(workspace))
	key := hex.EncodeToString(digest[:16])
	parent := filepath.Dir(workspace)
	return filepath.Join(parent, ".haco-restore-"+key+".json"), filepath.Join(parent, ".haco-restore-"+key+".lock")
}

func createWorkspaceReplacementProof(dir string) (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(raw)
	name := ".haco-restore-proof-" + token[:24]
	path := filepath.Join(dir, name)
	file, err := createExclusiveOwnedFile(path)
	if err != nil {
		return "", "", err
	}
	if _, err := file.Write([]byte(token + "\n")); err != nil {
		_ = file.Close()
		return "", "", err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return "", "", err
	}
	if err := file.Close(); err != nil {
		return "", "", err
	}
	if err := syncDirectory(dir); err != nil {
		return "", "", err
	}
	return name, token, nil
}

func workspaceReplacementProofMatches(dir string, journal workspaceRestoreJournal) (bool, error) {
	file, err := openOwnedRegularFile(filepath.Join(dir, journal.ProofName))
	if err != nil {
		return false, err
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, 256))
	if err != nil {
		return false, err
	}
	return string(payload) == journal.ProofToken+"\n", nil
}

func removeWorkspaceReplacementProof(dir string, journal workspaceRestoreJournal) error {
	path := filepath.Join(dir, journal.ProofName)
	ok, err := workspaceReplacementProofMatches(dir, journal)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("replacement proof content changed: %w", core.ErrIncompatibleState)
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return syncDirectory(dir)
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func syncWorkspaceTree(root string) error {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			syncErr := file.Sync()
			closeErr := file.Close()
			if syncErr != nil {
				return syncErr
			}
			return closeErr
		}
		if info.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		if err := syncDirectory(dir); err != nil {
			return err
		}
	}
	return nil
}

func syncRenameParents(oldPath, newPath string) error {
	oldParent := filepath.Dir(oldPath)
	newParent := filepath.Dir(newPath)
	if err := syncDirectory(oldParent); err != nil {
		return err
	}
	if newParent != oldParent {
		if err := syncDirectory(newParent); err != nil {
			return err
		}
	}
	return nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func verifyWorkspaceIdentity(path string, expected workspaceFileIdentity) error {
	got, err := identifyWorkspaceDirectory(path)
	if err != nil {
		return err
	}
	if got != expected {
		return fmt.Errorf("filesystem identity changed: got=%+v want=%+v: %w", got, expected, core.ErrIncompatibleState)
	}
	return nil
}

func removeOwnedTransactionDir(path string, expected workspaceFileIdentity) error {
	if expected != (workspaceFileIdentity{}) {
		got, err := identifyOwnedDirectory(path, 0o700)
		if err != nil {
			return err
		}
		if got != expected {
			return fmt.Errorf("transaction directory identity changed: %w", core.ErrIncompatibleState)
		}
	} else {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return core.ErrIncompatibleState
		}
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func runWorkspaceRestoreHook(hook workspaceRestoreHook, point workspaceRestoreCrashPoint) error {
	if hook == nil {
		return nil
	}
	return hook(point)
}

func workspaceRestoreAmbiguous(detail string) error {
	return errors.Join(fmt.Errorf("workspace restore state is ambiguous: %s", detail), core.ErrRecoveryRequired)
}
