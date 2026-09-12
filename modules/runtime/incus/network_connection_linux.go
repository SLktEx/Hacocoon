//go:build linux

package incus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"golang.org/x/sys/unix"
)

// ExternalNetworkAddress rejects local control addresses and complete managed
// bridge subnets, including currently unused addresses that Incus may reuse.
func (r *Runtime) ExternalNetworkAddress(ctx context.Context, address netip.Addr) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !address.IsValid() || address.Is4In6() || address.Zone() != "" ||
		!address.IsGlobalUnicast() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return core.ErrPolicyDenied
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return core.ErrRuntimeUnavailable
	}
	for _, iface := range interfaces {
		addresses, err := iface.Addrs()
		if err != nil {
			return core.ErrRuntimeUnavailable
		}
		for _, raw := range addresses {
			prefix, err := netip.ParsePrefix(raw.String())
			if err != nil {
				return core.ErrRuntimeUnavailable
			}
			if prefix.Addr().Unmap() == address || ((strings.HasPrefix(iface.Name, "hbr") || strings.HasPrefix(iface.Name, "haco-")) && prefix.Contains(address)) {
				return core.ErrPolicyDenied
			}
		}
	}
	return nil
}

// DialEnvironmentNetwork pins a kernel namespace descriptor, never a reusable
// target IP. A socket remains in that namespace even if the instance is deleted
// and Incus assigns its name/IP to a different generation.
func (r *Runtime) DialEnvironmentNetwork(ctx context.Context, ref, instance, protocol string, port int) (net.Conn, error) {
	if protocol != "tcp" && protocol != "udp" || port < 1 || port > 65535 {
		return nil, core.ErrInvalidArgument
	}
	if err := r.VerifyEnvironmentIdentity(ctx, ref, instance); err != nil {
		return nil, err
	}
	pid, err := r.networkInstancePID(ctx, ref)
	if err != nil {
		return nil, err
	}
	// An open proc directory cannot retarget to a new process after PID reuse.
	proc, err := unix.Open("/proc/"+strconv.Itoa(pid), unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, core.ErrCapabilityStale
	}
	defer unix.Close(proc)
	ns, err := unix.Openat(proc, "ns/net", unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, core.ErrCapabilityStale
	}
	defer unix.Close(ns)
	if err := r.VerifyEnvironmentIdentity(ctx, ref, instance); err != nil {
		return nil, err
	}
	again, err := r.networkInstancePID(ctx, ref)
	if err != nil || again != pid {
		return nil, core.ErrCapabilityStale
	}
	// Verify the process pinned by proc is still present; opening through proc
	// fails after its exit rather than following a replacement process.
	check, err := unix.Openat(proc, "ns/net", unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, core.ErrCapabilityStale
	}
	unix.Close(check)
	return dialNetworkNamespace(ctx, ns, protocol, port)
}
func (r *Runtime) networkInstancePID(ctx context.Context, ref string) (int, error) {
	if err := validateManagedInstanceRef(ref); err != nil || ref == trustedHostName {
		return 0, core.ErrInvalidArgument
	}
	result, err := r.runner.Run(ctx, "incus", "query", "/1.0/instances/"+ref+"/state?project="+r.project)
	if err != nil || result.StdoutTruncated {
		return 0, core.ErrRuntimeUnavailable
	}
	var state struct {
		PID    int    `json:"pid"`
		Status string `json:"status"`
	}
	if json.Unmarshal([]byte(result.Stdout), &state) != nil || state.PID <= 1 || state.Status != "Running" {
		return 0, core.ErrCapabilityStale
	}
	return state.PID, nil
}
func dialNetworkNamespace(ctx context.Context, namespace int, protocol string, port int) (net.Conn, error) {
	type result struct {
		conn net.Conn
		err  error
	}
	done := make(chan result, 1)
	go func() {
		runtime.LockOSThread()
		// Deliberately never unlock: Go destroys this dedicated thread on exit.
		// No thread with a guest namespace may return to the controller pool.
		if err := unix.Setns(namespace, unix.CLONE_NEWNET); err != nil {
			done <- result{err: err}
			return
		}
		kind := unix.SOCK_STREAM
		if protocol == "udp" {
			kind = unix.SOCK_DGRAM
		}
		fd, err := unix.Socket(unix.AF_INET, kind|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, 0)
		if err != nil {
			done <- result{err: err}
			return
		}
		file := os.NewFile(uintptr(fd), "network-connection")
		defer file.Close()
		err = unix.Connect(fd, &unix.SockaddrInet4{Port: port, Addr: [4]byte{127, 0, 0, 1}})
		if errors.Is(err, unix.EINPROGRESS) {
			deadline := time.Now().Add(10 * time.Second)
			for {
				if err = ctx.Err(); err != nil {
					break
				}
				if time.Now().After(deadline) {
					err = context.DeadlineExceeded
					break
				}
				poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLOUT}}
				n, pollErr := unix.Poll(poll, 100)
				if errors.Is(pollErr, unix.EINTR) {
					continue
				}
				if pollErr != nil {
					err = pollErr
					break
				}
				if n == 0 {
					continue
				}
				code, getErr := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
				err = getErr
				if err == nil && code != 0 {
					err = unix.Errno(code)
				}
				break
			}
		}
		if err != nil {
			done <- result{err: err}
			return
		}
		if err = ctx.Err(); err != nil {
			done <- result{err: err}
			return
		}
		conn, err := net.FileConn(file)
		done <- result{conn: conn, err: err}
	}()
	// Waiting keeps namespace alive until the dedicated thread finishes. The
	// connect poll observes context cancellation at most 100 ms later.
	resultValue := <-done
	if resultValue.err != nil {
		return nil, fmt.Errorf("open destination namespace connection: %w", resultValue.err)
	}
	return resultValue.conn, nil
}

// HostNetworkAddress permits explicitly registered loopback services and
// non-managed addresses. A raw Env IP cannot be registered as a Host service.
func (r *Runtime) HostNetworkAddress(ctx context.Context, address netip.Addr) error {
	if address.IsLoopback() {
		return ctx.Err()
	}
	return r.ExternalNetworkAddress(ctx, address)
}

// DialDevelopmentNetwork binds the socket to its observed outgoing device.
// Later installation of a more-specific Env bridge route cannot redirect an
// existing UDP association into that new managed network.
func (r *Runtime) DialDevelopmentNetwork(ctx context.Context, address netip.Addr, protocol string, port int, selectedHost bool) (net.Conn, error) {
	if protocol != "tcp" && protocol != "udp" || port < 1 || port > 65535 {
		return nil, core.ErrInvalidArgument
	}
	validate := r.ExternalNetworkAddress
	if selectedHost {
		validate = r.HostNetworkAddress
	}
	if err := validate(ctx, address); err != nil {
		return nil, err
	}
	result, err := r.runner.Run(ctx, "ip", "-json", "route", "get", address.String())
	if err != nil || result.StdoutTruncated {
		return nil, core.ErrRuntimeUnavailable
	}
	var routes []struct {
		Device string `json:"dev"`
	}
	if json.Unmarshal([]byte(result.Stdout), &routes) != nil || len(routes) != 1 {
		return nil, core.ErrIncompatibleState
	}
	device := routes[0].Device
	iface, err := net.InterfaceByName(device)
	if err != nil || iface.Index <= 0 || strings.HasPrefix(device, sandboxRoutedHostPrefix) || strings.HasPrefix(device, "haco-") ||
		(iface.Flags&net.FlagLoopback != 0 && (!selectedHost || !address.IsLoopback())) {
		return nil, core.ErrPolicyDenied
	}
	dialer := net.Dialer{Timeout: 10 * time.Second, Control: func(_, _ string, raw syscall.RawConn) error {
		current, err := net.InterfaceByName(device)
		if err != nil || current.Index != iface.Index {
			return core.ErrCapabilityStale
		}
		var bindErr error
		err = raw.Control(func(fd uintptr) {
			bindErr = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, device)
		})
		return errors.Join(err, bindErr)
	}}
	return dialer.DialContext(ctx, protocol, net.JoinHostPort(address.String(), strconv.Itoa(port)))
}
