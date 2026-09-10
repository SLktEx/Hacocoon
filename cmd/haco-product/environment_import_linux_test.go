//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type importClientFunc func(context.Context, io.Reader, string) (environmenttransfer.ImportResult, error)

func (f importClientFunc) ImportEnvironment(ctx context.Context, r io.Reader, name string) (environmenttransfer.ImportResult, error) {
	return f(ctx, r, name)
}
func TestImportFileRefusesSpecialAndMalformedInput(t *testing.T) {
	root := t.TempDir()
	valid := filepath.Join(root, "input.haco")
	if err := os.WriteFile(valid, productExportBytes(t), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(valid, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(filepath.Join(root, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "bad"), []byte("not a bundle"), 0600)
	client := importClientFunc(func(context.Context, io.Reader, string) (environmenttransfer.ImportResult, error) {
		t.Fatal("invalid input reached controller")
		return environmenttransfer.ImportResult{}, nil
	})
	for _, name := range []string{"link", "fifo", "bad", "missing", ""} {
		if _, err := loadEnvironmentImport(context.Background(), client, filepath.Join(root, name), ""); err == nil {
			t.Fatal(name)
		}
	}
}
func TestImportFilePinsReadOnlyInput(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "input.haco")
	data := productExportBytes(t)
	os.WriteFile(path, data, 0600)
	client := importClientFunc(func(ctx context.Context, r io.Reader, name string) (environmenttransfer.ImportResult, error) {
		if name != "new-env" {
			t.Fatal(name)
		}
		if err := os.Rename(path, path+".original"); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(path, []byte("replacement"), 0600)
		got, err := io.ReadAll(r)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatal("input descriptor changed", err)
		}
		return environmenttransfer.ImportResult{Environment: name, Workspace: "import-abcd", State: "running"}, nil
	})
	if _, err := loadEnvironmentImport(context.Background(), client, path, "new-env"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path + ".original")
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("source modified", err)
	}
}
func TestImportCommandUsesManagementStreamAndRetainsFailureReceipt(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failure], func(t *testing.T) {
			root := t.TempDir()
			os.Chmod(root, 0700)
			path := filepath.Join(root, "input.haco")
			data := productExportBytes(t)
			os.WriteFile(path, data, 0600)
			server := control.NewServer()
			if err := controlapi.RegisterEnvironmentImport(server, func(ctx context.Context, r io.Reader, name string) (environmenttransfer.ImportResult, error) {
				if name != "" {
					t.Error("default name not delegated", name)
				}
				staged, err := environmenttransfer.Stage(ctx, root, r, 1024)
				if err != nil {
					return environmenttransfer.ImportResult{}, err
				}
				defer staged.Close()
				result := environmenttransfer.ImportResult{Environment: "dev-imported", Workspace: "import-abcd", State: "running", Offline: []string{"workspace"}}
				if failure {
					result.State = "cleanup-required"
					return result, core.ErrRecoveryRequired
				}
				return result, nil
			}); err != nil {
				t.Fatal(err)
			}
			socket := filepath.Join(root, "control.sock")
			listener, err := control.ListenUnix(socket, 0600)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- server.Serve(ctx, listener) }()
			defer func() { cancel(); <-done }()
			t.Setenv("HACO_CONTROL_SOCKET", socket)
			var out, diagnostic bytes.Buffer
			code := environmentCommand(ctx, []string{"import", "--json", path}, &out, &diagnostic)
			var receipt environmenttransfer.ImportResult
			if err := json.Unmarshal(out.Bytes(), &receipt); err != nil {
				t.Fatal(err, out.String(), diagnostic.String())
			}
			if receipt.Workspace != "import-abcd" {
				t.Fatal(receipt)
			}
			if failure {
				if code != 1 || !strings.Contains(diagnostic.String(), "Workspace retained: import-abcd") {
					t.Fatal(code, diagnostic.String())
				}
			} else if code != 0 || receipt.Environment != "dev-imported" {
				t.Fatal(code, receipt, diagnostic.String())
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatal("source changed", err)
			}
		})
	}
}
func TestImportCommandUsage(t *testing.T) {
	for _, args := range [][]string{{"import"}, {"import", "file", "../bad"}, {"import", "a", "b", "c"}} {
		var out, diag bytes.Buffer
		if code := environmentCommand(context.Background(), args, &out, &diag); code != 2 {
			t.Fatal(args, code)
		}
	}
	var out, diag bytes.Buffer
	if code := environmentCommand(context.Background(), []string{"import", "--help"}, &out, &diag); code != 0 || !strings.Contains(diag.String(), "<file.haco> [new-env]") {
		t.Fatal(code, diag.String())
	}
}
