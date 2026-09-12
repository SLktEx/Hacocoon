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
	if req.Protocol == "" {
		req.Protocol = "tcp"
	}
	if req.Protocol != "tcp" && req.Protocol != "udp" {
		return core.ClientConnection{}, core.ErrUnsupported
	}
	if req.TargetPort < 1 || req.TargetPort > 65535 {
		return core.ClientConnection{}, core.ErrInvalidArgument
	}
	port, err := chooseLoopbackProtocolPort(ctx, req.Protocol, req.HostPort)
	if err != nil {
		return core.ClientConnection{}, err
	}
	req.HostPort = port
	id := fmt.Sprintf("%s-%d-%d", req.Protocol, req.HostPort, req.TargetPort)
	if err := r.addLoopbackProtocolProxy(ctx, ref, id, req.Protocol, req.HostPort, req.TargetPort); err != nil {
		return core.ClientConnection{}, err
	}
	return core.ClientConnection{ID: id, Kind: req.Protocol, Host: "127.0.0.1", Port: req.HostPort, TargetPort: req.TargetPort}, nil
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
	return r.addLoopbackProtocolProxy(ctx, ref, id, "tcp", hostPort, targetPort)
}
func (r *Runtime) addLoopbackProtocolProxy(ctx context.Context, ref, id, protocol string, hostPort, targetPort int) error {
	if protocol != "tcp" && protocol != "udp" {
		return core.ErrInvalidArgument
	}
	_, err := r.runner.Run(ctx, "incus", "config", "device", "add", ref, "haco-"+id, "proxy",
		fmt.Sprintf("listen=%s:127.0.0.1:%d", protocol, hostPort),
		fmt.Sprintf("connect=%s:127.0.0.1:%d", protocol, targetPort),
		"--project", r.project)
	if err != nil {
		return fmt.Errorf("add local proxy %s: %w", id, err)
	}
	return nil
}
