package clientforward

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
)

func TestWindowsDesktopFailureDoesNotOpenLinuxListener(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "installation-name-is-not-authority")
	t.Setenv("WSL_INTEROP", "")
	d := testDelegation()
	connect := delegationController(t, d.Installation, "")
	client, err := connect(d.Installation)
	if err != nil {
		t.Fatal(err)
	}
	// No native PowerShell in this isolated Linux fixture: ordinary automatic
	// dispatch must refuse, not silently claim a listener in this namespace.
	t.Setenv("PATH", t.TempDir())
	var out, diagnostic bytes.Buffer
	for _, lang := range []cliui.Language{cliui.English, cliui.Japanese} {
		out.Reset()
		diagnostic.Reset()
		code := DesktopCommand(context.Background(), []string{"--target-port", "8080", "demo"}, &out, &diagnostic, lang, client, nil)
		if code != 1 || out.Len() != 0 || !bytes.Contains(diagnostic.Bytes(), []byte("haco doctor")) {
			t.Fatalf("%s code %d out %q diagnostic %q", lang, code, out.String(), diagnostic.String())
		}
	}
}

func TestWindowsDataPathProjection(t *testing.T) {
	got, err := projectedWindowsPath(`C:\Users\名前 Space\AppData\Local`)
	if err != nil || got != "/mnt/c/Users/名前 Space/AppData/Local" {
		t.Fatal(got, err)
	}
	for _, path := range []string{`\\server\share`, `C:\Users\..\Other`, `C:\Users\x:stream`, `C:\Users\x.`, `C:\Users\x `, `C:\Users\\x`, "C:\\Users\nOops", `C:\Users/x`} {
		if _, err := projectedWindowsPath(path); err == nil {
			t.Fatal(path)
		}
	}
}

func TestInstalledWindowsHelperOwnership(t *testing.T) {
	for _, defect := range []string{"valid", "missing", "foreign", "symlink record", "fifo record", "symlink executable", "symlink directory"} {
		t.Run(defect, func(t *testing.T) {
			root := t.TempDir()
			target := testDelegation().Installation
			directory := filepath.Join(root, "Hacocoon", "client", "11111111111111111111111111111111")
			if err := os.MkdirAll(directory, 0700); err != nil {
				t.Fatal(err)
			}
			record := filepath.Join(directory, "installation.json")
			body := []byte(`{"registration_id":"{11111111-1111-1111-1111-111111111111}","schema_version":1}`)
			if defect == "foreign" {
				body = []byte(`{"registration_id":"{33333333-3333-3333-3333-333333333333}","schema_version":1}`)
			}
			if defect != "missing" && defect != "fifo record" {
				if err := os.WriteFile(record, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			helper := filepath.Join(directory, "haco-tunnel.exe")
			if err := os.WriteFile(helper, []byte("owned fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			if defect == "symlink record" || defect == "symlink executable" {
				path := record
				if defect == "symlink executable" {
					path = helper
				}
				if err := os.Rename(path, path+".other"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".other", path); err != nil {
					t.Fatal(err)
				}
			}
			if defect == "fifo record" {
				if err := syscall.Mkfifo(record, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if defect == "symlink directory" {
				if err := os.Rename(directory, directory+".other"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(directory+".other", directory); err != nil {
					t.Fatal(err)
				}
			}
			done := make(chan error, 1)
			go func() { _, err := installedWindowsHelper(root, target); done <- err }()
			select {
			case err := <-done:
				if (defect == "valid") != (err == nil) {
					t.Fatal(defect, err)
				}
			case <-time.After(time.Second):
				t.Fatal("ownership check blocked on special file")
			}
		})
	}
}
