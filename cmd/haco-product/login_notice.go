package main

import (
	"fmt"
	"golang.org/x/term"
	"os"

	"github.com/SLktEx/Hacocoon/internal/cliui"
)

func writeTrustedHostNotice(out *os.File) {
	writeTrustedHostNoticeInLanguage(out, cliLanguage())
}
func writeTrustedHostNoticeInLanguage(out *os.File, language cliui.Language) {
	message := language.Text("host.notice")
	if term.IsTerminal(int(out.Fd())) && os.Getenv("NO_COLOR") == "" {
		message = "\x1b[33m" + message + "\x1b[0m"
	}
	fmt.Fprintln(out, message)
}
func trustedHostNotice() string {
	return cliMessage("host.notice")
}
