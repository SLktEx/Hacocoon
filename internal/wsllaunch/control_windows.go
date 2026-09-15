package wsllaunch

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/reclamation"
	"github.com/SLktEx/Hacocoon/internal/streamio"
)

// ControlDialer always invokes the fixed installed product entry in one WSL.
// The caller's ordinary Windows/WSL identity is retained, without -u root.
func ControlDialer(distribution string) (func(context.Context) (net.Conn, error), error) {
	plan, err := Plan(distribution, os.Getenv("SystemRoot"), ControlStdio)
	return controlDialer(plan, err)
}

func RegisteredControlDialer(target reclamation.WSLTarget) (func(context.Context) (net.Conn, error), error) {
	plan, err := RegisteredPlan(target, os.Getenv("SystemRoot"))
	return controlDialer(plan, err)
}

func controlDialer(plan Invocation, err error) (func(context.Context) (net.Conn, error), error) {
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) (net.Conn, error) {
		cmd := exec.Command(plan.File, plan.Args...)
		cmd.Env = append([]string(nil), plan.Env...)
		cmd.Dir = filepath.Dir(plan.File)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		return streamio.StartFramedProcess(ctx, cmd)
	}, nil
}
