package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
)

func runSnapshot(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	return snapshotCommand(ctx, args, os.Stdout, os.Stderr)
}
func snapshotCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	usage := func() int {
		commandHelp(diagnostic, "snapshot", cliLanguage())
		return 2
	}
	if len(args) == 0 {
		return usage()
	}
	if args[0] == "--help" || args[0] == "-h" {
		usage()
		return 0
	}
	flags := flag.NewFlagSet("haco snapshot "+args[0], flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	var machine, details bool
	var name string
	switch args[0] {
	case "create":
		flags.StringVar(&name, "name", "", "Snapshot name")
		flags.BoolVar(&machine, "json", false, cliMessage("flag.json"))
	case "list":
		flags.BoolVar(&machine, "json", false, cliMessage("flag.json"))
	case "inspect":
		flags.BoolVar(&machine, "json", false, cliMessage("flag.json"))
		flags.BoolVar(&details, "details", false, cliMessage("snapshot.inspect.details"))
	case "delete":
	default:
		return usage()
	}
	if err := parseInterspersed(flags, args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	pos := flags.Args()
	if (args[0] == "list" && len(pos) > 1) || (args[0] != "list" && len(pos) != 1) {
		return usage()
	}
	req := controlapi.SnapshotRequest{Operation: args[0], Name: name}
	if len(pos) > 0 {
		if args[0] == "delete" || args[0] == "inspect" {
			req.ID = pos[0]
		} else {
			req.Environment = pos[0]
		}
	}
	client := controlapi.NewDefaultClient()
	response, err := client.Snapshot(ctx, req)
	var writeErr error
	if machine {
		if args[0] == "inspect" {
			writeErr = json.NewEncoder(out).Encode(response.Inspection)
		} else {
			writeErr = json.NewEncoder(out).Encode(response.Snapshots)
		}
	} else if args[0] == "inspect" {
		writeErr = writeSnapshotInspection(out, response.Inspection, details)
	} else if args[0] == "list" {
		table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(table, cliMessage("snapshot.columns"))
		for _, saved := range response.Snapshots {
			fmt.Fprintf(table, "%s\t%q\t%s\t%d\t%t\n", saved.ID, saved.Environment, saved.State, saved.Workspaces, saved.OCI)
		}
		writeErr = table.Flush()
	} else if args[0] == "create" {
		for _, saved := range response.Snapshots {
			_, writeErr = fmt.Fprintf(out, cliLanguage().Text("snapshot.created"), saved.ID, saved.State, saved.Environment)
		}
	} else if err == nil {
		_, writeErr = fmt.Fprintln(out, cliMessage("snapshot.deleted"))
	}
	if err != nil {
		fmt.Fprintf(diagnostic, "haco: %v\n", err)
		if args[0] == "delete" && regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,39}[a-z0-9])?$`).MatchString(req.ID) {
			_, _ = fmt.Fprintf(diagnostic, cliLanguage().Text("snapshot.inspect.next"), req.ID)
		}
		return 1
	}
	if writeErr != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("snapshot.write_failed"))
		return 1
	}
	return 0
}
