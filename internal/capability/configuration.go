package capability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// PolicySnapshot is a trusted administrator view, never an interaction event.
// Revision binds an edit to the exact bytes read, including file absence.
type PolicySnapshot struct {
	Revision string          `json:"revision"`
	Policy   json.RawMessage `json:"policy"`
}

type PolicyConfiguration struct {
	Evaluator *FilePolicyEvaluator
	Audit     AuditSink
}

func policyRevision(data []byte) string {
	h := sha256.New()
	if data != nil {
		h.Write([]byte{1})
	}
	h.Write(data)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
