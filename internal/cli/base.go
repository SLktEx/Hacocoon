package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func runBase(args []string) int {
	if len(args) > 0 && args[0] == "delete" {
		return runBaseManage(args)
	}
	if len(args) > 0 && args[0] == "list" {
		for _, arg := range args[1:] {
			if arg == "--all" {
				return runBaseManage(args)
			}
		}
	}
	if len(args) > 0 && args[0] == "import" {
		return runBaseImport(args[1:])
	}
	if len(args) > 0 && args[0] == "build" {
		return runBaseBuild(args[1:])
	}
	clean, jsonOutput, flagErr := splitJSONFlag(args)
	if flagErr != nil {
		fmt.Fprintln(os.Stderr, "haco:", flagErr)
		return 2
	}
	args = clean
	if len(args) == 0 || (args[0] != "list" && args[0] != "inspect") || (args[0] == "list" && len(args) != 1) || (args[0] == "inspect" && len(args) != 2) {
		commandHelp(os.Stderr, "base", cliLanguage())
		return 2
	}
	c, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if args[0] == "list" {
		bases, err := c.ListBases(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "haco:", err)
			return 1
		}
		if jsonOutput {
			if err := writeCLIResult(os.Stdout, bases, true); err != nil {
				return 1
			}
			return 0
		}
		for _, base := range bases {
			fmt.Fprintln(os.Stdout, base.Name)
		}
		return 0
	}
	info, err := c.InspectBase(ctx, core.BaseName(args[1]))
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	if jsonOutput {
		if err := writeCLIResult(os.Stdout, info, true); err != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintf(os.Stdout, "name: %s\nrevision: %s\n", info.Name, info.Revision)
	return 0
}
