//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCompactionRefusesCancellationAndNonVirtualFile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var p *pinnedDisk
	result, err := p.compact(ctx)
	if !errors.Is(err, context.Canceled) || result.Attempted {
		t.Fatal(result, err)
	}
	path := filepath.Join(t.TempDir(), "invalid.vhdx")
	if err := os.WriteFile(path, []byte("retained ordinary file"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err = pinDisk(path)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	result, err = p.compact(context.Background())
	if err == nil || result.Attempted {
		t.Fatal("non-virtual file accepted", result, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "retained ordinary file" {
		t.Fatal("invalid target changed", err)
	}
}

func TestDedicatedWSLVHDCompaction(t *testing.T) {
	if os.Getenv("HACO_E2E_RECLAIM_COMPACT") != "1" {
		t.Skip("requires separate exact offline managed-WSL disk authorization")
	}
	path := os.Getenv("HACO_E2E_RECLAIM_VHD")
	p, err := pinDisk(path)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	result, err := p.compact(context.Background())
	if err != nil {
		t.Fatalf("compaction failed; observation=%+v error=%v", result, err)
	}
	if !result.Attempted || !result.Completed {
		t.Fatal("compaction completion unproven", result)
	}
	t.Logf("PASS native compact; before=%+v after=%+v virtual=%+v; filesystem bytes/resume checked separately", result.Before, result.After, result.Virtual)
}
