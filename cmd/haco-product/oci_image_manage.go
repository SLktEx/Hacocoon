package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

type ociImageClient interface {
	OCIImage(context.Context, controlapi.OCIImageRequest) (oci.ManagedImageList, error)
}

func runOCIImageManage(args []string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	return ociImageManageCommand(ctx, c, args, os.Stdin, os.Stdout, os.Stderr)
}
func ociImageManageCommand(ctx context.Context, c ociImageClient, args []string, in io.Reader, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		commandHelp(diagnostic, "plugin oci image", cliLanguage())
		return 2
	}
	f := flag.NewFlagSet("haco plugin oci image "+args[0], flag.ContinueOnError)
	f.SetOutput(diagnostic)
	hostSource := f.Bool("host", false, cliMessage("detail.host_images"))
	runtime := f.String("runtime", "nerdctl", cliMessage("detail.runtime"))
	unused := f.Bool("unused", false, cliMessage("detail.unused_list"))
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
	expected := 1
	if args[0] == "delete" && !*unused {
		expected++
	}
	if *hostSource {
		expected--
	}
	if (*runtime != "docker" && *runtime != "nerdctl") || f.NArg() != expected {
		return 2
	}
	environment := f.Arg(0)
	selection := f.Arg(1)
	if *hostSource {
		environment = ""
		selection = f.Arg(0)
	}
	all, err := c.OCIImage(ctx, controlapi.OCIImageRequest{Operation: "list", Environment: environment, Host: *hostSource, Runtime: *runtime})
	if err != nil {
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	matchingTarget := all.Target.Environment == environment
	if all.Target.Detached {
		matchingTarget = !*hostSource && all.Target.Store.ID == environment && all.Target.Environment == "" && all.Target.Instance == ""
	}
	if !matchingTarget || all.Target.Host != *hostSource || all.Target.Runtime != *runtime {
		fmt.Fprintln(diagnostic, "haco: invalid image review identity")
		return 1
	}
	selected := []oci.ManagedImage{}
	seen := map[string]bool{}
	for _, image := range all.Images {
		if !oci.ValidImageSelection(all.Target, image.ID) || seen[image.ID] {
			fmt.Fprintln(diagnostic, "haco: invalid or duplicate image review identity")
			return 1
		}
		seen[image.ID] = true
		match := image.ID == selection
		for _, tag := range image.Tags {
			match = match || tag == selection
		}
		if *unused {
			match = len(image.Containers) == 0
		}
		if (args[0] == "list" && !*unused) || match {
			selected = append(selected, image)
		}
	}
	if args[0] == "list" {
		all.Images = selected
		if machine {
			err = json.NewEncoder(out).Encode(all)
		} else {
			err = writeOCIImages(out, all)
		}
		if err != nil {
			return 1
		}
		return 0
	}
	if len(selected) == 0 {
		if *unused {
			fmt.Fprintln(out, "No images without container users; nothing deleted")
			return 0
		}
		fmt.Fprintln(diagnostic, "haco: image not found; use image list to review exact IDs and tags")
		return 1
	}
	if !*unused && len(selected) != 1 {
		fmt.Fprintln(diagnostic, "haco: ambiguous image selection")
		return 1
	}
	review := all
	review.Images = selected
	if writeOCIImages(out, review) != nil {
		return 1
	}
	if len(selected[0].Containers) > 0 {
		fmt.Fprintln(diagnostic, "haco: image is referenced by a container; retained")
		return 1
	}
	if *unused {
		fmt.Fprintln(diagnostic, "Selection includes tagged images with no running or stopped container users. This is not an estimate of reclaimable bytes.")
	}
	if *hostSource {
		fmt.Fprintln(diagnostic, "This changes the Host source used for future Store copies. Existing independent copies remain.")
	}
	fmt.Fprintln(diagnostic, "Delete this image from the selected Store. Independent copies, saved snapshots and remote registry images remain. The runtime may refuse images with multiple tags or other references; no force is used.")
	if !yes {
		if !requireInteractiveConfirmation(in, diagnostic) {
			return 2
		}
		fmt.Fprint(diagnostic, "Delete the reviewed image(s)? [y/N] ")
		answer, err := bufio.NewReader(io.LimitReader(in, 128)).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if err != nil || (answer != "y" && answer != "yes") {
			fmt.Fprintln(diagnostic, "Image retained.")
			return 1
		}
	}
	for i, image := range selected {
		if _, err := c.OCIImage(ctx, controlapi.OCIImageRequest{Operation: "delete", Target: all.Target, ID: image.ID}); err != nil {
			fmt.Fprintf(diagnostic, "haco: deletion stopped after %d of %d images: %v\n", i, len(selected), err)
			return 1
		}
		fmt.Fprintf(out, "OCI image deleted: %s\n", image.ID)
	}
	return 0
}
func writeOCIImages(out io.Writer, all oci.ManagedImageList) error {
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	role := "independent copy"
	if all.Target.Host {
		role = "Host source for future copies"
		fmt.Fprintln(table, "Target: managed Host source")
	} else if all.Target.Detached {
		role = "detached retained Store"
		fmt.Fprintln(table, "Target: retained Store via a disposable Environment")
	} else {
		fmt.Fprintf(table, "Environment: %q (generation %q)\n", all.Target.Environment, all.Target.Instance)
	}
	fmt.Fprintf(table, "Store: %q (owner %q, %s)\nRuntime: %q\nSaved snapshots (independent): %q\n", all.Target.Store.ID, all.Target.Store.Owner, role, all.Target.Runtime, strings.Join(all.IndependentSnapshots, ","))
	fmt.Fprintln(table, "TYPE\tIMAGE ID\tTAGS\tDIGESTS\tCONTAINER USERS")
	for _, image := range all.Images {
		fmt.Fprintf(table, "oci-image\t%q\t%q\t%q\t%q\n", image.ID, strings.Join(image.Tags, ","), strings.Join(image.Digests, ","), strings.Join(image.Containers, ","))
	}
	return table.Flush()
}
