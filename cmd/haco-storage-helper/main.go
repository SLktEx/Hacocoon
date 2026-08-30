package main

import (
	"context"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/storagepriv"
)

func main() {
	helper := storagepriv.NewHelper(host.ExecRunner{})
	result := helper.Execute(context.Background(), os.Args[1:])
	if result.Stdout != "" {
		_, _ = fmt.Fprint(os.Stdout, result.Stdout)
	}
	if result.Stderr != "" {
		_, _ = fmt.Fprint(os.Stderr, result.Stderr)
	}
	if result.ExitCode != 0 {
		code := result.ExitCode
		if code < 1 || code > 255 {
			code = 1
		}
		os.Exit(code)
	}
}
