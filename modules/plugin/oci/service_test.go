package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeExecutor struct {
	outputs map[string]string
	calls   []fakeCall
}

type fakeCall struct {
	ref  string
	argv []string
}

func (f *fakeExecutor) ExecEnvironment(_ context.Context, ref string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	f.calls = append(f.calls, fakeCall{ref: ref, argv: append([]string(nil), req.Argv...)})
	output, ok := f.outputs[ref]
	if !ok {
		return core.ExecutionResult{ExitCode: 1, Stderr: "environment unavailable"}, errors.New("exec failed")
	}
	return core.ExecutionResult{ExitCode: 0, Stdout: output}, nil
}

func TestParseDriver(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  Driver
	}{
		{input: "nerdctl", want: DriverNerdctl},
		{input: " NERDCTL ", want: DriverNerdctl},
		{input: "docker", want: DriverDocker},
	} {
		got, err := ParseDriver(tc.input)
		if err != nil {
			t.Fatalf("ParseDriver(%q): %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("ParseDriver(%q)=%q want=%q", tc.input, got, tc.want)
		}
	}
	if _, err := ParseDriver("containerd"); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("unsupported driver err=%v", err)
	}
}

func TestSampleAllUsesNerdctlOnlyInsidePlugin(t *testing.T) {
	dir := t.TempDir()
	environmentPath := filepath.Join(dir, "environments.json")
	writeEnvironmentState(t, environmentPath, map[string]core.Environment{
		"a": {Name: "a", RuntimeRef: "ref-a"},
	})
	fingerprint := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	runtime := &fakeExecutor{outputs: map[string]string{
		"ref-a": "docker.io/library/node\t24\t" + fingerprint + "\n",
	}}
	service, err := New(runtime, environmentPath, NewStore(filepath.Join(dir, "oci-usage.json")), DriverNerdctl)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC) }

	report, err := service.SampleAll(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if report.Sampled != 1 || report.Failed != 0 {
		t.Fatalf("report=%#v", report)
	}
	want := []string{"nerdctl", "images", "--format", "{{.Repository}}\t{{.Tag}}\t{{.Digest}}"}
	if len(runtime.calls) != 1 || !reflect.DeepEqual(runtime.calls[0].argv, want) {
		t.Fatalf("calls=%#v want argv=%#v", runtime.calls, want)
	}
}

func TestSampleAllUsesDockerDriverWhenExplicitlySelected(t *testing.T) {
	dir := t.TempDir()
	environmentPath := filepath.Join(dir, "environments.json")
	writeEnvironmentState(t, environmentPath, map[string]core.Environment{
		"a": {Name: "a", RuntimeRef: "ref-a"},
	})
	fingerprint := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	runtime := &fakeExecutor{outputs: map[string]string{
		"ref-a": "docker.io/library/node\t24\t" + fingerprint + "\n",
	}}
	service, err := New(runtime, environmentPath, NewStore(filepath.Join(dir, "oci-usage.json")), DriverDocker)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC) }

	if _, err := service.SampleAll(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "images", "--digests", "--format", "{{.Repository}}\t{{.Tag}}\t{{.Digest}}"}
	if len(runtime.calls) != 1 || !reflect.DeepEqual(runtime.calls[0].argv, want) {
		t.Fatalf("calls=%#v want argv=%#v", runtime.calls, want)
	}
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
	service, err := New(runtime, environmentPath, store, DriverNerdctl)
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
	if recommendations[0].Reference != "docker.io/library/node:24" || recommendations[0].Digest != fingerprintA || recommendations[0].Environments != 2 || recommendations[0].Percent != 100 || !recommendations[0].AutoPromote {
		t.Fatalf("node recommendation=%#v", recommendations[0])
	}
	if recommendations[1].Reference != "docker.io/library/postgres:18" || recommendations[1].Environments != 1 || recommendations[1].Percent != 50 || recommendations[1].AutoPromote {
		t.Fatalf("postgres recommendation=%#v", recommendations[1])
	}
}

func TestAutoPromotionUsesTopTenPercentRoundedUp(t *testing.T) {
	recommendations := make([]Recommendation, 11)
	for i := range recommendations {
		recommendations[i].Reference = fmt.Sprintf("example.invalid/image-%02d:latest", i)
	}
	markAutoPromotions(recommendations, 10)
	for i, recommendation := range recommendations {
		want := i < 2
		if recommendation.AutoPromote != want {
			t.Fatalf("recommendation %d auto=%v want=%v", i, recommendation.AutoPromote, want)
		}
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
	service, err := New(runtime, environmentPath, store, DriverNerdctl)
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

func TestParseImageRowsRejectsMalformedDigest(t *testing.T) {
	_, err := parseImageRows("docker.io/library/node\t24\tsha256:not-a-digest\n", "nerdctl")
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
