package incus

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const compatFingerprint = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

func TestImageInfoCompatRunnerKeepsModernJSONPath(t *testing.T) {
	calls := 0
	next := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		calls++
		if name != "incus" || !reflect.DeepEqual(args, []string{"image", "info", "images:ubuntu/24.04", "--format", "json"}) {
			t.Fatalf("unexpected call: %s %#v", name, args)
		}
		return host.Result{Stdout: `{"fingerprint":"` + compatFingerprint + `"}`}, nil
	}}

	result, err := wrapImageInfoCompatRunner(next).Run(context.Background(), "incus", "image", "info", "images:ubuntu/24.04", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || result.Stdout != `{"fingerprint":"`+compatFingerprint+`"}` {
		t.Fatalf("calls=%d result=%#v", calls, result)
	}
}

func TestImageInfoCompatRunnerFallsBackForIncusSix(t *testing.T) {
	next := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		if name != "incus" {
			t.Fatalf("unexpected command: %s", name)
		}
		switch {
		case reflect.DeepEqual(args, []string{"image", "info", "local:alias", "--project", "hacocoon", "--format", "json"}):
			return host.Result{ExitCode: 1, Stderr: "Error: unknown flag: --format"}, errors.New("exit status 1")
		case reflect.DeepEqual(args, []string{"image", "info", "local:alias", "--project", "hacocoon"}):
			return host.Result{Stdout: "Fingerprint: " + compatFingerprint + "\nSize: 12.34MiB\nArchitecture: x86_64\n"}, nil
		default:
			t.Fatalf("unexpected args: %#v", args)
			return host.Result{}, nil
		}
	}}

	result, err := wrapImageInfoCompatRunner(next).Run(context.Background(), "incus", "image", "info", "local:alias", "--project", "hacocoon", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Fingerprint != compatFingerprint {
		t.Fatalf("fingerprint=%q", decoded.Fingerprint)
	}
}

func TestImageInfoCompatRunnerRejectsAmbiguousLegacyOutput(t *testing.T) {
	next := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if args[len(args)-2] == "--format" {
			return host.Result{ExitCode: 1, Stderr: "unknown flag"}, errors.New("exit status 1")
		}
		return host.Result{Stdout: "Fingerprint: " + compatFingerprint + "\nFingerprint: " + compatFingerprint + "\n"}, nil
	}}

	_, err := wrapImageInfoCompatRunner(next).Run(context.Background(), "incus", "image", "info", "images:ubuntu/24.04", "--format", "json")
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v want ErrIncompatibleState", err)
	}
}

func TestImageInfoCompatRunnerRejectsMalformedOrTruncatedLegacyOutput(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result host.Result
	}{
		{name: "malformed", result: host.Result{Stdout: "Fingerprint: ../../mutable\n"}},
		{name: "missing", result: host.Result{Stdout: "Size: 12MiB\n"}},
		{name: "truncated", result: host.Result{Stdout: "Fingerprint: " + compatFingerprint + "\n", StdoutTruncated: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if args[len(args)-2] == "--format" {
					return host.Result{ExitCode: 1, Stderr: "unknown flag"}, errors.New("exit status 1")
				}
				return tc.result, nil
			}}
			_, err := wrapImageInfoCompatRunner(next).Run(context.Background(), "incus", "image", "info", "images:ubuntu/24.04", "--format", "json")
			if !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatalf("err=%v want ErrIncompatibleState", err)
			}
		})
	}
}
