//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
)

type exportClientFunc func(context.Context, string, io.Writer) (controlapi.EnvironmentExportResult, error)

func (f exportClientFunc) ExportEnvironment(c context.Context, s string, w io.Writer) (controlapi.EnvironmentExportResult, error) {
	return f(c, s, w)
}
func productExportBytes(t *testing.T) []byte {
	t.Helper()
	m := environmenttransfer.Manifest{Version: 1, Source: "dev"}
	var readers []io.Reader
	for _, role := range []string{"rootfs", "workspace"} {
		data := []byte("owned " + role)
		hash := sha256.Sum256(data)
		m.Components = append(m.Components, environmenttransfer.Component{Role: role, Bytes: int64(len(data)), SHA256: hex.EncodeToString(hash[:])})
		readers = append(readers, bytes.NewReader(data))
	}
	var out bytes.Buffer
	if err := environmenttransfer.Write(&out, m, readers, 1024); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func TestEnvironmentExportPublicationOwnsBytesAndNeverOverwrites(t *testing.T) {
	for _, which := range []string{"success", "error", "digest", "malformed", "wrong-source", "cancel", "exists", "race", "parent-replaced"} {
		t.Run(which, func(t *testing.T) {
			parent := t.TempDir()
			destination := filepath.Join(parent, "dev.haco")
			data := productExportBytes(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			if which == "exists" {
				if err := os.WriteFile(destination, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			client := exportClientFunc(func(_ context.Context, source string, w io.Writer) (controlapi.EnvironmentExportResult, error) {
				calls++
				if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
					t.Fatal("named partial output", entries, err)
				}
				if which == "malformed" {
					data = []byte("not an archive")
				}
				if _, err := w.Write(data); err != nil {
					return controlapi.EnvironmentExportResult{}, err
				}
				digest := sha256.Sum256(data)
				result := controlapi.EnvironmentExportResult{Bytes: int64(len(data)), SHA256: hex.EncodeToString(digest[:])}
				switch which {
				case "error":
					return result, errors.New("late transfer failure")
				case "digest":
					result.SHA256 = strings.Repeat("0", 64)
				case "cancel":
					cancel()
				case "race":
					if err := os.WriteFile(destination, []byte("keep"), 0600); err != nil {
						t.Fatal(err)
					}
				case "parent-replaced":
					if err := os.Rename(parent, parent+"-moved"); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(parent, 0700); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { os.Remove(parent); os.Rename(parent+"-moved", parent) })
				}
				return result, nil
			})
			source := "dev"
			if which == "wrong-source" {
				source = "other"
			}
			_, err := saveEnvironmentExport(ctx, client, source, destination)
			switch which {
			case "success", "parent-replaced":
				if err != nil {
					t.Fatal(err)
				}
				actual := destination
				if which == "parent-replaced" {
					actual = filepath.Join(parent+"-moved", "dev.haco")
					if _, err := os.Stat(destination); !os.IsNotExist(err) {
						t.Fatal("publication redirected")
					}
				}
				got, err := os.ReadFile(actual)
				if err != nil || !bytes.Equal(got, data) {
					t.Fatal(err)
				}
				info, _ := os.Stat(actual)
				if info.Mode().Perm() != 0600 {
					t.Fatal(info.Mode())
				}
			case "exists", "race":
				if !errors.Is(err, os.ErrExist) {
					t.Fatal(err)
				}
				got, _ := os.ReadFile(destination)
				if string(got) != "keep" {
					t.Fatal("existing data overwritten")
				}
				if which == "exists" && calls != 0 {
					t.Fatal("captured before refusing existing destination")
				}
			default:
				if err == nil {
					t.Fatal("invalid export published")
				}
				if entries, readErr := os.ReadDir(parent); readErr != nil || len(entries) != 0 {
					t.Fatal("failure left named bytes", entries, readErr)
				}
			}
		})
	}
}
func TestEnvironmentExportCLIRequiresOnlyStoppedSource(t *testing.T) {
	data := productExportBytes(t)
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	server := control.NewServer()
	calls := 0
	if err := controlapi.RegisterEnvironmentExport(server, func(ctx context.Context, source string) (environmenttransfer.ExportResult, error) {
		calls++
		if source != "dev" {
			t.Error(source)
		}
		staged, err := environmenttransfer.Stage(ctx, root, bytes.NewReader(data), 1024)
		return environmenttransfer.ExportResult{Bundle: staged}, err
	}); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "export.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer listener.Close()
	go server.Serve(ctx, listener)
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	t.Chdir(t.TempDir())
	var out, diagnostic bytes.Buffer
	if code := environmentCommand(ctx, []string{"export", "dev"}, &out, &diagnostic); code != 0 {
		t.Fatal(code, diagnostic.String())
	}
	got, err := os.ReadFile("dev.haco")
	if err != nil || !bytes.Equal(got, data) || calls != 1 {
		t.Fatal(err, calls)
	}
	if !strings.Contains(out.String(), "dev.haco") {
		t.Fatal(out.String())
	}
	limited, stop := context.WithTimeout(ctx, time.Second)
	defer stop()
	if code := environmentCommand(limited, []string{"export", "dev"}, &out, &diagnostic); code == 0 || calls != 1 {
		t.Fatal("second export overwrote existing file", code, calls)
	}
}
