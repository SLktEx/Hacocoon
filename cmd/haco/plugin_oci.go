package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
	seedstatsapp "github.com/SLktEx/Hacocoon/internal/seedstats"
)

func ociPluginCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: haco plugin oci <image|seed> ...: %w", core.ErrInvalidArgument)
	}
	switch args[0] {
	case "image":
		return ociImageCommand(ctx, app, args[1:])
	case "seed":
		return ociSeedCommand(ctx, app, args[1:])
	default:
		return fmt.Errorf("unknown OCI plugin command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func ociImageCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: haco plugin oci image <delete> ...: %w", core.ErrInvalidArgument)
	}
	switch args[0] {
	case "delete":
		return ociImageDeleteCommand(ctx, app, args[1:])
	default:
		return fmt.Errorf("unknown OCI image command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func ociImageDeleteCommand(ctx context.Context, app *composition.App, args []string) error {
	if app == nil || app.SeedStats == nil {
		return core.ErrRuntimeUnavailable
	}
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

	report, deleteErr := app.SeedStats.DeleteImage(ctx, target, allEnvironments)
	if jsonOutput {
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			return err
		}
	} else {
		printOCIImageDeleteReport(report)
	}
	return deleteErr
}

func printOCIImageDeleteReport(report seedstatsapp.DeleteReport) {
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
	if app == nil || app.SeedStats == nil {
		return core.ErrRuntimeUnavailable
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: haco plugin oci seed <sample|recommend> [--json]: %w", core.ErrInvalidArgument)
	}
	jsonOutput := false
	if len(args) == 2 && args[1] == "--json" {
		jsonOutput = true
	} else if len(args) != 1 {
		return fmt.Errorf("usage: haco plugin oci seed <sample|recommend> [--json]: %w", core.ErrInvalidArgument)
	}

	switch args[0] {
	case "sample":
		report, err := app.SeedStats.SampleAll(ctx, 0)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(report)
		}
		printOCISeedSampleReport(report)
		return nil
	case "recommend":
		report, err := app.SeedStats.SampleAll(ctx, seedstatsapp.DefaultSampleMaxAge)
		if err != nil {
			return err
		}
		if !jsonOutput {
			printOCISeedSampleWarnings(report)
		}
		recommendations, err := app.SeedStats.Recommend(ctx, seedstatsapp.DefaultRecommendationWindow)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(struct {
				Sampling        seedstatsapp.SampleReport      `json:"sampling"`
				Recommendations []seedstatsapp.Recommendation `json:"recommendations"`
			}{Sampling: report, Recommendations: recommendations})
		}
		for _, recommendation := range recommendations {
			promotion := "recommend"
			if recommendation.AutoPromote {
				promotion = "auto"
			}
			fmt.Printf("%s\t%s@%s\t%d envs\t%.1f%%\tlast=%s\n",
				promotion,
				recommendation.Reference,
				recommendation.Digest,
				recommendation.Environments,
				recommendation.Percent,
				recommendation.LastSeen.UTC().Format("2006-01-02T15:04:05Z"),
			)
		}
		return nil
	default:
		return fmt.Errorf("unknown OCI seed command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func printOCISeedSampleReport(report seedstatsapp.SampleReport) {
	fmt.Printf("sampled: %d\nfresh: %d\nfailed: %d\n", report.Sampled, report.Fresh, report.Failed)
	printOCISeedSampleWarnings(report)
}

func printOCISeedSampleWarnings(report seedstatsapp.SampleReport) {
	for environment, reason := range report.Failures {
		fmt.Fprintf(os.Stderr, "seed telemetry: %s: %s\n", environment, reason)
	}
}
