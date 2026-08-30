package main

import (
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestParseResourceLimitStrictUnits(t *testing.T) {
	got, err := parseResourceLimit("8GiB", core.MaxMemoryResourceBytes, true)
	if err != nil {
		t.Fatal(err)
	}
	want := core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 8 << 30}
	if got != want {
		t.Fatalf("limit = %#v, want %#v", got, want)
	}
	for _, raw := range []string{"8G", "8GB", "8 GiB", "0GiB", "-1GiB", "+1GiB", "1.5GiB", "18446744073709551615TiB"} {
		if _, err := parseResourceLimit(raw, core.MaxMemoryResourceBytes, true); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("%q error = %v", raw, err)
		}
	}
}

func TestParseResourceLimitUnlimited(t *testing.T) {
	got, err := parseResourceLimit("unlimited", core.MaxCPUResourceValue, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != (core.ResourceLimit{Mode: core.ResourceLimitUnlimited}) {
		t.Fatalf("limit = %#v", got)
	}
}

func TestParseCreateSpecResources(t *testing.T) {
	spec, err := parseCreateSpec([]string{
		"--cpu", "4",
		"--memory", "8GiB",
		"--pids", "1024",
		"--root-size", "40GiB",
		"--workspace", "/tmp/work",
		"dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Resources.CPU.Value != 4 || spec.Resources.MemoryBytes.Value != 8<<30 || spec.Resources.PIDs.Value != 1024 || spec.Resources.RootBytes.Value != 40<<30 {
		t.Fatalf("resources = %#v", spec.Resources)
	}
	if _, err := parseCreateSpec([]string{"--cpu", "4", "--cpu", "8", "--workspace", "/tmp/work", "dev"}); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("duplicate cpu error = %v", err)
	}
}

func TestParseRunSpecResources(t *testing.T) {
	spec, _, err := parseRunSpec([]string{"--memory", "512MiB", "--workspace", "/tmp/work", "--", "true"})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Resources.MemoryBytes != (core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 512 << 20}) {
		t.Fatalf("memory = %#v", spec.Resources.MemoryBytes)
	}
}
