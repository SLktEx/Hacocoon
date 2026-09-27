package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net/url"
	"time"
)

const imageTagDescription = "Hacocoon Image tag v1"

func (p *BaseProvider) TagImage(ctx context.Context, source, target core.BaseName) (core.BaseInfo, error) {
	p.baseMu.Lock()
	defer p.baseMu.Unlock()
	if !basebuild.NamePattern.MatchString(string(target)) {
		return core.BaseInfo{}, core.ErrInvalidArgument
	}
	if _, ok := p.sources[target]; ok {
		return core.BaseInfo{}, core.ErrAlreadyExists
	}
	resolved, err := p.resolveBase(ctx, source)
	if err != nil {
		return core.BaseInfo{}, err
	}
	fingerprint, err := baseRevisionFingerprint(resolved.ref.Revision)
	if err != nil {
		return core.BaseInfo{}, err
	}
	aliases, err := p.baseAliases(ctx)
	if err != nil {
		return core.BaseInfo{}, err
	}
	for _, a := range aliases {
		if a.Name == builtBasePrefix+string(target) {
			return core.BaseInfo{}, core.ErrAlreadyExists
		}
	}
	// Tagging refers to a local image; it never downloads or changes its rootfs.
	var image baseImage
	if err := p.baseQuery(ctx, "GET", p.basePath("/"+fingerprint), nil, &image); err != nil {
		return core.BaseInfo{}, err
	}
	if image.Fingerprint != fingerprint || image.Type != "container" {
		return core.BaseInfo{}, core.ErrIncompatibleState
	}
	if err := p.baseQuery(ctx, "POST", p.basePath("/aliases"), map[string]string{"name": builtBasePrefix + string(target), "target": fingerprint, "description": imageTagDescription}, nil); err != nil {
		return core.BaseInfo{}, err
	}
	return core.BaseInfo{Name: target, Revision: resolved.ref.Revision}, nil
}
func (p *BaseProvider) RemoveImageTag(ctx context.Context, name core.BaseName) error {
	if !basebuild.NamePattern.MatchString(string(name)) {
		return core.ErrInvalidArgument
	}
	p.baseMu.Lock()
	defer p.baseMu.Unlock()
	aliases, err := p.baseAliases(ctx)
	if err != nil {
		return err
	}
	for _, a := range aliases {
		if a.Name == builtBasePrefix+string(name) {
			if a.Description != imageTagDescription || a.Type != "container" || !baseFingerprintPattern.MatchString(a.Target) {
				return core.ErrUnsupported
			}
			return p.baseQuery(ctx, "DELETE", p.basePath("/aliases/"+url.PathEscape(a.Name)), nil, nil)
		}
	}
	return core.ErrNotFound
}

// CommitImage uses the same native rootfs isolation and ownership receipts as
// Snapshot capture. Attached Workspace/Volume/OCI disks never enter the Image.
func (p *BaseProvider) CommitImage(ctx context.Context, env core.Environment, lease core.WorkspaceLease, name core.BaseName) (result core.BaseInfo, err error) {
	p.baseMu.Lock()
	defer p.baseMu.Unlock()
	if !basebuild.NamePattern.MatchString(string(name)) || lease.Ephemeral || !lease.MatchesEnvironment(env) || !core.ValidEnvironmentInstanceID(lease.InstanceID) {
		return result, core.ErrInvalidArgument
	}
	if _, ok := p.sources[name]; ok {
		return result, core.ErrAlreadyExists
	}
	instances, err := p.readSnapshotInstances(ctx)
	if err != nil {
		return result, err
	}
	pool := ""
	for _, instance := range instances {
		if instance.Name == env.RuntimeRef {
			for _, device := range instance.ExpandedDevices {
				if device["type"] == "disk" && device["path"] == "/" {
					if pool != "" {
						return result, core.ErrIncompatibleState
					}
					pool = device["pool"]
				}
			}
		}
	}
	var nonce [16]byte
	_, _ = rand.Read(nonce[:])
	root := snapshotRootfsPlan{Pool: pool, Source: env.RuntimeRef, SourceInstanceID: lease.InstanceID, Owner: hex.EncodeToString(nonce[:])}
	_, _ = rand.Read(nonce[:])
	plan := snapshotImagePlan{Name: string(name), Owner: hex.EncodeToString(nonce[:]), Rootfs: root}
	if err := plan.validate(); err != nil {
		return result, err
	}
	if err := p.checkSnapshotImageName(ctx, plan); err != nil {
		return result, err
	}
	// Native create stores the planned owner in Incus atomically. Cleanup uses
	// that exact owner even when create acknowledgement or verification fails.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if e := p.deleteSnapshotRootfs(cleanup, root); e != nil {
			err = errors.Join(err, fmt.Errorf("owned rootfs %s retained: %w", root.target(), core.ErrRecoveryRequired), e)
		}
	}()
	if err = p.createSnapshotRootfs(ctx, root); err != nil {
		return result, err
	}
	if err = p.createSnapshotImage(ctx, plan); err != nil {
		return result, err
	}
	image, err := p.snapshotImage(ctx, plan)
	if err != nil {
		return result, err
	}
	return core.BaseInfo{Name: name, Revision: image.Revision}, nil
}

// Tagged image aliases use a distinct ownership marker and immutable target.
func (p *BaseProvider) taggedImage(ctx context.Context, a baseAlias) (baseImage, error) {
	var image baseImage
	if a.Description != imageTagDescription || a.Type != "container" || !baseFingerprintPattern.MatchString(a.Target) {
		return image, core.ErrCapabilityStale
	}
	if err := p.baseQuery(ctx, "GET", p.basePath("/"+a.Target), nil, &image); err != nil {
		return image, err
	}
	if image.Fingerprint != a.Target || image.Type != "container" {
		return image, core.ErrCapabilityStale
	}
	return image, nil
}
