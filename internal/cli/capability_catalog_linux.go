//go:build linux

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/SLktEx/Hacocoon/internal/policy"
)

type capabilityCatalogClient interface {
	CapabilityCatalog(context.Context) (capability.CapabilityCatalog, error)
}

func capabilityCatalogCommand(ctx context.Context, client capabilityCatalogClient, args []string, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco config capabilities", flag.ContinueOnError)
	configureCLIFlags(flags, diagnostic)
	jsonOutput := flags.Bool("json", false, cliMessage("flag.json"))
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	if client == nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("config.capabilities_unavailable"))
		return 1
	}
	catalog, err := client.CapabilityCatalog(ctx)
	if err != nil || capability.ValidateCapabilityCatalog(catalog) != nil {
		// Do not print arbitrary backend errors or partial/invalid metadata.
		_, _ = fmt.Fprintln(diagnostic, cliMessage("config.capabilities_unavailable"))
		return 1
	}
	if err := writeCLIResult(out, catalog, *jsonOutput); err != nil {
		return 1
	}
	if !*jsonOutput {
		if _, err := fmt.Fprintln(out, cliMessage("config.capabilities_readonly")); err != nil {
			return 1
		}
	}
	return 0
}
