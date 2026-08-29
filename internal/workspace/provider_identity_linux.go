//go:build linux

package workspace

import (
	"fmt"
	"os"
	"syscall"
)

func stableWorkspaceIdentity(path string, info os.FileInfo) string {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("linux-fs:%d:%d", uint64(stat.Dev), stat.Ino)
	}
	return "path:" + path
}
