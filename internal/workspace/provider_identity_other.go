//go:build !linux

package workspace

import "os"

func stableWorkspaceIdentity(path string, _ os.FileInfo) string {
	return "path:" + path
}
