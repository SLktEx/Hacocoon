package environmenttransfer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"strings"
	"testing"
)

func namedDataSnapshot() (core.Snapshot, []snapshotArchive) {
	saved, archives := snapshotFixture(2, true)
	for i, key := range []string{"compiler", "packages"} {
		a := core.EnvironmentAttachment{Key: key, Target: "/root/.cache/" + key, Resource: core.PersistentResourceRef{ID: "env-data:" + strings.Repeat(string(rune('a'+i)), 32), Owner: strings.Repeat(string(rune('c'+i)), 32)}, Origin: core.ResourceGeneration{Name: "source-" + key, Kind: "build-cache", Compatibility: strings.Repeat("a", 64), Epoch: strings.Repeat("e", 32)}}
		saved.Source.Environment.Attachments = append(saved.Source.Environment.Attachments, a)
		c := core.SnapshotComponent{Role: "data:" + key, Binding: "private-data-binding-" + key, NativeRef: "private-native-" + key, Owner: strings.Repeat(string(rune('f'-i)), 32), State: "verified"}
		saved.Components = append(saved.Components, c)
		payload := []byte("uncollected " + key)
		sum := sha256.Sum256(payload)
		archives = append(archives, snapshotArchive{Component: c, Bytes: int64(len(payload)), SHA256: hex.EncodeToString(sum[:]), Data: bytes.NewReader(payload)})
	}
	return saved, archives
}

func TestNamedDataEnvelopeRetainsBytesWithoutSourceAuthority(t *testing.T) {
	saved, archives := namedDataSnapshot()
	var output bytes.Buffer
	if err := writeSnapshot(&output, saved, archives, 1<<20, snapshotWorkspaces(2)); err != nil {
		t.Fatal(err)
	}
	m, err := Inspect(bytes.NewReader(output.Bytes()), 1<<20)
	if err != nil || !m.HasOCI || len(m.Components) != 6 || len(m.Data) != 2 || m.Data[1].Key != "packages" {
		t.Fatal(m, err)
	}
	for _, secret := range []string{"private-data-binding", "private-native", "source-compiler", saved.Source.Environment.Attachments[0].Resource.Owner} {
		if bytes.Contains(output.Bytes(), []byte(secret)) {
			t.Fatal("source ownership serialized")
		}
	}
	for _, mode := range []string{"missing", "extra", "bad-target", "overlap", "bad-kind"} {
		t.Run(mode, func(t *testing.T) {
			s, a := namedDataSnapshot()
			switch mode {
			case "missing":
				a = a[:len(a)-1]
			case "extra":
				s.Source.Environment.Attachments = s.Source.Environment.Attachments[:1]
			case "bad-target":
				s.Source.Environment.Attachments[0].Target = "/root/../etc"
			case "overlap":
				s.Source.Environment.Attachments[1].Target = s.Source.Environment.Attachments[0].Target + "/child"
			case "bad-kind":
				s.Source.Environment.Attachments[1].Origin.Kind = "host-tools"
			}
			var rejected bytes.Buffer
			if err := writeSnapshot(&rejected, s, a, 1<<20, snapshotWorkspaces(2)); !errors.Is(err, ErrInvalidBundle) || rejected.Len() != 0 {
				t.Fatal("incomplete data exported", err)
			}
		})
	}
}
