//go:build !linux

package incus

import "fmt"

func workspaceOwnerIDs(path string) (uint32, uint32, error) {
	return 0, 0, fmt.Errorf("workspace ownership lookup for %q requires Linux", path)
}
