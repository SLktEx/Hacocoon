//go:build linux

package capability

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (c *PolicyConfiguration) Snapshot(ctx context.Context) (PolicySnapshot, error) {
	if c == nil || c.Evaluator == nil || !filepath.IsAbs(c.Evaluator.path) {
		return PolicySnapshot{}, core.ErrInvalidArgument
	}
	if err := ctx.Err(); err != nil {
		return PolicySnapshot{}, err
	}
	parent := filepath.Dir(c.Evaluator.path)
	info, err := os.Lstat(parent)
	if err != nil || !safePolicyFile(info, true) {
		return PolicySnapshot{}, core.ErrIncompatibleState
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return PolicySnapshot{}, err
	}
	defer root.Close()
	actual, err := root.Stat(".")
	if err != nil || !os.SameFile(info, actual) {
		return PolicySnapshot{}, core.ErrIncompatibleState
	}
	data, err := readSavedPolicy(root, filepath.Base(c.Evaluator.path))
	if err != nil {
		return PolicySnapshot{}, err
	}
	view := data
	if data == nil {
		view = []byte(`{"default":"deny","rules":[]}`)
	}
	policy, err := decodePolicy(view)
	if err != nil {
		return PolicySnapshot{}, core.ErrInvalidArgument
	}
	// Present the same canonical structure that replacement persists, while the
	// revision still binds to exact on-disk bytes (including omitted fields).
	view, err = json.Marshal(policy)
	if err != nil {
		return PolicySnapshot{}, err
	}
	return PolicySnapshot{Revision: policyRevision(data), Policy: view}, nil
}

// Replace uses the same lock/atomic writer as saved approval decisions. It
// deliberately does not merge an operator edit over a concurrent saved choice.
func (c *PolicyConfiguration) Replace(ctx context.Context, edit PolicySnapshot) (PolicySnapshot, error) {
	if c == nil || c.Evaluator == nil || c.Audit == nil || !filepath.IsAbs(c.Evaluator.path) || len(edit.Policy) > maxSavedPolicyBytes || len(edit.Revision) != 71 {
		return PolicySnapshot{}, core.ErrInvalidArgument
	}
	policy, err := decodePolicy(edit.Policy)
	if err != nil || len(bytes.TrimSpace(edit.Policy)) == 0 || bytes.TrimSpace(edit.Policy)[0] != '{' {
		return PolicySnapshot{}, core.ErrInvalidArgument
	}
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return PolicySnapshot{}, err
	}
	data = append(data, '\n')
	if len(data) > maxSavedPolicyBytes {
		return PolicySnapshot{}, core.ErrInvalidArgument
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return PolicySnapshot{}, err
	}
	requestID := hex.EncodeToString(nonce[:])
	next := PolicySnapshot{Revision: policyRevision(data), Policy: data}
	record := func(kind string) error {
		return c.Audit.Record(ctx, core.CapabilityAuditEvent{
			Time: time.Now().UTC(), RequestID: requestID, Type: kind,
			Capability: "policy.configuration", Action: "replace",
			Attributes: map[string]string{"previous_revision": edit.Revision, "revision": next.Revision},
		})
	}
	err = c.Evaluator.updatePolicy(ctx, func(before []byte) (PolicyFile, error) {
		if policyRevision(before) != edit.Revision {
			return PolicyFile{}, core.ErrIncompatibleState
		}
		if err := record("configuration-change-requested"); err != nil {
			return PolicyFile{}, err
		}
		return policy, nil
	})
	if err != nil {
		return PolicySnapshot{}, err
	}
	if err := record("configuration-changed"); err != nil {
		return PolicySnapshot{}, fmt.Errorf("configuration persisted but audit incomplete: %w", core.ErrRecoveryRequired)
	}
	return next, nil
}
