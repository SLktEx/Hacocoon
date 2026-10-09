//go:build linux

package incus

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/env"
	gitrepo "github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/state"
)

const workspaceMeasurementDirectory = "haco-storage-measurement"

// Only the selected fixture repository is measured. A saved area carries the
// complete canonical Snapshot; a live area carries its Workspace and generation.
// These are observation inputs, never authority to adopt or repair a resource.
type workspaceStorageArea struct {
	label      string
	work       gitrepo.Object
	env        core.Environment
	generation string
	saved      *core.Snapshot
}

type workspaceStorageSample struct {
	Volume  storageByteSample `json:"volume"`
	Payload storageByteSample `json:"payload"`
	Hashes  map[string]string `json:"payload_sha256"`
}

type workspaceStorageObserver struct {
	t       *testing.T
	ctx     context.Context
	runtime *Runtime
	catalog *state.EnvironmentJSONStore
	pool    string
}

func (m workspaceStorageObserver) command(name string, args ...string) string {
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

func (m workspaceStorageObserver) metadata(image string) {
	m.t.Helper()
	values := storageMeasurementVersions(m.t, m.command)
	values["storage_driver"] = "btrfs"
	values["input_image_fingerprint"] = image
	logStorageMeasurementVersions(m.t, "managed-workspace-lifecycle", values)
}

func workspaceMeasurementMember(work gitrepo.Object) (gitrepo.Object, error) {
	var found gitrepo.Object
	for _, member := range work.Copies() {
		if member.Repository == "one" {
			if found.ID != "" || member.Kind != "work" || member.State != "ready" {
				return found, fmt.Errorf("ambiguous measurement Workspace")
			}
			found = member
		}
	}
	if found.ID == "" {
		return found, fmt.Errorf("measurement Workspace unavailable")
	}
	return found, nil
}

func (m workspaceStorageObserver) savedVolume(saved core.Snapshot) snapshotVolumePlan {
	m.t.Helper()
	current, err := m.catalog.GetSnapshot(m.ctx, saved.ID)
	if err != nil || current.State != "ready" || !reflect.DeepEqual(current, saved) {
		m.t.Fatal("measurement Snapshot ownership changed")
	}
	var found *snapshotVolumePlan
	for _, component := range current.Components {
		prefix := "haco-runtime-v1:" + environmentapp.ProviderIncus + ":"
		if !strings.HasPrefix(component.NativeRef, prefix) || component.State != "verified" {
			m.t.Fatal("measurement Snapshot route unavailable")
		}
		ref, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(component.NativeRef, prefix))
		if err != nil {
			m.t.Fatal("invalid measurement Snapshot route")
		}
		component.NativeRef = string(ref)
		binding, err := m.runtime.decodeSnapshotComponent(component)
		if err != nil {
			m.t.Fatal("invalid measurement Snapshot binding")
		}
		if binding.Volume != nil && binding.Volume.SourceKind == "work" && binding.Volume.SourceID == "one" {
			if found != nil || binding.Volume.SourceInstanceID != saved.Source.InstanceID || !environmentapp.MatchesRuntimeRef(saved.Source.Environment.RuntimeRef, environmentapp.ProviderIncus, binding.Volume.SourceInstance) {
				m.t.Fatal("ambiguous measurement Snapshot Workspace")
			}
			found = binding.Volume
		}
	}
	if found == nil {
		m.t.Fatal("measurement Snapshot Workspace absent")
	}
	return *found
}

// This pure check is repeated before pause, after pause and before resume. It
// rejects a foreign/recreated instance, a changed attachment and unexpected state.
func validateWorkspaceMeasurementConsumer(instance snapshotInstanceObservation, status int, pool, volume, ref, generation string, wanted int) error {
	if wanted == 0 {
		if status != 102 && status != 103 {
			return fmt.Errorf("measurement consumer must initially be stopped or running")
		}
		wanted = status
	}
	if !core.ValidEnvironmentInstanceID(generation) || validateManagedInstanceRef(ref) != nil || ref == trustedHostName || instance.Name != ref || instance.Type != "container" || instance.Ephemeral || status != wanted {
		return fmt.Errorf("measurement consumer identity or state changed")
	}
	for _, config := range []map[string]string{instance.Config, instance.ExpandedConfig} {
		if config[environmentInstanceKey] != generation || config[managedEnvironmentMarkerKey] != managedEnvironmentMarkerValue {
			return fmt.Errorf("measurement consumer ownership changed")
		}
	}
	roots, workspaces := 0, 0
	for _, device := range instance.ExpandedDevices {
		if device["type"] != "disk" {
			continue
		}
		if device["path"] == "/" {
			if device["pool"] != pool || device["source"] != "" {
				return fmt.Errorf("measurement root pool changed")
			}
			roots++
		}
		if device["source"] == volume || device["path"] == "/workspace/one" {
			if device["pool"] != pool || device["source"] != volume || device["path"] != "/workspace/one" {
				return fmt.Errorf("measurement Workspace attachment changed")
			}
			workspaces++
		}
	}
	if roots != 1 || workspaces != 1 {
		return fmt.Errorf("measurement disk attachment ambiguous")
	}
	return nil
}

func (m workspaceStorageObserver) consumer(area workspaceStorageArea, volume string, wanted int) int {
	m.t.Helper()
	ref := "haco-" + area.env.Name
	current, err := m.catalog.GetEnvironment(m.ctx, area.env.Name)
	if err != nil || current.Workspace != area.env.Workspace || current.RuntimeRef != area.env.RuntimeRef || !environmentapp.MatchesRuntimeRef(current.RuntimeRef, environmentapp.ProviderIncus, ref) {
		m.t.Fatal("measurement Environment changed")
	}
	generation, err := m.catalog.EnvironmentInstance(m.ctx, current)
	if err != nil || generation != area.generation {
		m.t.Fatal("measurement Environment generation changed")
	}
	var observed struct {
		snapshotInstanceObservation
		StatusCode int `json:"status_code"`
	}
	if json.Unmarshal([]byte(m.command("incus", "query", "/1.0/instances/"+ref+"?project="+m.runtime.project)), &observed) != nil {
		m.t.Fatal("measurement consumer observation invalid")
	}
	if err := validateWorkspaceMeasurementConsumer(observed.snapshotInstanceObservation, observed.StatusCode, m.pool, volume, ref, generation, wanted); err != nil {
		m.t.Fatal(err)
	}
	return observed.StatusCode
}

func (m workspaceStorageObserver) path(area workspaceStorageArea, wanted int) (string, string, int) {
	m.t.Helper()
	var pool, volume, owner string
	status := 0
	if area.saved != nil {
		p := m.savedVolume(*area.saved)
		observed, err := m.runtime.snapshotVolumeObservation(m.ctx, p, true)
		if err != nil || observed == nil || len(observed.UsedBy) != 0 {
			m.t.Fatal("measurement saved Workspace ownership or consumers changed")
		}
		pool, volume, owner = p.Pool, p.target(), p.Owner
	} else {
		member, err := workspaceMeasurementMember(area.work)
		if err != nil {
			m.t.Fatal(err)
		}
		pool, volume, err = volumeRef(member)
		if err != nil || area.env.Workspace.ID != core.WorkspaceID("workspace:managed:"+area.work.Owner) {
			m.t.Fatal("measurement Workspace identity changed")
		}
		observed, err := (&RepositoryBackend{Runtime: m.runtime}).observeVolume(m.ctx, member)
		if err != nil || len(observed.UsedBy) != 1 || !environmentDataUsedBy(observed.UsedBy[0], m.runtime.project, "haco-"+area.env.Name) {
			m.t.Fatal("measurement Workspace ownership or consumers changed")
		}
		status = m.consumer(area, volume, wanted)
		owner = member.Owner
	}
	if pool != m.pool || !safeIncusRef(pool) {
		m.t.Fatal("measurement Workspace pool changed")
	}
	path := filepath.Join("/var/lib/incus/storage-pools", pool, "custom", m.runtime.project+"_"+volume)
	if err := storageMeasurementDirectory(path); err != nil {
		m.t.Fatal(err)
	}
	identity := sha256.Sum256([]byte(m.runtime.project + "\x00" + pool + "\x00" + volume + "\x00" + owner))
	return path, hex.EncodeToString(identity[:]), status
}

// Payload changes use only guest operations, under the same lifecycle order as
// observation. Recheck the full native ownership immediately around each exec.
func (m workspaceStorageObserver) guest(area workspaceStorageArea, script string) {
	m.t.Helper()
	ctx, cancel := context.WithTimeout(m.ctx, 2*time.Minute)
	defer cancel()
	m.ctx = ctx
	if area.saved != nil {
		m.t.Fatal("saved measurement data cannot be mutated")
	}
	unlock, err := m.catalog.LockLifecycle(m.ctx, "environment", area.env.Name)
	if err != nil {
		m.t.Fatal("measurement guest lifecycle exclusion unavailable")
	}
	defer unlock()
	release, err := m.catalog.LockLifecycle(m.ctx, "workspace", string(area.env.Workspace.ID))
	if err != nil {
		m.t.Fatal("measurement guest Workspace exclusion unavailable")
	}
	defer release()
	m.path(area, 103)
	m.command("incus", "exec", "haco-"+area.env.Name, "--project", m.runtime.project, "--", "/bin/sh", "-eu", "-c", script, "--", "/workspace/one/"+workspaceMeasurementDirectory)
	m.path(area, 103)
}

// observe holds the same Environment-then-Workspace locks as lifecycle operations.
// Only exact running fixture consumers are paused; stopped consumers stay stopped.
// Ambiguous ownership/state or failed observation retains the fixture in its
// last observed or uncertain state, which may include paused consumers. No
// deferred destructive cleanup or best-effort resume occurs.
func (m workspaceStorageObserver) observe(phase string, areas ...workspaceStorageArea) map[string]workspaceStorageSample {
	m.t.Helper()
	ctx, cancel := context.WithTimeout(m.ctx, 2*time.Minute)
	defer cancel()
	m.ctx = ctx
	envs, works := map[string]bool{}, map[string]bool{}
	labels := map[string]bool{}
	for _, area := range areas {
		if area.label == "" || labels[area.label] {
			m.t.Fatal("duplicate measurement area")
		}
		labels[area.label] = true
		env := area.env
		if area.saved != nil {
			env = area.saved.Source.Environment
		}
		envs[env.Name], works[string(env.Workspace.ID)] = true, true
	}
	for i, identities := range []map[string]bool{envs, works} {
		keys := make([]string, 0, len(identities))
		for key := range identities {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			unlock, err := m.catalog.LockLifecycle(m.ctx, []string{"environment", "workspace"}[i], key)
			if err != nil {
				m.t.Fatal("measurement lifecycle exclusion unavailable")
			}
			defer unlock()
		}
	}
	pool := storageMeasurementPool(m.t, m.ctx, m.runtime, m.pool, m.command)
	statuses := map[string]int{}
	paths := map[string]bool{}
	consumers := map[string]bool{}
	for _, area := range areas {
		path, _, status := m.path(area, 0)
		if paths[path] || area.saved == nil && consumers[area.env.Name] {
			m.t.Fatal("duplicate measurement resource or consumer")
		}
		paths[path] = true
		if area.saved == nil {
			consumers[area.env.Name] = true
		}
		statuses[area.label] = status
	}
	for _, area := range areas {
		if statuses[area.label] == 103 {
			m.command("incus", "pause", "haco-"+area.env.Name, "--project", m.runtime.project)
			statuses[area.label] = 110
		}
		m.path(area, statuses[area.label])
	}
	m.command("btrfs", "filesystem", "sync", filepath.Join("/var/lib/incus/storage-pools", m.pool))
	result := map[string]workspaceStorageSample{}
	for _, area := range areas {
		path, identity, _ := m.path(area, statuses[area.label])
		payload := filepath.Join(path, workspaceMeasurementDirectory)
		hashes, err := storageMeasurementHashes(payload)
		if err != nil {
			m.t.Fatal(err)
		}
		sample := workspaceStorageSample{Hashes: hashes}
		for _, scope := range []struct {
			path   string
			sample *storageByteSample
		}{{path, &sample.Volume}, {payload, &sample.Payload}} {
			*scope.sample, err = parseStorageByteSample(
				m.command("du", "--summarize", "--apparent-size", "--block-size=1", "--", scope.path),
				m.command("du", "--summarize", "--block-size=1", "--", scope.path),
				m.command("btrfs", "filesystem", "du", "--raw", "--summarize", scope.path), scope.path)
			if err != nil {
				m.t.Fatal("invalid Workspace storage observation", err)
			}
		}
		result[area.label] = sample
		encoded, err := json.Marshal(sample)
		if err != nil {
			m.t.Fatal(err)
		}
		m.t.Logf("storage_measurement fixture=managed-workspace-lifecycle phase=%s area=%s identity_sha256=%s bytes=%s", phase, area.label, identity, encoded)
		m.path(area, statuses[area.label])
	}
	pool.finish(phase, "managed-workspace-lifecycle")
	for _, area := range areas {
		if statuses[area.label] == 110 {
			m.path(area, 110)
			m.command("incus", "start", "haco-"+area.env.Name, "--project", m.runtime.project)
			m.path(area, 103)
		}
	}
	return result
}

// Identical hashes are content evidence, not extent-sharing evidence.
func requireSharedWorkspacePayload(sample workspaceStorageSample, hash string) error {
	if hash == "" || len(sample.Hashes) != 1 || sample.Hashes["base"] != hash || sample.Payload.LogicalBytes < storageMeasurementFileBytes || sample.Payload.ExtentTotalBytes == 0 || sample.Payload.ExtentSetSharedBytes == 0 || sample.Payload.ExtentExclusiveBytes >= sample.Payload.ExtentTotalBytes {
		return fmt.Errorf("Workspace payload content or extent sharing unproven")
	}
	return nil
}
