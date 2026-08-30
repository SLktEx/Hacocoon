package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
	seedbuildapp "github.com/SLktEx/Hacocoon/internal/seedbuild"
	ociplugin "github.com/SLktEx/Hacocoon/modules/plugin/oci"
)

func ociPluginCommand(ctx context.Context, app *composition.App, args []string) error {
	if app == nil || app.OCI == nil {
		return fmt.Errorf("OCI plugin is disabled; set HACO_PLUGIN_OCI=nerdctl or HACO_PLUGIN_OCI=docker: %w", core.ErrRuntimeUnavailable)
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: haco plugin oci <status|image|seed|docker> ...: %w", core.ErrInvalidArgument)
	}
	switch args[0] {
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("usage: haco plugin oci status: %w", core.ErrInvalidArgument)
		}
		fmt.Printf("driver: %s\n", app.OCI.Driver())
		return nil
	case "image":
		return ociImageCommand(ctx, app, args[1:])
	case "seed":
		return ociSeedCommand(ctx, app, args[1:])
	case "docker":
		return ociDockerCommand(ctx, app, args[1:])
	default:
		return fmt.Errorf("unknown OCI plugin command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func ociImageCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 || args[0] != "delete" {
		return fmt.Errorf("usage: haco plugin oci image delete <reference[@sha256:...]> [--all-environments] [--json]: %w", core.ErrInvalidArgument)
	}
	return ociImageDeleteCommand(ctx, app, args[1:])
}

func ociImageDeleteCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: haco plugin oci image delete <reference[@sha256:...]> [--all-environments] [--json]: %w", core.ErrInvalidArgument)
	}
	target := args[0]
	allEnvironments := false
	jsonOutput := false
	for _, arg := range args[1:] {
		switch arg {
		case "--all-environments":
			if allEnvironments {
				return core.ErrInvalidArgument
			}
			allEnvironments = true
		case "--json":
			if jsonOutput {
				return core.ErrInvalidArgument
			}
			jsonOutput = true
		default:
			return fmt.Errorf("unknown OCI image delete option %q: %w", arg, core.ErrInvalidArgument)
		}
	}

	report, deleteErr := app.OCI.DeleteImage(ctx, target, allEnvironments)
	if jsonOutput {
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			return err
		}
	} else {
		printOCIImageDeleteReport(report)
	}
	return deleteErr
}

func printOCIImageDeleteReport(report ociplugin.DeleteReport) {
	if report.Reference != "" {
		fmt.Printf("image: %s@%s\n", report.Reference, report.Digest)
	}
	if report.HostCache != "" {
		fmt.Printf("host-cache: %s\n", report.HostCache)
	}
	if report.SeedRebuildRequired {
		fmt.Println("seed: rebuild-required")
	}
	for _, environment := range report.RemovedEnvironments {
		fmt.Printf("environment: %s\tremoved\n", environment)
	}
	for _, environment := range report.SkippedEnvironments {
		fmt.Printf("environment: %s\tnot-present\n", environment)
	}
	for environment, reason := range report.Failures {
		fmt.Fprintf(os.Stderr, "environment: %s\tfailed\t%s\n", environment, reason)
	}
}

func ociSeedCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: haco plugin oci seed <sample|recommend|pin|unpin|re-enable|build|current> ...: %w", core.ErrInvalidArgument)
	}

	switch args[0] {
	case "sample":
		jsonOutput, err := parseOCISeedJSONOnly(args[1:])
		if err != nil {
			return err
		}
		report, err := app.OCI.SampleAll(ctx, 0)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(report)
		}
		printOCISampleReport(report)
		return nil
	case "recommend":
		jsonOutput, err := parseOCISeedJSONOnly(args[1:])
		if err != nil {
			return err
		}
		report, err := app.OCI.SampleAll(ctx, ociplugin.DefaultSampleMaxAge)
		if err != nil {
			return err
		}
		if !jsonOutput {
			printOCISampleWarnings(report)
		}
		recommendations, err := app.OCI.Recommend(ctx, ociplugin.DefaultRecommendationWindow)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(struct {
				Sampling        ociplugin.SampleReport     `json:"sampling"`
				Recommendations []ociplugin.Recommendation `json:"recommendations"`
			}{Sampling: report, Recommendations: recommendations})
		}
		for _, recommendation := range recommendations {
			promotion := "recommend"
			switch {
			case recommendation.Pinned && recommendation.AutoPromote:
				promotion = "pin+auto"
			case recommendation.Pinned:
				promotion = "pin"
			case recommendation.AutoPromote:
				promotion = "auto"
			}
			lastSeen := "never"
			if !recommendation.LastSeen.IsZero() {
				lastSeen = recommendation.LastSeen.UTC().Format("2006-01-02T15:04:05Z")
			}
			fmt.Printf("%s\t%s@%s\t%d envs\t%.1f%%\tlast=%s\tre-enabled=%t\n",
				promotion,
				recommendation.Reference,
				recommendation.Digest,
				recommendation.Environments,
				recommendation.Percent,
				lastSeen,
				recommendation.Reenabled,
			)
		}
		return nil
	case "pin":
		target, reenable, jsonOutput, err := parseOCISeedSelectionOptions(args[1:], true)
		if err != nil {
			return err
		}
		selection, selectionErr := app.OCI.PinSeedImage(ctx, target, reenable)
		if jsonOutput {
			if err := json.NewEncoder(os.Stdout).Encode(selection); err != nil {
				return err
			}
		} else {
			printOCISeedSelection(selection)
		}
		return selectionErr
	case "unpin":
		target, _, jsonOutput, err := parseOCISeedSelectionOptions(args[1:], false)
		if err != nil {
			return err
		}
		selection, selectionErr := app.OCI.UnpinSeedImage(ctx, target)
		if jsonOutput {
			if err := json.NewEncoder(os.Stdout).Encode(selection); err != nil {
				return err
			}
		} else {
			printOCISeedSelection(selection)
		}
		return selectionErr
	case "re-enable":
		target, _, jsonOutput, err := parseOCISeedSelectionOptions(args[1:], false)
		if err != nil {
			return err
		}
		selection, selectionErr := app.OCI.ReenableSeedImage(ctx, target)
		if jsonOutput {
			if err := json.NewEncoder(os.Stdout).Encode(selection); err != nil {
				return err
			}
		} else {
			printOCISeedSelection(selection)
		}
		return selectionErr
	case "build":
		if app.Seeds == nil {
			return fmt.Errorf("OCI Seed builder is unavailable for the configured plugin: %w", core.ErrRuntimeUnavailable)
		}
		base, jsonOutput, err := parseOCISeedBaseOptions(args[1:])
		if err != nil {
			return err
		}
		report, err := app.Seeds.Build(ctx, base)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(report)
		}
		printOCISampleWarnings(report.Sampling)
		printOCISeedBuildReport(report)
		return nil
	case "current":
		if app.Seeds == nil {
			return fmt.Errorf("OCI Seed builder is unavailable for the configured plugin: %w", core.ErrRuntimeUnavailable)
		}
		base, jsonOutput, err := parseOCISeedBaseOptions(args[1:])
		if err != nil {
			return err
		}
		manifest, err := app.Seeds.Current(ctx, base)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(manifest)
		}
		printOCISeedManifest(manifest)
		return nil
	default:
		return fmt.Errorf("unknown OCI seed command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func parseOCISeedJSONOnly(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if len(args) == 1 && args[0] == "--json" {
		return true, nil
	}
	return false, fmt.Errorf("OCI seed command accepts only [--json]: %w", core.ErrInvalidArgument)
}

func parseOCISeedSelectionOptions(args []string, allowReenable bool) (string, bool, bool, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" || strings.HasPrefix(args[0], "--") {
		return "", false, false, fmt.Errorf("OCI seed selection requires <reference@sha256:...>: %w", core.ErrInvalidArgument)
	}
	target := args[0]
	reenable := false
	jsonOutput := false
	for _, arg := range args[1:] {
		switch arg {
		case "--re-enable":
			if !allowReenable || reenable {
				return "", false, false, fmt.Errorf("invalid --re-enable option: %w", core.ErrInvalidArgument)
			}
			reenable = true
		case "--json":
			if jsonOutput {
				return "", false, false, fmt.Errorf("duplicate --json: %w", core.ErrInvalidArgument)
			}
			jsonOutput = true
		default:
			return "", false, false, fmt.Errorf("unknown OCI seed selection option %q: %w", arg, core.ErrInvalidArgument)
		}
	}
	return target, reenable, jsonOutput, nil
}

func parseOCISeedBaseOptions(args []string) (core.BaseName, bool, error) {
	var base core.BaseName
	jsonOutput := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			if jsonOutput {
				return "", false, fmt.Errorf("duplicate --json: %w", core.ErrInvalidArgument)
			}
			jsonOutput = true
		case "--base":
			if base != "" || i+1 >= len(args) {
				return "", false, fmt.Errorf("--base requires one value: %w", core.ErrInvalidArgument)
			}
			i++
			value := strings.TrimSpace(args[i])
			if !validOCISeedBaseName(value) || value != args[i] {
				return "", false, fmt.Errorf("invalid --base value %q: %w", args[i], core.ErrInvalidArgument)
			}
			base = core.BaseName(value)
		default:
			return "", false, fmt.Errorf("unknown OCI seed option %q: %w", args[i], core.ErrInvalidArgument)
		}
	}
	return base, jsonOutput, nil
}

func validOCISeedBaseName(value string) bool {
	if value == "" || len(value) > 128 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.HasPrefix(value, "-") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		for _, r := range segment {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
				return false
			}
		}
	}
	return true
}

func printOCISeedSelection(selection ociplugin.SeedSelection) {
	if selection.Reference != "" {
		fmt.Printf("image: %s@%s\n", selection.Reference, selection.Digest)
	}
	fmt.Printf("pinned: %t\n", selection.Pinned)
	if selection.PinnedAt != nil {
		fmt.Printf("pinned-at: %s\n", selection.PinnedAt.UTC().Format("2006-01-02T15:04:05Z"))
	}
	fmt.Printf("re-enabled: %t\n", selection.Reenabled)
	if selection.ReenabledAt != nil {
		fmt.Printf("re-enabled-at: %s\n", selection.ReenabledAt.UTC().Format("2006-01-02T15:04:05Z"))
	}
	fmt.Printf("deleted: %t\n", selection.Deleted)
	if selection.DeletedAt != nil {
		fmt.Printf("deleted-at: %s\n", selection.DeletedAt.UTC().Format("2006-01-02T15:04:05Z"))
	}
}

func printOCISeedBuildReport(report seedbuildapp.BuildReport) {
	fmt.Printf("base: %s@%s\n", report.Parent.Name, report.Parent.Revision)
	toolingState := "built"
	if report.ReusedToolingBase {
		toolingState = "reused"
	}
	fmt.Printf("tooling: %s (%s)\n", report.ToolingRevision, toolingState)
	fmt.Printf("seed: %s\n", report.SeedRevision)
	fmt.Printf("images: %d\n", len(report.Images))
	for _, image := range report.Images {
		fmt.Printf("  %s\n", image.String())
	}
	fmt.Printf("built-at: %s\n", report.BuiltAt.UTC().Format("2006-01-02T15:04:05Z"))
}

func printOCISeedManifest(manifest seedbuildapp.Manifest) {
	fmt.Printf("base: %s@%s\n", manifest.Parent.Name, manifest.Parent.Revision)
	fmt.Printf("tooling: %s\n", manifest.ToolingRevision)
	fmt.Printf("seed: %s\n", manifest.SeedRevision)
	if manifest.SeedAlias != "" {
		fmt.Printf("seed-alias: %s\n", manifest.SeedAlias)
	}
	fmt.Printf("images: %d\n", len(manifest.Images))
	for _, image := range manifest.Images {
		fmt.Printf("  %s\n", image.String())
	}
	fmt.Printf("built-at: %s\n", manifest.BuiltAt.UTC().Format("2006-01-02T15:04:05Z"))
}

func ociDockerCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 2 || len(args) > 3 {
		return fmt.Errorf("usage: haco plugin oci docker <status|prepare> <environment> [--json]: %w", core.ErrInvalidArgument)
	}
	action, environment := args[0], args[1]
	jsonOutput := false
	if len(args) == 3 {
		if args[2] != "--json" {
			return fmt.Errorf("unknown OCI Docker option %q: %w", args[2], core.ErrInvalidArgument)
		}
		jsonOutput = true
	}

	var (
		status ociplugin.DockerCompatibilityStatus
		err    error
	)
	switch action {
	case "status":
		status, err = app.OCI.DockerStatus(ctx, environment)
	case "prepare":
		status, err = app.OCI.PrepareDocker(ctx, environment)
	default:
		return fmt.Errorf("unknown OCI Docker command %q: %w", action, core.ErrInvalidArgument)
	}
	if jsonOutput {
		if encodeErr := json.NewEncoder(os.Stdout).Encode(status); encodeErr != nil {
			return encodeErr
		}
	} else {
		printOCIDockerStatus(status)
	}
	return err
}

func printOCIDockerStatus(status ociplugin.DockerCompatibilityStatus) {
	if status.Environment != "" {
		fmt.Printf("environment: %s\n", status.Environment)
	}
	fmt.Printf("docker-cli: %t\n", status.DockerCLI)
	fmt.Printf("dockerd: %t\n", status.DockerDaemon)
	fmt.Printf("containerd: %t (active=%t)\n", status.Containerd, status.ContainerdActive)
	fmt.Printf("systemd: %t\n", status.Systemd)
	fmt.Printf("docker-group: %t\n", status.DockerGroup)
	fmt.Printf("units: socket=%t service=%t\n", status.SocketUnitVerified, status.ServiceUnitVerified)
	fmt.Printf("haco-socket: enabled=%t active=%t\n", status.SocketEnabled, status.SocketActive)
	fmt.Printf("engine: active=%t (on-demand is expected)\n", status.EngineActive)
	fmt.Printf("vendor-docker: enabled=%t active=%t\n", status.VendorDockerEnabled, status.VendorDockerActive)
	fmt.Printf("ready: %t\n", status.Ready)
}

func printOCISampleReport(report ociplugin.SampleReport) {
	fmt.Printf("sampled: %d\nfresh: %d\nfailed: %d\n", report.Sampled, report.Fresh, report.Failed)
	printOCISampleWarnings(report)
}

func printOCISampleWarnings(report ociplugin.SampleReport) {
	for environment, reason := range report.Failures {
		fmt.Fprintf(os.Stderr, "OCI plugin telemetry: %s: %s\n", environment, reason)
	}
}
