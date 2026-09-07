//go:build linux

package capability

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const maxSavedPolicyBytes = 1 << 20

// Remember updates only saved_decisions. Administrator rules remain untouched.
func (e *FilePolicyEvaluator) Remember(ctx context.Context, request core.CapabilityRequest, choice SavedChoice) error {
	rule, err := RuleForSavedChoice(request, choice)
	if err != nil {
		return err
	}
	if e == nil || !filepath.IsAbs(e.path) {
		return core.ErrInvalidArgument
	}
	parent, name := filepath.Dir(e.path), filepath.Base(e.path)
	info, err := os.Lstat(parent)
	if err != nil || !safePolicyFile(info, true) {
		return fmt.Errorf("unsafe Policy directory")
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer root.Close()
	actual, err := root.Stat(".")
	if err != nil || !os.SameFile(info, actual) {
		return core.ErrIncompatibleState
	}
	lock, err := root.OpenFile(".policy-save.lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	info, err = lock.Stat()
	if err != nil || !safePolicyFile(info, false) {
		return fmt.Errorf("unsafe Policy lock")
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("Policy update busy: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	before, err := readSavedPolicy(root, name)
	if err != nil {
		return err
	}
	policy := PolicyFile{Default: core.PolicyDeny}
	if before != nil {
		policy, err = decodePolicy(before)
		if err != nil {
			return err
		}
	}
	replaced := false
	for i, existing := range policy.SavedDecisions {
		if existing.Capability == rule.Capability && existing.Action == rule.Action && existing.Resource == rule.Resource && existing.Environment == rule.Environment && existing.EnvironmentInstance == rule.EnvironmentInstance && maps.Equal(existing.Attributes, rule.Attributes) {
			policy.SavedDecisions[i] = rule
			replaced = true
			break
		}
	}
	if !replaced {
		policy.SavedDecisions = append(policy.SavedDecisions, rule)
	}
	if err = validatePolicy(policy); err != nil {
		return err
	}
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > maxSavedPolicyBytes {
		return fmt.Errorf("Policy size limit exceeded")
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return err
	}
	temporary := ".policy-" + hex.EncodeToString(nonce[:])
	f, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	latest, err := readSavedPolicy(root, name)
	if err != nil {
		return err
	}
	if !bytes.Equal(before, latest) {
		return fmt.Errorf("Policy changed during update")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = root.Rename(temporary, name); err != nil {
		return err
	}
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
func safePolicyFile(info os.FileInfo, directory bool) bool {
	if info == nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return false
	}
	if directory {
		return info.IsDir() && info.Mode().Perm()&0022 == 0
	}
	return info.Mode().IsRegular() && info.Mode().Perm()&0077 == 0 && stat.Nlink == 1
}
func readSavedPolicy(root *os.Root, name string) ([]byte, error) {
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !safePolicyFile(info, false) {
		return nil, fmt.Errorf("unsafe Policy file")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxSavedPolicyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSavedPolicyBytes {
		return nil, fmt.Errorf("Policy size limit exceeded")
	}
	return data, nil
}
