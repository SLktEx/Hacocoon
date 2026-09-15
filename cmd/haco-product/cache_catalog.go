package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/modules/standard/cache"
)

type cacheCatalogClient interface {
	MaintainCacheCatalog(context.Context, string, string) (controlapi.CacheCatalogResponse, error)
}

func runCacheCatalog(operation string, client cacheMaintenanceClient, asJSON, yes bool, in io.Reader, out, diagnostic io.Writer) int {
	c, ok := client.(cacheCatalogClient)
	if !ok {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.failed"))
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	r, err := c.MaintainCacheCatalog(ctx, "history", "")
	if err != nil || r.Failure != "" {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.failed"))
		return 1
	}
	if operation == "history" {
		if asJSON {
			if json.NewEncoder(out).Encode(r.Catalog) != nil {
				return 1
			}
		} else if printCacheCatalog(out, r.Catalog) != nil {
			return 1
		}
		return 0
	}
	if operation == "clear" {
		if printCacheCatalog(diagnostic, r.Catalog) != nil {
			return 1
		}
		if code := confirmDataDeletion(in, diagnostic, yes, "cache.all_warning", "cache.clear_prompt", "cache.clear_cancelled"); code != 0 {
			return code
		}
	}
	r, err = c.MaintainCacheCatalog(ctx, operation, r.Catalog.Revision)
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.failed"))
		return 1
	}
	if asJSON {
		if json.NewEncoder(out).Encode(r) != nil {
			return 1
		}
	} else {
		for i, g := range r.Catalog.Groups {
			if _, err := fmt.Fprintln(out, cliMessage("cache.group", i+1)); err != nil {
				return 1
			}
			if g.State == "not_started" {
				if _, err := fmt.Fprintln(out, cliMessage("cache.not_started")); err != nil {
					return 1
				}
				continue
			}
			if g.Result.Reset {
				if _, err := fmt.Fprintln(out, cliMessage("cache.cleared")); err != nil {
					return 1
				}
			}
			entries := g.Result.Entries
			if operation == "recover" {
				entries = g.Recovery.Entries
			}
			for _, entry := range entries {
				if printCacheHistoryEntry(out, entry) != nil {
					return 1
				}
			}
			if g.Failure != "" {
				if _, err := fmt.Fprintln(out, cliMessage("cache.recovery")); err != nil {
					return 1
				}
			}
		}
	}
	if r.Failure != "" {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.clear_partial"))
		return 1
	}
	return 0
}
func printCacheCatalog(out io.Writer, h cache.CatalogHistory) error {
	if len(h.Groups) == 0 {
		_, err := fmt.Fprintln(out, cliMessage("cache.history_empty"))
		return err
	}
	for i, g := range h.Groups {
		if _, err := fmt.Fprintln(out, cliMessage("cache.group", i+1)); err != nil {
			return err
		}
		if g.History.Name == "" {
			if _, err := fmt.Fprintln(out, cliMessage("cache.no_producer")); err != nil {
				return err
			}
			for _, entry := range g.History.Entries {
				if err := printCacheHistoryEntry(out, entry); err != nil {
					return err
				}
			}
		} else if err := printCacheHistory(out, g.History); err != nil {
			return err
		}
		if len(g.Environments) > 0 {
			if _, err := fmt.Fprintln(out, cliMessage("cache.independent_envs", strings.Join(g.Environments, ", "))); err != nil {
				return err
			}
		}
	}
	return nil
}
