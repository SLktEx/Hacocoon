//go:build linux

package incus

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"gopkg.in/yaml.v2"
)

const importIndexFixture = `name: saved
backend: btrfs
pool: old-pool
optimized: false
optimized_header: false
type: custom
config:
  volume:
    config:
      user.hacocoon.owner: old-owner
      user.hacocoon.snapshot-source: old-source
      security.unmapped: "true"
      volatile.idmap.last: '[{"Isuid":true,"Isgid":false,"Hostid":1000000,"Nsid":0,"Maprange":65536},{"Isuid":false,"Isgid":true,"Hostid":1000000,"Nsid":0,"Maprange":65536}]'
      volatile.idmap.next: '[{"Isuid":true,"Isgid":false,"Hostid":1000000,"Nsid":0,"Maprange":65536},{"Isuid":false,"Isgid":true,"Hostid":1000000,"Nsid":0,"Maprange":65536}]'
    name: saved
    type: custom
    content_type: filesystem
    project: old-project
    used_by: []
    created_at: 2026-09-01T00:00:00Z
`

func importVolumeFixture(t *testing.T, index string, extra ...*tar.Header) []byte {
	t.Helper()
	var out bytes.Buffer
	w := tar.NewWriter(&out)
	all := []*tar.Header{{Name: "backup/index.yaml", Typeflag: tar.TypeReg, Mode: 0600, Size: int64(len(index))}, {Name: "backup/volume", Typeflag: tar.TypeDir, Mode: 0700}, {Name: "backup/volume/data", Typeflag: tar.TypeReg, Mode: 0600, Size: 4}, {Name: "backup/volume/hardlink", Typeflag: tar.TypeLink, Linkname: "backup/volume/data"}, {Name: "backup/volume/symlink", Typeflag: tar.TypeSymlink, Linkname: "/guest/path"}}
	all = append(all, extra...)
	for i, h := range all {
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			io.WriteString(w, index)
		} else if i == 2 {
			io.WriteString(w, "data")
		} else if h.Size > 0 {
			w.Write(make([]byte, h.Size))
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func TestPrepareVolumeImportReplacesAuthorityPreservesDataAndIDMap(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	original := importVolumeFixture(t, importIndexFixture)
	fresh := map[string]string{"user.hacocoon.owner": strings.Repeat("a", 32), "user.hacocoon.resource": "oci:new", "user.hacocoon.kind": OCIStoreKind, "user.hacocoon.source-only": "false"}
	archive, err := prepareVolumeImport(context.Background(), bytes.NewReader(original), root, 1<<20, "new-project", "new-pool", "haco-persistent-"+fresh["user.hacocoon.owner"], fresh)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	tr := tar.NewReader(archive.Reader())
	h, err := tr.Next()
	if err != nil || h.Name != "backup/index.yaml" {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(tr)
	if err != nil {
		t.Fatal(err)
	}
	var got volumeImportIndex
	if err := yaml.UnmarshalStrict(raw, &got); err != nil {
		t.Fatal(err)
	}
	v := got.Config.Volume
	if got.Pool != "new-pool" || v.Project != "new-project" || got.Name != v.Name || v.Config["user.hacocoon.owner"] != fresh["user.hacocoon.owner"] || v.Config["user.hacocoon.resource"] != "oci:new" || len(v.Config) != 6 || v.Config["volatile.idmap.last"] == "" || v.CreatedAt == "2026-09-01T00:00:00Z" {
		t.Fatal("incorrect fresh metadata", got)
	}
	if bytes.Contains(raw, []byte("old-owner")) || bytes.Contains(raw, []byte("old-source")) || bytes.Contains(raw, []byte("security.unmapped")) {
		t.Fatal("old authority copied")
	}
	found := map[string]*tar.Header{}
	for {
		h, err = tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		found[h.Name] = h
		if h.Name == "backup/volume/data" {
			b, e := io.ReadAll(tr)
			if e != nil || string(b) != "data" || h.Mode != 0600 {
				t.Fatal("data or mode lost", e)
			}
		}
	}
	if found["backup/volume/hardlink"].Linkname != "backup/volume/data" || found["backup/volume/symlink"].Linkname != "/guest/path" {
		t.Fatal("links changed")
	}
	if !bytes.Equal(original, importVolumeFixture(t, importIndexFixture)) {
		t.Fatal("source changed")
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("named import staging", err)
	}
}
func TestPrepareVolumeImportRejectsUnsafeOrIncompleteArchives(t *testing.T) {
	good := importVolumeFixture(t, importIndexFixture)
	cases := map[string][]byte{
		"optimized": importVolumeFixture(t, strings.Replace(importIndexFixture, "optimized: false", "optimized: true", 1)),
		"snapshots": importVolumeFixture(t, `snapshots: [saved]
`+importIndexFixture),
		"duplicate-index":   importVolumeFixture(t, importIndexFixture, &tar.Header{Name: "backup/index.yaml", Typeflag: tar.TypeReg}),
		"parent-traversal":  importVolumeFixture(t, importIndexFixture, &tar.Header{Name: "backup/volume/../escape", Typeflag: tar.TypeReg}),
		"absolute":          importVolumeFixture(t, importIndexFixture, &tar.Header{Name: "/escape", Typeflag: tar.TypeReg}),
		"symlink-parent":    importVolumeFixture(t, importIndexFixture, &tar.Header{Name: "backup/volume/symlink/escape", Typeflag: tar.TypeReg}),
		"external-hardlink": importVolumeFixture(t, importIndexFixture, &tar.Header{Name: "backup/volume/escape", Typeflag: tar.TypeLink, Linkname: "/etc/passwd"}),
		"duplicate-data":    importVolumeFixture(t, importIndexFixture, &tar.Header{Name: "backup/volume/data", Typeflag: tar.TypeReg}),
		"missing-close":     good[:len(good)-512],
		"trailing-archive":  append(append([]byte{}, good...), good...),
		"duplicate-yaml": importVolumeFixture(t, `name: other
`+importIndexFixture),
		"unknown-field": importVolumeFixture(t, `unknown: true
`+importIndexFixture),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			os.Chmod(root, 0700)
			a, err := prepareVolumeImport(context.Background(), bytes.NewReader(input), root, 1<<20, "project", "pool", "name", nil)
			if err == nil || a != nil {
				t.Fatal("invalid import accepted")
			}
			if entries, e := os.ReadDir(root); e != nil || len(entries) != 0 {
				t.Fatal("unsafe staging residue", e)
			}
		})
	}
	for _, value := range []string{"[]", "null", `[{"Isuid":true,"Hostid":-1,"Nsid":0,"Maprange":1}]`, `[{"Isuid":true,"Hostid":0,"Nsid":0,"Maprange":9223372036854775807}]`, `[{"Isuid":true,"Hostid":0,"Nsid":0,"Maprange":1,"unknown":true}]`} {
		if validImportedIDMap(value) {
			t.Fatal("unsafe idmap", value)
		}
	}
}

func TestVolumeImportNativeBoundaryRefusesAmbiguityAndRetainsFailures(t *testing.T) {
	for _, scenario := range []string{"success", "owned", "foreign", "malformed", "truncated", "query-error", "native-exit", "native-error"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			resource := core.PersistentResource{ID: "oci:imported", Kind: OCIStoreKind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32), State: "creating"}
			imports := 0
			archivePath := ""
			lostReply := errors.New("lost native reply")
			runner := &fakeRunner{run: func(_ context.Context, _ int, command string, args []string) (host.Result, error) {
				if command != "incus" {
					t.Fatalf("unexpected executable %s", command)
				}
				if args[0] == "query" {
					switch scenario {
					case "query-error":
						return host.Result{}, lostReply
					case "malformed":
						return host.Result{Stdout: "{}"}, nil
					case "truncated":
						return host.Result{Stdout: "[]", StdoutTruncated: true}, nil
					case "owned", "foreign":
						owner := resource.Owner
						if scenario == "foreign" {
							owner = strings.Repeat("b", 32)
						}
						v := persistentVolumeObservation{Name: "haco-persistent-" + resource.Owner, Type: "custom", ContentType: "filesystem", Config: map[string]string{"user.hacocoon.owner": owner, "user.hacocoon.resource": resource.ID, "user.hacocoon.kind": resource.Kind, "user.hacocoon.source-only": "false"}}
						data, err := json.Marshal([]persistentVolumeObservation{v})
						return host.Result{Stdout: string(data)}, err
					default:
						return host.Result{Stdout: "[]"}, nil
					}
				}
				imports++
				if len(args) != 8 {
					t.Fatalf("unexpected mutation: %v", args)
				}
				archivePath = args[4]
				want := []string{"storage", "volume", "import", "pool", archivePath, "haco-persistent-" + resource.Owner, "--project", "hacocoon"}
				if !reflect.DeepEqual(args, want) || !strings.HasPrefix(archivePath, "/proc/") {
					t.Fatalf("unplanned destination: %v", args)
				}
				f, err := os.Open(archivePath)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				tr := tar.NewReader(f)
				if _, err = tr.Next(); err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(tr)
				if err != nil {
					t.Fatal(err)
				}
				var index volumeImportIndex
				if err = yaml.UnmarshalStrict(data, &index); err != nil {
					t.Fatal(err)
				}
				if index.Config.Volume.Config["user.hacocoon.owner"] != resource.Owner || index.Config.Volume.Config["user.hacocoon.resource"] != resource.ID {
					t.Fatal("native creation observed old authority")
				}
				switch scenario {
				case "native-exit":
					return host.Result{ExitCode: 1}, nil
				case "native-error":
					return host.Result{}, lostReply
				default:
					return host.Result{}, nil
				}
			}}
			backend := &PersistentResourceBackend{Runtime: New(runner), ImportRoot: root, ImportLimit: 1 << 20}
			err := backend.Import(context.Background(), resource, bytes.NewReader(importVolumeFixture(t, importIndexFixture)))
			switch scenario {
			case "success":
				if err != nil || imports != 1 {
					t.Fatal(imports, err)
				}
			case "native-exit", "native-error":
				if !errors.Is(err, core.ErrRecoveryRequired) || imports != 1 {
					t.Fatal(imports, err)
				}
				if scenario == "native-error" && !errors.Is(err, lostReply) {
					t.Fatal("lost underlying error", err)
				}
			default:
				if err == nil || imports != 0 {
					t.Fatal("ambiguous native target mutated", imports, err)
				}
			}
			if archivePath != "" {
				if _, err := os.Stat(archivePath); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("native input descriptor retained", err)
				}
			}
			if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
				t.Fatal("named staging residue", err)
			}
		})
	}
}
