//go:build linux

package recipes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The actual WSL resolver is the authority for drive mapping. Linux CI skips
// this native path check; the installed Windows gate also exercises it.
func TestReadWindowsScriptPathOnWSL(t *testing.T) {
	if _, err := os.Stat("/usr/bin/wslpath"); err != nil {
		t.Skip("native WSL resolver unavailable")
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	probe, err := exec.Command("/usr/bin/wslpath", "-w", wd).Output()
	if err != nil || len(probe) < 3 || probe[1] != ':' {
		t.Skip("test checkout is not on a Windows drive")
	}
	f, err := os.CreateTemp(wd, "host recipe path-*.sh")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	content := "\ufeffecho windows\r\n"
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	path := strings.TrimSpace(string(probe)) + "\\" + filepath.Base(f.Name())
	got, err := ReadScript(path)
	if err != nil || string(got) != content {
		t.Fatalf("native Windows path input: %v", err)
	}
}
