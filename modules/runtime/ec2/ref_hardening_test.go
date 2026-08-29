package ec2

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestDecodeRefRejectsAuthorityShapingInputs(t *testing.T) {
	valid := runtimeRef{
		Version:       1,
		InstanceID:    "i-0123456789abcdef0",
		WorkspacePath: "/srv/hacocoon/workspaces/demo",
		Bucket:        "hacocoon-workspaces-example",
		Prefix:        "tests/demo",
		ReadOnly:      false,
	}

	cases := map[string]runtimeRef{
		"invalid instance flag": func() runtimeRef { r := valid; r.InstanceID = "--all"; return r }(),
		"invalid instance shape": func() runtimeRef { r := valid; r.InstanceID = "i-nothex"; return r }(),
		"relative workspace": func() runtimeRef { r := valid; r.WorkspacePath = "../../victim"; return r }(),
		"noncanonical workspace": func() runtimeRef { r := valid; r.WorkspacePath = "/srv/hacocoon/../victim"; return r }(),
		"root workspace": func() runtimeRef { r := valid; r.WorkspacePath = "/"; return r }(),
		"workspace control byte": func() runtimeRef { r := valid; r.WorkspacePath = "/srv/demo\nother"; return r }(),
		"invalid bucket": func() runtimeRef { r := valid; r.Bucket = "Bad_Bucket"; return r }(),
		"absolute prefix": func() runtimeRef { r := valid; r.Prefix = "/tests/demo"; return r }(),
		"parent prefix": func() runtimeRef { r := valid; r.Prefix = "tests/../victim"; return r }(),
		"empty prefix segment": func() runtimeRef { r := valid; r.Prefix = "tests//demo"; return r }(),
		"prefix control byte": func() runtimeRef { r := valid; r.Prefix = "tests/demo\x00victim"; return r }(),
	}

	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			raw, err := encodeRef(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeRef(raw); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatalf("decodeRef(%q) err=%v, want ErrIncompatibleState", name, err)
			}
		})
	}
}

func TestDecodeRefRejectsUnknownAndTrailingJSON(t *testing.T) {
	payloads := []string{
		`{"version":1,"instance_id":"i-0123456789abcdef0","workspace_path":"/srv/demo","bucket":"hacocoon-workspaces-example","prefix":"tests/demo","read_only":false,"workspace_override":"/victim"}`,
		`{"version":1,"instance_id":"i-0123456789abcdef0","workspace_path":"/srv/demo","bucket":"hacocoon-workspaces-example","prefix":"tests/demo","read_only":false} {}`,
	}
	for _, payload := range payloads {
		raw := "ec2v1." + base64.RawURLEncoding.EncodeToString([]byte(payload))
		if _, err := decodeRef(raw); !errors.Is(err, core.ErrIncompatibleState) {
			t.Fatalf("decodeRef(%q) err=%v, want ErrIncompatibleState", payload, err)
		}
	}
}

func TestDecodeRefRejectsOversizedPayload(t *testing.T) {
	raw := "ec2v1." + strings.Repeat("A", maxRuntimeRefLength)
	if _, err := decodeRef(raw); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v, want ErrIncompatibleState", err)
	}
}

func TestDecodeRefAcceptsCanonicalRef(t *testing.T) {
	raw, err := encodeRef(runtimeRef{
		InstanceID:    "i-0123456789abcdef0",
		WorkspacePath: "/srv/hacocoon/workspaces/demo",
		Bucket:        "hacocoon-workspaces-example",
		Prefix:        "tests/demo",
		ReadOnly:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := decodeRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ref.InstanceID != "i-0123456789abcdef0" || ref.WorkspacePath != "/srv/hacocoon/workspaces/demo" || !ref.ReadOnly {
		t.Fatalf("ref=%#v", ref)
	}
}
