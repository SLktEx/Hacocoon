package main

import (
	"fmt"
	"io"
	"os"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Help is handled before main initializes composition.Local(). The temporary
// migration client must remain diagnosable even when Incus, Host state, or the
// controller are unavailable.
func init() {
	handled, err := handleHelpArgs(os.Args[1:], os.Stdout)
	if !handled {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hacoq:", err)
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
		return true, fmt.Errorf("usage: hacoq help: %w", core.ErrInvalidArgument)
	}
	return true, writeHacoHelp(out)
}

func writeHacoHelp(out io.Writer) error {
	if out == nil {
		return core.ErrInvalidArgument
	}
	if _, err := fmt.Fprintln(out, "Hacocoon temporary migration CLI: hacoq"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  This is the previous haco surface kept only during migration and is scheduled for deletion."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  Do not add new product features or integrations to hacoq."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  haco-host remains trusted logical Host-local tooling; it is not a second Core/state controller."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  help/version are standalone and do not require Incus or the controller."); err != nil {
		return err
	}

	sections := []struct {
		title  string
		domain commandDomain
	}{
		{"Legacy general controller-client operations", commandDomainGeneral},
		{"Legacy Physical Host bootstrap/recovery/service operations", commandDomainPhysicalHost},
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

	_, err := fmt.Fprintln(out, "\nThese commands exist only to keep old functionality reachable while the new haco product workflow is rebuilt.")
	return err
}
