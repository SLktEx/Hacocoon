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

func TestWorkerFailureReceiptPreservesIdentityAndBoundsPrivateOutput(t *testing.T) {
	for _, state := range []string{"failed", "recovery-required", "missing", "malformed", "symlink", "oversize"} {
		t.Run(state, func(t *testing.T) {
			directory := t.TempDir()
			id := strings.Repeat("d", 32)
			record := map[string]any{"state": state, "stage": "build", "execution": core.ExecutionResult{ExitCode: 73, Stdout: strings.Repeat("x", 20000), Stderr: strings.Repeat("y", 20000)}}
			path := filepath.Join(directory, "receipt.json")
			switch state {
			case "missing":
			case "malformed":
				if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "oversize":
				if err := os.WriteFile(path, []byte(strings.Repeat("x", 262145)), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink("foreign", path); err != nil {
					t.Fatal(err)
				}
			default:
				if err := saveReceipt(directory, record); err != nil {
					t.Fatal(err)
				}
			}
			client := importFunc(func(context.Context, io.Reader, basebuild.ImportRequest) (basebuild.Result, error) {
				t.Fatal("failed worker initiated import")
				return basebuild.Result{}, nil
			})
			got, err := finishBuild(context.Background(), directory, id, basebuild.ImportRequest{Name: "tools"}, client, errors.New("private subprocess text"))
			if err == nil || strings.Contains(err.Error(), "private subprocess text") || got.Builder != "haco-packer-"+id || got.Base.Name != "tools" {
				t.Fatal(got, err)
			}
			if state == "failed" {
				if got.State != "failed" || !errors.Is(err, core.ErrRuntimeUnavailable) {
					t.Fatal(got, err)
				}
			} else if got.State != "recovery-required" || !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(got, err)
			}
			if state == "failed" || state == "recovery-required" {
				if got.Execution == nil || got.Execution.ExitCode != 73 || len(got.Execution.Stdout) != 16384 || len(got.Execution.Stderr) != 16384 || !got.Execution.StdoutTruncated || !got.Execution.StderrTruncated {
					t.Fatal("private output contract lost", got.Execution)
				}
			}
		})
	}
}

func TestArtifactTransportRefusesUnownedOrChangedFiles(t *testing.T) {
	for _, mode := range []string{"foreign-entry", "missing-export", "extra-export", "symlink", "hardlink", "directory", "changed-size", "wrong-id", "wrong-base", "wrong-state", "invalid-size", "invalid-digest", "invalid-architecture", "receipt-write-conflict"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			export := filepath.Join(directory, "export")
			if err := os.Mkdir(export, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(export, "image.tar")
			if err := os.WriteFile(path, []byte("image"), 0600); err != nil {
				t.Fatal(err)
			}
			id := strings.Repeat("a", 32)
			digest := sha256.Sum256([]byte("image"))
			record := map[string]any{"id": id, "base_name": "tools", "state": "artifact-ready", "sha256": hex.EncodeToString(digest[:]), "size": json.Number("5"), "architecture": "x86_64"}
			if err := saveReceipt(directory, record); err != nil {
				t.Fatal(err)
			}
			write := func(path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch mode {
			case "foreign-entry":
				write(filepath.Join(directory, "foreign"))
			case "missing-export":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "extra-export":
				write(filepath.Join(export, "foreign"))
			case "symlink", "hardlink":
				foreign := filepath.Join(t.TempDir(), "keep")
				write(foreign)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				var err error
				if mode == "symlink" {
					err = os.Symlink(foreign, path)
				} else {
					err = os.Link(foreign, path)
				}
				if err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "changed-size":
				write(path)
			case "wrong-id":
				record["id"] = strings.Repeat("b", 32)
			case "wrong-base":
				record["base_name"] = "foreign"
			case "wrong-state":
				record["state"] = "import-pending"
			case "invalid-size":
				record["size"] = json.Number("9223372036854775808")
			case "invalid-digest":
				record["sha256"] = "invalid"
			case "invalid-architecture":
				record["architecture"] = "invalid"
			case "receipt-write-conflict":
				write(filepath.Join(directory, "receipt.next"))
			}
			client := importFunc(func(context.Context, io.Reader, basebuild.ImportRequest) (basebuild.Result, error) {
				t.Fatal("unsafe artifact reached controller")
				return basebuild.Result{}, nil
			})
			got, err := importArtifact(context.Background(), directory, id, basebuild.ImportRequest{Name: "tools"}, client, record)
			if !errors.Is(err, core.ErrRecoveryRequired) || got.State == "ready" {
				t.Fatal(got, err)
			}
			if _, err := os.Stat(filepath.Join(directory, "receipt.json")); err != nil {
				t.Fatal("lost diagnostic receipt", err)
			}
		})
	}
}

func TestPackerRejectsInvalidInputBeforeLaunchingWorker(t *testing.T) {
	valid := basebuild.PackerTemplate{Files: []basebuild.SourceFile{{Path: "base.pkr.hcl", Data: []byte("source")}}}
	client := importFunc(func(context.Context, io.Reader, basebuild.ImportRequest) (basebuild.Result, error) {
		t.Fatal("invalid request imported")
		return basebuild.Result{}, nil
	})
	for _, tc := range []struct {
		template basebuild.PackerTemplate
		request  basebuild.ImportRequest
		client   Importer
	}{
		{basebuild.PackerTemplate{}, basebuild.ImportRequest{Name: "tools"}, client},
		{valid, basebuild.ImportRequest{Name: "../foreign"}, client},
		{valid, basebuild.ImportRequest{Name: "tools"}, nil},
		{valid, basebuild.ImportRequest{Name: "tools", Artifact: &basebuild.Artifact{}}, client},
	} {
		if _, err := Build(context.Background(), tc.template, tc.request, tc.client); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(err)
		}
	}
}

func TestCanceledPackerLaunchRetainsExactRecoveryIdentity(t *testing.T) {
	for _, limit := range []int64{0, 1024} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		template := basebuild.PackerTemplate{Files: []basebuild.SourceFile{{Path: "base.pkr.hcl", Data: []byte("source")}}}
		client := importFunc(func(context.Context, io.Reader, basebuild.ImportRequest) (basebuild.Result, error) {
			t.Fatal("canceled worker imported")
			return basebuild.Result{}, nil
		})
		got, err := Build(ctx, template, basebuild.ImportRequest{Name: "tools", MaxBytes: limit}, client)
		if !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrRecoveryRequired) || got.State != "recovery-required" || !strings.HasPrefix(got.Builder, "haco-packer-") {
			t.Fatal(got, err)
		}
	}
}
