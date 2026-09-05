package storagepriv

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type recordingHostRunner struct {
	name string
	args []string
}

func (r *recordingHostRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	return host.Result{ExitCode: 0}, nil
}

func TestTranslatePrivilegedCommandAllowlist(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		args []string
		op   string
		out  []string
	}{
		{name: "attach", cmd: "losetup", args: []string{"--find", "--show", "/var/lib/hacocoon/images/local-default.raw"}, op: "loop-attach", out: []string{"/var/lib/hacocoon/images/local-default.raw"}},
		{name: "filesystem type", cmd: "blkid", args: []string{"-o", "value", "-s", "TYPE", "/dev/loop3"}, op: "fs-type", out: []string{"/dev/loop3"}},
		{name: "format", cmd: "mkfs.btrfs", args: []string{"-f", "/dev/loop3"}, op: "fs-format-btrfs", out: []string{"/dev/loop3"}},
		{name: "mount", cmd: "mount", args: []string{"/dev/loop3", "/var/lib/hacocoon/mounts/local-default", "-o", managedBtrfsMountOptions}, op: "mount-btrfs", out: []string{"/dev/loop3", "/var/lib/hacocoon/mounts/local-default"}},
		{name: "remount", cmd: "mount", args: []string{"/dev/loop3", "/var/lib/hacocoon/mounts/local-default", "-o", "remount," + managedBtrfsMountOptions}, op: "remount-btrfs", out: []string{"/dev/loop3", "/var/lib/hacocoon/mounts/local-default"}},
		{name: "resize", cmd: "btrfs", args: []string{"filesystem", "resize", "max", "/var/lib/hacocoon/mounts/local-default"}, op: "btrfs-resize", out: []string{"/var/lib/hacocoon/mounts/local-default", "max"}},
		{name: "balance", cmd: "btrfs", args: []string{"balance", "start", "-dusage=50", "-musage=50", "/var/lib/hacocoon/mounts/local-default"}, op: "btrfs-balance", out: []string{"/var/lib/hacocoon/mounts/local-default", "usage=50"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, out, privileged, err := translatePrivilegedCommand(tt.cmd, tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if !privileged || op != tt.op || !reflect.DeepEqual(out, tt.out) {
				t.Fatalf("translation = op=%q args=%#v privileged=%v", op, out, privileged)
			}
		})
	}
}

func TestTranslatePrivilegedCommandRejectsArbitraryAuthority(t *testing.T) {
	tests := []struct {
		cmd  string
		args []string
	}{
		{cmd: "mount", args: []string{"/dev/sda", "/", "-o", "bind"}},
		{cmd: "mount", args: []string{"/dev/loop3", "/var/lib/hacocoon/mounts/local-default", "-o", "compress=zstd:3"}},
		{cmd: "mount", args: []string{"/dev/loop3", "/var/lib/hacocoon/mounts/local-default", "-o", "compress=zstd:3,noatime,discard=async"}},
		{cmd: "losetup", args: []string{"--find", "--show", "--partscan", "/tmp/evil"}},
		{cmd: "mkfs.btrfs", args: []string{"-f", "--", "/dev/sda"}},
		{cmd: "btrfs", args: []string{"subvolume", "delete", "/"}},
		{cmd: "fstrim", args: []string{"-a"}},
	}
	for _, tt := range tests {
		if _, _, _, err := translatePrivilegedCommand(tt.cmd, tt.args); err == nil {
			t.Fatalf("accepted arbitrary privileged command: %s %#v", tt.cmd, tt.args)
		}
	}
}

func TestSudoRunnerPassesTypedOperationToHelperWhenAlreadyRoot(t *testing.T) {
	direct := &recordingHostRunner{}
	runner := &SudoRunner{
		root:       "/var/lib/hacocoon",
		direct:     direct,
		helperPath: "/trusted/haco-storage-helper",
		sudoPath:   "/usr/bin/sudo",
		euid:       func() int { return 0 },
		validateHelper: func(path string) error {
			if path != "/trusted/haco-storage-helper" {
				return fmt.Errorf("unexpected helper %q", path)
			}
			return nil
		},
	}
	if _, err := runner.Run(context.Background(), "mount", "/dev/loop7", "/var/lib/hacocoon/mounts/local-default", "-o", managedBtrfsMountOptions); err != nil {
		t.Fatal(err)
	}
	want := []string{"--root", "/var/lib/hacocoon", "mount-btrfs", "/dev/loop7", "/var/lib/hacocoon/mounts/local-default"}
	if direct.name != "/trusted/haco-storage-helper" || !reflect.DeepEqual(direct.args, want) {
		t.Fatalf("helper call = %q %#v, want helper %#v", direct.name, direct.args, want)
	}
}

func TestSudoRunnerLeavesNonPrivilegedProbeDirect(t *testing.T) {
	direct := &recordingHostRunner{}
	runner := &SudoRunner{
		root:           "/var/lib/hacocoon",
		direct:         direct,
		helperPath:     "/unused",
		euid:           func() int { return 1000 },
		validateHelper: func(string) error { return fmt.Errorf("helper must not be consulted") },
	}
	if _, err := runner.Run(context.Background(), "btrfs", "version"); err != nil {
		t.Fatal(err)
	}
	if direct.name != "btrfs" || !reflect.DeepEqual(direct.args, []string{"version"}) {
		t.Fatalf("direct probe = %q %#v", direct.name, direct.args)
	}
}
