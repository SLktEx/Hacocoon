package incus

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/baseasset"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// BaseAssetBackend retains independently owned Base rootfs material. Its caller
// must persist the complete plan before Create and its receipt before Verify.
type BaseAssetBackend struct{ Provider *BaseProvider }

var _ baseasset.Backend = (*BaseAssetBackend)(nil)

type baseAssetBinding struct {
	Version int    `json:"version"`
	Project string `json:"project"`
	Pool    string `json:"pool"`
	Source  string `json:"source"`
}

func (b *BaseAssetBackend) Plan(ctx context.Context, base core.BaseRef, scope, owner string) (string, string, error) {
	if b == nil || b.Provider == nil || b.Provider.Runtime == nil {
		return "", "", core.ErrRuntimeUnavailable
	}
	project, pool, ok := strings.Cut(scope, "/")
	p := baseStorageIdentity{Kind: "base", Pool: pool, Owner: owner, Base: base}
	if !ok || !safeIncusRef(project) || project != b.Provider.project || p.validate() != nil {
		return "", "", core.ErrInvalidArgument
	}
	source, ok := b.Provider.sources[base.Name]
	if !ok {
		return "", "", core.ErrNotFound
	}
	fp, _ := baseRevisionFingerprint(base.Revision)
	binding := baseAssetBinding{Version: 1, Project: project, Pool: pool, Source: pinImageSource(source, fp)}
	if !validBaseAssetSource(binding.Source, fp) {
		return "", "", core.ErrUnsupported
	}
	if err := b.verifyPool(ctx, pool); err != nil {
		return "", "", err
	}
	data, err := json.Marshal(binding)
	return "instance/" + p.target(), string(data), err
}
func validBaseAssetSource(source, fp string) bool {
	remote, image, ok := strings.Cut(source, ":")
	return ok && safeIncusRef(remote) && image == fp
}
func (b *BaseAssetBackend) decode(a core.BaseAsset) (baseStorageIdentity, baseAssetBinding, error) {
	p := baseStorageIdentity{Kind: "base", Owner: a.Owner, Base: a.Base}
	var binding baseAssetBinding
	if b == nil || b.Provider == nil || b.Provider.Runtime == nil || len(a.Binding) > 16384 || json.Unmarshal([]byte(a.Binding), &binding) != nil {
		return p, binding, core.ErrInvalidArgument
	}
	p.Pool = binding.Pool
	fp, err := baseRevisionFingerprint(a.Base.Revision)
	canonical, encodeErr := json.Marshal(binding)
	if err != nil || encodeErr != nil || string(canonical) != a.Binding || p.validate() != nil || a.Provider != "incus" || a.ID != "base-"+a.Owner || binding.Version != 1 || !safeIncusRef(binding.Project) || binding.Project != b.Provider.project || a.Scope != binding.Project+"/"+binding.Pool || a.NativeRef != "instance/"+p.target() || !validBaseAssetSource(binding.Source, fp) {
		return p, binding, core.ErrIncompatibleState
	}
	return p, binding, nil
}
func (b *BaseAssetBackend) verifyPool(ctx context.Context, pool string) error {
	out, err := b.Provider.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	var found struct{ Name, Driver string }
	if json.Unmarshal([]byte(out.Stdout), &found) != nil || found.Name != pool || found.Driver != "btrfs" {
		return core.ErrIncompatibleState
	}
	return nil
}
func (b *BaseAssetBackend) Create(ctx context.Context, a core.BaseAsset) error {
	p, binding, err := b.decode(a)
	if err != nil {
		return err
	}
	if a.State != "planned" {
		return core.ErrIncompatibleState
	}
	if err := b.verifyPool(ctx, p.Pool); err != nil {
		return err
	}
	args := []string{"init", binding.Source, p.target(), "--project", binding.Project, "--no-profiles", "--storage", binding.Pool}
	config := p.config()
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--config", key+"="+config[key])
	}
	out, err := b.Provider.runner.Run(ctx, "incus", args...)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.ErrRecoveryRequired
	}
	// No fallible provider call is allowed before the caller records completion.
	return nil
}
func (b *BaseAssetBackend) Verify(ctx context.Context, a core.BaseAsset) error {
	p, _, err := b.decode(a)
	if err != nil {
		return err
	}
	if a.State != "created" && a.State != "ready" {
		return core.ErrIncompatibleState
	}
	found, err := b.Provider.baseStorageObservation(ctx, p)
	if err != nil {
		return err
	}
	if found == nil {
		return core.ErrNotFound
	}
	return nil
}
