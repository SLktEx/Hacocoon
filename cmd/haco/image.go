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

func imageCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: haco image <list|inspect|seed> ...: %w", core.ErrInvalidArgument)
	}
	switch args[0] {
	case "list":
		return imageListCommand(ctx, app, args[1:])
	case "inspect":
		return imageInspectCommand(ctx, app, args[1:])
	case "seed":
		return imageSeedCommand(ctx, app, args[1:])
	default:
		return fmt.Errorf("unknown image command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func imageListCommand(ctx context.Context, app *composition.App, args []string) error {
	jsonOutput := false
	if len(args) == 1 && args[0] == "--json" {
		jsonOutput = true
	} else if len(args) != 0 {
		return fmt.Errorf("usage: haco image list [--json]: %w", core.ErrInvalidArgument)
	}
	bases, err := app.Bases.ListBases(ctx)
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(bases)
	}
	for _, base := range bases {
		fmt.Println(base.Name)
	}
	return nil
}

func imageInspectCommand(ctx context.Context, app *composition.App, args []string) error {
	jsonOutput := false
	if len(args) == 2 && args[1] == "--json" {
		jsonOutput = true
	} else if len(args) != 1 {
		return fmt.Errorf("usage: haco image inspect <base> [--json]: %w", core.ErrInvalidArgument)
	}
	info, err := app.Bases.InspectBase(ctx, core.BaseName(args[0]))
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(info)
	}
	fmt.Printf("name: %s\nrevision: %s\n", info.Name, info.Revision)
	return nil
}

func imageSeedCommand(ctx context.Context, app *composition.App, args []string) error {
	if app == nil || app.SeedStats == nil {
		return core.ErrRuntimeUnavailable
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: haco image seed <sample|recommend> [--json]: %w", core.ErrInvalidArgument)
	}
	jsonOutput := false
	if len(args) == 2 && args[1] == "--json" {
		jsonOutput = true
	} else if len(args) != 1 {
		return fmt.Errorf("usage: haco image seed <sample|recommend> [--json]: %w", core.ErrInvalidArgument)
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
		printSeedSampleReport(report)
		return nil
	case "recommend":
		report, err := app.SeedStats.SampleAll(ctx, seedstatsapp.DefaultSampleMaxAge)
		if err != nil {
			return err
		}
		if !jsonOutput {
			printSeedSampleWarnings(report)
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
		return fmt.Errorf("unknown seed command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func printSeedSampleReport(report seedstatsapp.SampleReport) {
	fmt.Printf("sampled: %d\nfresh: %d\nfailed: %d\n", report.Sampled, report.Fresh, report.Failed)
	printSeedSampleWarnings(report)
}

func printSeedSampleWarnings(report seedstatsapp.SampleReport) {
	for environment, reason := range report.Failures {
		fmt.Fprintf(os.Stderr, "seed telemetry: %s: %s\n", environment, reason)
	}
}
