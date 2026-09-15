package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/modules/standard/cache"
)

type cacheMaintenanceClient interface {
	RecoverCache(context.Context, string, string) (controlapi.CacheMaintenanceResponse, error)
	CacheHistory(context.Context, string, string) (controlapi.CacheMaintenanceResponse, error)
	ClearCache(context.Context, string, string, string) (controlapi.CacheMaintenanceResponse, error)
}

func runCacheMaintenance(args []string, client cacheMaintenanceClient, in io.Reader, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("cache", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	asJSON := flags.Bool("json", false, cliMessage("flag.json"))
	yes := flags.Bool("yes", false, cliMessage("cache.clear_yes"))
	all := flags.Bool("all", false, cliMessage("cache.all"))
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	rest := flags.Args()
	if *all {
		if len(rest) != 0 || args[0] != "clear" && *yes {
			return 2
		}
		return runCacheCatalog(args[0], client, *asJSON, *yes, in, out, diagnostic)
	}
	if len(rest) != 2 || args[0] != "clear" && *yes {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.maintenance_usage"))
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	response, err := client.CacheHistory(ctx, rest[0], rest[1])
	if err != nil || response.Failure != "" {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.failed"))
		return 1
	}
	if args[0] == "history" {
		if *asJSON {
			if json.NewEncoder(out).Encode(response.History) != nil {
				return 1
			}
			return 0
		}
		if printCacheHistory(out, response.History) != nil {
			return 1
		}
		return 0
	}
	if args[0] == "recover" {
		response, err = client.RecoverCache(ctx, rest[0], rest[1])
	} else {
		// Show the complete reviewed scope even with --yes; stdout remains JSON-only.
		if printCacheHistory(diagnostic, response.History) != nil {
			return 1
		}
		if code := confirmDataDeletion(in, diagnostic, *yes, "cache.clear_warning", "cache.clear_prompt", "cache.clear_cancelled"); code != 0 {
			return code
		}
		response, err = client.ClearCache(ctx, rest[0], rest[1], response.History.Revision)
	}
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.failed"))
		return 1
	}
	if *asJSON {
		if json.NewEncoder(out).Encode(response) != nil {
			return 1
		}
	} else {
		if response.Result.Reset {
			if _, err := fmt.Fprintln(out, cliMessage("cache.cleared")); err != nil {
				return 1
			}
		}
		entries := response.Result.Entries
		if args[0] == "recover" {
			entries = response.Recovery.Entries
		}
		for _, entry := range entries {
			if printCacheHistoryEntry(out, entry) != nil {
				return 1
			}
		}
	}
	if response.Failure != "" {
		key := "cache.clear_partial"
		if !response.Result.Reset {
			key = "cache.clear_changed"
		}
		if args[0] == "recover" {
			key = "cache.recovery"
		}
		_, _ = fmt.Fprintln(diagnostic, cliMessage(key))
		return 1
	}
	if args[0] == "recover" && !*asJSON {
		if _, err := fmt.Fprintln(out, cliMessage("cache.recovered")); err != nil {
			return 1
		}
	}
	return 0
}

func printCacheHistory(out io.Writer, h cache.History) error {
	if _, err := fmt.Fprintln(out, cliMessage("cache.history_title", h.Name, h.Path, h.Current)); err != nil {
		return err
	}
	for _, entry := range h.Entries {
		if err := printCacheHistoryEntry(out, entry); err != nil {
			return err
		}
	}
	if len(h.Entries) == 0 {
		_, err := fmt.Fprintln(out, cliMessage("cache.history_empty"))
		return err
	}
	return nil
}
func printCacheHistoryEntry(out io.Writer, entry cache.HistoryEntry) error {
	state := entry.State
	switch state {
	case "current", "retained", "creating", "deleting", "deleted", "cleanup-required", "recovery-required":
	default:
		state = "recovery-required"
	}
	_, err := fmt.Fprintln(out, cliMessage("cache.history_entry", entry.CreatedAt.UTC().Format(time.RFC3339), entry.Origin, cliMessage("cache.history_state."+state)))
	return err
}
