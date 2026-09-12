package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/SLktEx/Hacocoon/internal/cliui"
)

func cliLanguage() cliui.Language {
	return cliui.Resolve(os.Getenv)
}

// This is a presentation-only lookup. Do not pass JSON/protocol values, raw
// subprocess output or an error string through the catalog as a message ID.
func cliMessage(id string, args ...any) string {
	return cliLanguage().Format(id, args...)
}

func writeLocalizedHelp(out io.Writer, language cliui.Language) {
	fmt.Fprintln(out, "Hacocoon")
	fmt.Fprintln(out)
	fmt.Fprintln(out, language.Text("help.usage"))
	fmt.Fprintln(out, "  haco <command>")
	fmt.Fprintln(out)
	fmt.Fprintln(out, language.Text("help.commands"))
	for _, command := range [...]struct{ name, message string }{
		{"setup", "help.setup"},
		{"aws", "help.aws"},
		{"config", "help.config"},
		{"approve", "help.approve"},
		{"doctor", "help.doctor"},
		{"reclaim", "help.reclaim"},
		{"env", "help.env"},
		{"snapshot", "help.snapshot"},
		{"run", "help.run"},
		{"ssh setup", "help.ssh"},
		{"open", "help.open"},
		{"base", "help.base"},
		{"plugin", "help.plugin"},
		{"repo", "help.repo"},
		{"workspace", "help.workspace"},
		{"git", "help.git"},
		{"help", "help.help"},
		{"version", "help.version"},
	} {
		fmt.Fprintf(out, "  %-11s%s\n", command.name, language.Text(command.message))
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, language.Text("help.footer"))
}

// Keep flag names, accepted values and parse errors untouched. Only the
// human-facing Usage heading and descriptions supplied by callers are localized.
func configureCLIFlags(flags *flag.FlagSet, diagnostic io.Writer) {
	flags.SetOutput(diagnostic)
	language := cliLanguage()
	flags.Usage = func() {
		fmt.Fprintln(diagnostic, language.Format("flags.usage", flags.Name()))
		flags.PrintDefaults()
	}
}
