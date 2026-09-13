package core

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestStreamTokenCanonicalAndBounded(t *testing.T) {
	target := StreamTarget{Environment: "my-project", Instance: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Workspace: "work", AccessMode: WorkspaceReadWrite, Service: "ssh", Grant: "ssh-one"}
	token, err := EncodeStreamTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeStreamTarget(token)
	if err != nil || got != target {
		t.Fatal(got, err)
	}
	for _, bad := range []string{"", token + "=", token + "\n", strings.Repeat("A", 4097), base64.RawURLEncoding.EncodeToString([]byte(`{"host":"localhost","port":22}`)), base64.RawURLEncoding.EncodeToString([]byte(`null`))} {
		if _, err = DecodeStreamTarget(bad); err == nil {
			t.Fatal("accepted malformed target")
		}
	}
	target.Workspace = WorkspaceID(strings.Repeat("w", 4096))
	if _, err = EncodeStreamTarget(target); err == nil {
		t.Fatal("unusable oversized token")
	}
}
