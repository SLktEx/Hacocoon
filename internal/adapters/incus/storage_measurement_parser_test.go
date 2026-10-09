package incus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

// The raw header/row grammar is specified by upstream btrfs-progs v6.17,
// cmds/filesystem-du.c:504 and :583. Values below are parser examples, not new
// measurements; the 8 MiB shared case also matches the older cache receipt in
// docs/status/acceptance-evidence.md#cache-generation-foundation.
func TestStorageByteSampleKeepsAccountingSeparate(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		logical, allocated       uint64
		total, exclusive, shared uint64
	}{
		{"shared", 8388608, 8388608, 8388608, 0, 8388608},
		{"compressed", 8388608, 4096, 8388608, 0, 8388608},
		{"sparse", 8388608, 0, 0, 0, 0},
		{"shared_set_deduplicated", 16384, 16384, 16384, 0, 8192},
		{"exclusive", 4096, 4096, 4096, 4096, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "/owned/volume"
			got, err := parseStorageByteSample(fmt.Sprintf("%d\t%s\n", tc.logical, path), fmt.Sprintf("%d\t%s\n", tc.allocated, path), fmt.Sprintf("     Total   Exclusive  Set shared  Filename\n%d %d %d %s\n", tc.total, tc.exclusive, tc.shared, path), path)
			want := storageByteSample{tc.logical, tc.allocated, tc.total, tc.exclusive, tc.shared}
			if err != nil || got != want {
				t.Fatalf("got %+v, %v; want %+v", got, err, want)
			}
		})
	}
}

func TestStorageByteSampleRejectsAmbiguousMeasurements(t *testing.T) {
	path := "/owned/volume"
	du := "4096\t" + path + "\n"
	extents := "Total Exclusive Set shared Filename\n4096 0 4096 " + path + "\n"
	for _, tc := range []struct {
		name, logical, allocated, extents string
	}{
		{"missing_du", "", du, extents},
		{"wrong_du_path", strings.ReplaceAll(du, path, "/other/volume"), du, extents},
		{"duplicate_du", du + du, du, extents},
		{"negative_logical", "-1 " + path, du, extents},
		{"negative_allocated", du, "-1 " + path, extents},
		{"overflow", "18446744073709551616 " + path, du, extents},
		{"human_units", "4K " + path, du, extents},
		{"missing_extents", du, du, ""},
		{"missing_header", du, du, "4096 0 4096 " + path},
		{"wrong_header", du, du, strings.ReplaceAll(extents, "Set shared", "Shared")},
		{"wrong_extent_path", du, du, strings.ReplaceAll(extents, path, "/other/volume")},
		{"duplicate_extent_row", du, du, extents + "4096 0 4096 " + path},
		{"extra_warning", du, du, "warning: incomplete scan\n" + extents},
		{"negative_extents", du, du, strings.Replace(extents, "4096 0 4096", "4096 -1 4096", 1)},
		{"exclusive_exceeds_total", du, du, strings.Replace(extents, "4096 0 4096", "4096 8192 0", 1)},
		{"shared_exceeds_nonexclusive", du, du, strings.Replace(extents, "4096 0 4096", "4096 4096 4096", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseStorageByteSample(tc.logical, tc.allocated, tc.extents, path); err == nil {
				t.Fatal("accepted malformed or ambiguous storage evidence")
			}
		})
	}
}

func TestStorageMeasurementRejectsIncompleteCommandResults(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result host.Result
		err    error
	}{
		{"scan_warning_with_success_exit", host.Result{Stdout: "plausible numeric summary", Stderr: "WARNING: cannot access file: Permission denied"}, nil},
		{"stdout_truncated", host.Result{StdoutTruncated: true}, nil},
		{"stderr_truncated", host.Result{StderrTruncated: true}, nil},
		{"exit_failure", host.Result{ExitCode: 1}, nil},
		{"runner_failure", host.Result{}, errors.New("interrupted")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := storageMeasurementOutput("btrfs", tc.result, tc.err); err == nil {
				t.Fatal("accepted incomplete measurement")
			}
		})
	}
	if output, err := storageMeasurementOutput("btrfs", host.Result{Stdout: "complete"}, nil); err != nil || output != "complete" {
		t.Fatal("rejected complete measurement", err)
	}
}

func TestStorageMeasurementRejectsSymlinkedAreas(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	area := filepath.Join(root, "area")
	if err := os.Mkdir(area, 0700); err != nil {
		t.Fatal(err)
	}
	if err := storageMeasurementDirectory(area); err != nil {
		t.Fatal("rejected exact directory", err)
	}
	regular := filepath.Join(root, "file")
	if err := os.WriteFile(regular, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{regular, filepath.Join(root, "missing")} {
		if err := storageMeasurementDirectory(path); err == nil {
			t.Fatal("accepted non-directory area")
		}
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(area, link); err != nil {
		t.Skip("symlink creation unavailable")
	}
	if err := storageMeasurementDirectory(link); err == nil {
		t.Fatal("accepted symlinked measurement area")
	}
}

func TestStorageMeasurementIncusVersionIsLocaleIndependent(t *testing.T) {
	for _, label := range []string{"Success", "成功"} {
		input := `{"api_version":"1.0","auth":"trusted","status":"` + label + `","environment":{"server_version":"7.0.2"}}`
		if version, err := storageMeasurementIncusVersion(input); err != nil || version != "7.0.2" {
			t.Fatal("rejected locale-independent API observation", err)
		}
	}
	for _, input := range []string{
		`クライアントのバージョン: 7.0.2`,
		`{}`, `null`,
		`{"api_version":"1.0","auth":"untrusted","environment":{"server_version":"7.0.2"}}`,
		`{"api_version":"1.0","auth":"trusted","environment":{"server_version":"7.0.2\nsecret"}}`,
		`{"api_version":"1.0","auth":"trusted","environment":{"server_version":7}}`,
	} {
		if _, err := storageMeasurementIncusVersion(input); err == nil {
			t.Fatal("accepted missing or unsafe Incus version")
		}
	}
}
