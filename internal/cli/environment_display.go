package cli

import (
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/client/ssh/config"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
	"github.com/SLktEx/Hacocoon/internal/core"
)

var configEnvironmentName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,56}$`)

func writeSSHConfig(out io.Writer, name string, connections []core.ClientConnection) error {
	if !configEnvironmentName.MatchString(name) {
		return core.ErrInvalidArgument
	}
	var ssh []core.ClientConnection
	for _, c := range connections {
		if c.Kind == "ssh" {
			ssh = append(ssh, c)
		}
	}
	if len(ssh) != 1 {
		return fmt.Errorf("expected one prepared SSH connection; run 'haco env ssh --key <public-key-file> %s', or disconnect extra SSH connections", name)
	}
	c := ssh[0]
	command, err := sshconfig.StreamCommand(c.Target, os.Getenv("WSL_DISTRO_NAME"))
	if err != nil || c.Target.Environment != name || c.Port != 0 || c.Host != "" || c.User != "root" {
		return core.ErrIncompatibleState
	}
	_, err = fmt.Fprintf(out, "# Keep IdentityFile and the trusted host-key pin on your SSH client.\nHost haco-%s\n  HostName haco-%s\n  User root\n  HostKeyAlias haco-%s\n  StrictHostKeyChecking yes\n  ProxyCommand %s\n", name, name, name, command)
	return err
}

func writeEnvironmentStatus(out io.Writer, status core.EnvironmentStatus) int {
	return writeEnvironmentStatusLanguage(out, status, cliLanguage())
}

func writeEnvironmentStatusLanguage(out io.Writer, status core.EnvironmentStatus, language cliui.Language) int {
	env := status.Environment
	if _, err := fmt.Fprint(out, language.Format("env.status.header", displayCell(env.Name), displayCell(string(status.State)), displayCell(env.Workspace.Path), displayCell(string(env.AccessMode)))); err != nil {
		return 1
	}
	dnsLabel := language.Text("env.dns." + string(env.DNSMode.Effective()))
	if !env.DNSMode.Valid() {
		dnsLabel = language.Text("env.dns.unknown")
	}
	if _, err := fmt.Fprint(out, language.Format("env.status.dns", dnsLabel)); err != nil {
		return 1
	}
	if env.Base != nil {
		if _, err := fmt.Fprint(out, language.Format("env.status.base", displayCell(string(env.Base.Name)), displayCell(string(env.Base.Revision)))); err != nil {
			return 1
		}
	}
	if status.State == core.EnvironmentStopped {
		if _, err := fmt.Fprintln(out, language.Text("env.status.stopped")); err != nil {
			return 1
		}
	}
	return 0
}

func displayCell(value string) string {
	if strings.IndexFunc(value, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
		return strconv.QuoteToASCII(value)
	}
	return value
}

func writeEnvironmentList(out io.Writer, environments []core.Environment) error {
	return writeEnvironmentListLanguage(out, environments, cliLanguage())
}

func writeEnvironmentListLanguage(out io.Writer, environments []core.Environment, language cliui.Language) error {
	if len(environments) == 0 {
		_, err := fmt.Fprintln(out, language.Text("env.list.empty"))
		return err
	}
	environments = append([]core.Environment(nil), environments...)
	sort.Slice(environments, func(i, j int) bool { return environments[i].Name < environments[j].Name })
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, language.Text("env.list.header")); err != nil {
		return err
	}
	for _, env := range environments {
		base := "-"
		if env.Base != nil {
			base = string(env.Base.Name)
		}
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\n", displayCell(env.Name), displayCell(env.Workspace.Path), displayCell(base)); err != nil {
			return err
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintln(out, language.Text("env.list.next"))
	return err
}
