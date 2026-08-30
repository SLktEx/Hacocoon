package ebs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type FileJournal struct{ root string }

func NewFileJournal(root string) *FileJournal { return &FileJournal{root: root} }

var journalID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func (j *FileJournal) Save(ctx context.Context, op Operation) error {
	if j == nil || !journalID.MatchString(op.ID) || !validPhase(op.Phase) || op.Version < 1 || op.Version > operationVersion {
		return fmt.Errorf("invalid EBS journal operation: %w", core.ErrInvalidArgument)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := j.ensureRoot(); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(op, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(j.root, op.ID+".json")
	tmp, err := os.CreateTemp(j.root, "."+op.ID+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keepTmp := true
	defer func() {
		_ = tmp.Close()
		if keepTmp {
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
	keepTmp = false
	if err := syncDirectory(j.root); err != nil {
		return fmt.Errorf("sync EBS journal directory: %w", err)
	}
	return nil
}

func (j *FileJournal) Load(ctx context.Context, id string) (Operation, bool, error) {
	if j == nil || !journalID.MatchString(id) {
		return Operation{}, false, fmt.Errorf("invalid EBS journal id: %w", core.ErrInvalidArgument)
	}
	if err := ctx.Err(); err != nil {
		return Operation{}, false, err
	}
	path := filepath.Join(j.root, id+".json")
	payload, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Operation{}, false, nil
	}
	if err != nil {
		return Operation{}, false, err
	}
	var op Operation
	if err := json.Unmarshal(payload, &op); err != nil {
		return Operation{}, false, fmt.Errorf("decode EBS journal %s: %w", id, err)
	}
	if op.ID != id || op.Version < 1 || op.Version > operationVersion || !validPhase(op.Phase) || (op.RecoveryFrom != "" && !validPhase(op.RecoveryFrom)) {
		return Operation{}, false, fmt.Errorf("invalid EBS journal state %s: %w", id, core.ErrIncompatibleState)
	}
	return op, true, nil
}

func (j *FileJournal) List(ctx context.Context) ([]Operation, error) {
	if j == nil || strings.TrimSpace(j.root) == "" || strings.TrimSpace(j.root) != j.root {
		return nil, core.ErrInvalidArgument
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(j.root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !journalID.MatchString(id) {
			return nil, fmt.Errorf("unexpected EBS journal filename %q: %w", entry.Name(), core.ErrIncompatibleState)
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	operations := make([]Operation, 0, len(ids))
	for _, id := range ids {
		op, found, err := j.Load(ctx, id)
		if err != nil {
			return nil, err
		}
		if found {
			operations = append(operations, op)
		}
	}
	return operations, nil
}

func (j *FileJournal) Delete(ctx context.Context, id string) error {
	if j == nil || !journalID.MatchString(id) {
		return fmt.Errorf("invalid EBS journal id: %w", core.ErrInvalidArgument)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err := os.Remove(filepath.Join(j.root, id+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := syncDirectory(j.root); err != nil {
		return fmt.Errorf("sync EBS journal deletion: %w", err)
	}
	return nil
}

func (j *FileJournal) Lock(ctx context.Context, keys ...string) (func() error, error) {
	if j == nil || len(keys) == 0 {
		return nil, core.ErrInvalidArgument
	}
	if err := j.ensureRoot(); err != nil {
		return nil, err
	}
	lockRoot := filepath.Join(j.root, ".locks")
	if err := os.MkdirAll(lockRoot, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(lockRoot, 0o700); err != nil {
		return nil, err
	}

	unique := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsAny(key, "\r\n\x00") {
			return nil, core.ErrInvalidArgument
		}
		unique[key] = struct{}{}
	}
	ordered := make([]string, 0, len(unique))
	for key := range unique {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)

	type heldLock struct{ file *os.File }
	held := make([]heldLock, 0, len(ordered))
	releaseHeld := func() error {
		var errs []error
		for i := len(held) - 1; i >= 0; i-- {
			if err := unlockExclusiveFile(held[i].file); err != nil {
				errs = append(errs, err)
			}
			if err := held[i].file.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		held = nil
		return errors.Join(errs...)
	}

	for _, key := range ordered {
		if err := ctx.Err(); err != nil {
			_ = releaseHeld()
			return nil, err
		}
		digest := sha256.Sum256([]byte(key))
		path := filepath.Join(lockRoot, hex.EncodeToString(digest[:])+".lock")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			_ = releaseHeld()
			return nil, err
		}
		if err := file.Chmod(0o600); err != nil {
			_ = file.Close()
			_ = releaseHeld()
			return nil, err
		}
		locked, err := tryExclusiveFileLock(file)
		if err != nil {
			_ = file.Close()
			_ = releaseHeld()
			return nil, err
		}
		if !locked {
			_ = file.Close()
			_ = releaseHeld()
			return nil, fmt.Errorf("EBS replacement lock %q: %w", key, core.ErrStorageBusy)
		}
		held = append(held, heldLock{file: file})
	}

	var once sync.Once
	var releaseErr error
	release := func() error {
		once.Do(func() { releaseErr = releaseHeld() })
		return releaseErr
	}
	return release, nil
}

func (j *FileJournal) ensureRoot() error {
	if j == nil || strings.TrimSpace(j.root) == "" || strings.TrimSpace(j.root) != j.root {
		return core.ErrInvalidArgument
	}
	if err := os.MkdirAll(j.root, 0o700); err != nil {
		return err
	}
	return os.Chmod(j.root, 0o700)
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
