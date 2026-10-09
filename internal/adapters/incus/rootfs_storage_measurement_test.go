//go:build linux

package incus

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/env"
	"github.com/SLktEx/Hacocoon/internal/state"
)

const rootfsMeasurementDirectory = "/haco-rootfs-storage-measurement"
const rootfsMeasurementFixture = "snapshot-rootfs-image-lifecycle"

type rootfsLiveIdentity struct {
	env        core.Environment
	generation string
	// Empty for direct saved-rootfs restores; a matching BaseRef alone is not
	// evidence of initialization from an image. Ordinary Image Envs pin this too.
	fingerprint string
}

// Exactly one typed identity is selected. An Image area is the Incus-owned
// read-only optimized cache, authenticated by the complete saved Image plan.
// It is never adopted as a Hacocoon-owned instance or mutated by this observer.
type rootfsStorageArea struct {
	label string
	live  *rootfsLiveIdentity
	saved *core.Snapshot
	image *core.Snapshot
}

type rootfsStorageSample struct {
	Rootfs  storageByteSample `json:"rootfs"`
	Payload storageByteSample `json:"payload"`
	Hashes  map[string]string `json:"payload_sha256"`
}

type rootfsStorageObserver struct {
	t       *testing.T
	ctx     context.Context
	runtime *Runtime
	catalog *state.EnvironmentJSONStore
	pool    string
}

func (m rootfsStorageObserver) command(name string, args ...string) string {
	m.t.Helper()
	ctx, cancel := context.WithTimeout(m.ctx, time.Minute)
	defer cancel()
	out, runErr := m.runtime.runner.Run(ctx, name, args...)
	text, err := storageMeasurementOutput(name, out, runErr)
	if err != nil {
		m.t.Fatal(err)
	}
	return text
}

func (m rootfsStorageObserver) metadata(image core.BaseRef) {
	m.t.Helper()
	fp, err := baseRevisionFingerprint(image.Revision)
	if err != nil {
		m.t.Fatal("invalid measurement image identity")
	}
	values := storageMeasurementVersions(m.t, m.command)
	values["storage_driver"] = "btrfs"
	values["image_fingerprint"] = fp
	values["origin"] = "snapshot-generated-image"
	logStorageMeasurementVersions(m.t, rootfsMeasurementFixture, values)
}

func rootfsMeasurementSource(area rootfsStorageArea) (core.Environment, error) {
	count := 0
	var env core.Environment
	if area.live != nil {
		count++
		env = area.live.env
	}
	if area.saved != nil {
		count++
		env = area.saved.Source.Environment
	}
	if area.image != nil {
		count++
		env = area.image.Source.Environment
	}
	if count != 1 || area.label == "" || core.ValidateEnvironmentName(env.Name) != nil || env.Workspace.ID == "" {
		return env, fmt.Errorf("invalid typed rootfs measurement identity")
	}
	return env, nil
}

func (m rootfsStorageObserver) lock(areas []rootfsStorageArea) func() {
	m.t.Helper()
	envs, works := map[string]bool{}, map[string]bool{}
	labels := map[string]bool{}
	for _, area := range areas {
		env, err := rootfsMeasurementSource(area)
		if err != nil || labels[area.label] {
			m.t.Fatal("ambiguous rootfs measurement area")
		}
		labels[area.label] = true
		envs[env.Name] = true
		works[string(env.Workspace.ID)] = true
	}
	var unlocks []func()
	for i, identities := range []map[string]bool{envs, works} {
		keys := make([]string, 0, len(identities))
		for key := range identities {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			unlock, err := m.catalog.LockLifecycle(m.ctx, []string{"environment", "workspace"}[i], key)
			if err != nil {
				for j := len(unlocks) - 1; j >= 0; j-- {
					unlocks[j]()
				}
				m.t.Fatal("rootfs measurement lifecycle exclusion unavailable")
			}
			unlocks = append(unlocks, unlock)
		}
	}
	return func() {
		for i := len(unlocks) - 1; i >= 0; i-- {
			unlocks[i]()
		}
	}
}

func validateRootfsMeasurementConsumer(instance snapshotInstanceObservation, status int, pool, ref, generation, fingerprint string, wanted int) error {
	if wanted == 0 {
		if status != 102 && status != 103 {
			return fmt.Errorf("rootfs consumer must initially be stopped or running")
		}
		wanted = status
	}
	if !safeIncusRef(pool) || !core.ValidEnvironmentInstanceID(generation) || validateManagedInstanceRef(ref) != nil || ref == trustedHostName || instance.Name != ref || instance.Type != "container" || instance.Ephemeral || status != wanted {
		return fmt.Errorf("rootfs consumer identity or state changed")
	}
	if fingerprint != "" && !baseFingerprintPattern.MatchString(fingerprint) {
		return fmt.Errorf("rootfs image fingerprint invalid")
	}
	for _, config := range []map[string]string{instance.Config, instance.ExpandedConfig} {
		if config[environmentInstanceKey] != generation || config[managedEnvironmentMarkerKey] != managedEnvironmentMarkerValue || fingerprint != "" && config["volatile.base_image"] != fingerprint {
			return fmt.Errorf("rootfs consumer ownership or image changed")
		}
	}
	roots := 0
	for _, device := range instance.ExpandedDevices {
		if device["type"] != "disk" {
			continue
		}
		path := filepath.Clean(device["path"])
		if path == "/" {
			if device["path"] != "/" || device["pool"] != pool || device["source"] != "" {
				return fmt.Errorf("rootfs consumer pool changed")
			}
			roots++
			continue
		}
		if !filepath.IsAbs(path) || path == rootfsMeasurementDirectory || strings.HasPrefix(rootfsMeasurementDirectory, path+"/") || strings.HasPrefix(path, rootfsMeasurementDirectory+"/") {
			return fmt.Errorf("rootfs payload overlaps another disk")
		}
	}
	if roots != 1 {
		return fmt.Errorf("rootfs consumer root disk ambiguous")
	}
	return nil
}

func (m rootfsStorageObserver) live(identity rootfsLiveIdentity, wanted int) int {
	m.t.Helper()
	current, err := m.catalog.GetEnvironment(m.ctx, identity.env.Name)
	if err != nil || !reflect.DeepEqual(current, identity.env) || !environmentapp.MatchesRuntimeRef(current.RuntimeRef, environmentapp.ProviderIncus, "haco-"+current.Name) {
		m.t.Fatal("rootfs Environment identity changed")
	}
	generation, err := m.catalog.EnvironmentInstance(m.ctx, current)
	if err != nil || generation != identity.generation {
		m.t.Fatal("rootfs Environment generation changed")
	}
	if identity.fingerprint != "" {
		if current.Base == nil {
			m.t.Fatal("rootfs Environment Image absent")
		}
		fingerprint, err := baseRevisionFingerprint(current.Base.Revision)
		if err != nil || fingerprint != identity.fingerprint {
			m.t.Fatal("rootfs Environment Image changed")
		}
	}
	var observed struct {
		snapshotInstanceObservation
		StatusCode int `json:"status_code"`
	}
	if json.Unmarshal([]byte(m.command("incus", "query", "/1.0/instances/haco-"+current.Name+"?project="+m.runtime.project)), &observed) != nil {
		m.t.Fatal("invalid rootfs consumer observation")
	}
	if err := validateRootfsMeasurementConsumer(observed.snapshotInstanceObservation, observed.StatusCode, m.pool, "haco-"+current.Name, generation, identity.fingerprint, wanted); err != nil {
		m.t.Fatal(err)
	}
	return observed.StatusCode
}

func (m rootfsStorageObserver) savedPlans(saved core.Snapshot) (snapshotRootfsPlan, snapshotImagePlan) {
	m.t.Helper()
	current, err := m.catalog.GetSnapshot(m.ctx, saved.ID)
	if err != nil || current.State != "ready" || !reflect.DeepEqual(current, saved) {
		m.t.Fatal("rootfs Snapshot ownership changed")
	}
	root, image, err := rootfsMeasurementPlans(m.runtime, current)
	if err != nil || root.Pool != m.pool {
		m.t.Fatal("rootfs Snapshot binding unavailable")
	}
	return root, image
}

func rootfsMeasurementPlans(r *Runtime, saved core.Snapshot) (snapshotRootfsPlan, snapshotImagePlan, error) {
	var root *snapshotRootfsPlan
	var image *snapshotImagePlan
	for _, component := range saved.Components {
		prefix := "haco-runtime-v1:" + environmentapp.ProviderIncus + ":"
		if component.State != "verified" || !strings.HasPrefix(component.NativeRef, prefix) {
			return snapshotRootfsPlan{}, snapshotImagePlan{}, fmt.Errorf("invalid rootfs measurement Snapshot route")
		}
		ref, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(component.NativeRef, prefix))
		if err != nil {
			return snapshotRootfsPlan{}, snapshotImagePlan{}, err
		}
		component.NativeRef = string(ref)
		binding, err := r.decodeSnapshotComponent(component)
		if err != nil {
			return snapshotRootfsPlan{}, snapshotImagePlan{}, err
		}
		if binding.Rootfs != nil {
			if root != nil {
				return snapshotRootfsPlan{}, snapshotImagePlan{}, fmt.Errorf("duplicate saved rootfs")
			}
			root = binding.Rootfs
		}
		if binding.Image != nil {
			if image != nil {
				return snapshotRootfsPlan{}, snapshotImagePlan{}, fmt.Errorf("duplicate saved Image")
			}
			image = binding.Image
		}
	}
	if root == nil || image == nil || image.Rootfs != *root || root.SourceInstanceID != saved.Source.InstanceID || !environmentapp.MatchesRuntimeRef(saved.Source.Environment.RuntimeRef, environmentapp.ProviderIncus, root.Source) || saved.Image == nil || saved.Image.Name != core.BaseName(image.Name) {
		return snapshotRootfsPlan{}, snapshotImagePlan{}, fmt.Errorf("saved rootfs/Image identity mismatch")
	}
	if _, err := baseRevisionFingerprint(saved.Image.Revision); err != nil {
		return snapshotRootfsPlan{}, snapshotImagePlan{}, err
	}
	return *root, *image, nil
}

func validateRootfsMeasurementImageVolume(name, kind, content, readonly, fingerprint string) error {
	if !baseFingerprintPattern.MatchString(fingerprint) || name != fingerprint || kind != "image" || content != "filesystem" || strings.TrimSpace(readonly) != "ro=true" {
		return fmt.Errorf("immutable image cache identity unavailable")
	}
	return nil
}

// Every resolution revalidates provider identity. Paths are derived only from
// typed bindings, not accepted from a guest, catalog path or CLI output.
func (m rootfsStorageObserver) path(area rootfsStorageArea, wanted int) (string, string, int) {
	m.t.Helper()
	var path, identity string
	status := 0
	switch {
	case area.live != nil:
		status = m.live(*area.live, wanted)
		ref := "haco-" + area.live.env.Name
		path = filepath.Join("/var/lib/incus/storage-pools", m.pool, "containers", m.runtime.project+"_"+ref, "rootfs")
		identity = "live\x00" + ref + "\x00" + area.live.generation + "\x00" + area.live.fingerprint
	case area.saved != nil:
		root, _ := m.savedPlans(*area.saved)
		observed, err := m.runtime.snapshotRootfsObservation(m.ctx, root, true)
		if err != nil || observed == nil {
			m.t.Fatal("saved rootfs ownership unavailable")
		}
		path = filepath.Join("/var/lib/incus/storage-pools", m.pool, "containers", m.runtime.project+"_"+root.target(), "rootfs")
		identity = "saved\x00" + root.target() + "\x00" + root.Owner
	case area.image != nil:
		_, plan := m.savedPlans(*area.image)
		images, err := m.runtime.ownedSnapshotImages(m.ctx, plan)
		fp, fpErr := baseRevisionFingerprint(area.image.Image.Revision)
		if err != nil || fpErr != nil || len(images) != 1 || images[0].Fingerprint != fp {
			m.t.Fatal("Snapshot-generated Image ownership unavailable")
		}
		current, err := m.runtime.snapshotImage(m.ctx, plan)
		if err != nil || current != *area.image.Image {
			m.t.Fatal("Snapshot-generated Image alias changed")
		}
		var volume struct {
			Name, Type  string
			ContentType string `json:"content_type"`
		}
		// Incus optimized image volumes are keyed by fingerprint in the default
		// storage project, even when the published image belongs to this project.
		if json.Unmarshal([]byte(m.command("incus", "query", "/1.0/storage-pools/"+m.pool+"/volumes/image/"+fp+"?project=default")), &volume) != nil {
			m.t.Fatal("invalid image cache observation")
		}
		cache := filepath.Join("/var/lib/incus/storage-pools", m.pool, "images", fp)
		if err := storageMeasurementDirectory(cache); err != nil {
			m.t.Fatal(err)
		}
		if err := validateRootfsMeasurementImageVolume(volume.Name, volume.Type, volume.ContentType, m.command("btrfs", "property", "get", "-ts", cache, "ro"), fp); err != nil {
			m.t.Fatal(err)
		}
		path = filepath.Join(cache, "rootfs")
		identity = "image-cache\x00" + fp + "\x00" + plan.Owner
	default:
		m.t.Fatal("missing rootfs measurement identity")
	}
	if err := storageMeasurementDirectory(path); err != nil {
		m.t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(m.runtime.project + "\x00" + m.pool + "\x00" + identity))
	return path, hex.EncodeToString(digest[:]), status
}

func (m rootfsStorageObserver) guest(area rootfsStorageArea, script string) {
	m.t.Helper()
	ctx, cancel := context.WithTimeout(m.ctx, 2*time.Minute)
	defer cancel()
	m.ctx = ctx
	if area.live == nil || area.saved != nil || area.image != nil {
		m.t.Fatal("rootfs guest mutation requires an exact live Env")
	}
	unlock := m.lock([]rootfsStorageArea{area})
	defer unlock()
	m.path(area, 103)
	m.command("incus", "exec", "haco-"+area.live.env.Name, "--project", m.runtime.project, "--", "/bin/sh", "-eu", "-c", script, "--", rootfsMeasurementDirectory)
	m.path(area, 103)
}

// Read the mount namespace used by these filesystem reads, not guest device
// metadata. Refuse any submount that contains or lies within a measured rootfs;
// this includes same-filesystem bind mounts that du -x would not exclude.
func validateRootfsMeasurementMountScope(output, pool string, paths []string) error {
	var inventory struct {
		Filesystems []struct {
			Target   string            `json:"target"`
			Children []json.RawMessage `json:"children"`
		} `json:"filesystems"`
	}
	if len(output) > 2<<20 || json.Unmarshal([]byte(output), &inventory) != nil || len(inventory.Filesystems) == 0 || len(inventory.Filesystems) > 16384 || len(paths) == 0 {
		return fmt.Errorf("rootfs mount inventory unavailable")
	}
	if !filepath.IsAbs(pool) || filepath.Clean(pool) != pool {
		return fmt.Errorf("invalid rootfs pool scope")
	}
	for _, path := range paths {
		if !strings.HasPrefix(path, pool+"/") || filepath.Clean(path) != path {
			return fmt.Errorf("rootfs outside verified pool")
		}
	}
	poolMounts := 0
	for _, mount := range inventory.Filesystems {
		target := mount.Target
		if !filepath.IsAbs(target) || filepath.Clean(target) != target || len(mount.Children) != 0 {
			return fmt.Errorf("invalid flat rootfs mount inventory")
		}
		if target == pool {
			poolMounts++
			continue
		}
		if !strings.HasPrefix(target, pool+"/") {
			continue
		}
		for _, path := range paths {
			if target == path || strings.HasPrefix(target, path+"/") || strings.HasPrefix(path, target+"/") {
				return fmt.Errorf("rootfs observation overlaps a nested Host mount")
			}
		}
	}
	if poolMounts != 1 {
		return fmt.Errorf("verified pool mount absent or ambiguous in inventory")
	}
	return nil
}

func (m rootfsStorageObserver) mountScope(paths map[string]bool) {
	m.t.Helper()
	roots := make([]string, 0, len(paths))
	for path := range paths {
		roots = append(roots, path)
	}
	output := m.command("findmnt", "--kernel", "--json", "--list", "--output", "TARGET")
	if err := validateRootfsMeasurementMountScope(output, filepath.Join("/var/lib/incus/storage-pools", m.pool), roots); err != nil {
		m.t.Fatal(err)
	}
}

// No deferred resume or destructive cleanup: an ambiguous/failing sample retains
// the durable fixture, possibly paused, rather than acting on a changed owner.
func (m rootfsStorageObserver) observe(phase string, areas ...rootfsStorageArea) map[string]rootfsStorageSample {
	m.t.Helper()
	ctx, cancel := context.WithTimeout(m.ctx, 2*time.Minute)
	defer cancel()
	m.ctx = ctx
	unlock := m.lock(areas)
	defer unlock()
	pool := storageMeasurementPool(m.t, m.ctx, m.runtime, m.pool, m.command)
	statuses := map[string]int{}
	paths := map[string]bool{}
	consumers := map[string]bool{}
	before := map[string]os.FileInfo{}
	parents := map[string]os.FileInfo{}
	for _, area := range areas {
		path, _, status := m.path(area, 0)
		if paths[path] || area.live != nil && consumers[area.live.env.Name] {
			m.t.Fatal("duplicate rootfs measurement resource or consumer")
		}
		paths[path] = true
		if area.live != nil {
			consumers[area.live.env.Name] = true
		}
		statuses[area.label] = status
		info, err := os.Lstat(path)
		if err != nil {
			m.t.Fatal("rootfs directory identity unavailable")
		}
		before[area.label] = info
		parent, err := os.Lstat(filepath.Dir(path))
		if err != nil || !parent.IsDir() || parent.Mode()&os.ModeSymlink != 0 {
			m.t.Fatal("rootfs subvolume identity unavailable")
		}
		parents[area.label] = parent
	}
	recheck := func(area rootfsStorageArea) (string, string) {
		path, identity, _ := m.path(area, statuses[area.label])
		info, err := os.Lstat(path)
		if err != nil || !os.SameFile(info, before[area.label]) {
			m.t.Fatal("rootfs directory identity changed")
		}
		parent, parentErr := os.Lstat(filepath.Dir(path))
		if parentErr != nil || !parent.IsDir() || !os.SameFile(parent, parents[area.label]) {
			m.t.Fatal("rootfs subvolume identity changed")
		}
		return path, identity
	}
	for _, area := range areas {
		if statuses[area.label] == 103 {
			recheck(area)
			m.command("incus", "pause", "haco-"+area.live.env.Name, "--project", m.runtime.project)
			statuses[area.label] = 110
		}
		recheck(area)
	}
	m.mountScope(paths)
	m.command("btrfs", "filesystem", "sync", filepath.Join("/var/lib/incus/storage-pools", m.pool))
	result := map[string]rootfsStorageSample{}
	for _, area := range areas {
		path, identity := recheck(area)
		m.mountScope(paths)
		payload := filepath.Join(path, strings.TrimPrefix(rootfsMeasurementDirectory, "/"))
		hashes, err := storageMeasurementHashes(payload)
		if err != nil {
			m.t.Fatal(err)
		}
		sample := rootfsStorageSample{Hashes: hashes}
		for _, scope := range []struct {
			path   string
			sample *storageByteSample
		}{{path, &sample.Rootfs}, {payload, &sample.Payload}} {
			*scope.sample, err = parseStorageByteSample(m.command("du", "--summarize", "--apparent-size", "--block-size=1", "--", scope.path), m.command("du", "--summarize", "--block-size=1", "--", scope.path), m.command("btrfs", "filesystem", "du", "--raw", "--summarize", scope.path), scope.path)
			if err != nil {
				m.t.Fatal("invalid rootfs byte observation", err)
			}
		}
		result[area.label] = sample
		encoded, err := json.Marshal(sample)
		if err != nil {
			m.t.Fatal(err)
		}
		m.mountScope(paths)
		recheck(area)
		m.t.Logf("storage_measurement fixture=%s phase=%s area=%s identity_sha256=%s bytes=%s", rootfsMeasurementFixture, phase, area.label, identity, encoded)
	}
	pool.finish(phase, rootfsMeasurementFixture)
	for _, area := range areas {
		recheck(area)
		if statuses[area.label] == 110 {
			m.command("incus", "start", "haco-"+area.live.env.Name, "--project", m.runtime.project)
			statuses[area.label] = 103
			recheck(area)
		}
	}
	return result
}

func requireSharedRootfsPayload(sample rootfsStorageSample, hash string) error {
	if hash == "" || len(sample.Hashes) != 1 || sample.Hashes["base"] != hash || sample.Payload.LogicalBytes < storageMeasurementFileBytes || sample.Payload.AllocatedBytes == 0 || sample.Payload.ExtentTotalBytes == 0 || sample.Payload.ExtentSetSharedBytes == 0 || sample.Payload.ExtentExclusiveBytes >= sample.Payload.ExtentTotalBytes {
		return fmt.Errorf("rootfs payload content or extent sharing unproven")
	}
	return nil
}

func requireIndependentRootfsWrite(before, after rootfsStorageSample, hash string) error {
	if len(after.Hashes) != 2 || after.Hashes["base"] != hash || after.Hashes["delta"] == "" || after.Payload.LogicalBytes <= before.Payload.LogicalBytes || after.Payload.AllocatedBytes <= before.Payload.AllocatedBytes || after.Payload.ExtentExclusiveBytes <= before.Payload.ExtentExclusiveBytes {
		return fmt.Errorf("independent rootfs write allocation unproven")
	}
	return nil
}
