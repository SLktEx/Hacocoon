//go:build linux

package workspaceinput

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Capture never executes source Git, filters, hooks or configuration. It copies
// independent objects/refs and the selected worktree's HEAD/index. Host config,
// hooks, logs and other worktree administration are excluded.
func Capture(ctx context.Context, source, repository string) (result *os.File, err error) {
	if !gitadapter.ValidID(repository) {
		return nil, ErrInvalid
	}
	abs, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	root, err := openDirectory(abs)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	gitPath := filepath.Join(abs, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		return nil, err
	}
	if info.Mode().IsRegular() {
		raw, e := readSmall(root, ".git")
		if e != nil {
			return nil, e
		}
		if !strings.HasPrefix(raw, "gitdir: ") {
			return nil, ErrInvalid
		}
		gitPath = resolve(abs, strings.TrimSpace(strings.TrimPrefix(raw, "gitdir: ")))
	} else if !info.IsDir() {
		return nil, ErrInvalid
	}
	git, err := openDirectory(gitPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = git.Close() }()
	common := git
	if info.Mode().IsRegular() {
		rel, e := readSmall(git, "commondir")
		if e != nil {
			return nil, e
		}
		commonPath := resolve(gitPath, strings.TrimSpace(rel))
		if filepath.Dir(filepath.Dir(gitPath)) != commonPath || filepath.Base(filepath.Dir(gitPath)) != "worktrees" {
			return nil, ErrInvalid
		}
		back, e := readSmall(git, "gitdir")
		if e != nil {
			return nil, e
		}
		if resolve(gitPath, strings.TrimSpace(back)) != filepath.Join(abs, ".git") {
			return nil, ErrInvalid
		}
		common, err = openDirectory(commonPath)
		if err != nil {
			return nil, err
		}
		defer func() { _ = common.Close() }()
	}
	// Alternate/promisor repositories are not self-contained. Do not read paths
	// designated by their metadata or silently produce an incomplete copy.
	for _, name := range []string{"objects/info/alternates", "objects/info/http-alternates"} {
		if _, e := statAt(common, name); !errors.Is(e, os.ErrNotExist) {
			return nil, ErrInvalid
		}
	}
	for _, dir := range []*os.File{git, common} {
		if _, e := statAt(dir, "info/sparse-checkout"); !errors.Is(e, os.ErrNotExist) {
			return nil, ErrInvalid
		}
	}
	configuration, e := readSmall(common, "config")
	if e != nil {
		return nil, e
	}
	if strings.Contains(strings.ToLower(configuration), "sha256") {
		return nil, ErrInvalid
	}
	head, err := readSmall(git, "HEAD")
	if err != nil {
		return nil, err
	}
	branch := ""
	if strings.HasPrefix(head, "ref: refs/heads/") {
		branch = strings.TrimSpace(strings.TrimPrefix(head, "ref: refs/heads/"))
		if !gitadapter.ValidHeadRef("refs/heads/" + branch) {
			return nil, ErrInvalid
		}
	} else if !sha1Name(strings.TrimSpace(head)) {
		return nil, ErrInvalid
	}
	f, err := os.CreateTemp("", "haco-workspace-input-")
	if err != nil {
		return nil, err
	}
	defer func() {
		if result == nil {
			err = errors.Join(err, f.Close())
		}
	}()
	if err = os.Remove(f.Name()); err != nil {
		return nil, err
	}
	w := tar.NewWriter(&captureWriter{ctx: ctx, file: f, remaining: Limit})
	defer func() {
		if result == nil {
			_ = w.Close()
		}
	}()
	directory := func(name string) error {
		return w.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0700})
	}
	if err = directory("tree"); err != nil {
		return nil, err
	}
	if err = captureTree(ctx, w, root, ".", "tree", false); err != nil {
		return nil, err
	}
	if err = directory("tree/.git"); err != nil {
		return nil, err
	}
	for _, name := range []string{"objects", "refs"} {
		if err = directory("tree/.git/" + name); err != nil {
			return nil, err
		}
		if err = captureTree(ctx, w, common, name, "tree/.git/"+name, true); err != nil {
			return nil, err
		}
	}
	for _, pair := range []struct {
		root *os.File
		name string
	}{{git, "HEAD"}, {git, "index"}, {common, "packed-refs"}, {common, "shallow"}} {
		if err = captureFile(w, pair.root, pair.name, "tree/.git/"+pair.name, true); err != nil && (pair.name == "HEAD" || !errors.Is(err, os.ErrNotExist)) {
			return nil, err
		}
	}
	// Split-index extensions refer to immutable sharedindex files by digest.
	dirs := []*os.File{git}
	if common != git {
		dirs = append(dirs, common)
	}
	copiedIndex := map[string]bool{}
	for _, dir := range dirs {
		names, e := directoryNames(dir, ".")
		if e != nil {
			return nil, e
		}
		for _, name := range names {
			if strings.HasPrefix(name, "sharedindex.") && !copiedIndex[name] {
				copiedIndex[name] = true
				if !sha1Name(strings.TrimPrefix(name, "sharedindex.")) {
					return nil, ErrInvalid
				}
				if err = captureFile(w, dir, name, "tree/.git/"+name, true); err != nil {
					return nil, err
				}
			}
		}
	}
	config := "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = false\n[remote \"origin\"]\n\turl = haco://" + repository + "\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n"
	if branch != "" {
		config += fmt.Sprintf("[branch %q]\n\tremote = origin\n\tmerge = %q\n", branch, "refs/heads/"+branch)
	}
	if err = w.WriteHeader(&tar.Header{Name: "tree/.git/config", Typeflag: tar.TypeReg, Mode: 0600, Size: int64(len(config))}); err != nil {
		return nil, err
	}
	if _, err = io.WriteString(w, config); err != nil {
		return nil, err
	}
	if err = w.Close(); err != nil {
		return nil, err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if err = CopyTree(ctx, f, func(_ *tar.Header, r io.Reader) error { _, e := io.Copy(io.Discard, r); return e }); err != nil {
		return nil, err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return f, nil
}

func resolve(base, name string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.Clean(filepath.Join(base, name))
}
func sha1Name(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
func openDirectory(path string) (*os.File, error) {
	fd, err := unix.Openat2(unix.AT_FDCWD, path, &unix.OpenHow{Flags: unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC, Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS})
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	var st unix.Stat_t
	if unix.Fstat(fd, &st) != nil || st.Uid != uint32(os.Geteuid()) || st.Mode&0022 != 0 {
		_ = f.Close()
		return nil, ErrInvalid
	}
	return f, nil
}
func openAt(root *os.File, name string, flags uint64) (*os.File, error) {
	fd, err := unix.Openat2(int(root.Fd()), name, &unix.OpenHow{Flags: flags | unix.O_CLOEXEC | unix.O_NONBLOCK, Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS})
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}
func statAt(root *os.File, name string) (unix.Stat_t, error) {
	var st unix.Stat_t
	err := unix.Fstatat(int(root.Fd()), name, &st, unix.AT_SYMLINK_NOFOLLOW)
	return st, err
}
func readSmall(root *os.File, name string) (string, error) {
	f, err := openAt(root, name, unix.O_RDONLY)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() > 4096 {
		return "", ErrInvalid
	}
	b, err := io.ReadAll(io.LimitReader(f, 4097))
	if len(b) > 4096 {
		return "", ErrInvalid
	}
	return string(b), err
}
func directoryNames(root *os.File, name string) ([]string, error) {
	f, err := openAt(root, name, unix.O_RDONLY|unix.O_DIRECTORY)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	names, err := f.Readdirnames(-1)
	sort.Strings(names)
	return names, err
}
func captureTree(ctx context.Context, w *tar.Writer, root *os.File, from, to string, metadata bool) error {
	before, err := statAt(root, from)
	if err != nil {
		return err
	}
	names, err := directoryNames(root, from)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		if from == "." && !metadata && (name == ".git" || name == ".haco-workspace.json" || name == ".haco-workspace.lock") {
			continue
		}
		if metadata && (strings.HasSuffix(name, ".promisor") || name == "alternates" || name == "http-alternates") {
			return ErrInvalid
		}
		if !metadata && name == ".git" {
			return ErrInvalid
		}
		src, dst := filepath.Join(from, name), to+"/"+name
		st, err := statAt(root, src)
		if err != nil {
			return err
		}
		if st.Mode&unix.S_IFMT == unix.S_IFDIR {
			if err = w.WriteHeader(&tar.Header{Name: dst, Typeflag: tar.TypeDir, Mode: int64(st.Mode & 0777)}); err != nil {
				return err
			}
			if err = captureTree(ctx, w, root, src, dst, metadata); err != nil {
				return err
			}
		} else if err = captureFile(w, root, src, dst, metadata); err != nil {
			return err
		}
	}
	after, err := statAt(root, from)
	if err != nil {
		return err
	}
	if before.Mtim != after.Mtim || before.Ctim != after.Ctim || before.Ino != after.Ino {
		return ErrInvalid
	}
	return nil
}
func captureFile(w *tar.Writer, root *os.File, from, to string, metadata bool) error {
	f, err := openAt(root, from, unix.O_RDONLY)
	if errors.Is(err, unix.ELOOP) && !metadata {
		// Read the link itself, never its target. The archive validator confines it.
		var buf [4097]byte
		n, e := unix.Readlinkat(int(root.Fd()), from, buf[:])
		if e != nil {
			return e
		}
		if n == len(buf) {
			return ErrInvalid
		}
		return w.WriteHeader(&tar.Header{Name: to, Typeflag: tar.TypeSymlink, Mode: 0777, Linkname: string(buf[:n])})
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	var before, after unix.Stat_t
	if unix.Fstat(int(f.Fd()), &before) != nil || before.Mode&unix.S_IFMT != unix.S_IFREG {
		return ErrInvalid
	}
	if err = w.WriteHeader(&tar.Header{Name: to, Typeflag: tar.TypeReg, Mode: int64(before.Mode & 0777), Size: before.Size}); err != nil {
		return err
	}
	if _, err = io.CopyN(w, f, before.Size); err != nil {
		return err
	}
	if unix.Fstat(int(f.Fd()), &after) != nil || before.Mtim != after.Mtim || before.Ctim != after.Ctim || before.Size != after.Size {
		return fmt.Errorf("source changed during copy: %w", ErrInvalid)
	}
	return nil
}

type captureWriter struct {
	ctx       context.Context
	file      io.Writer
	remaining int64
}

func (w *captureWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > w.remaining {
		return 0, ErrInvalid
	}
	n, err := w.file.Write(p)
	w.remaining -= int64(n)
	return n, err
}
