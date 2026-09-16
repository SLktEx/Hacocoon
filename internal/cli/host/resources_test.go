package hostcli

import (
	"errors"
	"strconv"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestHostCreateResourceLimitsKeepUnitsAndBounds(t *testing.T) {
	for _, test := range []struct {
		flag, input string
		want        uint64
	}{
		{"--cpu", "4096", 4096}, {"--pids", "2147483647", 2147483647},
		{"--memory", "1B", 1}, {"--memory", "2KiB", 2 << 10},
		{"--memory", "3MiB", 3 << 20}, {"--memory", "4GiB", 4 << 30},
		{"--root-size", "5TiB", 5 << 40}, {"--root-size", "1048576TiB", 1 << 60},
	} {
		t.Run(test.flag+"/"+test.input, func(t *testing.T) {
			request, err := parseCreateRequest([]string{test.flag, test.input, "--workspace", "/work", "demo"})
			if err != nil {
				t.Fatal(err)
			}
			budget, err := core.ResolveResourceBudget(request.Resources)
			if err != nil {
				t.Fatal("CLI accepted a limit the Core rejects", err)
			}
			limits := map[string]core.ResourceLimit{"--cpu": budget.CPU, "--memory": budget.MemoryBytes, "--pids": budget.PIDs, "--root-size": budget.RootBytes}
			if got := limits[test.flag]; got.Mode != core.ResourceLimitFinite || got.Value != test.want {
				t.Fatal("resource amount changed", got)
			}
		})
	}
	for _, flag := range []string{"--cpu", "--pids", "--memory", "--root-size"} {
		for _, input := range []string{"", " ", "+1", "-1", "0", "1.5", "1e3", "1 ", "unlimited ", "Unlimited", "18446744073709551616"} {
			_, err := parseCreateRequest([]string{flag, input, "--workspace", "/work", "demo"})
			if !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("accepted %s %q: %v", flag, input, err)
			}
		}
		request, err := parseCreateRequest([]string{flag, "unlimited", "--workspace", "/work", "demo"})
		budget, resolveErr := core.ResolveResourceBudget(request.Resources)
		if err != nil || resolveErr != nil || budget != core.UnlimitedResourceBudget() {
			t.Fatal("explicit unlimited changed", flag, budget, err, resolveErr)
		}
	}
	for _, test := range []struct{ flag, input string }{
		{"--cpu", strconv.FormatUint(core.MaxCPUResourceValue+1, 10)},
		{"--pids", strconv.FormatUint(core.MaxPIDResourceValue+1, 10)},
		{"--memory", "1048577TiB"}, {"--root-size", "18446744073709551615TiB"},
		{"--memory", "KiB"}, {"--memory", "1.5GiB"}, {"--memory", "1GB"},
		{"--memory", "0B"}, {"--memory", "18446744073709551616B"},
	} {
		_, err := parseCreateRequest([]string{test.flag, test.input, "--workspace", "/work", "demo"})
		if !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("accepted %s %q: %v", test.flag, test.input, err)
		}
	}
}

func TestHostCreateRejectsAmbiguousAndIncompleteOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--read-only", "--read-only", "--workspace", "/work", "demo"},
		{"--base", "a", "--base", "b", "--workspace", "/work", "demo"},
		{"--cpu", "1", "--cpu", "2", "--workspace", "/work", "demo"},
		{"--workspace", "/work", "--base"}, {"--workspace", "/work", "--cpu"},
		{"--workspace", "/work", "--read-only"}, {"--workspace", " ", "demo"},
		{"--base", "development", "demo"}, {"--workspace", "/work", "--workspace"},
		{"--unknown", "value", "--workspace", "/work", "demo"},
		{"--workspace", "/work", ""}, {"--workspace", "/work", " "},
		{"--workspace", "/work", "--unknown"},
	} {
		if _, err := parseCreateRequest(args); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("accepted ambiguous creation %v: %v", args, err)
		}
	}
	request, err := parseCreateRequest([]string{"--workspace", "/work", "demo-2"})
	if err != nil || request.Name != "demo-2" || request.AccessMode != core.WorkspaceReadWrite {
		t.Fatal("ordinary creation without optional flags was rejected", request, err)
	}
}
