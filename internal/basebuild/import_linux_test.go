//go:build linux

package basebuild

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type importEnv struct {
	*fakeEnv
	exhausted *bool
}

func (f *importEnv) CreateFromArchive(ctx context.Context, s core.EnvironmentSpec, source io.ReadSeeker, root string, limit int64) (core.Environment, error) {
	if !*f.exhausted || root == "" || limit != MaxArchiveBytes || s.Base != "" || s.Resources != builderResources() {
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
	for _, mode := range []string{"name", "empty", "source-failed", "canceled", "directory"} {
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
