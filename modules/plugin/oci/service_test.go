package oci

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

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
