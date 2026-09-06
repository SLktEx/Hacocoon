package gitrepo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type Object struct {
	Kind       string   `json:"kind"`
	ID         string   `json:"id"`
	Repository string   `json:"repository"`
	Remote     string   `json:"remote"`
	Branch     string   `json:"branch"`
	NativeRef  string   `json:"native_ref"`
	Owner      string   `json:"owner"`
	State      string   `json:"state"`
	Members    []Object `json:"members,omitempty"`
}

type Backend interface {
	Plan(context.Context, string, string) (string, error)
	CreateVolume(context.Context, Object, *Object) error
	InspectVolume(context.Context, Object) error
	Populate(context.Context, Object) error
	RunGit(context.Context, AgentRequest) (Response, error)
	ConnectGit(context.Context, core.Environment, Object, string) error
}

type RepositoryService struct {
	Root    string
	Backend Backend
	mu      sync.Mutex
}

func NewRepositoryService(root string, backend Backend) *RepositoryService {
	return &RepositoryService{Root: root, Backend: backend}
}

func (s *RepositoryService) Clone(ctx context.Context, id, remote, branch string) (Object, error) {
	if !ValidID(id) || !ValidBranch(branch) || ValidateRemote(remote) != nil {
		return Object{}, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.create(ctx, Object{Kind: "repo", ID: id, Repository: id, Remote: remote, Branch: branch}, nil)
}

func (s *RepositoryService) CopyWorkspace(ctx context.Context, id, repository string) (Object, error) {
	if !ValidID(id) || !ValidID(repository) {
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
	return s.create(ctx, Object{Kind: "work", ID: id, Repository: repository, Remote: repo.Remote, Branch: repo.Branch}, &repo)
}

// CopyWorkspaceSet reserves the entire immutable collection before creating
// member volumes. Members have no independently resolvable state records.
func (s *RepositoryService) CopyWorkspaceSet(ctx context.Context, id string, repositories []string) (Object, error) {
	if !ValidID(id) || len(repositories) < 2 || len(repositories) > 8 {
		return Object{}, core.ErrInvalidArgument
	}
	seen := map[string]bool{}
	for _, repo := range repositories {
		if !ValidID(repo) || !ValidID(id+"-"+repo) || seen[repo] {
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
		ref, err := s.Backend.Plan(ctx, "work", id+"-"+name)
		if err != nil {
			return Object{}, err
		}
		object.Members = append(object.Members, Object{Kind: "work", ID: id + "-" + name, Repository: name, Remote: source.Remote, Branch: source.Branch, NativeRef: ref, Owner: randomID(), State: "creating"})
		sources = append(sources, source)
	}
	if err := s.reserve(object); err != nil {
		return Object{}, err
	}
	for i := range object.Members {
		member := &object.Members[i]
		if err := s.Backend.CreateVolume(ctx, *member, &sources[i]); err != nil {
			return object, errors.Join(err, core.ErrRecoveryRequired)
		}
		member.State = "created"
		if err := s.save(object); err != nil {
			return object, errors.Join(err, core.ErrRecoveryRequired)
		}
		if err := s.Backend.InspectVolume(ctx, *member); err != nil {
			return object, errors.Join(err, core.ErrRecoveryRequired)
		}
		if err := s.Backend.Populate(ctx, *member); err != nil {
			return object, errors.Join(err, core.ErrRecoveryRequired)
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
	if !ValidID(o.ID) || (o.Kind != "work" && o.Kind != "repo") || len(o.Owner) != 32 {
		return false
	}
	if len(o.Members) == 0 {
		return ValidID(o.Repository) && ValidBranch(o.Branch) && ValidateRemote(o.Remote) == nil && o.NativeRef != ""
	}
	if o.Kind != "work" || len(o.Members) < 2 || len(o.Members) > 8 || o.NativeRef != "" || o.Repository != "" || o.Remote != "" || o.Branch != "" {
		return false
	}
	seen := map[string]bool{}
	for _, member := range o.Members {
		if len(member.Members) != 0 || member.Kind != "work" || member.ID != o.ID+"-"+member.Repository || seen[member.Repository] || !validObject(member) || (o.State == "ready" && member.State != "ready") {
			return false
		}
		seen[member.Repository] = true
	}
	return true
}

func (s *RepositoryService) create(ctx context.Context, object Object, source *Object) (Object, error) {
	ref, err := s.Backend.Plan(ctx, object.Kind, object.ID)
	if err != nil {
		return Object{}, err
	}
	object.NativeRef = ref
	object.Owner = randomID()
	object.State = "creating"
	// Reserve exact ownership before touching the provider. An ambiguous
	// create never becomes assumed absence, nor permission to adopt/delete.
	if err := s.reserve(object); err != nil {
		return Object{}, err
	}
	if err := s.Backend.CreateVolume(ctx, object, source); err != nil {
		return object, errors.Join(err, core.ErrRecoveryRequired)
	}
	object.State = "created"
	if err := s.save(object); err != nil {
		return object, errors.Join(err, core.ErrRecoveryRequired)
	}
	if err := s.Backend.InspectVolume(ctx, object); err != nil {
		return object, errors.Join(err, core.ErrRecoveryRequired)
	}
	if err := s.Backend.Populate(ctx, object); err != nil {
		return object, errors.Join(err, core.ErrRecoveryRequired)
	}
	object.State = "ready"
	if err := s.save(object); err != nil {
		return object, errors.Join(err, core.ErrRecoveryRequired)
	}
	return object, nil
}

func (s *RepositoryService) Get(kind, id string) (Object, error) {
	if (kind != "repo" && kind != "work") || !ValidID(id) {
		return Object{}, core.ErrInvalidArgument
	}
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
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
