//go:build linux

package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type toolingTransport func(*http.Request) (*http.Response, error)

func (f toolingTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func toolingFixture(t *testing.T, mode string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	w := tar.NewWriter(z)
	names := []string{"bin/containerd", "bin/ctr", "bin/nerdctl"}
	if mode == "missing" {
		names = names[:2]
	}
	if mode == "duplicate" {
		names = append(names, "bin/ctr")
	}
	for _, name := range names {
		kind := byte(tar.TypeReg)
		data := []byte("fixture-" + name)
		if mode == "symlink" && name == "bin/ctr" {
			kind = tar.TypeSymlink
			data = nil
		}
		if mode == "hardlink" && name == "bin/ctr" {
			kind = tar.TypeLink
			data = nil
		}
		if mode == "traversal" && name == "bin/ctr" {
			name = "../ctr"
		}
		if err := w.WriteHeader(&tar.Header{Name: name, Typeflag: kind, Linkname: "/etc/passwd", Mode: 0755, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestMaintenanceToolingVerifiedCacheAndPrivateCopies(t *testing.T) {
	payload := toolingFixture(t, "")
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	var requests atomic.Int32
	s := MaintenanceTooling{Directory: filepath.Join(t.TempDir(), "cache"), client: &http.Client{Transport: toolingTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != maintenanceArchiveURL || r.Method != "GET" {
			t.Error("changed pinned request")
		}
		requests.Add(1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header)}, nil
	})}}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			directory, release, err := s.prepare(context.Background(), digest)
			if err != nil {
				t.Error(err)
				return
			}
			defer func() {
				if err := release(); err != nil {
					t.Error(err)
				}
			}()
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 3 {
				t.Error("wrong private inventory", err)
				return
			}
			for _, name := range []string{"containerd", "ctr", "nerdctl"} {
				file := filepath.Join(directory, name)
				data, err := os.ReadFile(file)
				if err != nil || string(data) != "fixture-bin/"+name {
					t.Error("changed binary", err)
				}
				info, err := os.Lstat(file)
				if err != nil || info.Mode().Perm() != 0500 {
					t.Error("unsafe tool mode", err)
				}
			}
		}()
	}
	wg.Wait()
	if requests.Load() != 1 {
		t.Fatal("cache not reused", requests.Load())
	}
	cache := filepath.Join(s.Directory, digest+".tar.gz")
	if err := os.WriteFile(cache, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.prepare(context.Background(), digest); err == nil {
		t.Fatal("accepted corrupt cache")
	}
	if requests.Load() != 1 {
		t.Fatal("silently replaced corrupt cache")
	}
}
func TestMaintenanceToolingRejectsUntrustedArchivesAndCache(t *testing.T) {
	for _, mode := range []string{"missing", "duplicate", "symlink", "hardlink", "traversal", "checksum", "cache-link", "directory-link", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			payload := toolingFixture(t, mode)
			digest := fmt.Sprintf("%x", sha256.Sum256(payload))
			if mode == "checksum" {
				digest = maintenanceArchiveSHA256
			}
			s := MaintenanceTooling{Directory: filepath.Join(t.TempDir(), "cache"), client: &http.Client{Transport: toolingTransport(func(r *http.Request) (*http.Response, error) {
				if err := r.Context().Err(); err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header)}, nil
			})}}
			if err := os.MkdirAll(s.Directory, 0700); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if mode == "cancelled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if mode == "cache-link" {
				target := filepath.Join(t.TempDir(), "archive")
				if err := os.WriteFile(target, payload, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(s.Directory, digest+".tar.gz")); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "directory-link" {
				target := s.Directory
				s.Directory = filepath.Join(t.TempDir(), "cache")
				if err := os.Symlink(target, s.Directory); err != nil {
					t.Fatal(err)
				}
			}
			directory, release, err := s.prepare(ctx, digest)
			if release != nil {
				_ = release()
			}
			if err == nil || directory != "" {
				t.Fatal("unsafe preparation accepted", mode, err)
			}
		})
	}
}

func TestMaintenanceToolingDoesNotExposeTransportCredentials(t *testing.T) {
	s := MaintenanceTooling{Directory: filepath.Join(t.TempDir(), "cache"), client: &http.Client{Transport: toolingTransport(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("redirect https://assets.example/path?sig=private-transfer-secret")
	})}}
	_, _, err := s.Prepare(context.Background())
	if err == nil || strings.Contains(err.Error(), "private-transfer-secret") || strings.Contains(err.Error(), "assets.example") {
		t.Fatal("unsafe download error", err)
	}
}
