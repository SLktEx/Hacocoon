package incus

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (r *Runtime) ForwardLocalPort(ctx context.Context, ref string, req core.LocalPortRequest) (core.ClientConnection, error) {
	if err := validateManagedInstanceRef(ref); err != nil {
		return core.ClientConnection{}, err
	}
	if req.Protocol != "" && req.Protocol != "tcp" {
		return core.ClientConnection{}, core.ErrUnsupported
	}
	if req.TargetPort < 1 || req.TargetPort > 65535 {
		return core.ClientConnection{}, core.ErrInvalidArgument
	}
	port, err := chooseLoopbackPort(ctx, req.HostPort)
	if err != nil {
		return core.ClientConnection{}, err
	}
	req.HostPort = port
	id := fmt.Sprintf("tcp-%d-%d", req.HostPort, req.TargetPort)
	if err := r.addLoopbackProxy(ctx, ref, id, req.HostPort, req.TargetPort); err != nil {
		return core.ClientConnection{}, err
	}
	return core.ClientConnection{ID: id, Kind: "tcp", Host: "127.0.0.1", Port: req.HostPort, TargetPort: req.TargetPort}, nil
}

func (r *Runtime) RemoveClientConnection(ctx context.Context, ref, connectionID string) error {
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	_, err := r.runner.Run(ctx, "incus", "config", "device", "remove", ref, "haco-"+connectionID, "--project", r.project)
	return err
}

func (r *Runtime) PrepareSSH(ctx context.Context, ref string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	return r.PrepareSSHAccess(ctx, ref, req)
}

func (r *Runtime) addLoopbackProxy(ctx context.Context, ref, id string, hostPort, targetPort int) error {
	_, err := r.runner.Run(ctx, "incus", "config", "device", "add", ref, "haco-"+id, "proxy",
		fmt.Sprintf("listen=tcp:127.0.0.1:%d", hostPort),
		fmt.Sprintf("connect=tcp:127.0.0.1:%d", targetPort),
		"--project", r.project)
	if err != nil {
		return fmt.Errorf("add local proxy %s: %w", id, err)
	}
	return nil
}
