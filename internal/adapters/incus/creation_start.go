package incus

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net/url"
	"strings"
)

// Guest preparation belongs to start, so create does not boot even briefly.
// A guest-local generation receipt makes SSH initialization repeatable after a
// failed start without rotating an established identity on ordinary resume.
func (p *SandboxProvider) initializeCreatedEnvironment(ctx context.Context, ref string) error {
	result, err := p.runner.Run(ctx, "incus", "config", "get", ref, "user.hacocoon.creation", "--project", p.project)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return errors.Join(err, core.ErrIncompatibleState)
	}
	if strings.TrimSpace(result.Stdout) == "" {
		return nil
	}
	if strings.TrimSpace(result.Stdout) != "stopped" {
		return core.ErrIncompatibleState
	}
	result, err = p.runner.Run(ctx, "incus", "config", "get", ref, environmentInstanceKey, "--project", p.project)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return errors.Join(err, core.ErrIncompatibleState)
	}
	identity := strings.TrimSpace(result.Stdout)
	if !core.ValidEnvironmentInstanceID(identity) {
		return core.ErrIncompatibleState
	}
	var observed struct {
		Name    string                       `json:"name"`
		Devices map[string]map[string]string `json:"devices"`
	}
	reader := &BaseProvider{Runtime: p.Runtime}
	if err := reader.baseQuery(ctx, "GET", "/1.0/instances/"+url.PathEscape(ref)+"?project="+url.QueryEscape(p.project), nil, &observed); err != nil {
		return err
	}
	if observed.Name != ref || observed.Devices == nil {
		return core.ErrIncompatibleState
	}

	initialization := freshGuestSSHIdentity
	if device, ok := observed.Devices["persistent-resource"]; ok {
		if device["type"] != "disk" || device["path"] != OCIStorePath {
			return core.ErrIncompatibleState
		}
		initialization += "\n" + persistentOCIConfiguration
	}
	// Publish one generation receipt only after all initial guest configuration
	// succeeds. Ordinary reopen never rewrites the user's OCI configuration.
	script := `set -eu
 test ! -L /etc/hacocoon-instance
 if test ! -f /etc/hacocoon-instance || test "$(cat /etc/hacocoon-instance)" != "$1"; then
 ` + initialization + `
 printf '%s\n' "$1" > /etc/hacocoon-instance
 chmod 600 /etc/hacocoon-instance
 fi
 test -w /workspace
 `
	result, err = p.runner.Run(ctx, "incus", "exec", ref, "--project", p.project, "--", "/bin/sh", "-ec", script, "haco-initialize", identity)
	if err != nil || result.ExitCode != 0 {
		return errors.Join(err, core.ErrIncompatibleState)
	}
	return nil
}
