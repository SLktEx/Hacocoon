package main

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

	"github.com/SLktEx/Hacocoon/internal/controlapi"
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
	if args[0] == "restore" {
		return snapshotRestoreCommand(ctx, args[1:], out, diagnostic)
	}
	flags := flag.NewFlagSet("haco snapshot "+args[0], flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	var machine bool
	switch args[0] {
	case "create", "list":
		flags.BoolVar(&machine, "json", false, cliMessage("flag.json"))
	case "delete":
	default:
		return usage()
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	pos := flags.Args()
	if (args[0] == "list" && len(pos) > 1) || (args[0] != "list" && len(pos) != 1) {
		return usage()
	}
	req := controlapi.SnapshotRequest{Operation: args[0]}
	if len(pos) > 0 {
		if args[0] == "delete" {
			req.ID = pos[0]
		} else {
			req.Environment = pos[0]
		}
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, cliMessage("error.controller"))
		return 1
	}
	response, err := client.Snapshot(ctx, req)
	var writeErr error
	if machine {
		writeErr = json.NewEncoder(out).Encode(response.Snapshots)
	} else if args[0] == "list" {
		table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, cliMessage("snapshot.columns"))
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
		return 1
	}
	if writeErr != nil {
		fmt.Fprintln(diagnostic, cliMessage("snapshot.write_failed"))
		return 1
	}
	return 0
}

func snapshotRestoreCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco snapshot restore", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	flags.Usage = func() {
		commandHelp(diagnostic, "snapshot restore", cliLanguage())
	}
	machine := flags.Bool("json", false, cliMessage("flag.json"))
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	pos := flags.Args()
	if len(pos) < 1 || len(pos) > 2 || !regexp.MustCompile(`^snap-[a-f0-9]{32}$`).MatchString(pos[0]) {
		flags.Usage()
		return 2
	}
	req := controlapi.SnapshotRestoreRequest{ID: pos[0]}
	if len(pos) == 2 {
		req.Environment = pos[1]
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, cliMessage("error.controller"))
		return 1
	}
	response, err := client.RestoreSnapshot(ctx, req)
	var writeErr error
	if *machine {
		writeErr = json.NewEncoder(out).Encode(response.Result)
	} else if response.Result.Environment != "" {
		_, writeErr = fmt.Fprintf(out, cliLanguage().Text("snapshot.restored_environment"), response.Result.Environment, response.Result.State)
		if writeErr == nil && response.Result.Workspace != "" {
			_, writeErr = fmt.Fprintf(out, cliLanguage().Text("snapshot.restored_workspace"), response.Result.Workspace)
		}
		if writeErr == nil && response.Result.OCI != "" {
			_, writeErr = fmt.Fprintf(out, cliLanguage().Text("snapshot.restored_oci"), response.Result.OCI)
		}
	}
	if err != nil {
		fmt.Fprintf(diagnostic, "haco: %v\n", err)
		return 1
	}
	if writeErr != nil {
		fmt.Fprintln(diagnostic, cliMessage("snapshot.restore_write_failed"))
		return 1
	}
	return 0
}
