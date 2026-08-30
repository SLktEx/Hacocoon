package incus

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
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

func TestImageInfoCompatRunnerUsesInstanceAPIForExpandedConfig(t *testing.T) {
	calls := 0
	next := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		calls++
		if name != "incus" || !reflect.DeepEqual(args, []string{"query", "/1.0/instances/builder?project=hacocoon"}) {
			t.Fatalf("unexpected call: %s %#v", name, args)
		}
		return host.Result{Stdout: `{"expanded_devices":{"root":{"type":"disk","path":"/"}}}`}, nil
	}}

	result, err := wrapImageInfoCompatRunner(next).Run(context.Background(), "incus", "config", "show", "builder", "--project", "hacocoon", "--expanded", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d want=1", calls)
	}
	var decoded struct {
		Devices map[string]map[string]string `json:"devices"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Devices["root"]["type"] != "disk" {
		t.Fatalf("devices=%#v", decoded.Devices)
	}
}

func TestImageInfoCompatRunnerExpandedConfigFallsBackOnlyWhenQueryUnsupported(t *testing.T) {
	calls := 0
	next := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		calls++
		if name != "incus" {
			t.Fatalf("unexpected command: %s", name)
		}
		switch {
		case reflect.DeepEqual(args, []string{"query", "/1.0/instances/builder?project=hacocoon"}):
			return host.Result{ExitCode: 2, Stderr: "unknown command query"}, errors.New("exit status 2")
		case reflect.DeepEqual(args, []string{"config", "show", "builder", "--project", "hacocoon", "--expanded", "--format", "json"}):
			return host.Result{Stdout: `{"devices":{"root":{"type":"disk","path":"/"}}}`}, nil
		default:
			t.Fatalf("unexpected args: %#v", args)
			return host.Result{}, nil
		}
	}}

	result, err := wrapImageInfoCompatRunner(next).Run(context.Background(), "incus", "config", "show", "builder", "--project", "hacocoon", "--expanded", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !strings.Contains(result.Stdout, `"devices"`) {
		t.Fatalf("calls=%d result=%#v", calls, result)
	}
}

func TestImageInfoCompatRunnerExpandedConfigFailsClosedOnBadAPIState(t *testing.T) {
	for _, raw := range []string{`{}`, `{"expanded_devices":`, `{"devices":{}}`} {
		t.Run(raw, func(t *testing.T) {
			next := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if !reflect.DeepEqual(args, []string{"query", "/1.0/instances/builder?project=hacocoon"}) {
					t.Fatalf("unexpected args: %#v", args)
				}
				return host.Result{Stdout: raw}, nil
			}}
			_, err := wrapImageInfoCompatRunner(next).Run(context.Background(), "incus", "config", "show", "builder", "--project", "hacocoon", "--expanded", "--format", "json")
			if !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatalf("err=%v want ErrIncompatibleState", err)
			}
		})
	}
}

func TestImageInfoCompatRunnerPreservesGuestSystemctlShowStderr(t *testing.T) {
	processErr := errors.New("exit status 1")
	next := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		if name != "incus" || !reflect.DeepEqual(args, []string{"exec", "builder", "--project", "hacocoon", "--", "systemctl", "show", "-p", "ActiveState", "--value", "containerd.service"}) {
			t.Fatalf("unexpected call: %s %#v", name, args)
		}
		return host.Result{ExitCode: 1, Stderr: "Unit containerd.service could not be found"}, processErr
	}}

	_, err := wrapImageInfoCompatRunner(next).Run(context.Background(), "incus", "exec", "builder", "--project", "hacocoon", "--", "systemctl", "show", "-p", "ActiveState", "--value", "containerd.service")
	if !errors.Is(err, processErr) {
		t.Fatalf("err=%v want wrapped process error", err)
	}
	if !strings.Contains(err.Error(), "Unit containerd.service could not be found") {
		t.Fatalf("err=%v missing stderr", err)
	}
}

func TestImageInfoCompatRunnerWaitsForGuestSystemdAndUnitSettlement(t *testing.T) {
	calls := 0
	next := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		calls++
		if name != "incus" || !reflect.DeepEqual(args, []string{"exec", "builder", "--project", "hacocoon", "--", "systemctl", "show", "-p", "ActiveState", "--value", "containerd.service"}) {
			t.Fatalf("unexpected call: %s %#v", name, args)
		}
		switch calls {
		case 1:
			return host.Result{ExitCode: 1, Stderr: "Failed to connect to system scope bus via local transport: No such file or directory"}, errors.New("exit status 1")
		case 2:
			return host.Result{Stdout: "activating\n"}, nil
		default:
			return host.Result{Stdout: "active\n"}, nil
		}
	}}

	result, err := wrapImageInfoCompatRunner(next).Run(context.Background(), "incus", "exec", "builder", "--project", "hacocoon", "--", "systemctl", "show", "-p", "ActiveState", "--value", "containerd.service")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || strings.TrimSpace(result.Stdout) != "active" {
		t.Fatalf("calls=%d result=%#v", calls, result)
	}
}
