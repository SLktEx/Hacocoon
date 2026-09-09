package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"regexp"
	"sort"
	"strings"
)

// ManagedImages uses current Store ownership and runtime inventories, never Seed
// selections, a Host cache, tombstones or direct layer-file deletion.
type ManagedImages struct {
	Catalog      ManagedImageCatalog
	Environments ResourceExecutor
	Host         HostImageExecutor
}
type ManagedImageCatalog interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
	EnvironmentInstance(context.Context, core.Environment) (string, error)
	GetPersistentResource(context.Context, string) (core.PersistentResource, error)
	ListSnapshots(context.Context) ([]core.Snapshot, error)
}
type HostImageExecutor interface {
	ExecHostImage(context.Context, core.PersistentResource, string, []string) (core.ExecutionResult, error)
}
type ResourceExecutor interface {
	ExecForResource(context.Context, string, string, core.PersistentResourceRef, core.ExecutionRequest) (core.ExecutionResult, error)
}
type ImageTarget struct {
	Host        bool                       `json:"host,omitempty"`
	Environment string                     `json:"environment"`
	Instance    string                     `json:"instance"`
	Store       core.PersistentResourceRef `json:"store"`
	Runtime     string                     `json:"runtime"`
}
type ManagedImage struct {
	ID         string   `json:"id"`
	Tags       []string `json:"tags"`
	Digests    []string `json:"digests"`
	Containers []string `json:"containers"`
}
type ManagedImageList struct {
	Target               ImageTarget    `json:"target"`
	Images               []ManagedImage `json:"images"`
	IndependentSnapshots []string       `json:"independent_snapshots"`
}

var imageIDPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var imageReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,999}$`)
var containerIDPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ValidImageSelection rejects incomplete reviewed identities before dispatch.
func ValidImageSelection(target ImageTarget, id string) bool {
	return validImageTarget(target) && imageIDPattern.MatchString(id)
}
func validImageTarget(target ImageTarget) bool {
	if !validImageRuntime(target.Runtime) || !core.ValidPersistentResourceRef(target.Store) {
		return false
	}
	if target.Host {
		return target.Store.ID == HostStoreID && target.Environment == "" && target.Instance == ""
	}
	return target.Environment != "" && core.ValidEnvironmentInstanceID(target.Instance) && target.Store.ID != HostStoreID
}
func (s *ManagedImages) ListHost(ctx context.Context, runtime string) (ManagedImageList, error) {
	if !validImageRuntime(runtime) {
		return ManagedImageList{}, core.ErrInvalidArgument
	}
	source, err := s.Catalog.GetPersistentResource(ctx, HostStoreID)
	if err != nil {
		return ManagedImageList{}, err
	}
	return s.inspect(ctx, ImageTarget{Host: true, Store: source.Ref(), Runtime: runtime})
}
func validImageRuntime(runtime string) bool { return runtime == "docker" || runtime == "nerdctl" }
func (s *ManagedImages) List(ctx context.Context, name, runtime string) (ManagedImageList, error) {
	if !validImageRuntime(runtime) {
		return ManagedImageList{}, core.ErrInvalidArgument
	}
	env, err := s.Catalog.GetEnvironment(ctx, name)
	if err != nil {
		return ManagedImageList{}, err
	}
	instance, err := s.Catalog.EnvironmentInstance(ctx, env)
	if err != nil {
		return ManagedImageList{}, err
	}
	target := ImageTarget{Environment: name, Instance: instance, Store: env.PersistentResource, Runtime: runtime}
	return s.inspect(ctx, target)
}
func (s *ManagedImages) checkTarget(ctx context.Context, target ImageTarget) error {
	if !validImageTarget(target) {
		return core.ErrInvalidArgument
	}
	resource, err := s.Catalog.GetPersistentResource(ctx, target.Store.ID)
	if err != nil {
		return err
	}
	if resource.Ref() != target.Store || resource.Kind != StoreKind || resource.SourceOnly != target.Host || resource.State != "ready" || (target.Host && resource.WorkspaceID != "") {
		return core.ErrCapabilityStale
	}
	return nil
}
func (s *ManagedImages) command(ctx context.Context, target ImageTarget, args ...string) (string, error) {
	if err := s.checkTarget(ctx, target); err != nil {
		return "", err
	}
	argv := []string{"/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/nonexistent"}
	if target.Runtime == "docker" {
		argv = append(argv, "docker", "--host", "unix:///run/docker.sock")
	} else {
		argv = append(argv, "nerdctl", "--address", "/run/containerd/containerd.sock", "--namespace", "default", "--snapshotter", "native")
	}
	var result core.ExecutionResult
	var err error
	if target.Host {
		if s.Host == nil {
			return "", core.ErrUnsupported
		}
		source, e := s.Catalog.GetPersistentResource(ctx, target.Store.ID)
		if e != nil {
			return "", e
		}
		if source.Ref() != target.Store {
			return "", core.ErrCapabilityStale
		}
		result, err = s.Host.ExecHostImage(ctx, source, target.Runtime, args)
	} else {
		result, err = s.Environments.ExecForResource(ctx, target.Environment, target.Instance, target.Store, core.ExecutionRequest{Argv: append(argv, args...)})
	}
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 || result.StdoutTruncated || result.StderrTruncated {
		return "", fmt.Errorf("%s image operation failed (exit %d): %w", target.Runtime, result.ExitCode, core.ErrRuntimeUnavailable)
	}
	return result.Stdout, nil
}
func (s *ManagedImages) inspect(ctx context.Context, target ImageTarget) (ManagedImageList, error) {
	result := ManagedImageList{Target: target, Images: []ManagedImage{}, IndependentSnapshots: []string{}}
	output, err := s.command(ctx, target, "image", "ls", "--quiet", "--no-trunc")
	if err != nil {
		return result, err
	}
	ids := map[string]bool{}
	for _, id := range strings.Fields(output) {
		if !imageIDPattern.MatchString(id) {
			return result, core.ErrIncompatibleState
		}
		ids[id] = true
	}
	if len(ids) > 4096 {
		return result, core.ErrInvalidArgument
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	for _, id := range ordered {
		raw, err := s.command(ctx, target, "image", "inspect", "--format", `{{json .Id}} {{json .RepoTags}} {{json .RepoDigests}}`, id)
		if err != nil {
			return result, err
		}
		image := ManagedImage{Containers: []string{}}
		d := json.NewDecoder(strings.NewReader(raw))
		if d.Decode(&image.ID) != nil || d.Decode(&image.Tags) != nil || d.Decode(&image.Digests) != nil || d.Decode(new(any)) != io.EOF || !imageIDPattern.MatchString(image.ID) || (target.Runtime == "docker" && image.ID != id) {
			return result, core.ErrIncompatibleState
		}
		// nerdctl lists manifest/index digests but Docker-compatible inspect
		// returns the platform config digest. Keep the runtime's deletion ID.
		if target.Runtime == "nerdctl" {
			matched := false
			for _, digest := range image.Digests {
				matched = matched || strings.HasSuffix(digest, "@"+id)
			}
			if !matched {
				return result, core.ErrIncompatibleState
			}
		}
		image.ID = id
		if image.Tags == nil {
			image.Tags = []string{}
		}
		if image.Digests == nil {
			image.Digests = []string{}
		}
		result.Images = append(result.Images, image)
	}
	raw, err := s.command(ctx, target, "container", "ls", "--all", "--quiet", "--no-trunc")
	if err != nil {
		return result, err
	}
	containers := strings.Fields(raw)
	if len(containers) > 4096 {
		return result, core.ErrInvalidArgument
	}
	for _, id := range containers {
		if !containerIDPattern.MatchString(id) {
			return result, core.ErrIncompatibleState
		}
		raw, err := s.command(ctx, target, "container", "inspect", "--format", `{{json .Image}} {{json .Name}}`, id)
		if err != nil {
			return result, err
		}
		var image, name string
		d := json.NewDecoder(strings.NewReader(raw))
		if d.Decode(&image) != nil || d.Decode(&name) != nil || d.Decode(new(any)) != io.EOF {
			return result, core.ErrIncompatibleState
		}
		used := map[string]bool{}
		if target.Runtime == "docker" {
			if !imageIDPattern.MatchString(image) {
				return result, core.ErrIncompatibleState
			}
			used[image] = true
		} else {
			// containerd retains the image reference, not Docker's config ID.
			// Resolve it through the runtime; never interpolate it in a shell.
			if !imageReferencePattern.MatchString(image) {
				return result, core.ErrIncompatibleState
			}
			raw, err := s.command(ctx, target, "image", "inspect", "--format", `{{json .RepoDigests}}`, "--", image)
			if err != nil {
				return result, err
			}
			var digests []string
			decoder := json.NewDecoder(strings.NewReader(raw))
			if decoder.Decode(&digests) != nil || decoder.Decode(new(any)) != io.EOF || len(digests) == 0 {
				return result, core.ErrIncompatibleState
			}
			for _, digest := range digests {
				_, id, ok := strings.Cut(digest, "@")
				if !ok || !imageIDPattern.MatchString(id) {
					return result, core.ErrIncompatibleState
				}
				used[id] = true
			}
		}
		for i := range result.Images {
			if used[result.Images[i].ID] {
				result.Images[i].Containers = append(result.Images[i].Containers, id+":"+name)
			}
		}
	}
	snapshots, err := s.Catalog.ListSnapshots(ctx)
	if err != nil {
		return result, err
	}
	for _, saved := range snapshots {
		if saved.Source.Environment.PersistentResource == target.Store {
			result.IndependentSnapshots = append(result.IndependentSnapshots, saved.ID)
		}
	}
	sort.Strings(result.IndependentSnapshots)
	return result, nil
}
func (s *ManagedImages) Delete(ctx context.Context, target ImageTarget, id string) error {
	if !imageIDPattern.MatchString(id) {
		return core.ErrInvalidArgument
	}
	before, err := s.inspect(ctx, target)
	if err != nil {
		return err
	}
	found := false
	for _, image := range before.Images {
		if image.ID == id {
			found = true
			if len(image.Containers) > 0 {
				return fmt.Errorf("image is referenced by a container: %w", core.ErrStorageBusy)
			}
		}
	}
	if !found {
		return core.ErrNotFound
	}
	if _, err := s.command(ctx, target, "image", "rm", id); err != nil {
		return err
	}
	after, err := s.inspect(ctx, target)
	if err != nil {
		return fmt.Errorf("image deletion could not be confirmed: %w", err)
	}
	for _, image := range after.Images {
		if image.ID == id {
			return core.ErrRecoveryRequired
		}
	}
	return nil
}
