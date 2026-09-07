package core

import (
	"regexp"
	"time"
)

// PersistentResourceRef is an exact, controller-owned attachment identity.
// This PoC allows one additional exclusive RW resource beside the Workspace.
type PersistentResourceRef struct {
	ID    string `json:"id,omitempty"`
	Owner string `json:"owner,omitempty"`
}

type PersistentResource struct {
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	Kind      string    `json:"kind"`
	NativeRef string    `json:"native_ref"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	// CopySource reserves the exact source until a verified copy is committed.
	CopySource PersistentResourceRef `json:"copy_source,omitempty"`
}

func (r PersistentResource) Ref() PersistentResourceRef {
	return PersistentResourceRef{ID: r.ID, Owner: r.Owner}
}

var persistentIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}:[a-z0-9][a-z0-9-]{0,39}$`)
var persistentOwnerPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func ValidPersistentResourceRef(r PersistentResourceRef) bool {
	return persistentIDPattern.MatchString(r.ID) && persistentOwnerPattern.MatchString(r.Owner)
}
