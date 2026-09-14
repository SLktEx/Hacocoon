package main

import (
	"fmt"
	"golang.org/x/term"
	"os"
)

func writeTrustedHostNotice(out *os.File) {
	message := trustedHostNotice()
	if term.IsTerminal(int(out.Fd())) && os.Getenv("NO_COLOR") == "" {
		message = "\x1b[33m" + message + "\x1b[0m"
	}
	fmt.Fprintln(out, message)
}
func trustedHostNotice() string {
	return cliMessage("host.notice")
}
