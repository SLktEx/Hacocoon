//go:build !linux

package ec2

import (
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type workspaceRestoreLock struct{}

func acquireWorkspaceRestoreLock(string) (*workspaceRestoreLock, error) {
	return nil, fmt.Errorf("crash-safe workspace restore requires Linux filesystem primitives: %w", core.ErrUnsupported)
}
func (*workspaceRestoreLock) Release() error            { return nil }
func openOwnedRegularFile(string) (*os.File, error)     { return nil, core.ErrUnsupported }
func createExclusiveOwnedFile(string) (*os.File, error) { return nil, core.ErrUnsupported }
func identifyWorkspaceDirectory(string) (workspaceFileIdentity, error) {
	return workspaceFileIdentity{}, core.ErrUnsupported
}
func identifyOwnedDirectory(string, os.FileMode) (workspaceFileIdentity, error) {
	return workspaceFileIdentity{}, core.ErrUnsupported
}
func renameWorkspaceNoReplace(string, string) error { return core.ErrUnsupported }
