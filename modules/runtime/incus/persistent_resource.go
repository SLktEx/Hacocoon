package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const OCIStoreKind = "oci-containerd"
const OCIStorePath = "/var/lib/hacocoon-oci"

type PersistentResourceBackend struct{ Runtime *Runtime }

func (b *PersistentResourceBackend) Plan(ctx context.Context, kind, owner string) (string, error) {
	if kind != OCIStoreKind || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:check", Owner: owner}) {
		return "", core.ErrInvalidArgument
	}
	pool, err := b.Runtime.defaultRootPool(ctx)
	if err != nil {
		return "", err
	}
	if err := b.Runtime.ensureProject(ctx); err != nil {
		return "", err
	}
	return pool + "/haco-persistent-" + owner, nil
}

func persistentVolume(r core.PersistentResource) (string, string, error) {
	parts := strings.Split(r.NativeRef, "/")
	if r.Kind != OCIStoreKind || !core.ValidPersistentResourceRef(r.Ref()) || len(parts) != 2 || !safeIncusRef(parts[0]) || parts[1] != "haco-persistent-"+r.Owner {
		return "", "", core.ErrInvalidArgument
	}
	return parts[0], parts[1], nil
}

func (b *PersistentResourceBackend) Create(ctx context.Context, r core.PersistentResource) error {
	pool, name, err := persistentVolume(r)
	if err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]any{"name": name, "type": "custom", "content_type": "filesystem", "config": map[string]string{"user.hacocoon.owner": r.Owner, "user.hacocoon.resource": r.ID, "user.hacocoon.kind": r.Kind, "user.hacocoon.source-only": strconv.FormatBool(r.SourceOnly)}})
	_, err = b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+b.Runtime.project, "-X", "POST", "--wait", "--data", string(data))
	return err
}

type persistentVolumeObservation struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	ContentType string            `json:"content_type"`
	Config      map[string]string `json:"config"`
	UsedBy      []string          `json:"used_by"`
}

func (b *PersistentResourceBackend) observe(ctx context.Context, r core.PersistentResource) (*persistentVolumeObservation, error) {
	pool, name, err := persistentVolume(r)
	if err != nil {
		return nil, err
	}
	out, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+b.Runtime.project+"&recursion=1")
	if err != nil || out.StdoutTruncated {
		return nil, core.ErrRuntimeUnavailable
	}
	var volumes []persistentVolumeObservation
	if json.Unmarshal([]byte(out.Stdout), &volumes) != nil || volumes == nil {
		return nil, core.ErrIncompatibleState
	}
	var found *persistentVolumeObservation
	for _, v := range volumes {
		if v.Name == name {
			if found != nil || v.Type != "custom" || v.ContentType != "filesystem" || v.Config["user.hacocoon.owner"] != r.Owner || v.Config["user.hacocoon.resource"] != r.ID || v.Config["user.hacocoon.kind"] != r.Kind || !matchesSourceOnlyMarker(v.Config["user.hacocoon.source-only"], r.SourceOnly) {
				return nil, core.ErrIncompatibleState
			}
			copy := v
			found = &copy
		}
	}
	return found, nil
}

func (b *PersistentResourceBackend) Verify(ctx context.Context, r core.PersistentResource) error {
	v, err := b.observe(ctx, r)
	if err != nil {
		return err
	}
	if v == nil {
		return core.ErrNotFound
	}
	if len(v.UsedBy) != 0 {
		if r.SourceOnly {
			return b.VerifyHostSource(ctx, r)
		}
		return core.ErrStorageBusy
	}
	return nil
}

func (b *PersistentResourceBackend) Delete(ctx context.Context, r core.PersistentResource) error {
	v, err := b.observe(ctx, r)
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	if len(v.UsedBy) > 0 {
		return core.ErrStorageBusy
	}
	pool, name, _ := persistentVolume(r)
	_, deleteErr := b.Runtime.runner.Run(ctx, "incus", "storage", "volume", "delete", pool, name, "--project", b.Runtime.project)
	after, err := b.observe(ctx, r)
	if err != nil {
		return fmt.Errorf("cannot verify deletion (%v): %w", deleteErr, err)
	}
	if after != nil {
		return fmt.Errorf("persistent volume still present: %w", core.ErrRecoveryRequired)
	}
	return nil
}

func (p *SandboxProvider) attachPersistentResource(ctx context.Context, ref string, r core.PersistentResource) error {
	if r.SourceOnly {
		return core.ErrPolicyDenied
	}
	if r == (core.PersistentResource{}) {
		return nil
	}
	if err := (&PersistentResourceBackend{Runtime: p.Runtime}).Verify(ctx, r); err != nil {
		return err
	}
	pool, name, err := persistentVolume(r)
	if err != nil {
		return err
	}
	if err := p.setAndVerifyConfig(ctx, ref, "security.nesting", "true"); err != nil {
		return err
	}
	_, err = p.runner.Run(ctx, "incus", "config", "device", "add", ref, "persistent-resource", "disk", "pool="+pool, "source="+name, "path="+OCIStorePath, "--project", p.project)
	return err
}

// Only persistent data roots are attached; runtime processes and /run remain
// private to the disposable Environment. Optional runtime binaries are supplied
// by the selected Base or installed by its user, never required by Core.
const persistentOCIConfiguration = `set -eu
# Incus start returns before guest systemd's management socket is necessarily
# ready. Probe readiness only; never repair/restart a rejected guest operation.
attempt=0
until systemctl show --property=Version --value >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    printf '%s\n' 'Environment systemd manager did not become ready' >&2
    exit 1
  fi
  sleep 0.5
done
was_active=false
if systemctl is-active --quiet containerd; then was_active=true; systemctl stop containerd; fi
mkdir -p /etc/containerd /etc/systemd/system
cat > /etc/containerd/config.toml <<'HACO_CONTAINERD'
version = 2
root = "/var/lib/hacocoon-oci/containerd"
state = "/run/containerd"
[grpc]
  address = "/run/containerd/containerd.sock"
HACO_CONTAINERD
cat > /etc/systemd/system/buildkit.service <<'HACO_BUILDKIT'
[Unit]
Description=Environment-local BuildKit with persistent OCI cache
After=network.target
[Service]
ExecStart=/usr/local/bin/buildkitd --root /var/lib/hacocoon-oci/buildkit --addr unix:///run/buildkit/buildkitd.sock --oci-worker-snapshotter native --containerd-worker=false
[Install]
WantedBy=multi-user.target
HACO_BUILDKIT
systemctl daemon-reload
if "$was_active"; then systemctl start containerd; fi
`

// Copy uses Incus's same-pool volume copy. Incus owns Btrfs COW and idmaps;
// neither source data nor a Host runtime socket is mounted into a new boundary.
func (b *PersistentResourceBackend) Copy(ctx context.Context, source, target core.PersistentResource) error {
	pool, name, err := persistentVolume(target)
	if err != nil {
		return err
	}
	sourcePool, sourceName, err := persistentVolume(source)
	if err != nil {
		return err
	}
	if pool != sourcePool || sourceName == name || source.ID == target.ID || target.CopySource != source.Ref() {
		return core.ErrInvalidArgument
	}
	// Refuse storage drift rather than silently claiming a full copy is COW.
	result, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool)
	if err != nil || result.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	var storage struct {
		Name   string `json:"name"`
		Driver string `json:"driver"`
	}
	if json.Unmarshal([]byte(result.Stdout), &storage) != nil || storage.Name != pool || storage.Driver != "btrfs" {
		return core.ErrIncompatibleState
	}
	observed, err := b.observe(ctx, source)
	if err != nil {
		return err
	}
	if observed == nil {
		return core.ErrNotFound
	}

	// Incus refuses a destination name collision. Supply new ownership markers,
	// never copy arbitrary source config (especially authority-bearing settings).
	config := map[string]string{"user.hacocoon.owner": target.Owner, "user.hacocoon.resource": target.ID, "user.hacocoon.kind": target.Kind, "user.hacocoon.source-only": strconv.FormatBool(target.SourceOnly)}
	for _, key := range []string{"volatile.idmap.last", "volatile.idmap.next"} {
		value := observed.Config[key]
		if value == "" {
			continue
		} // A never-attached empty Store has no idmap yet.
		var mapping []json.RawMessage
		if json.Unmarshal([]byte(value), &mapping) != nil || mapping == nil {
			return core.ErrIncompatibleState
		}
		config[key] = value
	}
	data, err := json.Marshal(map[string]any{"name": name, "type": "custom", "content_type": "filesystem", "config": config,
		"source": map[string]any{"type": "copy", "name": sourceName, "pool": pool, "project": b.Runtime.project, "volume_only": true}})
	if err != nil {
		return err
	}
	var resume func(context.Context) error
	if len(observed.UsedBy) != 0 {
		unlock, lockErr := lockHostOperation(ctx, b.Runtime.project)
		if lockErr != nil {
			return lockErr
		}
		defer unlock()
		resume, err = b.quiesceHostCopy(ctx, source, target, observed)
		if err != nil {
			return err
		}
	}
	result, err = b.Runtime.runner.Run(ctx, "incus", "query", "-X", "POST", "--wait", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+b.Runtime.project, "--data", string(data))
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return fmt.Errorf("persistent copy completion unconfirmed: %w", core.ErrRecoveryRequired)
	}
	if resume != nil {
		return resume(ctx)
	}
	return nil
}

func matchesSourceOnlyMarker(marker string, sourceOnly bool) bool {
	if sourceOnly {
		return marker == "true"
	}
	return marker == "" || marker == "false"
}
