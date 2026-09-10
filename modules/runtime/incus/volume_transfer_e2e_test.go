package incus

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/host"
)

// This proves native file-archive portability for synthetic Work/OCI bytes.
// It is not a Hacocoon importer and does not trust imported ownership metadata.
func TestRealIncusVolumeTransferE2E(t *testing.T) {
	testVolumeArchiveTransfer(t, true, "HACO_E2E_INCUS_VOLUME_TRANSFER")
}

// File-only evacuation must not require Incus backup's implicit Btrfs snapshot.
// This is a quiescent synthetic-volume primitive, not whole-WSL recovery.
func TestRealIncusReadableDataEvacuationE2E(t *testing.T) {
	testVolumeArchiveTransfer(t, false, "HACO_E2E_READABLE_DATA_EVACUATION")
}

func testVolumeArchiveTransfer(t *testing.T, native bool, gate string) {
	if os.Getenv(gate) != "1" {
		t.Skip("requires explicit root Incus/Btrfs volume transfer acceptance")
	}
	if os.Geteuid() != 0 {
		t.Fatal("root required for independent volume inspection")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	runner := host.ExecRunner{}
	run := func(name string, args ...string) string {
		t.Helper()
		out, err := runner.Run(ctx, name, args...)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatalf("fixture command %s %v failed: %v", name, args, err)
		}
		return strings.TrimSpace(out.Stdout)
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	owner := hex.EncodeToString(nonce[:])
	sourcePool, targetPool := "haco-export-"+owner, "haco-import-"+owner
	dir, err := os.MkdirTemp("/var/lib", "haco-volume-transfer-")
	if err != nil {
		t.Fatal(err)
	}
	// Record exact intended targets before creating resources. Failure retains them.
	plan, err := os.OpenFile(filepath.Join(dir, "plan.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(plan).Encode(map[string]any{"owner": owner, "pools": []string{sourcePool, targetPool}, "project": "default", "volumes": []string{"work", "oci"}}); err != nil {
		t.Fatal(err)
	}
	if err := plan.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := plan.Close(); err != nil {
		t.Fatal(err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.Sync(); err != nil {
		t.Fatal(err)
	}
	directory.Close()
	t.Logf("exact isolated ownership plan and retained archives: %s", dir)
	const ownerKey = "user.hacocoon.transfer-test"
	for _, pool := range []string{sourcePool, targetPool} {
		run("incus", "storage", "create", pool, "btrfs", "size=1GiB", ownerKey+"="+owner)
	}
	volumePath := func(pool, volume string) string {
		t.Helper()
		p := filepath.Join("/var/lib/incus/storage-pools", pool, "custom", "default_"+volume)
		info, err := os.Lstat(p)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("test volume path unavailable", err)
		}
		return p
	}
	write := func(path, value string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	digest := func(path string) [32]byte { t.Helper(); return sha256.Sum256([]byte(read(path))) }
	for _, volume := range []string{"work", "oci"} {
		run("incus", "storage", "volume", "create", sourcePool, volume, ownerKey+"="+owner, "--project", "default")
		src := volumePath(sourcePool, volume)
		write(filepath.Join(src, "retained"), "uncommitted and persistent bytes\n")
		if volume == "work" {
			run("git", "-C", src, "init", "--initial-branch=main")
			run("git", "-C", src, "add", "retained")
			run("git", "-C", src, "-c", "user.name=Transfer fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "local unpushed commit")
			write(filepath.Join(src, "retained"), "uncommitted and persistent bytes\nmodified\n")
			write(filepath.Join(src, "untracked"), "untracked bytes\n")
		}
		if err := os.Link(filepath.Join(src, "retained"), filepath.Join(src, "hardlink")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("retained", filepath.Join(src, "symlink")); err != nil {
			t.Fatal(err)
		}
		if !native {
			run("python3", "-c", "import os,sys; os.chown(sys.argv[1],100123,100124); os.setxattr(sys.argv[1],b'user.haco-evacuation',b'saved-attribute')", filepath.Join(src, "retained"))
		}
		archive := filepath.Join(dir, volume+".tar")
		if native {
			run("incus", "storage", "volume", "export", sourcePool, volume, archive, "--volume-only", "--compression=none", "--project", "default")
		} else {
			// Source is newly owned, unattached and has no concurrent writer. The
			// archive is outside both pools in the private fixture directory.
			run("env", "-u", "TAR_OPTIONS", "tar", "--acls", "--xattrs", "--numeric-owner", "--sparse", "-cpf", archive, "-C", src, ".")
			if err := os.Chmod(archive, 0600); err != nil {
				t.Fatal(err)
			}
		}
		before := digest(archive)
		if native {
			run("incus", "storage", "volume", "import", targetPool, archive, volume, "--project", "default")
		} else {
			// Extract only this test's own archive into its freshly owned empty
			// volume. This is not an arbitrary user archive importer.
			run("incus", "storage", "volume", "create", targetPool, volume, ownerKey+"="+owner, "--project", "default")
			run("env", "-u", "TAR_OPTIONS", "tar", "--acls", "--xattrs", "--numeric-owner", "--same-owner", "--same-permissions", "-xpf", archive, "-C", volumePath(targetPool, volume))
		}
		dst := volumePath(targetPool, volume)
		if !native {
			run("python3", "-c", "import os,sys; s=os.stat(sys.argv[1]); assert (s.st_uid,s.st_gid)==(100123,100124); assert os.getxattr(sys.argv[1],b'user.haco-evacuation')==b'saved-attribute'", filepath.Join(dst, "retained"))
		}
		if read(filepath.Join(src, "retained")) != read(filepath.Join(dst, "retained")) {
			t.Fatal("import lost bytes")
		}
		a, err := os.Stat(filepath.Join(dst, "retained"))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.Stat(filepath.Join(dst, "hardlink"))
		if err != nil || !os.SameFile(a, b) || a.Mode().Perm() != 0600 {
			t.Fatal("hardlink or mode lost", err)
		}
		if link, err := os.Readlink(filepath.Join(dst, "symlink")); err != nil || link != "retained" {
			t.Fatal("symlink lost", err)
		}
		if volume == "work" {
			if run("git", "-C", src, "rev-parse", "HEAD") != run("git", "-C", dst, "rev-parse", "HEAD") || read(filepath.Join(dst, "untracked")) != "untracked bytes\n" {
				t.Fatal("Git or untracked bytes lost")
			}
			if run("git", "-C", src, "status", "--porcelain") != run("git", "-C", dst, "status", "--porcelain") {
				t.Fatal("Git working state changed")
			}
		}
		// Native import preserves user config; file-only restoration uses the fresh fixture label. Neither establishes production import authority.
		if run("incus", "storage", "volume", "get", targetPool, volume, ownerKey, "--project", "default") != owner {
			t.Fatal("unexpected native imported config")
		}
		write(filepath.Join(dst, "retained"), "destination edit\n")
		if read(filepath.Join(src, "retained")) == "destination edit\n" || digest(archive) != before {
			t.Fatal("import changed source or archive")
		}
		for _, pool := range []string{sourcePool, targetPool} {
			if run("incus", "storage", "get", pool, ownerKey) != owner || run("incus", "storage", "volume", "get", pool, volume, ownerKey, "--project", "default") != owner {
				t.Fatal("refusing foreign cleanup")
			}
			run("incus", "storage", "volume", "delete", pool, volume, "--project", "default")
			if pool == sourcePool && read(filepath.Join(dst, "retained")) != "destination edit\n" {
				t.Fatal("source deletion affected imported data")
			}
		}
		if digest(archive) != before {
			t.Fatal("resource cleanup changed archive")
		}
	}
	for _, pool := range []string{sourcePool, targetPool} {
		if run("incus", "storage", "get", pool, ownerKey) != owner {
			t.Fatal("refusing foreign pool cleanup")
		}
		var volumes []any
		if err := json.Unmarshal([]byte(run("incus", "storage", "volume", "list", pool, "--format=json", "--all-projects")), &volumes); err != nil || len(volumes) != 0 {
			t.Fatal("pool not positively empty", err)
		}
		run("incus", "storage", "delete", pool)
	}
	t.Logf("archive transfer native=%t between independent Btrfs pools passed; archives retained outside both pools; whole-WSL and damaged-storage evacuation not tested", native)
}
