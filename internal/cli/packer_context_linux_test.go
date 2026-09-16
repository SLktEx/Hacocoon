package cli

import (
	"github.com/SLktEx/Hacocoon/internal/base/build"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestPackerContextReadsFilesAndRefusesLinksAndSpecialFiles(t *testing.T) {
	for _, kind := range []string{"ordinary", "symlink", "directory-link", "hardlink", "fifo", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "base.pkr.hcl"), []byte("source"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, "scripts"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "scripts/setup.sh"), []byte("echo useful"), 0600); err != nil {
				t.Fatal(err)
			}
			other := filepath.Join(t.TempDir(), "private")
			if err := os.WriteFile(other, []byte("must not upload"), 0600); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, "extra")
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(other, target)
			case "directory-link":
				err = os.Symlink(filepath.Dir(other), target)
			case "hardlink":
				err = os.Link(other, target)
			case "fifo":
				err = syscall.Mkfifo(target, 0600)
			case "oversize":
				err = os.WriteFile(target, []byte(strings.Repeat("x", basebuild.MaxContextBytes+1)), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := readPackerContext(root)
			if kind == "ordinary" {
				if err != nil || len(got.Files) != 2 || got.Files[1].Path != "scripts/setup.sh" {
					t.Fatal(got, err)
				}
			} else if err == nil {
				t.Fatal("unsafe context accepted", kind)
			}
		})
	}
}
