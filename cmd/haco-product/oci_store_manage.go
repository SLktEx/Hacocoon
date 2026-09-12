package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

type ociStoreClient interface {
	OCIStore(context.Context, controlapi.OCIStoreRequest) (controlapi.OCIStoreResponse, error)
}

func runOCIStoreManage(args []string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	return ociStoreManageCommand(ctx, c, args, os.Stdin, os.Stdout, os.Stderr)
}
func ociStoreManageCommand(ctx context.Context, c ociStoreClient, args []string, in io.Reader, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return 2
	}
	f := flag.NewFlagSet("haco plugin oci store "+args[0], flag.ContinueOnError)
	f.SetOutput(diagnostic)
	f.Usage = func() {
		fmt.Fprintln(diagnostic, "Usage: haco plugin oci store list [--json] | haco plugin oci store delete [--yes] <store>")
	}
	var yes, machine bool
	switch args[0] {
	case "list":
		f.BoolVar(&machine, "json", false, "machine-readable Stores and references")
	case "delete":
		f.BoolVar(&yes, "yes", false, "confirm deletion of this Store and all its OCI data")
	default:
		f.Usage()
		return 2
	}
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if (args[0] == "list" && f.NArg() != 0) || (args[0] == "delete" && f.NArg() != 1) {
		f.Usage()
		return 2
	}
	id := "oci:" + f.Arg(0)
	if args[0] == "delete" && !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: id, Owner: strings.Repeat("0", 32)}) {
		f.Usage()
		return 2
	}
	all, err := c.OCIStore(ctx, controlapi.OCIStoreRequest{Operation: "list"})
	if err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	if args[0] == "list" {
		if machine {
			err = json.NewEncoder(out).Encode(all)
		} else {
			err = writeOCIStores(out, all)
		}
		if err != nil {
			return 1
		}
		return 0
	}
	var selected *core.PersistentResource
	for i := range all.Resources {
		if all.Resources[i].ID == id {
			if selected != nil {
				fmt.Fprintln(diagnostic, "haco: duplicate Store identity")
				return 1
			}
			selected = &all.Resources[i]
		}
	}
	if selected == nil {
		fmt.Fprintln(diagnostic, "haco: Store not found")
		return 1
	}
	var use *oci.StoreUse
	for i := range all.Uses {
		if all.Uses[i].Resource == selected.Ref() {
			if use != nil {
				return 1
			}
			use = &all.Uses[i]
		}
	}
	if use == nil {
		fmt.Fprintln(diagnostic, "haco: Store references unavailable")
		return 1
	}
	if err := writeOCIStores(out, controlapi.OCIStoreResponse{Resources: []core.PersistentResource{*selected}, Uses: []oci.StoreUse{*use}}); err != nil {
		return 1
	}
	if selected.SourceOnly || len(use.Environments) > 0 || len(use.PendingCopies) > 0 {
		fmt.Fprintln(diagnostic, "haco: Store is referenced or is the protected Host source; retained")
		return 1
	}
	if selected.State != "ready" && selected.State != "deleting" {
		fmt.Fprintln(diagnostic, "haco: incomplete Store creation requires inspection; retained")
		return 1
	}
	fmt.Fprintln(diagnostic, "This deletes all images, container metadata, build cache and persistent data in this Store. Workspace files, the Host source and independent saved snapshots remain.")
	if !yes {
		if !requireInteractiveConfirmation(in, diagnostic) {
			return 2
		}
		fmt.Fprint(diagnostic, "Delete this OCI Store? [y/N] ")
		answer, err := bufio.NewReader(io.LimitReader(in, 128)).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if err != nil || (answer != "y" && answer != "yes") {
			fmt.Fprintln(diagnostic, "Store retained.")
			return 1
		}
	}
	if _, err := c.OCIStore(ctx, controlapi.OCIStoreRequest{Operation: "delete", ID: selected.ID, Owner: selected.Owner}); err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	fmt.Fprintln(out, "OCI Store deleted; Workspace, Host source and independent snapshots retained")
	return 0
}
func writeOCIStores(out io.Writer, all controlapi.OCIStoreResponse) error {
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "TYPE\tNAME\tMANAGED ID\tOWNER\tSTATE\tROLE\tWORKSPACE\tENVIRONMENTS\tPENDING COPIES\tSAVED SNAPSHOTS (independent)")
	for _, r := range all.Resources {
		var use *oci.StoreUse
		for i := range all.Uses {
			if all.Uses[i].Resource == r.Ref() {
				if use != nil {
					return core.ErrIncompatibleState
				}
				use = &all.Uses[i]
			}
		}
		if use == nil {
			return core.ErrIncompatibleState
		}
		fmt.Fprintf(table, "oci-store\t%q\t%q\t%q\t%q\t%q\t%q\t%q\t%q\t%q\n", strings.TrimPrefix(r.ID, "oci:"), r.ID, r.Owner, r.State, use.Role, r.WorkspaceID, strings.Join(use.Environments, ","), strings.Join(use.PendingCopies, ","), strings.Join(use.IndependentSnapshots, ","))
	}
	return table.Flush()
}
