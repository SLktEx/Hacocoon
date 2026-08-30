package seedstats

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeExecutor struct {
	outputs map[string]string
	calls   []string
}

func (f *fakeExecutor) ExecEnvironment(_ context.Context, ref string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	f.calls = append(f.calls, ref)
	if len(req.Argv) != 4 || req.Argv[0] != "nerdctl" || req.Argv[1] != "images" || req.Argv[2] != "--format" {
		return core.ExecutionResult{}, errors.New("unexpected command")
	}
	output, ok := f.outputs[ref]
	if !ok {
		return core.ExecutionResult{ExitCode: 1, Stderr: "environment unavailable"}, errors.New("exec failed")
	}
	return core.ExecutionResult{ExitCode: 0, Stdout: output}, nil
}

func TestSampleAllAndRecommendByEnvironmentShare(t *testing.T) {
	dir := t.TempDir()
	environmentPath := filepath.Join(dir, "environments.json")
	writeEnvironmentState(t, environmentPath, map[string]core.Environment{
		"a": {Name: "a", RuntimeRef: "ref-a"},
		"b": {Name: "b", RuntimeRef: "ref-b"},
	})
	fingerprintA := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	fingerprintB := "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	runtime := &fakeExecutor{outputs: map[string]string{
		"ref-a": "docker.io/library/node\t24\t" + fingerprintA + "\ndocker.io/library/postgres\t18\t" + fingerprintB + "\n",
		"ref-b": "docker.io/library/node\t24\t" + fingerprintA + "\n",
	}}
	store := NewStore(filepath.Join(dir, "oci-usage.json"))
	service, err := New(runtime, environmentPath, store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	report, err := service.SampleAll(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if report.Sampled != 2 || report.Failed != 0 || report.Fresh != 0 {
		t.Fatalf("report=%#v", report)
	}
	recommendations, err := service.Recommend(context.Background(), 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(recommendations) != 2 {
		t.Fatalf("recommendations=%#v", recommendations)
	}
	if recommendations[0].Reference != "docker.io/library/node:24" || recommendations[0].Digest != fingerprintA || recommendations[0].Environments != 2 || recommendations[0].Percent != 100 {
		t.Fatalf("node recommendation=%#v", recommendations[0])
	}
	if recommendations[1].Reference != "docker.io/library/postgres:18" || recommendations[1].Environments != 1 || recommendations[1].Percent != 50 {
		t.Fatalf("postgres recommendation=%#v", recommendations[1])
	}
}

func TestSampleAllSkipsFreshSnapshots(t *testing.T) {
	dir := t.TempDir()
	environmentPath := filepath.Join(dir, "environments.json")
	writeEnvironmentState(t, environmentPath, map[string]core.Environment{"a": {Name: "a", RuntimeRef: "ref-a"}})
	store := NewStore(filepath.Join(dir, "oci-usage.json"))
	now := time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC)
	if err := store.Put(context.Background(), Snapshot{Environment: "a", SampledAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	runtime := &fakeExecutor{outputs: map[string]string{"ref-a": ""}}
	service, err := New(runtime, environmentPath, store)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	report, err := service.SampleAll(context.Background(), 6*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if report.Fresh != 1 || report.Sampled != 0 || len(runtime.calls) != 0 {
		t.Fatalf("report=%#v calls=%#v", report, runtime.calls)
	}
}

func TestRecommendExcludesImagesWithoutDigest(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "oci-usage.json"))
	now := time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC)
	if err := store.Put(context.Background(), Snapshot{
		Environment: "a",
		SampledAt:   now,
		Images:      []Image{{Repository: "docker.io/library/node", Tag: "24"}},
	}); err != nil {
		t.Fatal(err)
	}
	service := &Service{store: store, now: func() time.Time { return now }}
	recommendations, err := service.Recommend(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(recommendations) != 0 {
		t.Fatalf("recommendations=%#v", recommendations)
	}
}

func TestParseNerdctlImagesRejectsMalformedDigest(t *testing.T) {
	_, err := parseNerdctlImages("docker.io/library/node\t24\tsha256:not-a-digest\n")
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v", err)
	}
}

func writeEnvironmentState(t *testing.T, path string, environments map[string]core.Environment) {
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
