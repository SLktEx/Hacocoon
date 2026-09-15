package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/modules/standard/cache"
	"io"
	"time"
)

type cacheEmptyClient interface {
	PreviewEmptyCache(context.Context, cache.EmptyScope) (controlapi.CacheEmptyResponse, error)
	EmptyCache(context.Context, cache.EmptyScope, string) (controlapi.CacheEmptyResponse, error)
}

func runCacheEmpty(args []string, client cacheEmptyClient, in io.Reader, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("cache empty", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	asJSON := flags.Bool("json", false, cliMessage("flag.json"))
	yes := flags.Bool("yes", false, cliMessage("cache.empty_yes"))
	all := flags.Bool("all", false, cliMessage("cache.empty_all"))
	previewOnly := flags.Bool("preview", false, cliMessage("cache.empty_preview"))
	if flags.Parse(args) != nil {
		return 2
	}
	rest := flags.Args()
	if *all && len(rest) != 0 || !*all && (len(rest) < 1 || len(rest) > 2) || *previewOnly && *yes {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.empty_usage"))
		return 2
	}
	scope := cache.EmptyScope{All: *all}
	if len(rest) > 0 {
		scope.Environment = rest[0]
	}
	if len(rest) > 1 {
		scope.Area = rest[1]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	response, err := client.PreviewEmptyCache(ctx, scope)
	if err != nil || response.Failure != "" {
		key := "cache.failed"
		if response.Failure == "not_found" {
			key = "cache.not_found"
		}
		_, _ = fmt.Fprintln(diagnostic, cliMessage(key))
		return 1
	}
	if *previewOnly || len(response.Preview.Areas) == 0 {
		if *asJSON {
			if json.NewEncoder(out).Encode(response) != nil {
				return 1
			}
		} else if printCacheEmpty(out, response.Preview) != nil {
			return 1
		}
		return 0
	}
	if printCacheEmpty(diagnostic, response.Preview) != nil {
		return 1
	}
	if code := confirmDataDeletion(in, diagnostic, *yes, "cache.empty_warning", "cache.empty_prompt", "cache.empty_cancelled"); code != 0 {
		return code
	}
	response, err = client.EmptyCache(ctx, scope, response.Preview.Revision)
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.empty_partial"))
		return 1
	}
	if *asJSON {
		if json.NewEncoder(out).Encode(response) != nil {
			return 1
		}
	} else if printCacheEmpty(out, response.Preview) != nil {
		return 1
	}
	if response.Failure != "" {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.empty_partial"))
		return 1
	}
	_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.empty_done"))
	return 0
}
func printCacheEmpty(out io.Writer, preview cache.EmptyPreview) error {
	for _, area := range preview.Areas {
		state := cliMessage("cache.state." + area.State)
		if _, err := fmt.Fprintln(out, cliMessage("cache.empty_area", area.Environment, area.Name, area.Path, state, area.SavedCopies)); err != nil {
			return err
		}
	}
	if len(preview.Areas) == 0 {
		_, err := fmt.Fprintln(out, cliMessage("cache.empty_none"))
		return err
	}
	return nil
}
