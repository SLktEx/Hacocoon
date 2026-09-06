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
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Remote     string `json:"remote"`
	Branch     string `json:"branch"`
	NativeRef  string `json:"native_ref"`
	Owner      string `json:"owner"`
	State      string `json:"state"`
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
	if len(content) > 16384 || json.Unmarshal(content, &object) != nil || object.ID != id || object.Kind != kind || !ValidID(object.Repository) || !ValidBranch(object.Branch) || ValidateRemote(object.Remote) != nil || len(object.Owner) != 32 || object.NativeRef == "" {
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
	if err := s.Backend.InspectVolume(ctx, object); err != nil {
		return core.Workspace{}, err
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
