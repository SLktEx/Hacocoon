//go:build linux

package incus

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"gopkg.in/yaml.v2"
	"io"
	"os"
	"strings"
	"testing"
)

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
