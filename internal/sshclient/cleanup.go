//go:build linux

package sshclient

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"

	"strings"
)

// Cleanup removes only fragments whose exact saved grant is positively stale.
// Transport failure and recovery-required state never authorize local deletion.
func Cleanup(ctx context.Context, c Controller, d Desktop) error {
	f, err := openFiles(ctx, d.Home, d.Windows)
	if err != nil {
		return err
	}
	dir, err := f.root.Open("hacocoon")
	if err != nil {
		f.close()
		return err
	}
	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		f.close()
		return err
	}
	owned := map[string]saved{}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".conf")
		if entry.IsDir() || name == entry.Name() || !namePattern.MatchString(name) {
			continue
		}
		b, e := f.read("hacocoon/" + entry.Name())
		if e != nil {
			f.close()
			return e
		}
		first, _, _ := strings.Cut(string(b), "\n")
		if !strings.HasPrefix(first, prefix) {
			continue
		}
		var meta saved
		if json.Unmarshal([]byte(strings.TrimPrefix(first, prefix)), &meta) != nil {
			f.close()
			return core.ErrIncompatibleState
		}
		if meta.Connection.Target == nil || meta.Distro != desktopDistro(d) {
			continue
		}
		owned[name] = meta
	}
	f.close()
	for name, meta := range owned {
		_, e := c.EnvironmentStatus(ctx, name)
		stale := false
		if e != nil {
			var status *control.StatusError
			if errors.Is(e, core.ErrNotFound) || (errors.As(e, &status) && status.Code == "not_found") {
				stale = true
			} else {
				return e
			}
		} else {
			connections, e := c.EnvironmentConnections(ctx, name)
			if e != nil {
				return e
			}
			stale = true
			for _, actual := range connections {
				if sameConnection(actual, meta.Connection) {
					stale = false
				}
			}
		}
		if stale {
			if err = Remove(ctx, d, name, meta.Connection.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
