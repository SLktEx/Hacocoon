//go:build linux

package environmenttransfer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"io"
	"strings"
	"time"
)

type ImportCatalog interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
	GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error)
}
type ImportEnvironments interface {
	CreateFromArchive(context.Context, core.EnvironmentSpec, io.ReadSeeker, string, int64) (core.Environment, error)
	StartForWorkspace(context.Context, string, core.WorkspaceID) error
	CleanupRestoredData(context.Context, core.Workspace, func(context.Context) error) error
}
type ImportWorkspaces interface {
	ImportWorkspace(context.Context, string, string, string, string, io.ReadSeeker) (gitrepo.Object, error)
	ImportWorkspaceSet(context.Context, string, []gitrepo.WorkspaceImport) (gitrepo.Object, error)
	DeleteWorkspace(context.Context, string, string) error
}
type ImportStores interface {
	ImportForWorkspace(context.Context, string, string, io.ReadSeeker, core.WorkspaceID) (core.PersistentResource, error)
	DeleteRestoredCopy(context.Context, core.PersistentResource) error
}
type Importer struct {
	Catalog      ImportCatalog
	Environments ImportEnvironments
	Workspaces   ImportWorkspaces
	Stores       ImportStores
	Root         string
	StoreKind    string // Trusted local composition, never bundle metadata.
}

// Import verifies the complete input before mutation. Existing native services
// own resources and exact cleanup receipts; no transfer catalog is introduced.
func (s *Importer) Import(ctx context.Context, source io.Reader, name string, limit int64) (result ImportResult, resultErr error) {
	if s == nil || source == nil || !validLimit(limit) || s.Catalog == nil || s.Environments == nil || s.Workspaces == nil || (name != "" && !sourceName.MatchString(name)) {
		return result, core.ErrInvalidArgument
	}
	staged, err := Stage(ctx, s.Root, source, limit)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, staged.Close()) }()
	manifest := staged.Manifest()
	count := len(manifest.Components) - 1
	if manifest.HasOCI {
		count--
	}
	if count > 8 || (manifest.HasOCI && (s.Stores == nil || s.StoreKind == "")) {
		return result, core.ErrUnsupported
	}
	if name == "" {
		prefix := manifest.Source
		if len(prefix) > 48 {
			prefix = prefix[:48]
		}
		name = prefix + "-imported"
	}
	if _, err := s.Catalog.GetEnvironment(ctx, name); err == nil {
		return result, core.ErrAlreadyExists
	} else if !errors.Is(err, core.ErrNotFound) {
		return result, err
	}
	if _, err := s.Catalog.GetWorkspaceLease(ctx, name); err == nil {
		return result, core.ErrRecoveryRequired
	} else if !errors.Is(err, core.ErrNotFound) {
		return result, err
	}
	rootfs, err := staged.ComponentReader("rootfs")
	if err != nil {
		return result, err
	}
	var oci io.ReadSeeker
	if manifest.HasOCI {
		oci, err = staged.ComponentReader("oci")
		if err != nil {
			return result, err
		}
	}
	inputs := make([]gitrepo.WorkspaceImport, 0, count)
	for i := 0; i < count; i++ {
		descriptor := Workspace{Role: workspaceRole(i), Name: workspaceRole(i)}
		if manifest.Version == 2 {
			descriptor = manifest.Workspaces[i]
		}
		data, err := staged.ComponentReader(descriptor.Role)
		if err != nil {
			return result, err
		}
		remote, branch := descriptor.Remote, descriptor.Branch
		// A source Host path is not a destination route. Preserve its Git data and
		// the original bundle, but register offline rather than adopting Host access.
		if !strings.HasPrefix(remote, "https://github.com/") {
			remote, branch = "", ""
		}
		if remote == "" {
			result.Offline = append(result.Offline, descriptor.Name)
		}
		inputs = append(inputs, gitrepo.WorkspaceImport{Repository: descriptor.Name, Remote: remote, Branch: branch, Archive: data})
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return result, err
	}
	workName := "import-" + hex.EncodeToString(nonce[:])
	result.Environment = name
	result.State = "failed"
	var object gitrepo.Object
	if len(inputs) == 1 {
		input := inputs[0]
		object, err = s.Workspaces.ImportWorkspace(ctx, workName, input.Repository, input.Remote, input.Branch, input.Archive)
	} else {
		object, err = s.Workspaces.ImportWorkspaceSet(ctx, workName, inputs)
	}
	result.Workspace = object.ID
	if err != nil {
		if object.ID != "" {
			result.State = "cleanup-required"
		}
		return result, err
	}
	if object.ID != workName || object.State != "ready" || object.Kind != "work" || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:identity", Owner: object.Owner}) {
		result.State = "cleanup-required"
		return result, core.ErrRecoveryRequired
	}
	work := core.Workspace{ID: core.WorkspaceID("workspace:managed:" + object.Owner), Path: "managed:" + object.ID}
	var resource core.PersistentResource
	fail := func(cause error) (ImportResult, error) {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		err := s.Environments.CleanupRestoredData(cleanup, work, func(ctx context.Context) error {
			if resource.ID != "" {
				if err := s.Stores.DeleteRestoredCopy(ctx, resource); err != nil {
					return err
				}
				result.OCI = ""
			}
			if err := s.Workspaces.DeleteWorkspace(ctx, object.ID, object.Owner); err != nil {
				return err
			}
			result.Workspace = ""
			return nil
		})
		if err != nil {
			result.State = "cleanup-required"
			return result, errors.Join(cause, err, core.ErrRecoveryRequired)
		}
		return result, cause
	}
	if manifest.HasOCI {
		resource, err = s.Stores.ImportForWorkspace(ctx, "oci:"+workName, s.StoreKind, oci, work.ID)
		result.OCI = resource.ID
		if err != nil {
			// An unfinished native import may still produce data. Keep its catalog and
			// related Workspace; the native owner must resolve uncertain completion.
			if resource.ID != "" {
				result.State = "cleanup-required"
				return result, errors.Join(err, core.ErrRecoveryRequired)
			}
			return fail(err)
		}
		if resource.ID != "oci:"+workName || resource.Kind != s.StoreKind || resource.SourceOnly || resource.State != "ready" || resource.WorkspaceID != work.ID || !core.ValidPersistentResourceRef(resource.Ref()) {
			result.State = "cleanup-required"
			return result, core.ErrRecoveryRequired
		}
	}
	environment, err := s.Environments.CreateFromArchive(ctx, core.EnvironmentSpec{Name: name, WorkspacePath: work.Path, PersistentResource: resource.ID, ExpectedResource: resource.Ref(), SkipDefaultResource: resource.ID == ""}, rootfs, s.Root, limit)
	if err != nil {
		return fail(err)
	}
	if environment.Name != name || environment.Workspace != work || environment.PersistentResource != resource.Ref() {
		result.State = "cleanup-required"
		return result, core.ErrRecoveryRequired
	}
	result.State = "start-failed"
	if err := s.Environments.StartForWorkspace(ctx, environment.Name, work.ID); err != nil {
		return result, err
	}
	result.State = "running"
	return result, nil
}
