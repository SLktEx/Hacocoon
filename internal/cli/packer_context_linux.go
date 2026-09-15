package cli

import (
	"github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"os"
	"sort"
	"strings"
	"syscall"
)

// The selected context is pinned; neither links nor special files can turn a
// build request into a read outside it or an unbounded FIFO/device operation.
func readPackerContext(directory string) (*basebuild.PackerTemplate, error) {
	before, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() {
		return nil, core.ErrInvalidArgument
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(before, opened) {
		return nil, core.ErrInvalidArgument
	}
	template := new(basebuild.PackerTemplate)
	total, visited := 0, 0
	var walk func(*os.Root, string) error
	walk = func(dir *os.Root, prefix string) error {
		file, err := dir.Open(".")
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		for {
			entries, readErr := file.ReadDir(64)
			for _, entry := range entries {
				visited++
				if visited > 1024 {
					return core.ErrInvalidArgument
				}
				if strings.HasPrefix(entry.Name(), ".") {
					continue
				}
				name := prefix + entry.Name()
				if !basebuild.ValidSourcePath(name) {
					return core.ErrInvalidArgument
				}
				info, err := dir.Lstat(entry.Name())
				if err != nil {
					return err
				}
				if info.IsDir() {
					child, err := dir.OpenRoot(entry.Name())
					if err != nil {
						return err
					}
					current, statErr := child.Stat(".")
					if statErr != nil || !os.SameFile(info, current) {
						_ = child.Close()
						return core.ErrInvalidArgument
					}
					err = walk(child, name+"/")
					closeErr := child.Close()
					if err == nil {
						err = closeErr
					}
					if err != nil {
						return err
					}
					continue
				}
				if !info.Mode().IsRegular() {
					return core.ErrInvalidArgument
				}
				source, err := dir.OpenFile(entry.Name(), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
				if err != nil {
					return err
				}
				current, statErr := source.Stat()
				if statErr != nil || !current.Mode().IsRegular() || !os.SameFile(info, current) || current.Sys().(*syscall.Stat_t).Nlink != 1 {
					_ = source.Close()
					return core.ErrInvalidArgument
				}
				data, readErr := io.ReadAll(io.LimitReader(source, int64(basebuild.MaxContextBytes-total+1)))
				closeErr := source.Close()
				if readErr == nil {
					readErr = closeErr
				}
				if readErr != nil {
					return readErr
				}
				total += len(data)
				if total > basebuild.MaxContextBytes || len(template.Files) >= basebuild.MaxContextFiles {
					return core.ErrInvalidArgument
				}
				template.Files = append(template.Files, basebuild.SourceFile{Path: name, Data: data})
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return readErr
			}
		}
		return nil
	}
	if err := walk(root, ""); err != nil {
		return nil, err
	}
	sort.Slice(template.Files, func(i, j int) bool { return template.Files[i].Path < template.Files[j].Path })
	return template, template.Validate()
}
