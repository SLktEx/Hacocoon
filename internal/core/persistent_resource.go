package core

import (
	"regexp"
	"time"
)

// PersistentResourceRef is an exact, controller-owned attachment identity.
// Retained data and disposable Env children share exact ownership references.
type PersistentResourceRef struct {
	ID    string `json:"id,omitempty"`
	Owner string `json:"owner,omitempty"`
}

type PersistentResource struct {
	// ImportPending requires supplied archive bytes; never replace them with an
	// ordinary empty create after interruption. Cleared only at publication.
	ImportPending bool `json:"import_pending,omitempty"`
	// PublicationOrigin and Producer are immutable receipts for whole-generation
	// publication recovery and history. They never keep the producer alive after
	// copy completion; CopySource alone is the in-flight reservation.
	PublicationOrigin ResourceGeneration    `json:"publication_origin,omitzero"`
	Producer          PersistentResourceRef `json:"producer,omitzero"`

	// EnvironmentInstance binds disposable data to one canonical Environment creation.
	EnvironmentInstance string `json:"environment_instance,omitempty"`
	// RestoreSource reserves immutable saved data until independent creation completes.
	RestoreSource string      `json:"restore_source,omitempty"`
	SourceOnly    bool        `json:"source_only,omitempty"`
	WorkspaceID   WorkspaceID `json:"workspace_id,omitempty"`
	ID            string      `json:"id"`
	Owner         string      `json:"owner"`
	Kind          string      `json:"kind"`
	NativeRef     string      `json:"native_ref"`
	State         string      `json:"state"`
	CreatedAt     time.Time   `json:"created_at"`
	// CopySource reserves the exact source until a verified copy is committed.
	CopySource PersistentResourceRef `json:"copy_source,omitempty"`
	// CopyCompleted is a durable receipt recorded only after provider completion
	// and verification, before restoring source writers or publishing the copy.
	CopyCompleted bool `json:"copy_completed,omitempty"`
}

func (r PersistentResource) Ref() PersistentResourceRef {
	return PersistentResourceRef{ID: r.ID, Owner: r.Owner}
}

var persistentIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}:[a-z0-9][a-z0-9-]{0,39}$`)
var persistentOwnerPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func ValidPersistentResourceRef(r PersistentResourceRef) bool {
	return persistentIDPattern.MatchString(r.ID) && persistentOwnerPattern.MatchString(r.Owner)
}
