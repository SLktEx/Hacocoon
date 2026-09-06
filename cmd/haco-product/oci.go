package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func runPlugin(args []string) int {
	if len(args) < 2 || args[0] != "oci" || args[1] != "distribute" {
		fmt.Fprintln(os.Stderr, "Usage: haco plugin oci distribute --runtime docker|nerdctl --image <image> <environment>")
		return 2
	}
	flags := flag.NewFlagSet("oci distribute", flag.ContinueOnError)
	driver := flags.String("runtime", "", "optional OCI runtime on both sides")
	image := flags.String("image", "", "trusted Host image to copy")
	if flags.Parse(args[2:]) != nil || flags.NArg() != 1 {
		return 2
	}
	selected, err := oci.ParseDriver(*driver)
	if err != nil || *image == "" {
		fmt.Fprintln(os.Stderr, "haco: --runtime docker|nerdctl and --image are required")
		return 2
	}
	c, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 12*time.Minute)
	defer cancel()
	result, err := c.DistributeImage(ctx, oci.TransferRequest{Environment: flags.Arg(0), Driver: selected, Image: *image})
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	if json.NewEncoder(os.Stdout).Encode(result) != nil {
		return 1
	}
	return 0
}
