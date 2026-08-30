//go:build linux

package incus

import (
	"fmt"
	"os"
	"syscall"
)

func workspaceOwnerIDs(path string) (uint32, uint32, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return 0, 0, fmt.Errorf("workspace %q does not expose Unix ownership", path)
	}
	return stat.Uid, stat.Gid, nil
}
