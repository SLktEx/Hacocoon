package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// EnableHostOCI enables nested workloads only on the verified, unprivileged
// managed Host after its canonical OCI source has become ready.
func (b *PersistentResourceBackend) EnableHostOCI(ctx context.Context, source core.PersistentResource) error {
	if source.State != "ready" || source.WorkspaceID != "" {
		return core.ErrRecoveryRequired
	}
	unlock, err := lockHostOperation(ctx, b.Runtime.project)
	if err != nil {
		return err
	}
	defer unlock()
	verify := func() (hostOCICopyInstance, error) {
		if err := b.VerifyHostSource(ctx, source); err != nil {
			return hostOCICopyInstance{}, err
		}
		i, err := b.hostCopyInstance(ctx, source)
		if err != nil {
			return i, err
		}
		if i.Type != "container" || i.Profiles == nil || len(i.Profiles) != 0 ||
			i.LocalConfig[trustedHostRoleKey] != trustedHostRoleValue ||
			i.LocalConfig[hostOCIStoreKey] != source.Owner ||
			i.StatusCode != 103 || i.Config[hostOCICopyKey] != "" ||
			(i.Config["security.privileged"] != "" && i.Config["security.privileged"] != "false") ||
			(i.Config["security.nesting"] != "" && i.Config["security.nesting"] != "false" && i.Config["security.nesting"] != "true") {
			return i, core.ErrRecoveryRequired
		}
		return i, nil
	}
	before, err := verify()
	if err != nil {
		return err
	}
	if before.Config["security.nesting"] == "true" {
		return nil
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "config", "set", trustedHostName, "security.nesting", "true", "--project", b.Runtime.project)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return core.ErrRecoveryRequired
	}
	after, err := verify()
	if err != nil || after.Config["security.nesting"] != "true" || after.LocalConfig["security.nesting"] != "true" {
		return core.ErrRecoveryRequired
	}
	return nil
}
