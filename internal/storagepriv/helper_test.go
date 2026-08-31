package storagepriv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type helperCall struct {
	name string
	args []string
}

type fakeHelperRunner struct {
	calls []helperCall
	run   func(string, []string) (host.Result, error)
}

func (r *fakeHelperRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	r.calls = append(r.calls, helperCall{name: name, args: append([]string(nil), args...)})
	if r.run != nil {
		return r.run(name, args)
	}
	return host.Result{ExitCode: 0}, nil
}

func TestHelperRejectsPathsOutsideManagedRootBeforeHostCommands(t *testing.T) {
	root, _, _ := helperFixture(t, "local-default")
	runner := &fakeHelperRunner{}
	helper := testHelper(runner)
	result := helper.Execute(context.Background(), []string{"--root", root, "loop-attach", filepath.Join(t.TempDir(), "evil.raw")})
	if result.ExitCode != helperValidationExit || !strings.Contains(result.Stderr, "outside") {
		t.Fatalf("result = %#v", result)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("host commands ran after path rejection: %#v", runner.calls)
	}
}

func TestHelperRejectsHardlinkedBackingBeforeLoopAttach(t *testing.T) {
	root, backing, _ := helperFixture(t, "local-default")
	victim := filepath.Join(root, "victim")
	if err := os.Remove(backing); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(victim, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(victim, backing); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	runner := &fakeHelperRunner{}
	result := testHelper(runner).Execute(context.Background(), []string{"--root", root, "loop-attach", backing})
	if result.ExitCode != helperValidationExit || !strings.Contains(result.Stderr, "hard links") {
		t.Fatalf("result = %#v", result)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("host commands ran after hardlink rejection: %#v", runner.calls)
	}
}

func TestHelperDetachesNewLoopWhenBackingInodeDoesNotMatch(t *testing.T) {
	root, backing, _ := helperFixture(t, "local-default")
	inode := backingInode(t, backing)
	runner := &fakeHelperRunner{run: func(name string, args []string) (host.Result, error) {
		if name != "losetup" {
			return host.Result{ExitCode: 0}, nil
		}
		switch {
		case len(args) == 4 && args[0] == "--find":
			return host.Result{ExitCode: 0, Stdout: "/dev/loop7\n"}, nil
		case len(args) == 5 && args[0] == "--json":
			return loopJSONWithInode("/dev/loop7", backing, inode+1), nil
		case len(args) == 2 && args[0] == "-d" && args[1] == "/dev/loop7":
			return host.Result{ExitCode: 0}, nil
		default:
			return host.Result{ExitCode: 2, Stderr: "unexpected losetup arguments"}, errors.New("unexpected losetup arguments")
		}
	}}
	result := testHelper(runner).Execute(context.Background(), []string{"--root", root, "loop-attach", backing})
	if result.ExitCode != helperValidationExit || !strings.Contains(result.Stderr, "backing inode") || !strings.Contains(result.Stderr, "was detached") {
		t.Fatalf("result = %#v", result)
	}
	foundDetach := false
	for _, call := range runner.calls {
		if call.name == "losetup" && fmt.Sprint(call.args) == fmt.Sprint([]string{"-d", "/dev/loop7"}) {
			foundDetach = true
		}
	}
	if !foundDetach {
		t.Fatalf("new loop was not detached after identity failure: %#v", runner.calls)
	}
}

func TestHelperRefusesFormattingExistingFilesystem(t *testing.T) {
	root, backing, _ := helperFixture(t, "local-default")
	runner := &fakeHelperRunner{run: func(name string, args []string) (host.Result, error) {
		switch name {
		case "losetup":
			return loopJSON("/dev/loop7", backing), nil
		case "blkid":
			return host.Result{ExitCode: 0, Stdout: "ext4\n"}, nil
		case "mkfs.btrfs":
			t.Fatal("mkfs.btrfs ran despite existing filesystem")
		}
		return host.Result{ExitCode: 0}, nil
	}}
	result := testHelper(runner).Execute(context.Background(), []string{"--root", root, "fs-format-btrfs", "/dev/loop7"})
	if result.ExitCode != helperValidationExit || !strings.Contains(result.Stderr, "existing filesystem type") {
		t.Fatalf("result = %#v", result)
	}
}

func TestHelperMountsOnlyMatchingManagedLoopAndMountpoint(t *testing.T) {
	root, backing, mountpoint := helperFixture(t, "local-default")
	mounted := false
	runner := &fakeHelperRunner{run: func(name string, args []string) (host.Result, error) {
		switch name {
		case "losetup":
			return loopJSON("/dev/loop7", backing), nil
		case "findmnt":
			if mounted {
				return host.Result{ExitCode: 0, Stdout: "/dev/loop7\n"}, nil
			}
			return host.Result{ExitCode: 1}, errors.New("not mounted")
		case "mount":
			mounted = true
			return host.Result{ExitCode: 0}, nil
		default:
			return host.Result{ExitCode: 0}, nil
		}
	}}
	result := testHelper(runner).Execute(context.Background(), []string{"--root", root, "mount-btrfs", "/dev/loop7", mountpoint})
	if result.ExitCode != 0 {
		t.Fatalf("mount result = %#v", result)
	}
	foundMount := false
	for _, call := range runner.calls {
		if call.name == "mount" && fmt.Sprint(call.args) == fmt.Sprint([]string{"/dev/loop7", mountpoint, "-o", "compress=zstd:3"}) {
			foundMount = true
		}
	}
	if !foundMount {
		t.Fatalf("mount call missing: %#v", runner.calls)
	}
}

func TestHelperRejectsMismatchedLoopForMountpoint(t *testing.T) {
	root, _, mountpoint := helperFixture(t, "local-default")
	otherBacking := filepath.Join(root, "images", "other.raw")
	if err := os.WriteFile(otherBacking, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeHelperRunner{run: func(name string, args []string) (host.Result, error) {
		if name == "losetup" {
			return loopJSON("/dev/loop8", otherBacking), nil
		}
		return host.Result{ExitCode: 0}, nil
	}}
	result := testHelper(runner).Execute(context.Background(), []string{"--root", root, "mount-btrfs", "/dev/loop8", mountpoint})
	if result.ExitCode != helperValidationExit || !strings.Contains(result.Stderr, "does not match") {
		t.Fatalf("result = %#v", result)
	}
}

func TestHelperRejectsArbitraryOperation(t *testing.T) {
	root, _, _ := helperFixture(t, "local-default")
	runner := &fakeHelperRunner{}
	result := testHelper(runner).Execute(context.Background(), []string{"--root", root, "exec", "sh", "-c", "id"})
	if result.ExitCode != helperValidationExit || !strings.Contains(result.Stderr, "unsupported privileged storage operation") {
		t.Fatalf("result = %#v", result)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("arbitrary operation reached runner: %#v", runner.calls)
	}
}

func TestHelperCallerUIDHonorsDirectSudoInvocation(t *testing.T) {
	h := NewHelper(&fakeHelperRunner{})
	h.euid = func() int { return 0 }
	h.executable = func() (string, error) { return "/trusted/haco-storage-helper", nil }
	h.getenv = func(name string) string {
		switch name {
		case "SUDO_UID":
			return "1001"
		case "SUDO_COMMAND":
			return "/trusted/haco-storage-helper --root /tmp/haco loop-find /tmp/haco/images/local.raw"
		default:
			return ""
		}
	}
	uid, err := h.callerUID()
	if err != nil {
		t.Fatal(err)
	}
	if uid != 1001 {
		t.Fatalf("caller uid = %d, want 1001", uid)
	}
}

func TestHelperCallerUIDIgnoresInheritedOuterSudoContextWhenAlreadyRoot(t *testing.T) {
	h := NewHelper(&fakeHelperRunner{})
	h.euid = func() int { return 0 }
	h.executable = func() (string, error) { return "/usr/local/libexec/hacocoon/haco-storage-helper", nil }
	h.getenv = func(name string) string {
		switch name {
		case "SUDO_UID":
			return "1001"
		case "SUDO_COMMAND":
			return "/usr/local/bin/haco host ensure"
		default:
			return ""
		}
	}
	uid, err := h.callerUID()
	if err != nil {
		t.Fatal(err)
	}
	if uid != 0 {
		t.Fatalf("caller uid = %d, want root for direct helper call", uid)
	}
}

func TestHelperCallerUIDRejectsMalformedUIDForDirectSudoInvocation(t *testing.T) {
	h := NewHelper(&fakeHelperRunner{})
	h.euid = func() int { return 0 }
	h.executable = func() (string, error) { return "/trusted/haco-storage-helper", nil }
	h.getenv = func(name string) string {
		switch name {
		case "SUDO_UID":
			return "not-a-uid"
		case "SUDO_COMMAND":
			return "/trusted/haco-storage-helper --root /tmp/haco loop-find /tmp/haco/images/local.raw"
		default:
			return ""
		}
	}
	if _, err := h.callerUID(); err == nil || !strings.Contains(err.Error(), "invalid SUDO_UID") {
		t.Fatalf("callerUID error = %v", err)
	}
}

func testHelper(runner host.Runner) *Helper {
	h := NewHelper(runner)
	h.euid = func() int { return 0 }
	h.executable = func() (string, error) { return "/trusted/haco-storage-helper", nil }
	h.getenv = func(name string) string {
		switch name {
		case "SUDO_UID":
			return strconv.Itoa(os.Geteuid())
		case "SUDO_COMMAND":
			return "/trusted/haco-storage-helper --root /tmp/haco loop-attach /tmp/haco/images/local.raw"
		default:
			return ""
		}
	}
	h.resolveTool = func(name string) (string, error) { return name, nil }
	return h
}

func helperFixture(t *testing.T, id string) (root, backing, mountpoint string) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "haco-root")
	images := filepath.Join(root, "images")
	mounts := filepath.Join(root, "mounts")
	if err := os.MkdirAll(images, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mounts, 0o700); err != nil {
		t.Fatal(err)
	}
	backing = filepath.Join(images, id+".raw")
	if err := os.WriteFile(backing, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	mountpoint = filepath.Join(mounts, id)
	if err := os.Mkdir(mountpoint, 0o700); err != nil {
		t.Fatal(err)
	}
	return root, backing, mountpoint
}

func backingInode(t *testing.T, backing string) uint64 {
	t.Helper()
	info, err := os.Lstat(backing)
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("backing file has no syscall.Stat_t: %#v", info.Sys())
	}
	return stat.Ino
}

func loopJSON(device, backing string) host.Result {
	return loopJSONWithInode(device, backing, mustBackingInode(backing))
}

func loopJSONWithInode(device, backing string, inode uint64) host.Result {
	payload := struct {
		LoopDevices []struct {
			Name      string `json:"name"`
			BackFile  string `json:"back-file"`
			BackInode uint64 `json:"back-ino"`
		} `json:"loopdevices"`
	}{}
	payload.LoopDevices = append(payload.LoopDevices, struct {
		Name      string `json:"name"`
		BackFile  string `json:"back-file"`
		BackInode uint64 `json:"back-ino"`
	}{Name: device, BackFile: backing, BackInode: inode})
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return host.Result{ExitCode: 0, Stdout: string(data)}
}

func mustBackingInode(backing string) uint64 {
	info, err := os.Lstat(backing)
	if err != nil {
		panic(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		panic("backing file has no syscall.Stat_t")
	}
	return stat.Ino
}
