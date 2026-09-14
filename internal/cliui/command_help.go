package cliui

import (
	"fmt"
	"io"
	"strings"
)

// CommandHelp is client-owned presentation metadata, not dispatch or authority.
type CommandHelp struct{ Path, Syntax, Message, Example string }

// WriteCommandHelp renders one hierarchy using the shared bilingual catalog.
func WriteCommandHelp(out io.Writer, program, path string, pages []CommandHelp, language Language) bool {
	for _, page := range pages {
		if page.Path != path {
			continue
		}
		fmt.Fprint(out, HelpLines("", language.Text(page.Message), 60))
		fmt.Fprintln(out, language.Text("help.usage"))
		// Keep each option on a separate continuation line, independent of TTY width.
		syntax := strings.ReplaceAll(page.Syntax, " [", "\n    [")
		fmt.Fprintf(out, "  %s %s\n", strings.TrimSpace(program+" "+path), syntax)
		children := false
		for _, child := range pages {
			suffix, ok := strings.CutPrefix(child.Path, path+" ")
			if path == "" {
				suffix, ok = child.Path, child.Path != ""
			}
			if !ok || strings.Contains(suffix, " ") {
				continue
			}
			if !children {
				fmt.Fprintln(out, "\n"+language.Text("help.commands"))
				children = true
			}
			fmt.Fprint(out, HelpLines(fmt.Sprintf("  %-14s", suffix), language.Text(child.Message), 60))
		}
		if children {
			fmt.Fprintf(out, "\n  %s <command> --help\n", strings.TrimSpace(program+" "+path))
		}
		fmt.Fprintln(out, "\n"+language.Text("help.example"))
		fmt.Fprintln(out, "  "+page.Example)
		return true
	}
	return false
}
