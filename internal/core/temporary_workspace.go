package core

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
)

var temporaryWorkspacePath = regexp.MustCompile("^temporary:[a-f0-9]{32}$")

// A temporary Workspace lives entirely in its Environment's owned root storage.
// Its identity is independent of the Environment name and cannot name a Host path.
func NewTemporaryWorkspace() Workspace {
	var nonce [16]byte
	_, _ = rand.Read(nonce[:])
	path := "temporary:" + hex.EncodeToString(nonce[:])
	return Workspace{ID: WorkspaceID("workspace:" + path), Path: path}
}

func ValidTemporaryWorkspace(work Workspace) bool {
	return temporaryWorkspacePath.MatchString(work.Path) && work.ID == WorkspaceID("workspace:"+work.Path)
}

func IsTemporaryWorkspacePath(path string) bool {
	return temporaryWorkspacePath.MatchString(path)
}
