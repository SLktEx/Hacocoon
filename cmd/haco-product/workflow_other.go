//go:build !linux

package main

import (
	"context"
	"fmt"
	"io"
)

func workflowCommand(_ context.Context, _ []string, _ io.Writer, diagnostic io.Writer) int {
	fmt.Fprintln(diagnostic, "haco: Workspace path workflows run on the Linux/WSL client")
	return 1
}
