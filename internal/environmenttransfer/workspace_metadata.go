package environmenttransfer

import "github.com/SLktEx/Hacocoon/modules/standard/gitrepo"

// Workspace binds transport order to portable repository routing metadata. It is
// never a destination identity, credential, grant or permission to contact a URL.
// In particular, file remotes refer to the source Host, not the destination Host.
type Workspace struct {
	Role   string `json:"role"`
	Name   string `json:"name"`
	Remote string `json:"remote"`
	Branch string `json:"branch"`
}

func (m Manifest) validateWorkspaces(count int) error {
	if m.Version == 1 {
		if len(m.Workspaces) != 0 {
			return ErrInvalidBundle
		}
		return nil
	}
	if len(m.Workspaces) != count {
		return ErrInvalidBundle
	}
	names := map[string]bool{}
	for i, w := range m.Workspaces {
		if w.Role != workspaceRole(i) || !gitrepo.ValidID(w.Name) || names[w.Name] || len(w.Remote) > 4096 {
			return ErrInvalidBundle
		}
		names[w.Name] = true
		// Legacy saved objects may lack routing. Preserve that absence; do not
		// infer management routing from the guest's mutable .git/config.
		if w.Remote == "" && w.Branch == "" {
			continue
		}
		if gitrepo.ValidateRemote(w.Remote) != nil || !gitrepo.ValidBranch(w.Branch) {
			return ErrInvalidBundle
		}
	}
	return nil
}
