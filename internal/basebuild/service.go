// Package basebuild runs definitions in disposable, canonically owned Environments.
package basebuild

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const MaxScriptBytes = 64 * 1024

var NamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,62}$`)

type Definition struct {
	Name core.BaseName `json:"name"`
	From core.BaseName `json:"from,omitempty"`
	// BuilderName scopes ordinary policy by an operator-selected name. Ownership
	// still uses a fresh temporary Workspace and the canonical Env lease.
	BuilderName string `json:"builder_name,omitempty"`
	// Run is the simple guest shell definition; exactly one engine is selected.
	Run    string          `json:"run,omitempty"`
	Packer *PackerTemplate `json:"packer,omitempty"`
}

func (d Definition) Validate() error {
	if d.BuilderName != "" {
		if err := core.ValidateEnvironmentName(d.BuilderName); err != nil {
			return err
		}
	}
	if !NamePattern.MatchString(string(d.Name)) || d.Name == d.From {
		return core.ErrInvalidArgument
	}
	if d.Packer != nil {
		if d.Run != "" {
			return core.ErrInvalidArgument
		}
		return d.Packer.Validate()
	}
	if strings.TrimSpace(d.Run) == "" || len(d.Run) > MaxScriptBytes || strings.ContainsRune(d.Run, 0) {
		return core.ErrInvalidArgument
	}
	return nil
}

type Environments interface {
	Create(context.Context, core.EnvironmentSpec) (core.Environment, error)
	ExecForWorkspace(context.Context, string, core.WorkspaceID, core.ExecutionRequest) (core.ExecutionResult, error)
	StopForWorkspace(context.Context, string, core.WorkspaceID) error
	DeleteTemporary(context.Context, string, core.Workspace) error
	PublishTemporaryBase(context.Context, string, core.Workspace, core.BaseName) (core.BaseInfo, error)
}
type Execute func(context.Context, core.ExecutionRequest) (core.ExecutionResult, error)
type PackerProvisioner interface {
	Provision(context.Context, PackerTemplate, Execute) error
}
type Service struct {
	Environments Environments
	Packer       PackerProvisioner
}
type Result struct {
	Base    core.BaseInfo `json:"base"`
	Builder string        `json:"builder,omitempty"`
	State   string        `json:"state"`
	Stage   string        `json:"stage,omitempty"`
	// Execution is private build output for the requesting client, never logs.
	Execution *core.ExecutionResult `json:"execution,omitempty"`
}

// Build has no crash-replay catalog. Canonical Env leases and native image
// properties retain exact identities when a process or publication fails.
func (s *Service) Build(ctx context.Context, d Definition) (result Result, err error) {
	if s == nil || s.Environments == nil {
		return result, core.ErrUnsupported
	}
	if err = d.Validate(); err != nil {
		return result, err
	}
	if d.Packer != nil && s.Packer == nil {
		return result, core.ErrUnsupported
	}
	return s.build(ctx, d.Name, d.BuilderName, func(ctx context.Context, name string, work core.Workspace) (core.Environment, error) {
		return s.Environments.Create(ctx, core.EnvironmentSpec{Name: name, Base: d.From, TemporaryWorkspace: &work, SkipDefaultResource: true, Resources: builderResources()})
	}, func(execute Execute, result *Result) error {
		if d.Packer != nil {
			if err = s.Packer.Provision(ctx, *d.Packer, execute); err != nil {
				var failure *ProvisionFailure
				if errors.As(err, &failure) {
					result.Stage = failure.Stage
					result.Execution = &failure.Execution
				}
				return err
			}
		} else {
			execution, runErr := execute(ctx, core.ExecutionRequest{WorkingDirectory: "/", Argv: []string{"/bin/sh", "-eu", "-s"}, Stdin: []byte(d.Run)})
			if runErr != nil || execution.ExitCode != 0 {
				return fmt.Errorf("base build script failed (exit %d): %w", execution.ExitCode, core.ErrRuntimeUnavailable)
			}
		}
		return nil
	})
}

// build owns the single execution/publication/cleanup sequence for definitions
// and archive inputs. The creator always uses canonical Environment ownership.
func (s *Service) build(ctx context.Context, base core.BaseName, name string, create func(context.Context, string, core.Workspace) (core.Environment, error), provision func(Execute, *Result) error) (result Result, err error) {
	work, err := core.NewTemporaryWorkspace()
	if err != nil {
		return result, err
	}
	if name == "" {
		var nonce [16]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			return result, err
		}
		name = "build-" + hex.EncodeToString(nonce[:])
	}
	result = Result{Base: core.BaseInfo{Name: base}, Builder: name, State: "failed"}
	env, err := create(ctx, name, work)
	if err != nil {
		return result, err
	}
	// Cleanup verifies the unique temporary Workspace under the lifecycle lock.
	// Publishing uncertainty keeps the builder so native operation evidence remains.
	preserve := false
	defer func() {
		if preserve {
			return
		}
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		e := s.Environments.DeleteTemporary(cleanup, env.Name, work)
		if e != nil {
			result.State = "cleanup-required"
			err = errors.Join(err, e, core.ErrRecoveryRequired)
		} else {
			result.Builder = ""
		}
	}()
	execute := func(ctx context.Context, request core.ExecutionRequest) (core.ExecutionResult, error) {
		return s.Environments.ExecForWorkspace(ctx, env.Name, work.ID, request)
	}
	if provision != nil {
		if err = provision(execute, &result); err != nil {
			return result, err
		}
	}
	// Instance-local cleanup runs only inside the untrusted guest. Never execute
	// definition text on the Host or include captured guest output in diagnostics.
	execution, err := execute(ctx, core.ExecutionRequest{WorkingDirectory: "/", Argv: []string{"/bin/sh", "-eu", "-c", cleanInstance}})
	if err != nil || execution.ExitCode != 0 {
		return result, fmt.Errorf("Base instance cleanup failed: %w", core.ErrRuntimeUnavailable)
	}
	if err = s.Environments.StopForWorkspace(ctx, env.Name, work.ID); err != nil {
		return result, err
	}
	result.Base, err = s.Environments.PublishTemporaryBase(ctx, env.Name, work, base)
	if err != nil {
		preserve = true
		result.State = "publication-unconfirmed"
		return result, err
	}
	result.State = "ready"
	return result, nil
}

// Guest paths may be hostile; these operations have only guest authority. Do not
// claim sanitization of arbitrary user secrets: the definition owns its contents.
const cleanInstance = `rm -f /etc/ssh/ssh_host_* /var/lib/dbus/machine-id
rm -rf /root/.ssh /home/*/.ssh /run/hacocoon /var/lib/hacocoon /workspace
mkdir -p /workspace
sync`

// Finite defaults apply to every builder before the untrusted rootfs starts.
func builderResources() core.ResourceBudget {
	finite := func(value uint64) core.ResourceLimit {
		return core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: value}
	}
	return core.ResourceBudget{CPU: finite(2), MemoryBytes: finite(4 << 30), PIDs: finite(1024), RootBytes: finite(64 << 30)}
}
