//go:build linux

package incus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/host"
	"golang.org/x/sys/unix"
)

func TestExportSnapshotVolumeOwnershipAndCleanup(t *testing.T) {
	for _, kind := range []string{"work", "oci"} {
		t.Run(kind, func(t *testing.T) {
			for _, mode := range []string{"ok", "foreign", "changed-owner", "native-error", "cleanup-remains", "cleanup-unknown", "reused-backup-name", "oversize", "empty"} {
				t.Run(mode, func(t *testing.T) {
					p := snapshotVolumeFixture(kind)
					observed, exports, backupCalls := 0, 0, 0
					data := []byte("native-volume-archive")
					old := volumeBackupObservation{Name: "existing.backup", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
					runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
						if name != "incus" {
							t.Fatal(name)
						}
						if args[0] == "query" {
							if strings.Contains(args[1], "/backups?") {
								backupCalls++
								items := []volumeBackupObservation{old}
								if backupCalls > 1 {
									if mode == "cleanup-unknown" {
										return host.Result{Stdout: "[]", StdoutTruncated: true}, nil
									}
									if mode == "cleanup-remains" {
										items = append(items, volumeBackupObservation{Name: "backup0", CreatedAt: old.CreatedAt.Add(time.Hour)})
									}
									if mode == "reused-backup-name" {
										items[0].CreatedAt = old.CreatedAt.Add(time.Hour)
									}
								}
								raw, _ := json.Marshal(items)
								return host.Result{Stdout: string(raw)}, nil
							}
							observed++
							config := p.targetConfig()
							if mode == "foreign" || mode == "changed-owner" && observed > 1 {
								config["user.hacocoon.owner"] = "foreign"
							}
							raw, _ := json.Marshal([]persistentVolumeObservation{{Name: p.target(), Type: "custom", ContentType: "filesystem", Config: config, UsedBy: []string{}}})
							return host.Result{Stdout: string(raw)}, nil
						}
						exports++
						if len(args) != 12 || !reflect.DeepEqual(args[:5], []string{"storage", "volume", "export", p.Pool, p.target()}) || !reflect.DeepEqual(args[6:], []string{"--project", "hacocoon", "--volume-only", "--compression=none", "--quiet", "--force"}) || !strings.HasPrefix(args[5], fmt.Sprintf("/proc/%d/fd/", os.Getpid())) {
							t.Fatal("unexpected command", args)
						}
						// --force must only address this process's live, unlinked,
						// private regular file; never weaken public overwrite refusal.
						var target unix.Stat_t
						if err := unix.Stat(args[5], &target); err != nil || target.Nlink != 0 || target.Mode&unix.S_IFMT != unix.S_IFREG || target.Mode&0077 != 0 || target.Uid != uint32(os.Geteuid()) {
							t.Fatalf("unsafe forced export target: %#v %v", target, err)
						}
						payload := data
						if mode == "empty" {
							payload = nil
						}
						if err := os.WriteFile(args[5], payload, 0600); err != nil {
							t.Fatal(err)
						}
						if mode == "native-error" {
							return host.Result{ExitCode: 1}, errors.New("native failure after bytes")
						}
						return host.Result{}, nil
					}}
					runtime := New(runner)
					c, err := runtime.snapshotComponent(snapshotBinding{Version: 1, Project: "hacocoon", Volume: &p})
					if err != nil {
						t.Fatal(err)
					}
					c.State = "verified"
					root := t.TempDir()
					if err := os.Chmod(root, 0700); err != nil {
						t.Fatal(err)
					}
					limit := int64(1024)
					if mode == "oversize" {
						limit = 1
					}
					archive, err := runtime.ExportSnapshotVolume(context.Background(), c, root, limit)
					if mode == "ok" {
						if err != nil {
							t.Fatal(err)
						}
						defer archive.Close()
						raw, err := io.ReadAll(archive.Reader())
						hash := sha256.Sum256(data)
						if err != nil || !reflect.DeepEqual(raw, data) || archive.Size() != int64(len(data)) || archive.Digest() != hex.EncodeToString(hash[:]) {
							t.Fatal("wrong native bytes", err)
						}
						if _, err := archive.file.WriteAt([]byte("x"), 0); err == nil {
							t.Fatal("writable archive escaped")
						}
					} else if err == nil || archive != nil {
						t.Fatal("failed export reported complete", archive, err)
					}
					if mode == "foreign" && exports != 0 {
						t.Fatal("foreign volume exported")
					}
					entries, err := os.ReadDir(root)
					if err != nil || len(entries) != 0 {
						t.Fatal("named residue", entries, err)
					}
				})
			}
		})
	}
}
func TestNativeArchiveOutputIsWritableByChildOnlyDuringCapture(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	var path string
	archive, err := captureNativeArchive(context.Background(), root, 1024, func(output string) error {
		path = output
		return exec.Command("sh", "-c", `printf native-bytes > "$1"`, "sh", output).Run()
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(archive.Reader())
	if err != nil || string(raw) != "native-bytes" {
		t.Fatal(string(raw), err)
	}
	if _, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		t.Fatal("capture writable fd remained open")
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
}
