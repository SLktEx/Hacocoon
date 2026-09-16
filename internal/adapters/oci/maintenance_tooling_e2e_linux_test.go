//go:build linux

package oci

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt in to a public, pinned download; no downloaded executable is run here.
func TestRealMaintenanceToolingDownloadE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_OCI_TOOLING_DOWNLOAD") != "1" {
		t.Skip("opt in for real pinned tool download and extraction")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	tools := &MaintenanceTooling{Directory: filepath.Join(t.TempDir(), "cache")}
	directory, release, err := tools.Prepare(ctx)
	if err != nil {
		t.Fatal("real maintenance tool acquisition", err)
	}
	defer func() {
		if err := release(); err != nil {
			t.Error(err)
		}
	}()
	for _, name := range []string{"containerd", "ctr", "nerdctl"} {
		file, err := os.Open(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		var header [4]byte
		n, readErr := file.Read(header[:])
		file.Close()
		if readErr != nil || n != 4 || header != [4]byte{0x7f, 'E', 'L', 'F'} {
			t.Fatal("invalid verified release member", name, readErr)
		}
	}
	t.Log("PASS real HTTPS acquisition, pinned full-archive hash and fixed member extraction; no Host execution")
}
