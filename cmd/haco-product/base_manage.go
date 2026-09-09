package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/basemanage"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"io"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"
)

type baseImageClient interface {
	ListBaseImages(context.Context) ([]basemanage.Image, error)
	DeleteBaseImage(context.Context, basemanage.Identity) error
}

func runBaseManage(args []string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	return baseManageCommand(ctx, client, args, os.Stdin, os.Stdout, os.Stderr)
}
func baseManageCommand(ctx context.Context, client baseImageClient, args []string, in io.Reader, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return 2
	}
	flags := flag.NewFlagSet("haco base "+args[0], flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	flags.Usage = func() {
		fmt.Fprintln(diagnostic, "Usage: haco base list --all [--json] | haco base delete [--yes] <name-or-fingerprint>")
	}
	var all, machine, yes bool
	switch args[0] {
	case "list":
		flags.BoolVar(&all, "all", false, "show every retained built-image revision")
		flags.BoolVar(&machine, "json", false, "machine-readable image metadata")
	case "delete":
		flags.BoolVar(&yes, "yes", false, "confirm deletion of the reviewed Base image")
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
	if (args[0] == "list" && (!all || flags.NArg() != 0)) || (args[0] == "delete" && flags.NArg() != 1) {
		flags.Usage()
		return 2
	}
	images, err := client.ListBaseImages(ctx)
	if err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	if args[0] == "list" {
		if machine {
			err = json.NewEncoder(out).Encode(images)
		} else {
			err = writeBaseImages(out, images)
		}
		if err != nil {
			return 1
		}
		return 0
	}
	target := flags.Arg(0)
	prefix := regexp.MustCompile(`^[0-9a-f]{8,64}$`).MatchString(target)
	var selected *basemanage.Image
	for i := range images {
		v := &images[i]
		if (string(v.Name) == target && v.Current) || (prefix && strings.HasPrefix(v.Fingerprint, target)) {
			if selected != nil {
				fmt.Fprintln(diagnostic, "haco: ambiguous Base; select a longer fingerprint from 'haco base list --all'")
				return 1
			}
			selected = v
		}
	}
	if selected == nil {
		fmt.Fprintln(diagnostic, "haco: built Base not found; use 'haco base list --all' for retained revisions")
		return 1
	}
	if err := writeBaseImages(out, []basemanage.Image{*selected}); err != nil {
		return 1
	}
	if len(selected.Environments) > 0 || len(selected.NativeUsers) > 0 || len(selected.ProtectedAliases) > 0 {
		fmt.Fprintln(diagnostic, "haco: Base has Environment or protected alias references; image retained")
		return 1
	}
	fmt.Fprintln(diagnostic, "This deletes the selected local Base image and its aliases. Existing Workspace, OCI and independent snapshot data remain. This revision cannot be used for new creates afterward.")
	if !yes {
		fmt.Fprint(diagnostic, "Delete this Base image? [y/N] ")
		answer, err := bufio.NewReader(io.LimitReader(in, 128)).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if err != nil || (answer != "y" && answer != "yes") {
			fmt.Fprintln(diagnostic, "Base retained.")
			return 1
		}
	}
	if err := client.DeleteBaseImage(ctx, selected.Identity); err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	fmt.Fprintln(out, "Base image deleted; Workspace, OCI and independent snapshots retained")
	return 0
}
func writeBaseImages(out io.Writer, images []basemanage.Image) error {
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "TYPE\tNAME\tFINGERPRINT\tBUILD OWNER\tCURRENT\tENVIRONMENTS\tNATIVE USERS\tPROTECTED ALIASES\tSAVED SNAPSHOTS (independent)")
	for _, v := range images {
		fmt.Fprintf(table, "base-image\t%q\t%q\t%q\t%t\t%q\t%q\t%q\t%q\n", v.Name, v.Fingerprint, v.BuildInstance, v.Current, strings.Join(v.Environments, ","), strings.Join(v.NativeUsers, ","), strings.Join(v.ProtectedAliases, ","), strings.Join(v.IndependentSnapshots, ","))
	}
	return table.Flush()
}
