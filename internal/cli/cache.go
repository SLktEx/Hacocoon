package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/storage/cache"
	"io"
	"os"
	"time"
)

type cacheClient interface {
	CacheSettings(context.Context) (cache.SettingsSnapshot, error)
	ConfigureCache(context.Context, cache.SettingsSnapshot) (cache.SettingsSnapshot, error)
	CacheStatus(context.Context, string) (controlapi.CacheResponse, error)
	CollectCache(context.Context, string, string) (controlapi.CacheResponse, error)
}

func runCache(args []string) int {
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, cliMessage("cache.failed"))
		return 1
	}
	return runCacheWith(args, client, os.Stdout, os.Stderr)
}
func runCacheWith(args []string, client cacheClient, out, diagnostic io.Writer) int {
	if len(args) > 0 && (args[0] == "history" || args[0] == "clear" || args[0] == "recover") {
		maintenance, ok := client.(cacheMaintenanceClient)
		if !ok {
			return 1
		}
		return runCacheMaintenance(args, maintenance, os.Stdin, out, diagnostic)
	}
	if len(args) == 0 {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("error.usage", "haco cache <settings|configure|status|collect|history|clear|recover>"))
		return 2
	}
	flags := flag.NewFlagSet("cache", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	asJSON := flags.Bool("json", false, cliMessage("flag.json"))
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	rest := flags.Args()
	command := args[0]
	if command == "settings" && len(rest) != 0 || command == "configure" && len(rest) != 1 || command == "status" && len(rest) != 1 || command == "collect" && (len(rest) < 1 || len(rest) > 2) {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.usage"))
		return 2
	}
	if command != "settings" && command != "configure" && command != "status" && command != "collect" {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.usage"))
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if command == "settings" || command == "configure" {
		var configured cache.Configuration
		if command == "configure" {
			data, err := readCacheInput(rest[0])
			if err != nil {
				_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.input"))
				return 2
			}
			configured, err = cache.DecodeConfiguration(data)
			if err != nil {
				_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.input"))
				return 2
			}
		}
		settings, err := client.CacheSettings(ctx)
		if err == nil && command == "configure" {
			settings.Configuration = configured
			settings, err = client.ConfigureCache(ctx, settings)
		}
		if err != nil {
			_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.failed"))
			return 1
		}
		if *asJSON {
			if err := json.NewEncoder(out).Encode(settings); err != nil {
				return 1
			}
			return 0
		}
		if command == "configure" {
			if _, err := fmt.Fprintln(out, cliMessage("cache.configured")); err != nil {
				return 1
			}
		}
		for _, a := range settings.Configuration.Areas {
			scope := cliMessage("cache.scope_workspace")
			if a.Scope == "shared" {
				scope = cliMessage("cache.scope_shared", a.Group)
			}
			target := a.Path
			if a.Repository != "" {
				target = a.Repository + ":" + a.Path
			}
			if _, err := fmt.Fprintln(out, cliMessage("cache.setting", a.Name, target, a.Compatibility, scope)); err != nil {
				return 1
			}
		}
		if len(settings.Configuration.Areas) == 0 {
			if _, err := fmt.Fprintln(out, cliMessage("cache.none_settings")); err != nil {
				return 1
			}
		}
		return 0
	}
	var response controlapi.CacheResponse
	var err error
	if command == "status" {
		response, err = client.CacheStatus(ctx, rest[0])
	} else {
		area := ""
		if len(rest) == 2 {
			area = rest[1]
		}
		response, err = client.CollectCache(ctx, rest[0], area)
	}
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("cache.failed"))
		return 1
	}
	if *asJSON {
		if err := json.NewEncoder(out).Encode(response); err != nil {
			return 1
		}
	} else {
		for _, a := range response.Areas {
			state := a.State
			switch state {
			case "enrolled", "published", "skipped", "recovery-required", "cleanup-required", "failed":
			default:
				state = "failed"
			}
			if _, err := fmt.Fprintln(out, cliMessage("cache.area", a.Name, a.Path, a.Origin, a.Current, cliMessage("cache.state."+state))); err != nil {
				return 1
			}
		}
		if len(response.Areas) == 0 && response.Failure == "" {
			if _, err := fmt.Fprintln(out, cliMessage("cache.none_env")); err != nil {
				return 1
			}
		}
	}
	if response.Failure != "" {
		key := "cache.failed"
		switch response.Failure {
		case "incompatible_state":
			key = "cache.stop_first"
		case "recovery_required":
			key = "cache.recovery"
		case "not_found":
			key = "cache.not_found"
		}
		_, _ = fmt.Fprintln(diagnostic, cliMessage(key))
		return 1
	}
	if command == "collect" && !*asJSON {
		if _, err := fmt.Fprintln(out, cliMessage("cache.next")); err != nil {
			return 1
		}
	}
	return 0
}
func readCacheInput(name string) ([]byte, error) {
	f, err := openCacheInput(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > cache.MaxConfigurationBytes {
		return nil, fmt.Errorf("invalid configuration file")
	}
	return io.ReadAll(io.LimitReader(f, cache.MaxConfigurationBytes+1))
}
