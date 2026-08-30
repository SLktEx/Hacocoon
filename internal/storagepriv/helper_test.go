package storagepriv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	runner := &fakeHelperRunner{run: func(name string, args []string) (host.Result, error) {
		switch name {
		case "losetup":
			return loopJSON("/dev/loop7", backing), nil
		case "findmnt":
			return host.Result{ExitCode: 1}, errors.New("not mounted")
		case "mount":
			return host.Result{ExitCode: 0}, nil
		default:
			return host.Result{ExitCode: 0}, nil
		}
	}}
	result := testHelper(runner).Execute(context.Background(), []string{"--root", root, "mount-btrfs", "/dev/loop7", mountpoint})
	if result.ExitCode != 0 {
		t.Fatalf("mount result = %#v", result)
	}
	last := runner.calls[len(runner.calls)-1]
	if last.name != "mount" || fmt.Sprint(last.args) != fmt.Sprint([]string{"/dev/loop7", mountpoint, "-o", "compress=zstd:3"}) {
		t.Fatalf("mount call = %#v", last)
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

func testHelper(runner host.Runner) *Helper {
	h := NewHelper(runner)
	h.euid = func() int { return 0 }
	h.getenv = func(name string) string {
		if name == "SUDO_UID" {
			return strconv.Itoa(os.Geteuid())
		}
		return ""
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

func loopJSON(device, backing string) host.Result {
	data := fmt.Sprintf(`{"loopdevices":[{"name":%q,"back-file":%q}]}`, device, backing)
	return host.Result{ExitCode: 0, Stdout: data}
}
