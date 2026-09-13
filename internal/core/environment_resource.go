package core

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxEnvironmentAttachments = 32

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
		if len(a.Target) > 1024 || !utf8.ValidString(a.Target) || !strings.HasPrefix(a.Target, "/") || a.Target == "/" || path.Clean(a.Target) != a.Target || strings.ContainsFunc(a.Target, unicode.IsControl) {
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

func ValidEnvironmentResourceRef(ref PersistentResourceRef) bool {
	return ValidPersistentResourceRef(ref) && strings.HasPrefix(ref.ID, "env-data:") && persistentOwnerPattern.MatchString(strings.TrimPrefix(ref.ID, "env-data:"))
}
