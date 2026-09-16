//go:build linux

package incus

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
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

func workspaceOwnerIDsMappable(uid, gid uint32) bool {
	return rootSubIDContains("/etc/subuid", uid) && rootSubIDContains("/etc/subgid", gid)
}

func rootSubIDContains(path string, id uint32) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	target := uint64(id)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) != 3 || fields[0] != "root" {
			continue
		}
		start, startErr := strconv.ParseUint(fields[1], 10, 32)
		count, countErr := strconv.ParseUint(fields[2], 10, 32)
		if startErr != nil || countErr != nil || count == 0 || target < start {
			continue
		}
		if target-start < count {
			return true
		}
	}
	return false
}
