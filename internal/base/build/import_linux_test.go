//go:build linux

package basebuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type importEnv struct {
	*fakeEnv
	exhausted *bool
}

func (f *importEnv) CreateFromArchive(ctx context.Context, s core.EnvironmentSpec, source io.ReadSeeker, root string, limit int64) (core.Environment, error) {
	if !*f.exhausted || root == "" || limit != DefaultArchiveLimitBytes || s.Base != "" || s.Resources != builderResources() {
		f.t.Fatal("input not captured before isolated creation")
	}
	raw, err := io.ReadAll(source)
	if err != nil || string(raw) != "archive bytes" {
		f.t.Fatal("source differs", err)
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		f.t.Fatal(err)
	}
	return f.Create(ctx, s)
}

type observedEOF struct {
	io.Reader
	exhausted *bool
}

func (r observedEOF) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		*r.exhausted = true
	}
	return n, err
}

func TestBaseArchiveImportSharesBuilderCleanupAndPublication(t *testing.T) {
	for _, failure := range []string{"", "create", "clean", "stop", "publish", "delete"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			complete := false
			f := &importEnv{fakeEnv: &fakeEnv{t: t, fail: failure}, exhausted: &complete}
			got, err := (&Service{Environments: f}).Import(context.Background(), ImportRequest{Name: "imported"}, observedEOF{strings.NewReader("archive bytes"), &complete}, root)
			if (err != nil) != (failure != "") {
				t.Fatal(got, err)
			}
			if failure == "" && (!reflect.DeepEqual(f.calls, []string{"create", "clean", "stop", "publish", "delete"}) || got.State != "ready" || got.Builder != "") {
				t.Fatal(got, f.calls)
			}
			if failure == "publish" && (got.State != "publication-unconfirmed" || got.Builder == "" || f.calls[len(f.calls)-1] != "publish") {
				t.Fatal("lost uncertain publication", got, f.calls)
			}
			if failure == "delete" && (got.State != "cleanup-required" || got.Base.Revision == "" || got.Builder == "") {
				t.Fatal("lost published Base", got)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatal("named input remains", err)
			}
		})
	}
}
func TestBaseArchiveImportRejectsInvalidInputBeforeCreation(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"name", "empty", "source-failed", "canceled", "directory", "limit"} {
		t.Run(mode, func(t *testing.T) {
			complete := false
			f := &importEnv{fakeEnv: &fakeEnv{t: t}, exhausted: &complete}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req := ImportRequest{Name: "imported"}
			var source io.Reader = strings.NewReader("archive bytes")
			path := root
			switch mode {
			case "name":
				req.Name = "--privileged"
			case "limit":
				req.MaxBytes = 4
			case "empty":
				source = strings.NewReader("")
			case "source-failed":
				source = failedArchiveReader{}
			case "canceled":
				cancel()
			case "directory":
				path = "relative"
			}
			if _, err := (&Service{Environments: f}).Import(ctx, req, source, path); err == nil || len(f.calls) != 0 {
				t.Fatal("invalid input created a builder", err, f.calls)
			}
		})
	}
}

type failedArchiveReader struct{}

func (failedArchiveReader) Read([]byte) (int, error) { return 0, errors.New("interrupted upload") }

func TestArtifactIntegrityAndIdentityBeforeImportCreation(t *testing.T) {
	for _, mode := range []string{"digest", "size", "id", "architecture", "foreign-architecture"} {
		t.Run(mode, func(t *testing.T) {
			complete := false
			f := &importEnv{fakeEnv: &fakeEnv{t: t}, exhausted: &complete}
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte("archive bytes"))
			a := &Artifact{ID: strings.Repeat("a", 32), SHA256: hex.EncodeToString(digest[:]), Size: 13, Architecture: "x86_64"}
			switch mode {
			case "digest":
				a.SHA256 = strings.Repeat("b", 64)
			case "foreign-architecture":
				a.Architecture = "aarch64"
				if runtime.GOARCH == "arm64" {
					a.Architecture = "x86_64"
				}
			case "size":
				a.Size = 1
			case "id":
				a.ID = "../escape"
			case "architecture":
				a.Architecture = "unknown"
			}
			_, err := (&Service{Environments: f}).Import(context.Background(), ImportRequest{Name: "tools", Artifact: a}, strings.NewReader("archive bytes"), root)
			if err == nil || len(f.calls) != 0 {
				t.Fatal("unverified artifact created/published a Base", err, f.calls)
			}
		})
	}
}

func TestVerifiedArtifactUsesCanonicalImportAndExactBuilderIdentity(t *testing.T) {
	arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	if arch == "" {
		t.Skip("supported Incus architecture required")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	complete := false
	f := &importEnv{fakeEnv: &fakeEnv{t: t, fail: "publish"}, exhausted: &complete}
	digest := sha256.Sum256([]byte("archive bytes"))
	id := strings.Repeat("a", 32)
	req := ImportRequest{Name: "tools", Artifact: &Artifact{ID: id, SHA256: hex.EncodeToString(digest[:]), Size: 13, Architecture: arch}}
	got, err := (&Service{Environments: f}).Import(context.Background(), req, observedEOF{strings.NewReader("archive bytes"), &complete}, root)
	if err == nil || got.State != "publication-unconfirmed" || got.Builder != "build-"+id {
		t.Fatal("canonical uncertain ownership lost", got, err)
	}
	if !reflect.DeepEqual(f.calls, []string{"create", "clean", "stop", "publish"}) {
		t.Fatal(f.calls)
	}
}

func TestImportRootDiskScalesWithArtifact(t *testing.T) {
	for _, size := range []int64{1, 65 << 30, 500 << 30, 2 << 40, MaxArchiveLimitBytes} {
		got := importBuilderResources(size)
		want := builderResources()
		if uint64(size) > core.MaxRootDiskResourceBytes/2 {
			want.RootBytes = core.ResourceLimit{Mode: core.ResourceLimitUnlimited}
		} else if uint64(size)*2 > want.RootBytes.Value {
			want.RootBytes.Value = uint64(size) * 2
		}
		if got != want || got.RootBytes.Value > core.MaxRootDiskResourceBytes {
			t.Fatal(size, got)
		}
	}
}

// Digest verification must observe cancellation between chunks even after the
// entire upload has been captured; large local artifacts can take time to hash.
func TestArtifactDigestReadObservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := &cancelDigestReader{cancel: cancel}
	n, err := io.Copy(sha256.New(), &importReader{ctx, source})
	if n != 1 || !errors.Is(err, context.Canceled) || source.reads != 1 {
		t.Fatal("digest read continued after cancellation", n, err, source.reads)
	}
}

type cancelDigestReader struct {
	cancel context.CancelFunc
	reads  int
}

func (r *cancelDigestReader) Read(p []byte) (int, error) {
	r.reads++
	p[0] = 'x'
	r.cancel()
	return 1, nil
}
