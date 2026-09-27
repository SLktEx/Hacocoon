//go:build linux

package packer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	basebuild "github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type importFunc func(context.Context, io.Reader, basebuild.ImportRequest) (basebuild.Result, error)

func (f importFunc) ImportBase(ctx context.Context, r io.Reader, req basebuild.ImportRequest) (basebuild.Result, error) {
	return f(ctx, r, req)
}

func TestArtifactImportFailureAndUnknownReplyRetainReceiptWithoutRetry(t *testing.T) {
	for _, mode := range []string{"ready", "stream", "import", "unknown", "cleanup", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "export"), 0700); err != nil {
				t.Fatal(err)
			}
			payload := []byte("image archive")
			if err := os.WriteFile(filepath.Join(dir, "export", "image.tar"), payload, 0600); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(payload)
			id := strings.Repeat("a", 32)
			record := map[string]any{"id": id, "base_name": "tools", "state": "artifact-ready", "stage": "import", "sha256": hex.EncodeToString(digest[:]), "size": json.Number(strconv.Itoa(len(payload))), "architecture": "x86_64", "fingerprint": strings.Repeat("b", 64)}
			if err := saveReceipt(dir, record); err != nil {
				t.Fatal(err)
			}
			calls := 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := importFunc(func(ctx context.Context, r io.Reader, req basebuild.ImportRequest) (basebuild.Result, error) {
				calls++
				persisted, err := readReceipt(dir)
				if err != nil {
					t.Fatal(err)
				}
				if persisted["state"] != "import-pending" || persisted["import_builder"] != "build-"+id {
					t.Fatal("mutation before durable exact identity")
				}
				if req.Artifact == nil || req.Artifact.ID != id || req.Artifact.SHA256 != record["sha256"] || req.Artifact.Size != int64(len(payload)) {
					t.Fatal("transport identity lost", req)
				}
				bytes, err := io.ReadAll(r)
				if err != nil || string(bytes) != string(payload) {
					t.Fatal("stream differs")
				}
				switch mode {
				case "stream":
					return basebuild.Result{}, io.ErrUnexpectedEOF
				case "import":
					return basebuild.Result{State: "failed", Builder: "build-" + id}, core.ErrRuntimeUnavailable
				case "unknown":
					return basebuild.Result{State: "publication-unconfirmed", Builder: "build-" + id}, core.ErrRecoveryRequired
				case "cancel":
					cancel()
					return basebuild.Result{}, ctx.Err()
				case "cleanup":
					if err := os.WriteFile(filepath.Join(dir, "foreign"), []byte("keep"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				return basebuild.Result{Base: core.BaseInfo{Name: "tools", Revision: core.BaseRevision("sha256:" + strings.Repeat("c", 64))}, State: "ready"}, nil
			})
			got, err := importArtifact(ctx, dir, id, basebuild.ImportRequest{Name: "tools"}, client, record)
			if calls != 1 {
				t.Fatal("import was replayed", calls)
			}
			if mode == "ready" {
				if err != nil || got.State != "ready" {
					t.Fatal(got, err)
				}
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Fatal("successful transport remains", err)
				}
			} else {
				if !errors.Is(err, core.ErrRecoveryRequired) || got.State == "ready" {
					t.Fatal("failure reported successful", got, err)
				}
				retained, err := readReceipt(dir)
				if err != nil {
					t.Fatal("lost receipt", err)
				}
				if retained["id"] != id || retained["sha256"] != record["sha256"] {
					t.Fatal("lost identity", retained)
				}
				if _, err := os.Stat(filepath.Join(dir, "export", "image.tar")); err != nil {
					t.Fatal("uncertain artifact removed", err)
				}
			}
		})
	}
}
