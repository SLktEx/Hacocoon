package core

import (
	"io"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxEnvironmentAttachments = 32

// EnvironmentResourceImport contains portable data, never source ownership.
// The lifecycle owner creates fresh local identities and retains the input only
// for this invocation. Provider placement restrictions still apply.
type EnvironmentResourceImport struct {
	Key, Target, Kind, Digest string
	Archive                   io.ReadSeeker
}

// EnvironmentAttachment is an immutable placement and source-generation receipt.
// Its resource is independently owned, writable and disposable with this Env.
// Standard chooses paths and compatibility; Core stores their exact identities.
type EnvironmentAttachment struct {
	Key      string                `json:"key"`
	Target   string                `json:"target"`
	Resource PersistentResourceRef `json:"resource"`
	Origin   ResourceGeneration    `json:"origin"`
}

type EnvironmentResourcePlan struct {
	Attachment EnvironmentAttachment
	Resource   PersistentResource
}

type EnvironmentResourceRequest struct {
	EnvironmentID string
	InstanceID    string
	Workspace     Workspace
	Base          BaseName
	ReadOnly      bool
}

type EnvironmentResourceSelection struct {
	Key    string
	Target string
	Origin ResourceGeneration
}

type EnvironmentRuntimeAttachment struct {
	Attachment EnvironmentAttachment
	Resource   PersistentResource
}

// EnvironmentResourceBinding accompanies placement and resume under the same
// canonical Workspace lease. Providers resolve its storage identity; callers
// never reconstruct that identity from guest or native device descriptions.
type EnvironmentResourceBinding struct {
	InstanceID    string
	WorkspacePath string
	ReadOnly      bool
	Attachments   []EnvironmentRuntimeAttachment
}

func (s EnvironmentRuntimeSpec) ResourceBinding() EnvironmentResourceBinding {
	return EnvironmentResourceBinding{InstanceID: s.InstanceID, WorkspacePath: s.WorkspacePath, ReadOnly: s.ReadOnly, Attachments: s.Attachments}
}

func ValidEnvironmentAttachments(attachments []EnvironmentAttachment) bool {
	if len(attachments) > MaxEnvironmentAttachments {
		return false
	}
	previous := ""
	refs := map[string]bool{}
	for i, a := range attachments {
		if !resourceSourceName.MatchString(a.Key) || a.Key <= previous || !ValidEnvironmentResourceRef(a.Resource) || refs[a.Resource.ID] || !ValidResourceGeneration(a.Origin) {
			return false
		}
		if !ValidEnvironmentDataPath(a.Target) {
			return false
		}
		for _, before := range attachments[:i] {
			if before.Target == a.Target || strings.HasPrefix(before.Target, a.Target+"/") || strings.HasPrefix(a.Target, before.Target+"/") {
				return false
			}
		}
		refs[a.Resource.ID] = true
		previous = a.Key
	}
	return true
}

// ValidEnvironmentDataPath checks portable syntax only; providers additionally
// exclude protected locations and inspect every ancestor without following links.
func ValidEnvironmentDataPath(target string) bool {
	return len(target) <= 1024 && utf8.ValidString(target) && strings.HasPrefix(target, "/") && target != "/" && path.Clean(target) == target && !strings.ContainsFunc(target, unicode.IsControl)
}

func ValidEnvironmentResourceRef(ref PersistentResourceRef) bool {
	return ValidPersistentResourceRef(ref) && strings.HasPrefix(ref.ID, "env-data:") && persistentOwnerPattern.MatchString(strings.TrimPrefix(ref.ID, "env-data:"))
}
