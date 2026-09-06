package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
)

func runPlugin(args []string) int {
	usage := func() int {
		fmt.Fprintln(os.Stderr, "Usage: haco plugin oci store create|inspect|delete <store> | haco plugin oci store list")
		return 2
	}
	if len(args) < 3 || args[0] != "oci" || args[1] != "store" {
		return usage()
	}
	req := controlapi.OCIStoreRequest{Operation: args[2]}
	switch req.Operation {
	case "list":
		if len(args) != 3 {
			return usage()
		}
	case "create", "inspect", "delete":
		if len(args) != 4 {
			return usage()
		}
		req.ID = "oci:" + args[3]
	default:
		return usage()
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
	result, err := c.OCIStore(ctx, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	if json.NewEncoder(os.Stdout).Encode(result) != nil {
		return 1
	}
	return 0
}
