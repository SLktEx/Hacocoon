package gitcap

// remote_url is transport metadata captured by the broker for compatibility.
// Provider execution re-resolves the remote from the workspace and never uses
// this opaque value to choose authority, scope, target, or credentials.
func (*Provider) NonAuthorityParameters() []string {
	return []string{"remote_url"}
}
