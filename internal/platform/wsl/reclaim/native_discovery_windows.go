//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/platform/winprocess"
	"github.com/SLktEx/Hacocoon/internal/platform/wsl/launch"
	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
	"github.com/SLktEx/Hacocoon/internal/streamio"
	"golang.org/x/sys/windows"
)

// Only this file's fixed read-only operation builders supply arguments. The
// detached mutation worker is never started through this kill-on-close owner.
// Windows job membership does not cover WSL-service-created Linux processes.
func (r registration) startOwnedDiscovery(ctx context.Context, args []string) (*winprocess.Private, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.revalidate(); err != nil {
		return nil, err
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, err
	}
	root, err := windows.GetWindowsDirectory()
	if err != nil {
		return nil, err
	}
	return winprocess.Start(filepath.Join(system, "wsl.exe"), args,
		[]string{"SystemRoot=" + root, "WINDIR=" + root}, system)
}

func (r registration) readOwnedInstallation(ctx context.Context) (installationIdentity, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	args, err := r.wslArguments(wslReadRegistration)
	if err != nil {
		return installationIdentity{}, err
	}
	process, err := r.startOwnedDiscovery(ctx, args)
	if err != nil {
		return installationIdentity{}, err
	}
	stop := context.AfterFunc(ctx, func() { _ = process.Close() })
	defer stop()
	var output registrationOutput
	readErr := process.Input.Close()
	if readErr == nil {
		_, readErr = io.Copy(&output, process.Output)
	}
	if readErr == nil {
		readErr = process.Wait(ctx)
	}
	// Explicit Close observes all owned Windows descendants, even after root
	// exit or an inherited output handle. Linux-side teardown is not proved.
	if err := finishNativeDiscovery(ctx, readErr, process.Close); err != nil {
		return installationIdentity{}, err
	}
	if output.overflow {
		return installationIdentity{}, errors.New("installation observation exceeded output limit")
	}
	if err := r.revalidate(); err != nil {
		return installationIdentity{}, err
	}
	return decodeInstallation(output.data, r.ID)
}

type ownedDiscoveryConn struct {
	*streamio.FramedConn
	process *winprocess.Private
}

func (c *ownedDiscoveryConn) Close() error {
	return errors.Join(c.FramedConn.Close(), c.process.Close())
}

func (r registration) readOwnedControllerTarget(ctx context.Context, enrolled reclamation.WSLTarget) (reclamation.WSLTarget, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	root, err := windows.GetWindowsDirectory()
	if err != nil {
		return reclamation.WSLTarget{}, err
	}
	plan, err := wsllaunch.RegisteredPlan(enrolled, root)
	if err != nil {
		return reclamation.WSLTarget{}, err
	}
	process, err := r.startOwnedDiscovery(ctx, plan.Args)
	if err != nil {
		return reclamation.WSLTarget{}, err
	}
	stop := context.AfterFunc(ctx, func() { _ = process.Close() })
	defer stop()
	conn := &ownedDiscoveryConn{FramedConn: streamio.NewFramedConn(process.Output, process.Input), process: process}
	used := false
	client, err := controlapi.NewClientWithDialer(func(ctx context.Context) (net.Conn, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if used {
			return nil, errors.New("native target discovery cannot be replayed")
		}
		used = true
		return conn, nil
	})
	var observed reclamation.WSLTarget
	if err == nil {
		observed, err = client.ReclamationTarget(ctx)
	}
	// transport.Client.Call currently discards its deferred Close error. Keep
	// the concrete owner and inspect the cached Windows-job absence ourselves.
	if err := finishNativeDiscovery(ctx, err, conn.Close); err != nil {
		return reclamation.WSLTarget{}, err
	}
	if err := r.revalidate(); err != nil {
		return reclamation.WSLTarget{}, err
	}
	return observed, nil
}
