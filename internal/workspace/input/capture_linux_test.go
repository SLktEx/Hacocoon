//go:build linux

package workspaceinput

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitFixture(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "core.hooksPath=/dev/null", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture Git: %v: %s", err, out)
	}
	return string(out)
}
func checkoutFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitFixture(t, dir, "init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(dir, "tracked"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, dir, "add", "tracked")
	gitFixture(t, dir, "commit", "-m", "initial")
	return dir
}
func extractFixture(t *testing.T, input io.Reader) string {
	t.Helper()
	dest := t.TempDir()
	r := tar.NewReader(input)
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Join(dest, strings.TrimPrefix(h.Name, "tree"))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(name, 0700); err != nil {
				t.Fatal(err)
			}
		case tar.TypeReg:
			raw, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(name, raw, 0600); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatal("unexpected fixture entry", h.Name)
		}
	}
	return dest
}

func TestCaptureIndependentGitStateWithoutHostConfiguration(t *testing.T) {
	for _, mode := range []string{"checkout", "linked", "split-index", "packed-refs"} {
		t.Run(mode, func(t *testing.T) {
			main := checkoutFixture(t)
			source := main
			if mode == "linked" {
				source = filepath.Join(t.TempDir(), "linked")
				gitFixture(t, main, "worktree", "add", "-b", "feature/task", source)
			}
			if mode == "packed-refs" {
				gitFixture(t, main, "pack-refs", "--all")
			}
			if err := os.WriteFile(filepath.Join(source, "tracked"), []byte("staged"), 0600); err != nil {
				t.Fatal(err)
			}
			gitFixture(t, source, "add", "tracked")
			if mode == "split-index" {
				gitFixture(t, source, "update-index", "--split-index")
			}
			if err := os.WriteFile(filepath.Join(source, "tracked"), []byte("dirty"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "extra"), []byte("untracked"), 0600); err != nil {
				t.Fatal(err)
			}
			before := gitFixture(t, source, "status", "--porcelain")
			config := filepath.Join(main, ".git", "config")
			raw, err := os.ReadFile(config)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(config, append(raw, []byte("\n[credential]\n helper = secret-fixture-value\n[include]\n path = /never-copy-this-host-config\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			archive, err := Capture(context.Background(), source, "sample")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = archive.Close() }()
			dest := extractFixture(t, archive)
			if got := gitFixture(t, dest, "status", "--porcelain"); got != before {
				t.Fatalf("state changed: %q != %q", got, before)
			}
			if got := gitFixture(t, dest, "show", ":tracked"); got != "staged" {
				t.Fatal("index lost", got)
			}
			for _, name := range []string{"commondir", "gitdir", "worktrees", "logs", "hooks"} {
				if _, err := os.Lstat(filepath.Join(dest, ".git", name)); !os.IsNotExist(err) {
					t.Fatal("Host administration copied", name)
				}
			}
			got, err := os.ReadFile(filepath.Join(dest, ".git", "config"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(got), "secret") || strings.Contains(string(got), main) || !strings.Contains(string(got), "haco://sample") {
				t.Fatal("unsafe destination config")
			}
			if err = os.WriteFile(filepath.Join(dest, "tracked"), []byte("independent"), 0600); err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(filepath.Join(source, "tracked"))
			if err != nil || string(original) != "dirty" {
				t.Fatal("source modified", err)
			}
		})
	}
}
func TestCaptureRefusesExternalGitObjectsAndSpecialFiles(t *testing.T) {
	for _, mode := range []string{"alternate", "git-symlink", "working-fifo", "escaping-link", "foreign-worktree"} {
		t.Run(mode, func(t *testing.T) {
			dir := checkoutFixture(t)
			switch mode {
			case "alternate":
				if err := os.WriteFile(filepath.Join(dir, ".git", "objects", "info", "alternates"), []byte("/elsewhere/objects"), 0600); err != nil {
					t.Fatal(err)
				}
			case "git-symlink":
				if err := os.Symlink("/etc/passwd", filepath.Join(dir, ".git", "refs", "evil")); err != nil {
					t.Fatal(err)
				}
			case "working-fifo":
				cmd := exec.Command("mkfifo", filepath.Join(dir, "fifo"))
				if err := cmd.Run(); err != nil {
					t.Fatal(err)
				}
			case "escaping-link":
				if err := os.Symlink("/etc/passwd", filepath.Join(dir, "outside")); err != nil {
					t.Fatal(err)
				}
			case "foreign-worktree":
				other := checkoutFixture(t)
				if err := os.Rename(filepath.Join(dir, ".git"), filepath.Join(dir, "oldgit")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: "+filepath.Join(other, ".git")), 0600); err != nil {
					t.Fatal(err)
				}
			}
			f, err := Capture(context.Background(), dir, "sample")
			if f != nil {
				_ = f.Close()
			}
			if err == nil {
				t.Fatal("unsafe input accepted")
			}
		})
	}
}
