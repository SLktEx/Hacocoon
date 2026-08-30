package seedstats

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type deleteExecutor struct {
	images map[string]string
	rmi    []string
}

func (f *deleteExecutor) ExecEnvironment(_ context.Context, ref string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	if reflect.DeepEqual(req.Argv, imageListArgv()) {
		return core.ExecutionResult{ExitCode: 0, Stdout: f.images[ref]}, nil
	}
	if len(req.Argv) == 3 && req.Argv[0] == "nerdctl" && req.Argv[1] == "rmi" {
		f.rmi = append(f.rmi, ref+":"+req.Argv[2])
		f.images[ref] = ""
		return core.ExecutionResult{ExitCode: 0}, nil
	}
	return core.ExecutionResult{ExitCode: 1, Stderr: "unexpected command"}, errors.New("unexpected command")
}

type deleteHostRunner struct {
	images string
	rmi    []string
}

func (f *deleteHostRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	if name != "nerdctl" {
		return host.Result{ExitCode: 1, Stderr: "unexpected binary"}, errors.New("unexpected binary")
	}
	if reflect.DeepEqual(args, []string{"--namespace", DefaultSeedNamespace, "images", "--format", "{{.Repository}}\t{{.Tag}}\t{{.Digest}}"}) {
		return host.Result{ExitCode: 0, Stdout: f.images}, nil
	}
	if len(args) == 4 && args[0] == "--namespace" && args[1] == DefaultSeedNamespace && args[2] == "rmi" {
		f.rmi = append(f.rmi, args[3])
		f.images = ""
		return host.Result{ExitCode: 0}, nil
	}
	return host.Result{ExitCode: 1, Stderr: "unexpected command"}, errors.New("unexpected command")
}

func TestDeleteImageRemovesHostAndAllEnvironmentsAndSuppressesAutoPromotion(t *testing.T) {
	dir := t.TempDir()
	environmentPath := filepath.Join(dir, "environments.json")
	writeDeleteEnvironmentState(t, environmentPath, map[string]core.Environment{
		"a": {Name: "a", RuntimeRef: "ref-a"},
		"b": {Name: "b", RuntimeRef: "ref-b"},
	})
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	row := "docker.io/library/node\t24\t" + digest + "\n"
	store := NewStore(filepath.Join(dir, "oci-usage.json"))
	old := time.Date(2026, 8, 30, 5, 0, 0, 0, time.UTC)
	for _, environment := range []string{"a", "b"} {
		if err := store.Put(context.Background(), Snapshot{
			Environment: environment,
			SampledAt:   old,
			Images:      []Image{{Repository: "docker.io/library/node", Tag: "24", Digest: digest}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	runtime := &deleteExecutor{images: map[string]string{"ref-a": row, "ref-b": row}}
	hostRunner := &deleteHostRunner{images: row}
	service, err := New(runtime, environmentPath, store, WithHostRunner(hostRunner))
	if err != nil {
		t.Fatal(err)
	}
	deletedAt := old.Add(time.Hour)
	service.now = func() time.Time { return deletedAt }

	report, err := service.DeleteImage(context.Background(), "docker.io/library/node:24", true)
	if err != nil {
		t.Fatal(err)
	}
	if report.HostCache != "removed" || !report.SeedRebuildRequired {
		t.Fatalf("report=%#v", report)
	}
	if !reflect.DeepEqual(report.RemovedEnvironments, []string{"a", "b"}) {
		t.Fatalf("removed=%#v", report.RemovedEnvironments)
	}
	if !reflect.DeepEqual(hostRunner.rmi, []string{"docker.io/library/node:24"}) {
		t.Fatalf("host rmi=%#v", hostRunner.rmi)
	}

	recommendations, err := service.Recommend(context.Background(), 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(recommendations) != 0 {
		t.Fatalf("deleted image survived recommendation: %#v", recommendations)
	}

	// Runtime use after deletion is allowed, but manual deletion is an explicit
	// Seed-selection override. A later sample must not silently auto-promote the
	// exact deleted identity again.
	newSample := deletedAt.Add(time.Hour)
	if err := store.Put(context.Background(), Snapshot{
		Environment: "a",
		SampledAt:   newSample,
		Images:      []Image{{Repository: "docker.io/library/node", Tag: "24", Digest: digest}},
	}); err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return newSample }
	recommendations, err = service.Recommend(context.Background(), 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(recommendations) != 0 {
		t.Fatalf("manual deletion must override automatic promotion: %#v", recommendations)
	}
}

func TestDeleteImageRefusesMovedTag(t *testing.T) {
	dir := t.TempDir()
	environmentPath := filepath.Join(dir, "environments.json")
	writeDeleteEnvironmentState(t, environmentPath, map[string]core.Environment{
		"a": {Name: "a", RuntimeRef: "ref-a"},
	})
	oldDigest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newDigest := "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	store := NewStore(filepath.Join(dir, "oci-usage.json"))
	if err := store.Put(context.Background(), Snapshot{
		Environment: "a",
		SampledAt:   time.Now().UTC(),
		Images:      []Image{{Repository: "docker.io/library/node", Tag: "24", Digest: oldDigest}},
	}); err != nil {
		t.Fatal(err)
	}
	runtime := &deleteExecutor{images: map[string]string{
		"ref-a": "docker.io/library/node\t24\t" + newDigest + "\n",
	}}
	hostRunner := &deleteHostRunner{}
	service, err := New(runtime, environmentPath, store, WithHostRunner(hostRunner))
	if err != nil {
		t.Fatal(err)
	}

	report, err := service.DeleteImage(context.Background(), "docker.io/library/node:24", true)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v report=%#v", err, report)
	}
	if len(runtime.rmi) != 0 || len(hostRunner.rmi) != 0 {
		t.Fatalf("moved tag must not be deleted: env=%#v host=%#v", runtime.rmi, hostRunner.rmi)
	}
}

func TestExplicitDigestDoesNotDeleteMovedTag(t *testing.T) {
	dir := t.TempDir()
	environmentPath := filepath.Join(dir, "environments.json")
	writeDeleteEnvironmentState(t, environmentPath, map[string]core.Environment{
		"a": {Name: "a", RuntimeRef: "ref-a"},
	})
	oldDigest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newDigest := "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	runtime := &deleteExecutor{images: map[string]string{
		"ref-a": "docker.io/library/node\t24\t" + newDigest + "\n",
	}}
	hostRunner := &deleteHostRunner{images: "docker.io/library/node\t24\t" + newDigest + "\n"}
	store := NewStore(filepath.Join(dir, "oci-usage.json"))
	service, err := New(runtime, environmentPath, store, WithHostRunner(hostRunner))
	if err != nil {
		t.Fatal(err)
	}

	report, err := service.DeleteImage(context.Background(), "docker.io/library/node:24@"+oldDigest, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.HostCache != "not-present" || len(report.RemovedEnvironments) != 0 || !reflect.DeepEqual(report.SkippedEnvironments, []string{"a"}) {
		t.Fatalf("report=%#v", report)
	}
	if len(runtime.rmi) != 0 || len(hostRunner.rmi) != 0 {
		t.Fatalf("explicit old digest must not delete moved tag: env=%#v host=%#v", runtime.rmi, hostRunner.rmi)
	}
	deletions, err := store.ListDeletions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(deletions) != 1 || deletions[0].Digest != oldDigest {
		t.Fatalf("deletions=%#v", deletions)
	}
}

func TestStoreMigratesV1AndPersistsDeletion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oci-usage.json")
	legacy := map[string]any{"version": 1, "snapshots": map[string]any{}}
	payload, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	deletion := Deletion{
		Reference: "docker.io/library/node:24",
		Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		DeletedAt: time.Now().UTC(),
	}
	if err := store.PutDeletion(context.Background(), deletion); err != nil {
		t.Fatal(err)
	}
	deletions, err := store.ListDeletions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(deletions) != 1 || deletions[0].Key() != deletion.Key() {
		t.Fatalf("deletions=%#v", deletions)
	}
}

func writeDeleteEnvironmentState(t *testing.T, path string, environments map[string]core.Environment) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"version": 3, "environments": environments})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
}
