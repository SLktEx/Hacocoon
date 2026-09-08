package incus

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// RecoverCompletedCopy restores only a copy whose positive provider completion
// was durably recorded by the canonical resource service. Existence is not proof.
func (b *PersistentResourceBackend) RecoverCompletedCopy(ctx context.Context, source, target core.PersistentResource) error {
	if target.State != "creating" || !target.CopyCompleted || target.CopySource != source.Ref() {
		return core.ErrRecoveryRequired
	}
	unlock, err := lockHostOperation(ctx, b.Runtime.project)
	if err != nil {
		return err
	}
	defer unlock()
	if err := b.Verify(ctx, target); err != nil {
		return err
	}
	observed, err := b.observe(ctx, source)
	if err != nil || observed == nil {
		return core.ErrRecoveryRequired
	}
	if len(observed.UsedBy) == 0 {
		return b.Verify(ctx, source)
	}
	if !b.hostCopyConsumer(source, observed) {
		return core.ErrRecoveryRequired
	}
	inspect := func() (hostOCICopyInstance, error) {
		current, e := b.hostCopyInstance(ctx, source)
		if e != nil || current.LocalConfig[trustedHostRoleKey] != trustedHostRoleValue || current.Config[hostOCIStoreKey] != source.Owner {
			return current, core.ErrRecoveryRequired
		}
		volume, e := b.observe(ctx, source)
		if e != nil || !b.hostCopyConsumer(source, volume) {
			return current, core.ErrRecoveryRequired
		}
		return current, nil
	}
	before, err := inspect()
	if err != nil {
		return err
	}
	marker := before.Config[hostOCICopyKey]
	if marker == "" {
		if before.StatusCode != 103 && before.StatusCode != 102 {
			return core.ErrRecoveryRequired
		}
		return nil // Already restored; only catalog commit remained.
	}
	if len(marker) > 512 || before.LocalConfig[hostOCICopyKey] != marker {
		return core.ErrRecoveryRequired
	}
	var journal hostOCICopyJournal
	decoder := json.NewDecoder(strings.NewReader(marker))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&journal) != nil || decoder.Decode(new(any)) != io.EOF || journal.Version != 1 || journal.Owner != target.Owner {
		return core.ErrRecoveryRequired
	}
	valid := func(value string) bool { return value == "" || value == "true" || value == "false" }
	if !valid(journal.Autostart) || !valid(journal.ExpandedAutostart) {
		return core.ErrRecoveryRequired
	}
	guard := func(i hostOCICopyInstance) bool {
		return i.Config[hostOCICopyKey] == marker && i.LocalConfig[hostOCICopyKey] == marker && i.Config["boot.autostart"] == "false" && i.LocalConfig["boot.autostart"] == "false"
	}
	if !guard(before) {
		return core.ErrRecoveryRequired
	}
	if before.StatusCode == 110 || before.StatusCode == 102 {
		result, err := b.Runtime.runner.Run(ctx, "incus", "start", trustedHostName, "--project", b.Runtime.project)
		if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
			return core.ErrRecoveryRequired
		}
	} else if before.StatusCode != 103 {
		return core.ErrRecoveryRequired
	}
	running, err := inspect()
	if err != nil || running.StatusCode != 103 || !guard(running) {
		return core.ErrRecoveryRequired
	}
	var restored any = journal.Autostart
	if journal.Autostart == "" {
		restored = nil
	}
	if err := b.patchHostCopy(ctx, map[string]any{hostOCICopyKey: nil, "boot.autostart": restored}); err != nil {
		return err
	}
	after, err := inspect()
	if err != nil || after.StatusCode != 103 || after.Config[hostOCICopyKey] != "" || after.LocalConfig[hostOCICopyKey] != "" || after.LocalConfig["boot.autostart"] != journal.Autostart || after.Config["boot.autostart"] != journal.ExpandedAutostart {
		return core.ErrRecoveryRequired
	}
	return nil
}

func (r *Runtime) ConfigureHostCopyRecovery(recover func(context.Context) error) {
	r.trustedHostCopyRecovery = recover
}
