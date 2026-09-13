package cliui

import (
	"fmt"
	"io"
	"strings"
)

// CommandHelp is client-owned presentation metadata, not dispatch or authority.
type CommandHelp struct {
	Path, Syntax, Message, Example string
	Arguments, Options             []HelpField
	Notes                          []string
}

// HelpField describes an argument or option; it never changes parsing defaults.
type HelpField struct{ Syntax, Message string }

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
		prefix := "  " + strings.TrimSpace(program+" "+path) + " "
		if syntax == "" {
			fmt.Fprintln(out, strings.TrimRight(prefix, " "))
		}
		for _, part := range strings.Split(syntax, "\n") {
			if part != "" {
				fmt.Fprint(out, HelpLines(prefix, strings.TrimSpace(part), 60))
			}
			prefix = "    "
		}
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
		for _, section := range []struct {
			title  string
			fields []HelpField
		}{
			{"help.arguments", page.Arguments}, {"help.options", page.Options},
		} {
			if len(section.fields) == 0 {
				continue
			}
			fmt.Fprintln(out, "\n"+language.Text(section.title))
			for _, field := range section.fields {
				fmt.Fprintln(out, "  "+field.Syntax)
				fmt.Fprint(out, HelpLines("    ", language.Text(field.Message), 60))
			}
		}
		if len(page.Notes) != 0 {
			fmt.Fprintln(out, "\n"+language.Text("help.requirements"))
			for _, note := range page.Notes {
				fmt.Fprint(out, HelpLines("  ", language.Text(note), 60))
			}
		}
		fmt.Fprintln(out, "\n"+language.Text("help.example"))
		fmt.Fprintln(out, "  "+page.Example)
		return true
	}
	return false
}
