//go:build linux

package sshclient

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func preparedSSHFiles(t *testing.T) (Desktop, *fakeController) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen unavailable")
	}
	desktop := Desktop{Home: t.TempDir()}
	directory := filepath.Join(desktop.Home, ".ssh")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "config"), []byte("Host personal\n  HostName personal.example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	controller := &fakeController{state: core.EnvironmentRunning}
	if alias, err := Setup(context.Background(), controller, desktop, "dev"); err != nil || alias != "haco-dev" {
		t.Fatal(alias, err)
	}
	return desktop, controller
}

// Keep comparisons useful without putting generated private keys in failures.
func sshFileSnapshot(t *testing.T, home string) map[string][32]byte {
	t.Helper()
	directory := filepath.Join(home, ".ssh")
	result := map[string][32]byte{}
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var data []byte
		switch {
		case entry.Type().IsRegular():
			data, err = os.ReadFile(path)
		case entry.Type()&os.ModeSymlink != 0:
			var link string
			link, err = os.Readlink(path)
			data = []byte(link)
		}
		if err != nil {
			return err
		}
		result[strings.TrimPrefix(path, directory)] = sha256.Sum256(append([]byte(info.Mode().String()+"\n"), data...))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSetupRejectsFIFOsWithoutWaitingForAWriter(t *testing.T) {
	for _, role := range []string{"config", "public-key", "private-key", "managed-fragment", "host-key-pin"} {
		t.Run(role, func(t *testing.T) {
			desktop, controller := preparedSSHFiles(t)
			name := map[string]string{
				"config": "config", "public-key": "hacocoon/identity.pub", "private-key": "hacocoon/identity",
				"managed-fragment": "hacocoon/dev.conf", "host-key-pin": knownPath(saved{Runtime: "runtime-owned", Connection: controller.connection}),
			}[role]
			path := filepath.Join(desktop.Home, ".ssh", name)
			if err := os.Rename(path, path+".retained"); err != nil {
				t.Fatal(err)
			}
			if err := syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			before := sshFileSnapshot(t, desktop.Home)
			done := make(chan error, 1)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			go func() { _, err := Setup(ctx, controller, desktop, "dev"); done <- err }()
			var setupErr error
			select {
			case setupErr = <-done:
			case <-time.After(500 * time.Millisecond):
				t.Error("SSH setup blocked opening a FIFO, even after cancellation")
				// Release the pre-fix blocking open so a failing test does not leak
				// its worker or leave the setup lock held. No bytes are supplied.
				writer, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = writer.Close() }()
				select {
				case setupErr = <-done:
				case <-time.After(time.Second):
					t.Fatal("setup did not release after the FIFO open was unblocked")
				}
			}
			if setupErr == nil || controller.count != 1 || controller.starts != 0 {
				t.Fatal("unsafe input prepared or started a connection", setupErr, controller.count, controller.starts)
			}
			if !reflect.DeepEqual(before, sshFileSnapshot(t, desktop.Home)) {
				t.Fatal("refused FIFO changed managed or personal SSH files")
			}
		})
	}
}

func TestSetupRefusesUserConflictsAndMalformedFilesBeforePreparation(t *testing.T) {
	for _, defect := range []string{"invalid-utf8", "nul", "quoted-user-alias", "unmanaged-fragment", "malformed-receipt", "foreign-distribution", "replacement-runtime", "oversized-config", "incomplete-key"} {
		t.Run(defect, func(t *testing.T) {
			desktop, controller := preparedSSHFiles(t)
			name, data := "config", []byte{}
			switch defect {
			case "invalid-utf8":
				data = []byte{0xff}
			case "nul":
				data = []byte("Host personal\x00\n")
			case "quoted-user-alias":
				data = []byte("Host personal \"haco-dev\"\n  HostName personal.example\n")
			case "oversized-config":
				data = []byte(strings.Repeat("#", 1024*1024+1))
			case "unmanaged-fragment":
				name, data = "hacocoon/dev.conf", []byte("# personal config\nHost haco-dev\n")
			case "malformed-receipt":
				name, data = "hacocoon/dev.conf", []byte(prefix+"{\nHost haco-dev\n")
			case "foreign-distribution", "replacement-runtime":
				name = "hacocoon/dev.conf"
				metadata := saved{Runtime: "runtime-owned", Connection: controller.connection}
				if defect == "foreign-distribution" {
					metadata.Distro = "another-installation"
					t.Setenv("WSL_DISTRO_NAME", "this-installation")
				} else {
					metadata.Runtime = "replacement"
				}
				text, _, err := render("dev", metadata)
				if err != nil {
					t.Fatal(err)
				}
				data = []byte(text)
			case "incomplete-key":
				if err := os.Remove(filepath.Join(desktop.Home, ".ssh/hacocoon/identity.pub")); err != nil {
					t.Fatal(err)
				}
			}
			if defect != "incomplete-key" {
				if err := os.WriteFile(filepath.Join(desktop.Home, ".ssh", name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := sshFileSnapshot(t, desktop.Home)
			alias, err := Setup(context.Background(), controller, desktop, "dev")
			if err == nil || alias != "" || controller.count != 1 || controller.starts != 0 {
				t.Fatal("conflict was adopted or prepared", alias, err, controller.count, controller.starts)
			}
			if defect == "quoted-user-alias" && !errors.Is(err, core.ErrAlreadyExists) {
				t.Fatal("user alias conflict lost its classification", err)
			}
			if !reflect.DeepEqual(before, sshFileSnapshot(t, desktop.Home)) {
				t.Fatal("refused input changed personal data or managed ownership")
			}
		})
	}
}
