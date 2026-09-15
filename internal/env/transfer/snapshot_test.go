package environmenttransfer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func snapshotFixture(count int, oci bool) (core.Snapshot, []SnapshotArchive) {
	s := core.Snapshot{ID: "snap-" + strings.Repeat("a", 32), State: "ready", Source: core.SnapshotSource{InstanceID: "env-" + strings.Repeat("b", 32), Environment: core.Environment{Name: "dev"}}}
	roles := []string{"rootfs"}
	for i := 0; i < count; i++ {
		roles = append(roles, fmt.Sprintf("workspace:repo-%03d", i))
	}
	if oci {
		roles = append(roles, "oci")
		s.Source.Environment.PersistentResource = core.PersistentResourceRef{ID: "oci:dev", Owner: strings.Repeat("c", 32)}
	}
	var archives []SnapshotArchive
	for _, role := range roles {
		c := core.SnapshotComponent{Role: role, Binding: "binding-" + role, NativeRef: "native-" + role, Owner: "owner-" + role, State: "verified"}
		s.Components = append(s.Components, c)
		payload := []byte("archive " + role)
		h := sha256.Sum256(payload)
		archives = append(archives, SnapshotArchive{Component: c, Bytes: int64(len(payload)), SHA256: hex.EncodeToString(h[:]), Data: bytes.NewReader(payload)})
	}
	return s, archives
}
func TestWriteSnapshotPreservesEveryWorkspace(t *testing.T) {
	for _, count := range []int{1, 2, maxWorkspaces} {
		for _, oci := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d-%v", count, oci), func(t *testing.T) {
				saved, archives := snapshotFixture(count, oci)
				// Neither backend completion order nor catalog ordering defines transport order.
				for i, j := 0, len(archives)-1; i < j; i, j = i+1, j-1 {
					archives[i], archives[j] = archives[j], archives[i]
					saved.Components[i], saved.Components[j] = saved.Components[j], saved.Components[i]
				}
				var out bytes.Buffer
				if err := WriteSnapshot(&out, saved, archives, 1<<20); err != nil {
					t.Fatal(err)
				}
				m, err := Inspect(bytes.NewReader(out.Bytes()), 1<<20)
				if err != nil || len(m.Components) != len(archives) {
					t.Fatal(m, err)
				}
				for i := 0; i < count; i++ {
					if m.Components[i+1].Role != workspaceRole(i) {
						t.Fatal(m)
					}
				}
				// Source management binding and native resource names must not reach metadata.
				if bytes.Contains(out.Bytes(), []byte("binding-")) || bytes.Contains(out.Bytes(), []byte("native-")) {
					t.Fatal("source authority serialized")
				}
			})
		}
	}
}
func TestWriteSnapshotRejectsInventoryMismatchBeforeWriting(t *testing.T) {
	for _, change := range []string{"missing-work", "missing-oci", "extra", "owner", "binding", "duplicate", "incomplete", "missing-source-oci", "duplicate-role", "unsupported-role", "too-many-workspaces"} {
		t.Run(change, func(t *testing.T) {
			s, a := snapshotFixture(2, true)
			switch change {
			case "missing-work":
				a = append(a[:1], a[2:]...)
			case "missing-oci":
				a = a[:len(a)-1]
			case "extra":
				a = append(a, a[0])
			case "owner":
				a[1].Component.Owner = "other"
			case "binding":
				a[1].Component.Binding = "other"
			case "duplicate":
				a[1] = a[0]
			case "incomplete":
				s.State = "capturing"
			case "missing-source-oci":
				s.Source.Environment.PersistentResource = core.PersistentResourceRef{}
			case "duplicate-role":
				s.Components[2].Role = s.Components[1].Role
			case "unsupported-role":
				s.Components[2].Role = "other-data"
			case "too-many-workspaces":
				s, a = snapshotFixture(maxWorkspaces+1, true)
			}
			var out bytes.Buffer
			if err := WriteSnapshot(&out, s, a, 1<<20); !errors.Is(err, ErrInvalidBundle) {
				t.Fatal(err)
			}
			if out.Len() != 0 {
				t.Fatal("incomplete inventory emitted bytes")
			}
		})
	}
}
func TestWriteSnapshotDoesNotRequireLegacyBaseFilesystem(t *testing.T) {
	s, a := snapshotFixture(1, false)
	s.Components = append(s.Components, core.SnapshotComponent{Role: "base", Binding: "legacy-base-binding", NativeRef: "legacy-base-native", Owner: "legacy-base-owner", State: "verified"})
	before := append([]core.SnapshotComponent(nil), s.Components...)
	var out bytes.Buffer
	if err := WriteSnapshot(&out, s, a, 1024); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Components, before) {
		t.Fatal("legacy catalog changed")
	}
	m, err := Inspect(bytes.NewReader(out.Bytes()), 1024)
	if err != nil || len(m.Components) != 2 {
		t.Fatal(m, err)
	}
	for _, c := range m.Components {
		if c.Role == "base" {
			t.Fatal("Base filesystem reintroduced")
		}
	}
}
func TestWriteSnapshotPropagatesComponentReadFailure(t *testing.T) {
	s, a := snapshotFixture(2, true)
	a[2].Data = bytes.NewReader(nil)
	if err := WriteSnapshot(io.Discard, s, a, 1<<20); err == nil {
		t.Fatal("incomplete native archive succeeded")
	}
}
