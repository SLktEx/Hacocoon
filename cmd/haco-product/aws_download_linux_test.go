//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	awsplugin "github.com/SLktEx/Hacocoon/modules/capability/aws"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type downloadClientFunc func(context.Context, awsplugin.GetSpec, io.Writer) (core.CapabilityResult, error)

func (f downloadClientFunc) DownloadS3(c context.Context, s awsplugin.GetSpec, w io.Writer) (core.CapabilityResult, error) {
	return f(c, s, w)
}
func downloadResult(data []byte) core.CapabilityResult {
	hash := sha256.Sum256(data)
	encoded, _ := json.Marshal(awsplugin.DownloadReceipt{Bytes: int64(len(data)), SHA256: hex.EncodeToString(hash[:])})
	return core.CapabilityResult{Provider: awsplugin.Capability, Output: string(encoded), RequestID: "fixed", ExecutionState: core.CapabilitySucceeded, AuditComplete: true}
}
func TestDownloadPublishesOnlyVerifiedAuditedCompletion(t *testing.T) {
	for _, mode := range []string{"ok", "denied", "partial", "digest", "audit", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			parent := t.TempDir()
			path := filepath.Join(parent, "result")
			os.WriteFile(path, []byte("old"), 0600)
			data := []byte{0, 255, 10, 11}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := downloadClientFunc(func(_ context.Context, _ awsplugin.GetSpec, w io.Writer) (core.CapabilityResult, error) {
				if mode == "denied" {
					return core.CapabilityResult{}, core.ErrPolicyDenied
				}
				if _, err := w.Write(data); err != nil {
					return core.CapabilityResult{}, err
				}
				result := downloadResult(data)
				if mode == "partial" {
					return result, io.ErrUnexpectedEOF
				}
				if mode == "digest" {
					result.Output = "{}"
				}
				if mode == "audit" {
					result.AuditComplete = false
				}
				if mode == "cancel" {
					cancel()
				}
				return result, nil
			})
			err := saveAWSDownload(ctx, client, awsplugin.GetSpec{}, path)
			actual, _ := os.ReadFile(path)
			if mode == "ok" {
				if err != nil || string(actual) != string(data) {
					t.Fatal(actual, err)
				}
			} else if err == nil || string(actual) != "old" {
				t.Fatal("changed before verified success", mode, actual, err)
			}
			entries, _ := os.ReadDir(parent)
			if len(entries) != 1 {
				t.Fatal("temporary data retained", entries)
			}
		})
	}
}
func TestDownloadCannotFollowDestinationSymlinkOrParentReplacement(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	os.WriteFile(outside, []byte("protected"), 0600)
	parent := filepath.Join(root, "chosen")
	os.Mkdir(parent, 0700)
	path := filepath.Join(parent, "result")
	os.Symlink(outside, path)
	client := downloadClientFunc(func(context.Context, awsplugin.GetSpec, io.Writer) (core.CapabilityResult, error) {
		t.Fatal("followed symlink")
		return core.CapabilityResult{}, nil
	})
	if err := saveAWSDownload(context.Background(), client, awsplugin.GetSpec{}, path); err == nil {
		t.Fatal("symlink accepted")
	}
	os.Remove(path)
	data := []byte("new")
	moved := filepath.Join(root, "moved")
	client = downloadClientFunc(func(_ context.Context, _ awsplugin.GetSpec, w io.Writer) (core.CapabilityResult, error) {
		if err := os.Rename(parent, moved); err != nil {
			return core.CapabilityResult{}, err
		}
		os.Mkdir(parent, 0700)
		os.WriteFile(path, []byte("other directory"), 0600)
		if _, err := w.Write(data); err != nil {
			return core.CapabilityResult{}, err
		}
		return downloadResult(data), nil
	})
	if err := saveAWSDownload(context.Background(), client, awsplugin.GetSpec{}, path); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(filepath.Join(moved, "result"))
	replacement, _ := os.ReadFile(path)
	protected, _ := os.ReadFile(outside)
	if string(original) != "new" || string(replacement) != "other directory" || string(protected) != "protected" {
		t.Fatal("publication followed replacement")
	}
	if _, err := os.Stat(filepath.Join(moved, "not-present")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestDownloadDoesNotAdoptSubstitutedPrivateDirectory(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "result")
	var moved string
	data := []byte("download")
	client := downloadClientFunc(func(_ context.Context, _ awsplugin.GetSpec, w io.Writer) (core.CapabilityResult, error) {
		entries, _ := os.ReadDir(parent)
		if len(entries) != 1 {
			return core.CapabilityResult{}, errors.New("unexpected staging")
		}
		path := filepath.Join(parent, entries[0].Name())
		moved = path + "-moved"
		if err := os.Rename(path, moved); err != nil {
			return core.CapabilityResult{}, err
		}
		os.Mkdir(path, 0700)
		os.WriteFile(filepath.Join(path, "content"), []byte("foreign"), 0600)
		if _, err := w.Write(data); err != nil {
			return core.CapabilityResult{}, err
		}
		return downloadResult(data), nil
	})
	if err := saveAWSDownload(context.Background(), client, awsplugin.GetSpec{}, destination); err == nil {
		t.Fatal("ambiguous cleanup was success")
	}
	actual, _ := os.ReadFile(destination)
	if string(actual) != "download" {
		t.Fatal("published substituted data")
	}
	entries, _ := os.ReadDir(parent)
	found := false
	for _, entry := range entries {
		if entry.IsDir() && filepath.Join(parent, entry.Name()) != moved {
			foreign, _ := os.ReadFile(filepath.Join(parent, entry.Name(), "content"))
			if string(foreign) != "foreign" {
				t.Fatal("removed substituted content")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing foreign directory")
	}
}
