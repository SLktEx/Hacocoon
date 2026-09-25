package gitrepo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type Object struct {
	RestoredFrom string `json:"restored_from,omitempty"`
	Kind         string `json:"kind"`
	ID           string `json:"id"`
	Repository   string `json:"repository"`
	Remote       string `json:"remote"`
	// Branch belongs to a Workspace route. Legacy source records may contain
	// it, but it is not used to select or authorize a source repository ref.
	Branch    string   `json:"branch,omitempty"`
	NativeRef string   `json:"native_ref"`
	Owner     string   `json:"owner"`
	State     string   `json:"state"`
	Members   []Object `json:"members,omitempty"`
}

type Backend interface {
	Plan(context.Context, string, string) (string, error)
	CreateVolume(context.Context, Object, *Object) error
	InspectVolume(context.Context, Object) error
	Populate(context.Context, Object) error
	RunGit(context.Context, gitadapter.AgentRequest) (gitadapter.Response, error)
	ConnectGit(context.Context, core.Environment, Object, string) error
}

type RepositoryService struct {
	SnapshotCatalog SnapshotWorkspaceCatalog
	Root            string
	Backend         Backend
	mu              sync.Mutex
}

func NewRepositoryService(root string, backend Backend) *RepositoryService {
	return &RepositoryService{Root: root, Backend: backend}
}

func (s *RepositoryService) Add(ctx context.Context, id, remote string) (Object, error) {
	if !gitadapter.ValidID(id) || gitadapter.ValidateRemote(remote) != nil {
		return Object{}, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.readObject("repo", id)
	switch {
	case err == nil:
		if existing.Repository != id || existing.Remote != remote || existing.Branch != "" {
			return Object{}, core.ErrAlreadyExists
		}
		switch existing.State {
		case "ready":
			if err := s.Backend.InspectVolume(ctx, existing); err != nil {
				return existing, errors.Join(err, core.ErrRecoveryRequired)
			}
			return existing, nil
		case "creating", "created":
			return s.resumePrepared(ctx, existing, func(ctx context.Context, object Object) error {
				return s.Backend.CreateVolume(ctx, object, nil)
			}, s.Backend.Populate)
		default:
			return existing, core.ErrIncompatibleState
		}
	case errors.Is(err, core.ErrNotFound):
		return s.create(ctx, Object{Kind: "repo", ID: id, Repository: id, Remote: remote}, nil)
	default:
		return Object{}, err
	}
}

func (s *RepositoryService) CopyWorkspace(ctx context.Context, id, repository string) (Object, error) {
	return s.CopyWorkspaceBranch(ctx, id, repository, "")
}

func (s *RepositoryService) CopyWorkspaceBranch(ctx context.Context, id, repository, branch string) (Object, error) {
	if !gitadapter.ValidID(id) || !gitadapter.ValidID(repository) || (branch != "" && !gitadapter.ValidBranch(branch)) {
		return Object{}, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	repo, err := s.Get("repo", repository)
	if err != nil {
		return Object{}, err
	}
	if err := s.Backend.InspectVolume(ctx, repo); err != nil {
		return Object{}, err
	}
	branch, err = s.resolveBranch(ctx, repo, branch)
	if err != nil {
		return Object{}, err
	}
	return s.create(ctx, Object{Kind: "work", ID: id, Repository: repository, Remote: repo.Remote, Branch: branch}, &repo)
}

// Resolve and fetch under the source lock before making an independent copy.
// The remote default is observed here; it is never repository identity.
func (s *RepositoryService) resolveBranch(ctx context.Context, repo Object, branch string) (string, error) {
	result, err := s.Backend.RunGit(ctx, gitadapter.AgentRequest{Operation: "resolve", Repository: repo.ID, Remote: repo.Remote, Branch: branch})
	if err != nil {
		return "", err
	}
	const prefix = "refs/heads/"
	if !strings.HasPrefix(result.Ref, prefix) || !gitadapter.ValidBranch(strings.TrimPrefix(result.Ref, prefix)) || (branch != "" && result.Ref != prefix+branch) || !gitadapter.ValidOID(result.OID) {
		return "", core.ErrIncompatibleState
	}
	return strings.TrimPrefix(result.Ref, prefix), nil
}

// CopyWorkspaceSet reserves the entire immutable collection before creating
// member volumes. Members have no independently resolvable state records.
func (s *RepositoryService) CopyWorkspaceSet(ctx context.Context, id string, repositories []string) (Object, error) {
	if !gitadapter.ValidID(id) || len(repositories) < 2 || len(repositories) > 8 {
		return Object{}, core.ErrInvalidArgument
	}
	seen := map[string]bool{}
	for _, repo := range repositories {
		if !gitadapter.ValidID(repo) || !gitadapter.ValidID(id+"-"+repo) || seen[repo] {
			return Object{}, core.ErrInvalidArgument
		}
		seen[repo] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	object := Object{Kind: "work", ID: id, Owner: randomID(), State: "creating"}
	sources := make([]Object, 0, len(repositories))
	for _, name := range repositories {
		source, err := s.Get("repo", name)
		if err != nil {
			return Object{}, err
		}
		if err := s.Backend.InspectVolume(ctx, source); err != nil {
			return Object{}, err
		}
		branch, err := s.resolveBranch(ctx, source, "")
		if err != nil {
			return Object{}, err
		}
		ref, err := s.Backend.Plan(ctx, "work", id+"-"+name)
		if err != nil {
			return Object{}, err
		}
		object.Members = append(object.Members, Object{Kind: "work", ID: id + "-" + name, Repository: name, Remote: source.Remote, Branch: branch, NativeRef: ref, Owner: randomID(), State: "creating"})
		sources = append(sources, source)
	}
	return s.createPreparedSet(ctx, object, func(ctx context.Context, i int, member Object) error {
		return s.Backend.CreateVolume(ctx, member, &sources[i])
	}, s.Backend.Populate)
}

// createPreparedSet reserves all member identities before native creation and
// publishes only the whole collection. Members never get independent records.
// Callers hold the service lock throughout preparation.
func (s *RepositoryService) createPreparedSet(ctx context.Context, object Object, create func(context.Context, int, Object) error, populate func(context.Context, Object) error) (Object, error) {
	if err := s.reserve(object); err != nil {
		return Object{}, err
	}
	for i := range object.Members {
		member := &object.Members[i]
		if err := create(ctx, i, *member); err != nil {
			return object, errors.Join(err, core.ErrRecoveryRequired)
		}
		member.State = "created"
		if err := s.save(object); err != nil {
			return object, errors.Join(err, core.ErrRecoveryRequired)
		}
		if err := s.Backend.InspectVolume(ctx, *member); err != nil {
			return object, errors.Join(err, core.ErrRecoveryRequired)
		}
		if populate != nil {
			if err := populate(ctx, *member); err != nil {
				return object, errors.Join(err, core.ErrRecoveryRequired)
			}
		}
		member.State = "ready"
		if err := s.save(object); err != nil {
			return object, errors.Join(err, core.ErrRecoveryRequired)
		}
	}
	object.State = "ready"
	if err := s.save(object); err != nil {
		return object, errors.Join(err, core.ErrRecoveryRequired)
	}
	return object, nil
}

func (o Object) Copies() []Object {
	if len(o.Members) != 0 {
		return o.Members
	}
	return []Object{o}
}

func validObject(o Object) bool {
	if o.RestoredFrom != "" && !validSavedID(o.RestoredFrom) {
		return false
	}
	if !gitadapter.ValidID(o.ID) || (o.Kind != "work" && o.Kind != "repo") || len(o.Owner) != 32 {
		return false
	}
	if len(o.Members) == 0 {
		routing := (o.Branch == "" || gitadapter.ValidBranch(o.Branch)) && gitadapter.ValidateRemote(o.Remote) == nil
		if o.Kind == "work" {
			routing = gitadapter.ValidWorkspaceRouting(o.Remote, o.Branch)
		}
		return gitadapter.ValidID(o.Repository) && routing && o.NativeRef != ""
	}
	if o.Kind != "work" || len(o.Members) < 2 || len(o.Members) > 8 || o.NativeRef != "" || o.Repository != "" || o.Remote != "" || o.Branch != "" {
		return false
	}
	seen := map[string]bool{}
	for _, member := range o.Members {
		expectedID := workspaceMemberID(o.ID, member.Repository, member.Owner)
		if o.RestoredFrom != "" {
			expectedID = restoredWorkspaceMemberID(o.ID, member.Repository, member.Owner)
		}
		if len(member.Members) != 0 || member.Kind != "work" || member.RestoredFrom != o.RestoredFrom || member.ID != expectedID || seen[member.Repository] || !validObject(member) || (o.State == "ready" && member.State != "ready") {
			return false
		}
		seen[member.Repository] = true
	}
	return true
}

func (s *RepositoryService) create(ctx context.Context, object Object, source *Object) (Object, error) {
	return s.createPrepared(ctx, object, func(ctx context.Context, object Object) error {
		return s.Backend.CreateVolume(ctx, object, source)
	}, s.Backend.Populate)
}

// createPrepared is the existing single-volume ownership/publication transition.
// Native imports already contain their data and do not run Git population.
func (s *RepositoryService) createPrepared(ctx context.Context, object Object, create func(context.Context, Object) error, populate func(context.Context, Object) error) (Object, error) {
	if err := ctx.Err(); err != nil {
		return Object{}, err
	}
	ref, err := s.Backend.Plan(ctx, object.Kind, object.ID)
	if err != nil {
		return Object{}, err
	}
	object.NativeRef = ref
	object.Owner = randomID()
	object.State = "creating"
	// Reserve exact ownership before touching the provider. Retries keep this
	// identity and continue the same transition instead of creating a new one.
	if err := s.reserve(object); err != nil {
		return Object{}, err
	}
	return s.resumePrepared(ctx, object, create, populate)
}

// resumePrepared makes the single-volume transition idempotent. A retry starts
// from the durable state already recorded and repeats only operations that are
// safe to repeat with the same provider identity.
func (s *RepositoryService) resumePrepared(ctx context.Context, object Object, create func(context.Context, Object) error, populate func(context.Context, Object) error) (Object, error) {
	if err := ctx.Err(); err != nil {
		return object, errors.Join(err, core.ErrRecoveryRequired)
	}
	switch object.State {
	case "creating":
		if err := create(ctx, object); err != nil {
			return object, errors.Join(err, core.ErrRecoveryRequired)
		}
		object.State = "created"
		if err := s.save(object); err != nil {
			return object, errors.Join(err, core.ErrRecoveryRequired)
		}
	case "created":
		// Provider creation already has a durable positive receipt.
	default:
		return object, core.ErrIncompatibleState
	}
	if err := s.Backend.InspectVolume(ctx, object); err != nil {
		return object, errors.Join(err, core.ErrRecoveryRequired)
	}
	if populate != nil {
		if err := populate(ctx, object); err != nil {
			return object, errors.Join(err, core.ErrRecoveryRequired)
		}
	}
	object.State = "ready"
	if err := s.save(object); err != nil {
		return object, errors.Join(err, core.ErrRecoveryRequired)
	}
	return object, nil
}

func (s *RepositoryService) Get(kind, id string) (Object, error) {
	if (kind != "repo" && kind != "work") || !gitadapter.ValidID(id) {
		return Object{}, core.ErrInvalidArgument
	}
	object, err := s.readObject(kind, id)
	if err != nil {
		return object, err
	}
	if object.State != "ready" {
		return object, fmt.Errorf("%s %s has incomplete preparation; owned data retained: %w", kind, id, core.ErrRecoveryRequired)
	}
	return object, nil
}

func (s *RepositoryService) Workspace(ctx context.Context, id string) (core.Workspace, error) {
	object, err := s.Get("work", id)
	if err != nil {
		return core.Workspace{}, err
	}
	for _, member := range object.Copies() {
		if err := s.Backend.InspectVolume(ctx, member); err != nil {
			return core.Workspace{}, err
		}
	}
	return core.Workspace{ID: core.WorkspaceID("workspace:managed:" + object.Owner), Path: "managed:" + id}, nil
}

func (s *RepositoryService) path(kind, id string) string {
	return filepath.Join(s.Root, kind+"-"+id+".json")
}
func (s *RepositoryService) reserve(object Object) error {
	if err := os.MkdirAll(s.Root, 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path(object.Kind, object.ID), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return core.ErrAlreadyExists
	}
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(object); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return syncDir(s.Root)
}
func (s *RepositoryService) save(object Object) error {
	return writeRecord(s.path(object.Kind, object.ID), object)
}
func writeRecord(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".record-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := json.NewEncoder(f).Encode(value); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}
func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func randomID() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	return hex.EncodeToString(value[:])
}

func (s *RepositoryService) readObject(kind, id string) (Object, error) {
	var object Object
	content, err := os.ReadFile(s.path(kind, id))
	if os.IsNotExist(err) {
		return object, core.ErrNotFound
	}
	if err != nil {
		return object, err
	}
	if len(content) > 16384 || json.Unmarshal(content, &object) != nil || object.ID != id || object.Kind != kind || !validObject(object) {
		return Object{}, core.ErrIncompatibleState
	}
	return object, nil
}

// Short native IDs preserve long portable repository names without weakening
// member ownership: the fallback is derived from the recorded fresh owner.
func workspaceMemberID(group, repository, owner string) string {
	id := group + "-" + repository
	if !gitadapter.ValidID(id) {
		return "work-" + owner
	}
	return id
}
