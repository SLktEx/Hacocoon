package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"io"
	"strings"
	"text/tabwriter"
)

type sourceManageClient interface {
	RepositoryManage(context.Context, controlapi.RepositoryManageRequest) (controlapi.RepositoryManageResponse, error)
}

func sourceManageCommand(ctx context.Context, c sourceManageClient, args []string, in io.Reader, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return 2
	}
	f := flag.NewFlagSet("haco repo "+args[0], flag.ContinueOnError)
	f.SetOutput(diagnostic)
	f.Usage = func() { commandHelp(diagnostic, "repo "+args[0], cliLanguage()) }
	var yes, machine bool
	switch args[0] {
	case "list":
		f.BoolVar(&machine, "json", false, cliMessage("flag.json"))
	case "delete":
		f.BoolVar(&yes, "yes", false, cliMessage("detail.yes"))
	default:
		return 2
	}
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if (args[0] == "list" && f.NArg() != 0) || (args[0] == "delete" && (f.NArg() != 1 || !gitrepo.ValidID(f.Arg(0)))) {
		f.Usage()
		return 2
	}
	all, err := c.RepositoryManage(ctx, controlapi.RepositoryManageRequest{Operation: "list"})
	if err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	if args[0] == "list" {
		if machine {
			err = json.NewEncoder(out).Encode(all)
		} else {
			err = writeSources(out, all.Sources)
		}
		if err != nil {
			return 1
		}
		return 0
	}
	var selected *gitrepo.SourceUse
	for i := range all.Sources {
		if all.Sources[i].Source.ID == f.Arg(0) {
			if selected != nil {
				return 1
			}
			selected = &all.Sources[i]
		}
	}
	if selected == nil {
		fmt.Fprintln(diagnostic, "haco: source repository not found")
		return 1
	}
	if err := writeSources(out, []gitrepo.SourceUse{*selected}); err != nil {
		return 1
	}
	if len(selected.Workspaces) > 0 {
		fmt.Fprintln(diagnostic, "haco: referenced by Workspace Git routing; retained")
		return 1
	}
	if selected.Source.State != "ready" && selected.Source.State != "deleting" {
		fmt.Fprintln(diagnostic, "haco: incomplete preparation requires inspection; retained")
		return 1
	}
	fmt.Fprintln(diagnostic, "This deletes the selected Host source repository and its local Git data. Remote repositories, Workspaces, OCI Stores and independent snapshots remain.")
	if !yes {
		if !requireInteractiveConfirmation(in, diagnostic) {
			return 2
		}
		fmt.Fprint(diagnostic, "Delete this source repository? [y/N] ")
		answer, err := bufio.NewReader(io.LimitReader(in, 128)).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if err != nil || (answer != "yes" && answer != "y") {
			fmt.Fprintln(diagnostic, "Source retained.")
			return 1
		}
	}
	_, err = c.RepositoryManage(ctx, controlapi.RepositoryManageRequest{Operation: "delete", ID: selected.Source.ID, Owner: selected.Source.Owner})
	if err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	fmt.Fprintln(out, "Source repository deleted; remote and independent data retained")
	return 0
}
func writeSources(out io.Writer, sources []gitrepo.SourceUse) error {
	t := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(t, "TYPE\tNAME\tOWNER\tSTATE\tREMOTE\tBRANCH\tWORKSPACE USERS")
	for _, s := range sources {
		fmt.Fprintf(t, "source-repository\t%q\t%q\t%q\t%q\t%q\t%q\n", s.Source.ID, s.Source.Owner, s.Source.State, s.Source.Remote, s.Source.Branch, strings.Join(s.Workspaces, ","))
	}
	return t.Flush()
}
