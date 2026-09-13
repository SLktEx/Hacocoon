package incus

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strconv"
	"strings"
)

// MigrateSSHAccess is removal-only migration for pre-ProxyCommand installations.
// It never recreates a proxy or reserves a port. Remove it after those releases
// are no longer supported; new grant/session code must not call proxy creation.
func (r *Runtime) MigrateSSHAccess(ctx context.Context, ref string) error {
	connections, err := r.ListClientConnections(ctx, ref)
	if err != nil {
		return err
	}
	for _, c := range connections {
		if c.Kind == "ssh" && c.Port != 0 {
			if err = r.revokeLegacySSHProxy(ctx, ref, c.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
func (r *Runtime) revokeLegacySSHProxy(ctx context.Context, ref, id string) error {
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	if !strings.HasPrefix(id, "ssh-") {
		return core.ErrInvalidArgument
	}
	port, err := strconv.Atoi(strings.TrimPrefix(id, "ssh-"))
	if err != nil || port < 1 || port > 65535 || id != fmt.Sprintf("ssh-%d", port) {
		return core.ErrInvalidArgument
	}
	connections, err := r.ListClientConnections(ctx, ref)
	if err != nil {
		return err
	}
	found := false
	for _, c := range connections {
		if c.ID == id && c.Kind == "ssh" && c.Port == port && c.TargetPort == 22 {
			found = true
		}
	}
	if !found {
		return core.ErrNotFound
	}
	if _, err = r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project, "--", "sh", "-ceu", managedSSHRevokeScript, "haco-ssh-revoke", "haco:"+id); err != nil {
		return err
	}
	return r.RemoveClientConnection(ctx, ref, id)
}
