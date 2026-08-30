package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
	ociplugin "github.com/SLktEx/Hacocoon/modules/plugin/oci"
)

func ociPluginCommand(ctx context.Context, app *composition.App, args []string) error {
	if app == nil || app.OCI == nil {
		return fmt.Errorf("OCI plugin is disabled; set HACO_PLUGIN_OCI=nerdctl or HACO_PLUGIN_OCI=docker: %w", core.ErrRuntimeUnavailable)
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: haco plugin oci <status|seed> ...: %w", core.ErrInvalidArgument)
	}
	switch args[0] {
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("usage: haco plugin oci status: %w", core.ErrInvalidArgument)
		}
		fmt.Printf("driver: %s\n", app.OCI.Driver())
		return nil
	case "seed":
		return ociSeedCommand(ctx, app, args[1:])
	default:
		return fmt.Errorf("unknown OCI plugin command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func ociSeedCommand(ctx context.Context, app *composition.App, args []string) error {
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
				Sampling        ociplugin.SampleReport      `json:"sampling"`
				Recommendations []ociplugin.Recommendation `json:"recommendations"`
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

func printOCISampleReport(report ociplugin.SampleReport) {
	fmt.Printf("sampled: %d\nfresh: %d\nfailed: %d\n", report.Sampled, report.Fresh, report.Failed)
	printOCISampleWarnings(report)
}

func printOCISampleWarnings(report ociplugin.SampleReport) {
	for environment, reason := range report.Failures {
		fmt.Fprintf(os.Stderr, "OCI plugin telemetry: %s: %s\n", environment, reason)
	}
}
