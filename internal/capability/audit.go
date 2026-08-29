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
	if err := os.MkdirAll(filepath.Dir(a.path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encodeErr := encoder.Encode(event)
	closeErr := file.Close()
	if encodeErr != nil {
		return fmt.Errorf("encode audit event: %w", encodeErr)
	}
	return closeErr
}
