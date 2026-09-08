package core

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
)

var temporaryWorkspacePath = regexp.MustCompile("^temporary:[a-f0-9]{32}$")

// A temporary Workspace lives entirely in its Environment's owned root storage.
// Its identity is independent of the Environment name and cannot name a Host path.
func NewTemporaryWorkspace() (Workspace, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Workspace{}, err
	}
	path := "temporary:" + hex.EncodeToString(nonce[:])
	return Workspace{ID: WorkspaceID("workspace:" + path), Path: path}, nil
}

func ValidTemporaryWorkspace(work Workspace) bool {
	return temporaryWorkspacePath.MatchString(work.Path) && work.ID == WorkspaceID("workspace:"+work.Path)
}

func IsTemporaryWorkspacePath(path string) bool {
	return temporaryWorkspacePath.MatchString(path)
}
