package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"io"
	"strings"
	"text/tabwriter"
)

func managedWorkspaceCommand(ctx context.Context, args []string, in io.Reader, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return 2
	}
	flags := flag.NewFlagSet("haco workspace "+args[0], flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	flags.Usage = func() {
		fmt.Fprintln(diagnostic, "Usage: haco workspace list [--json] | haco workspace delete [--yes] <workspace>")
	}
	var machine, yes bool
	switch args[0] {
	case "list":
		flags.BoolVar(&machine, "json", false, "machine-readable retained Workspaces")
	case "delete":
		flags.BoolVar(&yes, "yes", false, "confirm deletion of this Workspace and all its Git data")
	default:
		flags.Usage()
		return 2
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	pos := flags.Args()
	if (args[0] == "list" && len(pos) != 0) || (args[0] == "delete" && (len(pos) != 1 || !gitrepo.ValidID(pos[0]))) {
		flags.Usage()
		return 2
	}
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: cannot open controller client")
		return 1
	}
	all, err := client.ListManagedWorkspaces(ctx)
	if err != nil {
		fmt.Fprintf(diagnostic, "haco: %v\n", err)
		return 1
	}
	if args[0] == "list" {
		if machine {
			err = json.NewEncoder(out).Encode(all)
		} else {
			err = writeManagedWorkspaces(out, all)
		}
		if err != nil {
			return 1
		}
		return 0
	}
	var selected *workspace.ManagedWorkspace
	for i := range all {
		if all[i].Name == pos[0] {
			if selected != nil {
				fmt.Fprintln(diagnostic, "haco: duplicate Workspace identity")
				return 1
			}
			selected = &all[i]
		}
	}
	if selected == nil {
		fmt.Fprintln(diagnostic, "haco: Workspace not found")
		return 1
	}
	if err := writeManagedWorkspaces(out, []workspace.ManagedWorkspace{*selected}); err != nil {
		return 1
	}
	if len(selected.Environments) != 0 {
		fmt.Fprintln(diagnostic, "haco: Workspace is referenced by an Environment or unfinished lease; delete the Environment first")
		return 1
	}
	if selected.State != "ready" && selected.State != "deleting" {
		fmt.Fprintln(diagnostic, "haco: unfinished creation requires inspection; no data deleted")
		return 1
	}
	fmt.Fprintln(diagnostic, "This deletes every repository and file in this Workspace, including uncommitted, untracked and unpushed work. Saved snapshots, OCI Stores and source repositories remain.")
	if !yes {
		fmt.Fprint(diagnostic, "Delete this Workspace? [y/N] ")
		answer, readErr := bufio.NewReader(io.LimitReader(in, 128)).ReadString('\n')
		if readErr != nil || (strings.ToLower(strings.TrimSpace(answer)) != "y" && strings.ToLower(strings.TrimSpace(answer)) != "yes") {
			fmt.Fprintln(diagnostic, "Workspace retained.")
			return 1
		}
	}
	if err := client.DeleteManagedWorkspace(ctx, selected.Workspace); err != nil {
		fmt.Fprintf(diagnostic, "haco: %v; ownership record retained if cleanup is incomplete\n", err)
		return 1
	}
	fmt.Fprintln(out, "Workspace deleted; saved snapshots, OCI Stores and source repositories retained")
	return 0
}
func writeManagedWorkspaces(out io.Writer, all []workspace.ManagedWorkspace) error {
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "TYPE\tNAME\tMANAGED ID\tSTATE\tREPOSITORIES\tENVIRONMENTS\tSAVED SNAPSHOTS (independent)\tOCI STORES (retained)")
	for _, w := range all {
		fmt.Fprintf(table, "workspace\t%q\t%q\t%q\t%q\t%q\t%q\t%q\n", w.Name, w.Workspace.ID, w.State, strings.Join(w.Repositories, ","), strings.Join(w.Environments, ","), strings.Join(w.Snapshots, ","), strings.Join(w.Stores, ","))
	}
	return table.Flush()
}
