//go:build linux

package recipes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const recipeFile = "recipe.sh"

type store struct {
	root *os.Root
	lock *os.File
}

func private(info os.FileInfo, directory bool) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid()) && info.Mode().Perm()&0077 == 0 &&
		((directory && info.IsDir()) || (!directory && info.Mode().IsRegular() && stat.Nlink == 1))
}
func openStore(path string) (*store, error) {
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil || !private(info, true) {
		return nil, fmt.Errorf("unsafe setup recipe directory")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	observed, err := root.Stat(".")
	if err != nil || !os.SameFile(info, observed) {
		root.Close()
		return nil, fmt.Errorf("setup recipe directory changed")
	}
	s := &store{root: root}
	lock, err := root.OpenFile("setup.lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		s.close()
		return nil, err
	}
	s.lock = lock
	info, err = lock.Stat()
	if err != nil || !private(info, false) {
		s.close()
		return nil, fmt.Errorf("unsafe setup recipe lock")
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		s.close()
		return nil, fmt.Errorf("setup recipe is already running")
	}
	return s, nil
}
func (s *store) close() {
	if s.lock != nil {
		s.lock.Close()
	}
	if s.root != nil {
		s.root.Close()
	}
}
func (s *store) read() ([]byte, error) {
	content, err := s.readFile(recipeFile)
	if err != nil {
		return nil, err
	}
	text := string(content)
	if err := (Update{Script: &text}).Validate(); err != nil {
		return nil, err
	}
	return content, nil
}
func (s *store) readFile(name string) ([]byte, error) {
	f, err := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open setup recipe recipe: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !private(info, false) {
		return nil, fmt.Errorf("unsafe setup recipe recipe")
	}
	content, err := io.ReadAll(io.LimitReader(f, MaxScriptBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > MaxScriptBytes {
		return nil, fmt.Errorf("setup state exceeds size limit")
	}
	return content, nil
}
func (s *store) sync() error {
	dir, err := s.root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (s *store) save(content []byte) error {
	return s.saveFile(recipeFile, content)
}
func (s *store) saveFile(target string, content []byte) error {
	if len(content) > MaxScriptBytes {
		return fmt.Errorf("setup state exceeds size limit")
	}
	if _, err := s.readFile(target); err != nil {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	name := ".recipe-" + hex.EncodeToString(nonce[:])
	f, err := s.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer s.root.Remove(name)
	if _, err := f.Write(content); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := s.root.Rename(name, target); err != nil {
		return err
	}
	return s.sync()
}
func (s *store) remove() error {
	content, err := s.read()
	if err != nil || content == nil {
		return err
	}
	if err := s.root.Remove(recipeFile); err != nil {
		return err
	}
	return s.sync()
}

func openInput(path string) (*os.File, error) {
	// The supported Windows entry is the Linux client launched by wsl.exe.
	// Let WSL resolve its configured drive mounts; no drive-letter map is guessed.
	if len(path) >= 3 && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, "/usr/bin/wslpath", "-u", path).Output()
		if err != nil {
			return nil, fmt.Errorf("Windows script paths require the WSL Physical Host client")
		}
		path = strings.TrimSuffix(string(output), "\n")
		if !filepath.IsAbs(path) || strings.ContainsAny(path, "\r\n\x00") {
			return nil, fmt.Errorf("invalid WSL script path")
		}
	}
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
