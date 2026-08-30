package main

import (
	"fmt"
	"io"
	"os"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Help is handled before main initializes composition.Local(). A general client
// must be able to explain its execution domains even when Incus, Host state, or
// the controller are unavailable.
func init() {
	handled, err := handleHelpArgs(os.Args[1:], os.Stdout)
	if !handled {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		os.Exit(2)
	}
	os.Exit(0)
}

func isHelpInvocation(args []string) bool {
	return (len(args) == 1 && (args[0] == "--help" || args[0] == "-h")) || (len(args) > 0 && args[0] == "help")
}

func handleHelpArgs(args []string, out io.Writer) (bool, error) {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		return true, writeHacoHelp(out)
	}
	if len(args) == 0 || args[0] != "help" {
		return false, nil
	}
	if len(args) != 1 {
		return true, fmt.Errorf("usage: haco help: %w", core.ErrInvalidArgument)
	}
	return true, writeHacoHelp(out)
}

func writeHacoHelp(out io.Writer) error {
	if out == nil {
		return core.ErrInvalidArgument
	}
	if _, err := fmt.Fprintln(out, "Hacocoon CLI roles"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  haco      general Hacocoon client; bootstrap/recovery commands may be Physical-Host-local"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  haco-host trusted logical Host-local tooling; never a second Core/state controller"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  help/version are standalone and do not require Incus or the controller"); err != nil {
		return err
	}

	sections := []struct {
		title  string
		domain commandDomain
	}{
		{"General controller-client operations", commandDomainGeneral},
		{"Physical Host bootstrap/recovery/service operations", commandDomainPhysicalHost},
		{"Trusted haco-host-local migration targets", commandDomainTrustedHost},
		{"Temporary compatibility aliases", commandDomainCompatibility},
	}
	for _, section := range sections {
		if _, err := fmt.Fprintf(out, "\n%s:\n", section.title); err != nil {
			return err
		}
		for _, classification := range commandClassificationsByDomain(section.domain) {
			line := fmt.Sprintf("  %-12s %s", classification.Name, classification.State)
			if classification.Replacement != "" {
				line += "; use " + classification.Replacement
			}
			if _, err := fmt.Fprintln(out, line); err != nil {
				return err
			}
		}
	}

	_, err := fmt.Fprintln(out, "\nUse `haco env ...` for Environment lifecycle. Unmigrated commands fail closed in trusted haco-host instead of creating guest-local Hacocoon state.")
	return err
}
