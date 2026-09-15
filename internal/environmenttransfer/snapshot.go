package environmenttransfer

import (
	"io"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// SnapshotArchive pairs exported bytes with the exact protected source component.
// It is not an ownership receipt. Callers must hold canonical source reservations
// and verify native ownership while producing archives; guest claims are not input.
type SnapshotArchive struct {
	Component core.SnapshotComponent
	Bytes     int64
	SHA256    string
	Data      io.Reader
}

// WriteSnapshot matches every archive against the complete protected inventory
// before emitting bytes. It neither opens provider paths nor reserves snapshots.
// Legacy Base components stay in their catalog but are not part of rootfs transport.
// A failed return must never publish the output, including underlying write errors.
func WriteSnapshot(dst io.Writer, saved core.Snapshot, archives []SnapshotArchive, limit int64) error {
	return writeSnapshot(dst, saved, archives, limit, nil)
}

func writeSnapshot(dst io.Writer, saved core.Snapshot, archives []SnapshotArchive, limit int64, workspaces []Workspace) error {
	ordered, err := snapshotComponents(saved)
	if err != nil {
		return err
	}
	if len(archives) != len(ordered) {
		return ErrInvalidBundle
	}
	bySource := make(map[core.SnapshotComponent]SnapshotArchive, len(archives))
	for _, a := range archives {
		if _, duplicate := bySource[a.Component]; duplicate {
			return ErrInvalidBundle
		}
		bySource[a.Component] = a
	}
	m := Manifest{DNSMode: saved.Source.Environment.DNSMode, Version: 1, Source: saved.Source.Environment.Name, HasOCI: saved.Source.Environment.PersistentResource.ID != ""}
	for _, a := range saved.Source.Environment.Attachments {
		m.Data = append(m.Data, Data{Key: a.Key, Target: a.Target, Kind: a.Origin.Kind})
	}
	if workspaces != nil {
		m.Version = 2
		m.Workspaces = append([]Workspace(nil), workspaces...)
	}
	readers := make([]io.Reader, 0, len(ordered))
	dataIndex := 0
	for i, c := range ordered {
		a, found := bySource[c]
		if !found {
			return ErrInvalidBundle
		}
		role := c.Role
		if i > 0 && strings.HasPrefix(c.Role, "workspace:") {
			role = workspaceRole(i - 1)
		}
		if strings.HasPrefix(c.Role, "data:") {
			role = dataRole(dataIndex)
			dataIndex++
		}
		m.Components = append(m.Components, Component{Role: role, Bytes: a.Bytes, SHA256: a.SHA256})
		readers = append(readers, a.Data)
	}
	return Write(dst, m, readers, limit)
}

// Validate the full inventory before a producer creates any transport resource.
func snapshotComponents(saved core.Snapshot) ([]core.SnapshotComponent, error) {
	if saved.State != "ready" || !core.ValidEnvironmentAttachments(saved.Source.Environment.Attachments) || !core.ValidEnvironmentInstanceID(saved.Source.InstanceID) || !sourceName.MatchString(saved.Source.Environment.Name) ||
		len(saved.Components) < 2 || len(saved.Components) > maxComponents+1 {
		return nil, ErrInvalidBundle
	}
	var root, oci *core.SnapshotComponent
	var work []core.SnapshotComponent
	data := map[string]core.SnapshotComponent{}
	baseSeen := false
	refs := map[string]bool{}
	roles := map[string]bool{}
	for _, c := range saved.Components {
		if c.State != "verified" || c.Binding == "" || c.Owner == "" || c.NativeRef == "" || refs[c.NativeRef] || roles[c.Role] {
			return nil, ErrInvalidBundle
		}
		refs[c.NativeRef], roles[c.Role] = true, true
		switch {
		case c.Role == "rootfs":
			value := c
			root = &value
		case c.Role == "oci":
			value := c
			oci = &value
		case c.Role == "base":
			if baseSeen {
				return nil, ErrInvalidBundle
			}
			baseSeen = true
		case strings.HasPrefix(c.Role, "workspace:") && len(c.Role) > len("workspace:"):
			work = append(work, c)
		case strings.HasPrefix(c.Role, "data:"):
			data[strings.TrimPrefix(c.Role, "data:")] = c
		default:
			return nil, ErrInvalidBundle
		}
	}
	if root == nil || len(work) == 0 || len(work) > maxWorkspaces || (oci != nil) != (saved.Source.Environment.PersistentResource.ID != "") {
		return nil, ErrInvalidBundle
	}
	sort.Slice(work, func(i, j int) bool { return work[i].Role < work[j].Role })
	ordered := append([]core.SnapshotComponent{*root}, work...)
	if oci != nil {
		ordered = append(ordered, *oci)
	}
	if len(data) != len(saved.Source.Environment.Attachments) {
		return nil, ErrInvalidBundle
	}
	for _, a := range saved.Source.Environment.Attachments {
		c, ok := data[a.Key]
		if !ok || a.Origin.Kind != "build-cache" {
			return nil, ErrInvalidBundle
		}
		ordered = append(ordered, c)
	}
	return ordered, nil
}
