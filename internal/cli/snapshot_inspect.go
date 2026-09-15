package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func writeSnapshotInspection(out io.Writer, result *core.SnapshotInspection, details bool) error {
	if result == nil {
		return nil
	}
	if _, err := fmt.Fprintf(out, cliLanguage().Text("snapshot.inspect.header"), result.Environment, result.State); err != nil {
		return err
	}
	for index, c := range result.Components {
		role := cliMessage("snapshot.inspect.rootfs")
		switch {
		case strings.HasPrefix(c.Role, "workspace:"):
			role = fmt.Sprintf(cliLanguage().Text("snapshot.inspect.workspace"), strings.TrimPrefix(c.Role, "workspace:"))
		case strings.HasPrefix(c.Role, "data:"):
			role = fmt.Sprintf(cliLanguage().Text("snapshot.inspect.data"), strings.TrimPrefix(c.Role, "data:"))
		case c.Role == "oci":
			role = "OCI Store"
		case c.Role == "base":
			role = "Base"
		case c.Role != "rootfs":
			role = cliMessage("snapshot.inspect.other")
		}
		check := "unavailable"
		switch c.Check {
		case "absent", "ready", "busy", "unsupported", "ownership_changed", "invalid":
			check = c.Check
		}
		if _, err := fmt.Fprintf(out, "%d. %s — %s\n", index+1, role, cliMessage("snapshot.inspect."+check)); err != nil {
			return err
		}
		if c.References != nil {
			if _, err := fmt.Fprintf(out, cliLanguage().Text("snapshot.inspect.references"), *c.References); err != nil {
				return err
			}
		}
		if details {
			if _, err := fmt.Fprintf(out, cliLanguage().Text("snapshot.inspect.identity"), c.State, c.Provider, c.Project, c.Pool, c.Object, c.Presence); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintln(out, cliMessage("snapshot.inspect.backing")); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, cliLanguage().Text("snapshot.inspect.retry"), result.ID)
	return err
}
