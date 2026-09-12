package incus

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
)

// Only current destination ownership and Incus idmap bookkeeping survive a copy.
// Callers validate destination identity and record completion before verification.
func (r *Runtime) copySavedVolume(ctx context.Context, p *snapshotVolumePlan, pool, name string, targetConfig map[string]string) error {
	observed, err := r.snapshotVolumeObservation(ctx, *p, true)
	if err != nil {
		return err
	}
	if observed == nil {
		return core.ErrNotFound
	}
	result, err := r.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	var storage struct{ Name, Driver string }
	if json.Unmarshal([]byte(result.Stdout), &storage) != nil || storage.Name != pool || storage.Driver != "btrfs" {
		return core.ErrUnsupported
	}
	config := map[string]string{}
	for k := range observed.Config {
		config[k] = ""
	}
	for _, key := range []string{"volatile.idmap.last", "volatile.idmap.next"} {
		value := observed.Config[key]
		if value == "" {
			continue
		}
		var mapping []json.RawMessage
		if json.Unmarshal([]byte(value), &mapping) != nil || mapping == nil {
			return core.ErrIncompatibleState
		}
		config[key] = value
	}
	for k, v := range targetConfig {
		config[k] = v
	}
	request := map[string]any{"name": name, "type": "custom", "content_type": "filesystem", "config": config,
		"source": map[string]any{"type": "copy", "name": p.target(), "pool": pool, "project": r.project, "volume_only": true}}
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	result, err = r.runner.Run(ctx, "incus", "query", "-X", "POST", "--wait", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+r.project, "--data", string(raw))
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return core.ErrRecoveryRequired
	}
	return nil // Registry records completion before verification or another copy.
}

// Catalog receipts may be routed. Bind the route to the canonical plan rather
// than stripping arbitrary prefixes or permitting another provider identity.
func (r *Runtime) decodeSavedComponent(c core.SnapshotComponent) (snapshotBinding, error) {
	var plan snapshotBinding
	if len(c.Binding) == 0 || len(c.Binding) > 16384 || json.Unmarshal([]byte(c.Binding), &plan) != nil {
		return plan, core.ErrInvalidArgument
	}
	expected, err := r.snapshotComponent(plan)
	if err != nil {
		return plan, err
	}
	if !environmentapp.MatchesRuntimeRef(c.NativeRef, environmentapp.ProviderIncus, expected.NativeRef) {
		return plan, core.ErrCapabilityStale
	}
	c.NativeRef = expected.NativeRef
	return r.decodeSnapshotComponent(c)
}
