package gitadapter

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxHeads = 1024
const AllHeadsRef = "refs/heads/*"
const readCachePrefix = "refs/hacocoon/read-heads/"

type Head struct {
	Ref string `json:"ref"`
	OID string `json:"oid"`
}

// ValidHeadRef accepts full head names as data, never refspec patterns or command options.
func ValidHeadRef(ref string) bool {
	if len(ref) > 1024 || !utf8.ValidString(ref) || !strings.HasPrefix(ref, "refs/heads/") {
		return false
	}
	name := strings.TrimPrefix(ref, "refs/heads/")
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "@{") || strings.HasSuffix(name, ".") {
		return false
	}
	for _, c := range name {
		if unicode.IsControl(c) || strings.ContainsRune(" ~^:?*[\\", c) {
			return false
		}
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}

func ValidateHeads(heads []Head) (map[string]string, error) {
	if len(heads) == 0 || len(heads) > MaxHeads {
		return nil, fmt.Errorf("git head count exceeds supported range")
	}
	result := make(map[string]string, len(heads))
	for _, head := range heads {
		if !ValidHeadRef(head.Ref) || !ValidOID(head.OID) || head.OID == ZeroOID || result[head.Ref] != "" {
			return nil, fmt.Errorf("invalid or duplicate Git head")
		}
		result[head.Ref] = head.OID
	}
	return result, nil
}

func readHeads(git func([]byte, ...string) ([]byte, error), req AgentRequest, pack func([]byte) (int64, error)) (Response, error) {
	if req.Operation == "fetch" {
		return fetchHead(git, req, pack)
	}
	return remoteHeads(git, req.Remote)
}

// remoteHeads uses Git's ls-remote --symref records. HEAD is a current remote
// observation, never a property of the registered repository. Missing/dangling
// HEAD does not prevent discovery and explicit use of the available branches.
func remoteHeads(git func([]byte, ...string) ([]byte, error), remote string) (Response, error) {
	data, err := git(nil, "ls-remote", "--symref", "--", remote, "HEAD", "refs/heads/*")
	if err != nil {
		return Response{}, err
	}
	result := Response{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		value, ref, ok := strings.Cut(line, "\t")
		if !ok {
			return Response{}, fmt.Errorf("invalid remote head listing")
		}
		if ref == "HEAD" {
			if target, symbolic := strings.CutPrefix(value, "ref: "); symbolic {
				if result.Ref != "" || !ValidHeadRef(target) {
					return Response{}, fmt.Errorf("invalid remote HEAD")
				}
				result.Ref = target
			} else if !ValidOID(value) || value == ZeroOID {
				return Response{}, fmt.Errorf("invalid remote HEAD object")
			}
			continue
		}
		// ls-remote patterns also match ref tails, so HEAD can include a tag
		// or another namespace ending in /HEAD. Only heads enter this API.
		if !strings.HasPrefix(ref, "refs/heads/") {
			if !ValidOID(value) || value == ZeroOID {
				return Response{}, fmt.Errorf("invalid remote ref listing")
			}
			continue
		}
		result.Heads = append(result.Heads, Head{Ref: ref, OID: value})
	}
	current, err := ValidateHeads(result.Heads)
	if err != nil {
		return Response{}, err
	}
	result.OID = current[result.Ref]
	if result.OID == "" {
		result.Ref = ""
	}
	return result, nil
}

func fetchHead(git func([]byte, ...string) ([]byte, error), req AgentRequest, pack func([]byte) (int64, error)) (Response, error) {
	if _, err := ValidateHeads(req.Heads); err != nil || len(req.Heads) != 1 {
		return Response{}, fmt.Errorf("fetch requires one authorized head")
	}
	head := req.Heads[0]
	tracking := readCachePrefix + strings.TrimPrefix(head.Ref, "refs/heads/")
	if _, err := git(nil, "fetch", "--no-tags", "--no-recurse-submodules", "--", req.Remote, "+"+head.Ref+":"+tracking); err != nil {
		return Response{}, fmt.Errorf("trusted remote fetch failed; check registration and Host authentication")
	}
	value, err := git(nil, "rev-parse", "--verify", tracking)
	if err != nil || strings.TrimSpace(string(value)) != head.OID {
		return Response{}, fmt.Errorf("remote changed since listing; fetch again")
	}
	kind, err := git(nil, "cat-file", "-t", head.OID)
	if err != nil || string(kind) != "commit\n" {
		return Response{}, fmt.Errorf("remote head is not a SHA-1 commit")
	}
	result := Response{OID: head.OID, Ref: head.Ref}
	revisions := head.OID + "\n"
	for _, have := range req.Haves {
		// A hint never grants access to another ref or hidden Host object.
		// Ignore missing/non-ancestor hints without exposing their presence.
		if _, err := git(nil, "merge-base", "--is-ancestor", have, head.OID); err == nil {
			revisions += "^" + have + "\n"
		}
	}
	result.PackBytes, err = pack([]byte(revisions))
	return result, err
}
