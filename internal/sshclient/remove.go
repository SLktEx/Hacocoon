//go:build linux

package sshclient

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"os"
	"strings"
)

// Remove owns only an identified Hacocoon fragment. A revoked grant selector
// cannot remove a newer setup. Shared keys, pins and user config are preserved.
func Remove(ctx context.Context, d Desktop, name, grant string) error {
	if !namePattern.MatchString(name) {
		return core.ErrInvalidArgument
	}
	f, err := openFiles(ctx, d.Home, d.Windows)
	if err != nil {
		return err
	}
	defer f.close()
	path := "hacocoon/" + name + ".conf"
	b, err := f.read(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	first, _, _ := strings.Cut(string(b), "\n")
	var previous saved
	if !strings.HasPrefix(first, prefix) || json.Unmarshal([]byte(strings.TrimPrefix(first, prefix)), &previous) != nil {
		return core.ErrIncompatibleState
	}
	if previous.Connection.Target != nil && previous.Connection.Target.Environment != name {
		return core.ErrIncompatibleState
	}
	if previous.Distro != desktopDistro(d) {
		return nil
	}
	if grant != "" && previous.Connection.ID != grant {
		return nil
	}
	return f.root.Remove(path)
}

func desktopDistro(d Desktop) string {
	if d.Windows {
		return os.Getenv("WSL_DISTRO_NAME")
	}
	return ""
}
