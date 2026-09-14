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
	_, _ = fmt.Fprintln(out, "Hacocoon")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, language.Text("help.usage"))
	_, _ = fmt.Fprintln(out, "  haco <command>")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, language.Text("help.commands"))
	for _, command := range [...]struct{ name, message string }{
		{"setup", "help.setup"},
		{"network", "command.network"},
		{"aws", "help.aws"},
		{"config", "help.config"},
		{"experimental edit vscode", "help.experimental"},
		{"approve", "help.approve"},
		{"doctor", "help.doctor"},
		{"reclaim", "help.reclaim"},
		{"env", "help.env"},
		{"snapshot", "help.snapshot"},
		{"run", "help.run"},
		{"ssh setup", "help.ssh"},
		{"ssh cleanup", "command.ssh.cleanup"},
		{"open", "command.open"},
		{"base", "help.base"},
		{"plugin", "help.plugin"},
		{"repo", "help.repo"},
		{"workspace", "command.workspace"},
		{"git", "help.git"},
		{"help", "help.help"},
		{"version", "help.version"},
	} {
		_, _ = fmt.Fprint(out, cliui.HelpLines(fmt.Sprintf("  %-11s", command.name), language.Text(command.message), 60))
	}
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, language.Text("help.daily"))
	for _, step := range []struct{ command, message string }{
		{"haco env list", "help.daily.list"}, {"haco env status <name>", "help.daily.status"},
		{"haco env start <name>", "help.daily.start"}, {"haco open <name>", "help.daily.open"},
		{"haco env stop <name>", "help.daily.stop"}, {"haco env delete <name>", "help.daily.delete"},
	} {
		_, _ = fmt.Fprintln(out, "  "+step.command)
		_, _ = fmt.Fprint(out, cliui.HelpLines("    ", language.Text(step.message), 60))
	}
	_, _ = fmt.Fprintln(out, "  haco env create --workspace <controller-path|managed:name> <name>")
	_, _ = fmt.Fprint(out, cliui.HelpLines("", language.Text("help.daily.footer"), 60))
}

// Keep flag names, accepted values and parse errors untouched. Only the
// human-facing Usage heading and descriptions supplied by callers are localized.
func configureCLIFlags(flags *flag.FlagSet, diagnostic io.Writer) {
	flags.SetOutput(diagnostic)
	language := cliLanguage()
	flags.Usage = func() {
		_, _ = fmt.Fprintln(diagnostic, language.Format("flags.usage", flags.Name()))
		flags.PrintDefaults()
	}
}
