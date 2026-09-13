package core

import (
	"regexp"
	"strings"
)

// ResourceGeneration selects one complete, immutable managed-data source.
// Compatibility is an opaque digest chosen by the owning Standard component;
// Core neither interprets cache contents nor chooses paths or tool versions.
type ResourceGeneration struct {
	Name          string                `json:"name"`
	Kind          string                `json:"kind"`
	Compatibility string                `json:"compatibility"`
	Epoch         string                `json:"epoch"`
	Number        uint64                `json:"number"`
	Current       PersistentResourceRef `json:"current,omitzero"`
}

var resourceSourceName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
var resourceKind = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var compatibilityDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ValidResourceGenerationSpec(name, kind, compatibility string) bool {
	return resourceSourceName.MatchString(name) && resourceKind.MatchString(kind) && compatibilityDigest.MatchString(compatibility)
}

func ValidGenerationResource(ref PersistentResourceRef) bool {
	return ValidPersistentResourceRef(ref) && strings.HasPrefix(ref.ID, "generation:") && persistentOwnerPattern.MatchString(strings.TrimPrefix(ref.ID, "generation:"))
}

func ValidResourceGeneration(g ResourceGeneration) bool {
	if !ValidResourceGenerationSpec(g.Name, g.Kind, g.Compatibility) || !persistentOwnerPattern.MatchString(g.Epoch) {
		return false
	}
	if g.Number == 0 {
		return g.Current == (PersistentResourceRef{})
	}
	return ValidGenerationResource(g.Current)
}
