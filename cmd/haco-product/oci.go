package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
)

func runPlugin(args []string) int {
	if len(args) >= 2 && args[0] == "oci" && args[1] == "image" {
		return runOCIImageManage(args[2:])
	}
	if len(args) >= 3 && args[0] == "oci" && args[1] == "store" && (args[2] == "list" || args[2] == "delete") {
		return runOCIStoreManage(args[2:])
	}
	usage := func() int {
		fmt.Fprintln(os.Stderr, "Usage: haco plugin oci store create [--json] <store> [--from <store>] | haco plugin oci store inspect [--json] <store> | haco plugin oci store delete [--yes] <store> | haco plugin oci store list [--json]")
		return 2
	}
	clean, jsonOutput, flagErr := splitJSONFlag(args)
	if flagErr != nil {
		fmt.Fprintln(os.Stderr, "haco:", flagErr)
		return 2
	}
	args = clean
	if len(args) < 3 || args[0] != "oci" || args[1] != "store" {
		return usage()
	}
	req, ok := parseOCIStoreRequest(args)
	if !ok {
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
	if err := writeCLIResult(os.Stdout, result, jsonOutput); err != nil {
		return 1
	}
	return 0
}

func parseOCIStoreRequest(args []string) (controlapi.OCIStoreRequest, bool) {
	if len(args) < 3 || args[0] != "oci" || args[1] != "store" {
		return controlapi.OCIStoreRequest{}, false
	}
	req := controlapi.OCIStoreRequest{Operation: args[2]}
	if req.Operation == "list" {
		return req, len(args) == 3
	}
	if req.Operation != "create" && req.Operation != "inspect" && req.Operation != "delete" {
		return req, false
	}
	for i := 3; i < len(args); i++ {
		value := args[i]
		if value == "--from" {
			if req.Operation != "create" || req.From != "" || i+1 == len(args) {
				return req, false
			}
			i++
			value = args[i]
			if value == "" || strings.HasPrefix(value, "-") {
				return req, false
			}
			req.From = "oci:" + value
		} else {
			if req.ID != "" || value == "" || strings.HasPrefix(value, "-") {
				return req, false
			}
			req.ID = "oci:" + value
		}
	}
	return req, req.ID != ""
}
