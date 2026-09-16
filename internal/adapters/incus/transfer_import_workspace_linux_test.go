//go:build linux

package incus

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/host"
	"gopkg.in/yaml.v2"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func portableWorkspaceFixture(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	w := tar.NewWriter(&out)
	for _, entry := range []struct {
		name, content, link string
		kind                byte
		mode                int64
	}{
		{"tree", "", "", tar.TypeDir, 0755},
		{"tree/.git", "", "", tar.TypeDir, 0700},
		{"tree/.git/HEAD", "ref: refs/heads/main\n", "", tar.TypeReg, 0600},
		{"tree/.git/config", "[core]\nrepositoryformatversion = 0\n", "", tar.TypeReg, 0600},
		{"tree/run.sh", "#!/bin/sh\nprintf hello\n", "", tar.TypeReg, 0755},
		{"tree/data.bin", string(bytes.Repeat([]byte{0, 255, 13, 10}, 512)), "", tar.TypeReg, 0640},
		{"tree/current", "", "data.bin", tar.TypeSymlink, 0777},
	} {
		if err := w.WriteHeader(&tar.Header{Name: entry.name, Size: int64(len(entry.content)), Typeflag: entry.kind, Linkname: entry.link, Mode: entry.mode, Uid: 22001, Gid: 22002, Uname: "source-user", Gname: "source-group"}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, entry.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestPortableWorkspaceImportPreservesTreeAndUsesFreshOwnership(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	object := gitrepo.Object{Kind: "work", ID: "imported", Repository: "repo", Owner: strings.Repeat("a", 32), State: "creating", NativeRef: "pool/haco-work-imported"}
	var imported []byte
	b := &RepositoryBackend{Runtime: New(&fakeRunner{run: func(_ context.Context, _ int, command string, args []string) (host.Result, error) {
		if command != "incus" {
			t.Fatal("unexpected archive consumer", command)
		}
		if args[0] == "query" {
			return host.Result{Stdout: "[]"}, nil
		}
		if len(args) != 8 || args[0] != "storage" || args[2] != "import" || args[3] != "pool" || args[5] != "haco-work-imported" {
			t.Fatal("archive delivered to wrong destination", args)
		}
		file, err := os.Open(args[4])
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = file.Close() }()
		imported, err = io.ReadAll(file)
		return host.Result{}, err
	}}), ImportRoot: root, ImportLimit: 1 << 20}
	if err := b.ImportWorkspaceTreeVolume(context.Background(), object, bytes.NewReader(portableWorkspaceFixture(t))); err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(bytes.NewReader(imported))
	header, err := reader.Next()
	if err != nil || header.Name != "backup/index.yaml" {
		t.Fatal("native archive omitted its index", header, err)
	}
	indexBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var index volumeImportIndex
	if err := yaml.UnmarshalStrict(indexBytes, &index); err != nil {
		t.Fatal(err)
	}
	if index.Name != "haco-work-imported" || index.Pool != "pool" || index.Backend != "btrfs" || index.Config.Volume.Config["user.hacocoon.owner"] != object.Owner || index.Config.Volume.Config["user.hacocoon.repository"] != object.Repository {
		t.Fatal("native import did not bind to the fresh destination", index)
	}
	seen := make(map[string]bool)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seen[header.Name] = true
		if header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" || !strings.HasPrefix(header.Name, "backup/volume") || len(header.PAXRecords) != 0 {
			t.Fatal("source ownership or metadata crossed native import", header)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		switch header.Name {
		case "backup/volume/run.sh":
			if header.Mode != 0755 || string(data) != "#!/bin/sh\nprintf hello\n" {
				t.Fatal("executable content/mode changed")
			}
		case "backup/volume/data.bin":
			if header.Mode != 0640 || !bytes.Equal(data, bytes.Repeat([]byte{0, 255, 13, 10}, 512)) {
				t.Fatal("binary content/mode changed")
			}
		case "backup/volume/current":
			if header.Typeflag != tar.TypeSymlink || header.Linkname != "data.bin" {
				t.Fatal("confined relative link changed")
			}
		}
	}
	if len(seen) != 7 || !seen["backup/volume/.git/HEAD"] || !seen["backup/volume/.git/config"] {
		t.Fatal("tree entries were lost", seen)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("native staging retained portable input", entries, err)
	}
}

func TestPortableWorkspaceImportRefusesInvalidInputBeforeNativeMutation(t *testing.T) {
	for _, failure := range []string{"nil", "empty", "truncated", "metadata-limit", "data-limit", "cancelled", "invalid-ref", "missing-stage"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			b := &RepositoryBackend{Runtime: New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
				t.Error("invalid portable input reached native provider")
				return host.Result{}, errors.New("unexpected native call")
			}}), ImportRoot: root, ImportLimit: 1 << 20}
			object := gitrepo.Object{Kind: "work", ID: "imported", Repository: "repo", Owner: strings.Repeat("a", 32), State: "creating", NativeRef: "pool/haco-work-imported"}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var input io.Reader = bytes.NewReader(portableWorkspaceFixture(t))
			switch failure {
			case "nil":
				input = nil
			case "empty":
				input = bytes.NewReader(nil)
			case "truncated":
				input = bytes.NewReader(portableWorkspaceFixture(t)[:100])
			case "metadata-limit":
				b.ImportLimit = 512
			case "data-limit":
				b.ImportLimit = 6144
			case "cancelled":
				cancel()
			case "invalid-ref":
				object.NativeRef = "../outside"
			case "missing-stage":
				b.ImportRoot = filepath.Join(root, "missing")
			}
			if err := b.ImportWorkspaceTreeVolume(ctx, object, input); err == nil {
				t.Fatal("invalid input acknowledged")
			} else if failure == "nil" && !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatal("nil input lost invalid-argument classification", err)
			}
			if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
				t.Fatal("failed conversion retained staging files", entries, err)
			}
		})
	}
}

func TestWorkspaceNativeImportReservesDestinationAndReplacesSourceConfig(t *testing.T) {
	for _, mode := range []string{"ok", "existing", "malformed", "truncated", "native-failed"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			object := gitrepo.Object{Kind: "work", ID: "imported", Repository: "repo", Owner: strings.Repeat("a", 32), State: "creating", NativeRef: "pool/haco-work-imported"}
			imports := 0
			runner := &fakeRunner{run: func(_ context.Context, _ int, command string, args []string) (host.Result, error) {
				if command != "incus" {
					t.Fatal("unexpected executable", command)
				}
				if args[0] == "query" {
					switch mode {
					case "existing":
						data, _ := json.Marshal([]persistentVolumeObservation{{Name: "haco-work-imported"}})
						return host.Result{Stdout: string(data)}, nil
					case "malformed":
						return host.Result{Stdout: "{}"}, nil
					case "truncated":
						return host.Result{Stdout: "[]", StdoutTruncated: true}, nil
					default:
						return host.Result{Stdout: "[]"}, nil
					}
				}
				imports++
				if len(args) != 8 || args[0] != "storage" || args[2] != "import" || args[3] != "pool" || args[5] != "haco-work-imported" {
					t.Fatal("wrong mutation", args)
				}
				f, err := os.Open(args[4])
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = f.Close() }()
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
				config := index.Config.Volume.Config
				if config["user.hacocoon.owner"] != object.Owner || config["user.hacocoon.role"] != "work" || config["user.hacocoon.repository"] != "repo" || config["user.hacocoon.snapshot-source"] != "" {
					t.Fatal("old authority reached native import", config)
				}
				if mode == "native-failed" {
					return host.Result{ExitCode: 1}, nil
				}
				return host.Result{}, nil
			}}
			b := &RepositoryBackend{Runtime: New(runner), ImportRoot: root, ImportLimit: 1 << 20}
			err := b.ImportWorkspaceVolume(context.Background(), object, bytes.NewReader(importVolumeFixture(t, importIndexFixture)))
			if mode == "ok" {
				if err != nil || imports != 1 {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("failure accepted")
			}
			if mode == "native-failed" && !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(err)
			}
			if mode != "ok" && mode != "native-failed" && imports != 0 {
				t.Fatal("existing or ambiguous target overwritten")
			}
			if entries, e := os.ReadDir(root); e != nil || len(entries) != 0 {
				t.Fatal("staging residue", e)
			}
		})
	}
}
