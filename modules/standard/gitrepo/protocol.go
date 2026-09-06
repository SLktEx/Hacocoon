// Package gitrepo supplies the bounded Git remote-helper integration. Git and
// credentials live in the trusted logical Host, outside untrusted Workspaces.
package gitrepo

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	MaxPack        = 32 << 20
	MaxMessage     = 48 << 20
	GuestSocket    = "/var/lib/hacocoon-git.sock"
	RepositoryRoot = "/var/lib/hacocoon-repos"
	WorkspaceRoot  = "/var/lib/hacocoon-workspaces"
	Capability     = "git.repository"
)

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)
var branchPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_/-]{0,127}$`)

func ValidID(s string) bool { return idPattern.MatchString(s) }
func ValidBranch(s string) bool {
	return branchPattern.MatchString(s) && !strings.Contains(s, "//") && !strings.HasSuffix(s, "/")
}
func ValidOID(s string) bool {
	if len(s) != 40 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && s == strings.ToLower(s)
}

// Remote URLs are registered by a trusted client, never supplied by a guest.
// File remotes permit an ordinary, credential-free local acceptance repository.
func ValidateRemote(s string) error {
	u, err := url.Parse(s)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || strings.ContainsAny(s, "\r\n\x00") {
		return fmt.Errorf("invalid credential-free remote URL")
	}
	if u.Scheme == "https" && u.Host == "github.com" {
		parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git"), "/")
		slug := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
		if len(parts) == 2 && slug.MatchString(parts[0]) && slug.MatchString(parts[1]) {
			return nil
		}
	}
	if u.Scheme == "file" && u.Host == "" && strings.HasPrefix(u.Path, "/") && !strings.Contains(u.Path, "/../") {
		return nil
	}
	return fmt.Errorf("PoC supports credential-free github.com HTTPS or trusted-host file URLs")
}

type Request struct {
	Operation  string `json:"operation"`
	Repository string `json:"repository"`
	Ref        string `json:"ref,omitempty"`
	OldOID     string `json:"old_oid,omitempty"`
	NewOID     string `json:"new_oid,omitempty"`
	Pack       []byte `json:"pack,omitempty"`
}

type Response struct {
	OID     string `json:"oid,omitempty"`
	Ref     string `json:"ref,omitempty"`
	Pack    []byte `json:"pack,omitempty"`
	Summary string `json:"summary,omitempty"`
	Error   string `json:"error,omitempty"`
}

// AgentRequest is sent only from the controller to the verified trusted Host.
// It is a separate type so guest requests cannot smuggle paths or upstreams.
type AgentRequest struct {
	Operation  string `json:"operation"`
	Repository string `json:"repository"`
	Workspace  string `json:"workspace,omitempty"`
	Remote     string `json:"remote"`
	Branch     string `json:"branch"`
	OldOID     string `json:"old_oid,omitempty"`
	NewOID     string `json:"new_oid,omitempty"`
	Pack       []byte `json:"pack,omitempty"`
}
