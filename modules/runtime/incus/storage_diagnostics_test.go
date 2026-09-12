package incus

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/diagnostics"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const diagnosticBackingFile = "/var/lib/incus/disks/haco-local-default.img"
const diagnosticLiveOptions = "rw,noatime,compress=zstd:3,space_cache=v2,subvolid=5,subvol=/"

func storageDiagnosticFixture(t *testing.T, name string, args []string) host.Result {
	t.Helper()
	commands := map[string][]string{
		"stat":    {"--printf=%Hd:%Ld:%i:%f", "--", diagnosticBackingFile},
		"losetup": {"--list", "--json", "--associated", diagnosticBackingFile, "--output", "NAME,BACK-INO,BACK-MAJ:MIN,OFFSET,SIZELIMIT"},
		"findmnt": {"--kernel", "--list", "--json", "--nofsroot", "--output", "SOURCE,FSTYPE,OPTIONS,FSROOT", "--mountpoint", "/var/lib/incus/storage-pools/haco-local-default"},
	}
	if !reflect.DeepEqual(args, commands[name]) {
		t.Fatalf("unexpected/mutating storage command: %s %v", name, args)
	}
	// Reduced from the real WSL observation; BACK-MAJ:MIN and inode identify
	// the actual backing file, not merely its name in a shared kernel.
	switch name {
	case "stat":
		return host.Result{Stdout: "8:48:48945:8180"}
	case "losetup":
		return host.Result{Stdout: `{"loopdevices":[{"name":"/dev/loop0","back-ino":48945,"back-maj:min":"8:48","offset":0,"sizelimit":0}]}`}
	case "findmnt":
		return host.Result{Stdout: `{"filesystems":[{"source":"/dev/loop0","fstype":"btrfs","options":"` + diagnosticLiveOptions + `","fsroot":"/"}]}`}
	default:
		t.Fatalf("unexpected executable: %s", name)
		return host.Result{}
	}
}

func TestLiveStorageObservesIdentityWithoutMutation(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		return storageDiagnosticFixture(t, name, args), nil
	}}
	match, known := New(runner).inspectLiveStorage(context.Background(), diagnosticStorage.Name, diagnosticBackingFile)
	if !match || !known || len(runner.calls) != 5 {
		t.Fatalf("match=%v known=%v calls=%v", match, known, runner.calls)
	}
}

func TestLiveStorageRefusesMalformedOrChangingIdentity(t *testing.T) {
	for _, scenario := range []string{"missing", "null", "truncated", "symlink", "block-file", "other-inode", "other-filesystem-same-name", "duplicate-loop", "missing-offset", "offset", "sizelimit", "option-device", "duplicate-mount", "wrong-device", "wrong-fstype", "subvolume", "replaced-image", "reattached-loop", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runner := &fakeRunner{run: func(_ context.Context, call int, name string, args []string) (host.Result, error) {
				result := storageDiagnosticFixture(t, name, args)
				switch {
				case scenario == "missing":
					return host.Result{Stderr: "private-file"}, errors.New("private-file")
				case scenario == "null" && name == "losetup":
					result.Stdout = "null"
				case scenario == "truncated" && name == "findmnt":
					result.StdoutTruncated = true
				case scenario == "symlink" && name == "stat":
					result.Stdout = "8:48:48945:a1ff"
				case scenario == "block-file" && name == "stat":
					result.Stdout = "8:48:48945:6180"
				case scenario == "other-inode" && name == "losetup":
					result.Stdout = strings.ReplaceAll(result.Stdout, "48945", "48946")
				case scenario == "other-filesystem-same-name" && name == "losetup":
					result.Stdout = strings.ReplaceAll(result.Stdout, "8:48", "8:64")
				case scenario == "missing-offset" && name == "losetup":
					result.Stdout = strings.ReplaceAll(result.Stdout, `"offset":0,`, "")
				case scenario == "offset" && name == "losetup":
					result.Stdout = strings.ReplaceAll(result.Stdout, `"offset":0`, `"offset":4096`)
				case scenario == "sizelimit" && name == "losetup":
					result.Stdout = strings.ReplaceAll(result.Stdout, `"sizelimit":0`, `"sizelimit":4096`)
				case scenario == "option-device" && name == "losetup":
					result.Stdout = strings.ReplaceAll(result.Stdout, "/dev/loop0", "--detach-all")
				case scenario == "duplicate-loop" && name == "losetup", scenario == "duplicate-mount" && name == "findmnt":
					var value map[string][]json.RawMessage
					if err := json.Unmarshal([]byte(result.Stdout), &value); err != nil {
						t.Fatal(err)
					}
					for key, items := range value {
						value[key] = append(items, items[0])
					}
					result = jsonResult(value)
				case scenario == "wrong-device" && name == "findmnt":
					result.Stdout = strings.ReplaceAll(result.Stdout, "/dev/loop0", "/dev/loop1")
				case scenario == "wrong-fstype" && name == "findmnt":
					result.Stdout = strings.ReplaceAll(result.Stdout, "btrfs", "ext4")
				case scenario == "subvolume" && name == "findmnt":
					result.Stdout = strings.ReplaceAll(result.Stdout, `"fsroot":"/"`, `"fsroot":"/containers/unrelated"`)
				case scenario == "replaced-image" && call == 3:
					result.Stdout = "8:48:48946:8180"
				case scenario == "reattached-loop" && call == 4:
					result.Stdout = strings.ReplaceAll(result.Stdout, "/dev/loop0", "/dev/loop2")
				case scenario == "canceled" && name == "findmnt":
					cancel()
				}
				return result, nil
			}}
			match, known := New(runner).inspectLiveStorage(ctx, diagnosticStorage.Name, diagnosticBackingFile)
			if match || known {
				t.Fatalf("accepted uncertain identity: match=%v known=%v", match, known)
			}
		})
	}
}

func TestLiveStorageRefusesUnscopedSourcesBeforeExecution(t *testing.T) {
	for _, source := range []string{"", "-x", "relative/disks/haco-local-default.img", "/tmp/../disks/haco-local-default.img", "/dev/sda", "/var/lib/incus/disks/other.img", "/var/lib/incus/disks/haco-local-default.img\n", strings.Repeat("/", 4097)} {
		runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
			t.Fatal("executed invalid source")
			return host.Result{}, nil
		}}
		if match, known := New(runner).inspectLiveStorage(context.Background(), diagnosticStorage.Name, source); match || known {
			t.Fatalf("accepted %q", source)
		}
	}
}

func TestLiveMountPolicyDistinguishesPendingAndUnknown(t *testing.T) {
	for _, options := range []string{diagnosticLiveOptions, diagnosticLiveOptions + ",nodiscard"} {
		if match, known := liveMountPolicy(options); !match || !known {
			t.Fatalf("rejected applied policy: %s", options)
		}
	}
	for _, options := range []string{"rw,relatime,compress=zstd:3", "rw,noatime,compress=zstd:1", "rw,noatime", "ro,noatime,compress=zstd:3", diagnosticLiveOptions + ",discard", diagnosticLiveOptions + ",discard=async", diagnosticLiveOptions + ",autodefrag", diagnosticLiveOptions + ",compress-force=zstd:3"} {
		if match, known := liveMountPolicy(options); match || !known {
			t.Fatalf("did not identify pending policy: %s", options)
		}
	}
	for _, options := range []string{"", "rw,,noatime", "rw,rw", "rw,\x1b[2J", strings.Repeat("x", 16385)} {
		if match, known := liveMountPolicy(options); match || known {
			t.Fatalf("accepted malformed options: %q", options)
		}
	}
}

func TestDoctorReportsPendingMountWithoutRepair(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		result := diagnosticFixture(t, name, args)
		if name == "findmnt" {
			result.Stdout = strings.ReplaceAll(result.Stdout, "noatime", "relatime")
		}
		return result, nil
	}}
	report, err := New(runner).DiagnoseHost(context.Background(), diagnosticStorage)
	if err != nil || report.Validate() != nil || report.Healthy() || report.Checks[1].Status != diagnostics.OK || report.Checks[2].Status != diagnostics.Pending || report.Checks[2].Action == "" {
		t.Fatalf("incorrect pending mount report: %+v err=%v", report, err)
	}
}
