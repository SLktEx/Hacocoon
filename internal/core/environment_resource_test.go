package core

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func validEnvironmentAttachment() EnvironmentAttachment {
	return EnvironmentAttachment{Key: "compiler", Target: "/home/dev/.cache/compiler", Resource: PersistentResourceRef{ID: "env-data:" + strings.Repeat("a", 32), Owner: strings.Repeat("b", 32)}, Origin: ResourceGeneration{Name: "compiler", Kind: "build-cache", Compatibility: strings.Repeat("c", 64), Epoch: strings.Repeat("d", 32)}}
}
func TestEnvironmentAttachmentPathsAndIdentity(t *testing.T) {
	original := validEnvironmentAttachment()
	if !ValidEnvironmentAttachments([]EnvironmentAttachment{original}) {
		t.Fatal("valid rejected")
	}
	for _, target := range []string{"", "/", "relative", "/home/../etc", "//home/dev", "/home/dev/", "/home/dev/./cache", "/home/dev/cache\x00", "/home/dev/cache\n", strings.Repeat("/a", 513), string([]byte{'/', 0xff})} {
		a := original
		a.Target = target
		if ValidEnvironmentAttachments([]EnvironmentAttachment{a}) {
			t.Errorf("unsafe path %q", target)
		}
	}
	for _, fault := range []string{"foreign-id", "wrong-owner", "key", "epoch"} {
		a := original
		switch fault {
		case "foreign-id":
			a.Resource.ID = "oci:retained"
		case "wrong-owner":
			a.Resource.Owner = "bad"
		case "key":
			a.Key = "-option"
		case "epoch":
			a.Origin.Epoch = "bad"
		}
		if ValidEnvironmentAttachments([]EnvironmentAttachment{a}) {
			t.Fatal(fault)
		}
	}
	for _, target := range []string{original.Target, original.Target + "/child", "/home/dev/.cache"} {
		b := original
		b.Key = "second"
		b.Resource.ID = "env-data:" + strings.Repeat("f", 32)
		b.Target = target
		if ValidEnvironmentAttachments([]EnvironmentAttachment{original, b}) {
			t.Fatal("overlapping path", target)
		}
	}
}
func TestEnvironmentAttachmentLimitAndAggregateEquality(t *testing.T) {
	a := validEnvironmentAttachment()
	areas := make([]EnvironmentAttachment, MaxEnvironmentAttachments)
	for i := range areas {
		areas[i] = a
		areas[i].Key = fmt.Sprintf("cache-%02d", i)
		areas[i].Target = "/cache/" + areas[i].Key
		areas[i].Resource.ID = fmt.Sprintf("env-data:%032x", i+1)
	}
	if !ValidEnvironmentAttachments(areas) {
		t.Fatal("bounded plan rejected")
	}
	if ValidEnvironmentAttachments(append(slices.Clone(areas), a)) {
		t.Fatal("oversized plan")
	}
	env := Environment{Attachments: []EnvironmentAttachment{a}, Base: &BaseRef{Name: "one", Revision: "revision"}}
	other := env
	other.Attachments = slices.Clone(env.Attachments)
	other.Base = &BaseRef{Name: "one", Revision: "revision"}
	if !env.Equal(other) {
		t.Fatal("independently decoded values differ")
	}
	other.Attachments[0].Target = "/changed"
	if env.Equal(other) {
		t.Fatal("attachment mutation ignored")
	}
	if !(Environment{}).Equal(Environment{Attachments: []EnvironmentAttachment{}}) {
		t.Fatal("absent and empty differ")
	}
}
