package incus

import (
	"context"
	"encoding/json"
	"fmt"
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
	data, _ := json.Marshal(map[string]any{"name": name, "type": "custom", "content_type": "filesystem", "config": map[string]string{"user.hacocoon.owner": r.Owner, "user.hacocoon.resource": r.ID, "user.hacocoon.kind": r.Kind}})
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
			if found != nil || v.Type != "custom" || v.ContentType != "filesystem" || v.Config["user.hacocoon.owner"] != r.Owner || v.Config["user.hacocoon.resource"] != r.ID || v.Config["user.hacocoon.kind"] != r.Kind {
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
	if r.ID == "" {
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
