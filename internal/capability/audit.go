package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type JSONLAudit struct {
	path string
	mu   sync.Mutex
}

func NewJSONLAudit(path string) *JSONLAudit { return &JSONLAudit{path: path} }

func (a *JSONLAudit) Record(_ context.Context, event core.CapabilityAuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	dir := filepath.Dir(a.path)
	if err := ensurePrivateAuditDirectory(dir); err != nil {
		return err
	}
	file, err := os.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure audit file: %w", err)
	}
	encoder := json.NewEncoder(file)
	encodeErr := encoder.Encode(event)
	syncErr := file.Sync()
	closeErr := file.Close()
	if encodeErr != nil {
		return fmt.Errorf("encode audit event: %w", encodeErr)
	}
	if syncErr != nil {
		return fmt.Errorf("sync audit event: %w", syncErr)
	}
	return closeErr
}

func ensurePrivateAuditDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if dir == "." {
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if info.Mode().Perm()&0o077 != 0 {
			return fmt.Errorf("audit directory %s has unsafe permissions %o", dir, info.Mode().Perm())
		}
		return nil
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure audit directory %s: %w", filepath.Clean(dir), err)
	}
	return nil
}
