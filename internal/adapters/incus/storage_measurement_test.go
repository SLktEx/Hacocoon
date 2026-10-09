package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

// These are deliberately separate observations. du allocation counts referenced
// blocks, including CoW duplicates. FIEMAP extent lengths are not compressed
// physical usage. Only the backing-file sample is whole-pool Linux allocation;
// it also contains rootfs/metadata and is not an isolated operation write counter.
type storageByteSample struct {
	LogicalBytes         uint64 `json:"logical_bytes"`
	AllocatedBytes       uint64 `json:"allocated_bytes"`
	ExtentTotalBytes     uint64 `json:"extent_total_bytes"`
	ExtentExclusiveBytes uint64 `json:"extent_exclusive_bytes"`
	ExtentSetSharedBytes uint64 `json:"extent_set_shared_bytes"`
}

type hostStorageObserver struct {
	t       *testing.T
	ctx     context.Context
	runtime *Runtime
	source  core.PersistentResource
}

func (m hostStorageObserver) command(name string, args ...string) string {
	m.t.Helper()
	out, runErr := m.runtime.runner.Run(m.ctx, name, args...)
	text, err := storageMeasurementOutput(name, out, runErr)
	if err != nil {
		m.t.Fatal(err)
	}
	return text
}

func storageMeasurementOutput(name string, out host.Result, runErr error) (string, error) {
	// btrfs du can report EACCES/ENOTTY warnings, skip files and still exit 0.
	// Such a row is incomplete, even if all numeric columns parse successfully.
	if runErr != nil || out.ExitCode != 0 || out.StdoutTruncated || out.StderrTruncated || (name == "btrfs" && strings.TrimSpace(out.Stderr) != "") {
		return "", fmt.Errorf("storage observation command %s failed: exit=%d runner_error=%t", name, out.ExitCode, runErr != nil)
	}
	return out.Stdout, nil
}

func storageMeasurementDirectory(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path {
		return fmt.Errorf("storage observation path changed or unavailable")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("storage observation requires a directory")
	}
	return nil
}

func (m hostStorageObserver) path(resource core.PersistentResource, consumer string) string {
	m.t.Helper()
	pool, volume, err := persistentVolume(resource)
	sourcePool, _, sourceErr := persistentVolume(m.source)
	if err != nil || sourceErr != nil || pool != sourcePool || pool != m.runtime.project {
		m.t.Fatal("storage observation requires this fixture's exact pool and resource")
	}
	observed, err := (&PersistentResourceBackend{Runtime: m.runtime}).observe(m.ctx, resource)
	if err != nil || observed == nil {
		m.t.Fatal("storage observation ownership", err)
	}
	if consumer == "" && len(observed.UsedBy) != 0 || consumer != "" && (len(observed.UsedBy) != 1 || !environmentDataUsedBy(observed.UsedBy[0], m.runtime.project, consumer)) {
		m.t.Fatal("storage observation consumer changed")
	}
	path := filepath.Join("/var/lib/incus/storage-pools", pool, "custom", m.runtime.project+"_"+volume)
	if err := storageMeasurementDirectory(path); err != nil {
		m.t.Fatal(err)
	}
	return path
}

// These fixed version probes have no credential/config output. The retained CI
// receipt identifies the exact tested commit; no root Git trust override is used.
func (m hostStorageObserver) metadata(guest func(string, string) string, imageID string) {
	m.t.Helper()
	serverVersion, err := storageMeasurementIncusVersion(m.command("incus", "query", "/1.0"))
	if err != nil {
		m.t.Fatal(err)
	}
	values := map[string]string{
		"go":          runtime.Version(),
		"kernel":      strings.TrimSpace(m.command("uname", "-srmo")),
		"btrfs_progs": strings.TrimSpace(m.command("btrfs", "--version")),
		"incus":       serverVersion,
		"nerdctl":     strings.TrimSpace(guest(trustedHostName, "nerdctl --version")),
		"containerd":  strings.TrimSpace(guest(trustedHostName, "containerd --version")),
		"buildkit":    strings.TrimSpace(guest(trustedHostName, "buildctl --version")),
		"image_id":    imageID,
	}
	osRelease, err := os.ReadFile("/etc/os-release")
	if err != nil || len(osRelease) > 64*1024 {
		m.t.Fatal("OS version unavailable")
	}
	for _, line := range strings.Split(string(osRelease), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && (key == "ID" || key == "VERSION_ID") {
			values["os_"+strings.ToLower(key)] = strings.Trim(value, `"`)
		}
	}
	guest(trustedHostName, `grep -Fx 'snapshotter = "native"' /etc/nerdctl/nerdctl.toml >/dev/null`)
	values["snapshotter"] = "native"
	safeIdentity := regexp.MustCompile(`^[a-zA-Z0-9 ._+:/()\n-]+$`)
	for key, value := range values {
		if len(value) == 0 || len(value) > 512 || !safeIdentity.MatchString(value) {
			m.t.Fatal("invalid measurement identity", key)
		}
	}
	data, err := json.Marshal(values)
	if err != nil {
		m.t.Fatal(err)
	}
	m.t.Logf("storage_measurement fixture=standard-host-busybox-buildkit identity=%s", data)
}

// observe freezes only the already verified disposable consumers through Incus.
// Failure retains the fixture, including any paused consumers, for inspection.
// No backing subvolume/file/loop/mount is mutated by the observer.
func (m hostStorageObserver) observe(phase string, target *core.PersistentResource, receiver string) map[string]storageByteSample {
	m.t.Helper()
	backend := &PersistentResourceBackend{Runtime: m.runtime}
	instance, err := backend.hostCopyInstance(m.ctx, m.source)
	if err != nil || instance.StatusCode != 103 || instance.Config[hostOCICopyKey] != "" {
		m.t.Fatal("storage observation requires the running owned Host with no pending copy")
	}
	paths := map[string]string{"host": m.path(m.source, trustedHostName)}
	if target != nil {
		paths["copy"] = m.path(*target, receiver)
	}
	if receiver != "" {
		if target == nil {
			m.t.Fatal("storage observation receiver has no owned Store")
		}
		verifyHostAreaReceiver(m.t, m.runtime.project, receiver, *target, func(args ...string) string {
			return m.command("incus", args...)
		})
		m.command("incus", "pause", receiver, "--project", m.runtime.project)
	}
	m.command("incus", "pause", trustedHostName, "--project", m.runtime.project)
	m.command("btrfs", "filesystem", "sync", paths["host"])
	result := map[string]storageByteSample{}
	for _, name := range []string{"host", "copy"} {
		path, ok := paths[name]
		if !ok {
			continue
		}
		for _, area := range []string{"", "containerd/io.containerd.content.v1.content/blobs", "buildkit"} {
			label := name
			if area != "" {
				label += "/" + area
			}
			dir := filepath.Join(path, area)
			// Guests populated these descendants. With all exact consumers frozen,
			// refuse symlinked/missing areas before any recursive inspection.
			if err := storageMeasurementDirectory(dir); err != nil {
				m.t.Fatal(label, err)
			}
			sample, err := parseStorageByteSample(
				m.command("du", "--summarize", "--apparent-size", "--block-size=1", "--", dir),
				m.command("du", "--summarize", "--block-size=1", "--", dir),
				m.command("btrfs", "filesystem", "du", "--raw", "--summarize", dir), dir)
			if err != nil {
				m.t.Fatal("invalid storage observation", label, err)
			}
			result[label] = sample
			data, err := json.Marshal(sample)
			if err != nil {
				m.t.Fatal(err)
			}
			m.t.Logf("storage_measurement phase=%s area=%s bytes=%s", phase, label, data)
		}
	}
	pool, _, err := persistentVolume(m.source)
	if err != nil {
		m.t.Fatal(err)
	}
	backing := filepath.Join("/var/lib/incus/disks", pool+".img")
	info, err := os.Lstat(backing)
	if err != nil || !info.Mode().IsRegular() {
		m.t.Fatal("fixture backing file unavailable")
	}
	allocation, err := parseStorageDU(m.command("du", "--summarize", "--block-size=1", "--", backing), backing)
	if err != nil {
		m.t.Fatal(err)
	}
	m.t.Logf("storage_measurement phase=%s pool_logical_bytes=%d pool_allocated_bytes=%d", phase, info.Size(), allocation)
	m.command("incus", "start", trustedHostName, "--project", m.runtime.project)
	if receiver != "" {
		m.command("incus", "start", receiver, "--project", m.runtime.project)
	}
	return result
}

// /1.0's structured environment field is locale independent; `incus version`
// has translated client/server labels and is not a machine observation contract.
func storageMeasurementIncusVersion(output string) (string, error) {
	var api struct {
		Version     string `json:"api_version"`
		Auth        string `json:"auth"`
		Environment struct {
			ServerVersion string `json:"server_version"`
		} `json:"environment"`
	}
	if len(output) > 128*1024 || json.Unmarshal([]byte(output), &api) != nil || api.Version != "1.0" || api.Auth != "trusted" || len(api.Environment.ServerVersion) > 32 || !incusReleaseVersion.MatchString(api.Environment.ServerVersion) {
		return "", fmt.Errorf("Incus server version unavailable")
	}
	return api.Environment.ServerVersion, nil
}

func parseStorageDU(output, path string) (uint64, error) {
	fields := strings.Fields(strings.TrimSpace(output))
	if len(fields) != 2 || fields[1] != path {
		return 0, fmt.Errorf("expected one exact du path")
	}
	return strconv.ParseUint(fields[0], 10, 64)
}

// btrfs-progs v6.17: cmds/filesystem-du.c, du_add_file (summary rows) and
// cmd_filesystem_du (header). Set shared deduplicates intervals, so it need not
// equal total minus exclusive. This parser never interprets it as pool usage.
// https://github.com/kdave/btrfs-progs/blob/v6.17/cmds/filesystem-du.c#L496-L508
func parseStorageByteSample(logical, allocated, extents, path string) (storageByteSample, error) {
	var sample storageByteSample
	var err error
	if sample.LogicalBytes, err = parseStorageDU(logical, path); err != nil {
		return sample, err
	}
	if sample.AllocatedBytes, err = parseStorageDU(allocated, path); err != nil {
		return sample, err
	}
	lines := strings.Split(strings.TrimSpace(extents), "\n")
	if len(lines) != 2 || strings.Join(strings.Fields(lines[0]), " ") != "Total Exclusive Set shared Filename" {
		return sample, fmt.Errorf("expected the Btrfs du header and one row")
	}
	fields := strings.Fields(lines[1])
	if len(fields) != 4 || fields[3] != path {
		return sample, fmt.Errorf("expected one exact Btrfs du path")
	}
	for i, value := range []*uint64{&sample.ExtentTotalBytes, &sample.ExtentExclusiveBytes, &sample.ExtentSetSharedBytes} {
		if *value, err = strconv.ParseUint(fields[i], 10, 64); err != nil {
			return sample, err
		}
	}
	if sample.ExtentExclusiveBytes > sample.ExtentTotalBytes || sample.ExtentSetSharedBytes > sample.ExtentTotalBytes-sample.ExtentExclusiveBytes {
		return sample, fmt.Errorf("inconsistent Btrfs extent counts")
	}
	return sample, nil
}
