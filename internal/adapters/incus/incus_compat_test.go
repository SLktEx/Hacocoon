package incus

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestShowProfileJSONFallsBackToRawQuery(t *testing.T) {
	unsupportedFormat := errors.New("unknown flag: --format")
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case reflect.DeepEqual(args, []string{"profile", "show", "default", "--project", defaultResourceProject, "--format", "json"}):
			return host.Result{ExitCode: 1, Stderr: "Error: unknown flag: --format"}, unsupportedFormat
		case reflect.DeepEqual(args, []string{"query", "/1.0/profiles/default?project=default"}):
			return rootProfileResult(), nil
		default:
			return host.Result{}, errors.New("unexpected Incus call")
		}
	}}

	result, err := New(runner).showProfileJSON(context.Background(), "default", defaultResourceProject)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != rootProfileResult().Stdout {
		t.Fatalf("profile JSON = %q", result.Stdout)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}
