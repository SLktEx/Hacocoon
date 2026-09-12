package gitrepo

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxHeads = 1024
const allHeadsRef = "refs/heads/*"
const readCachePrefix = "refs/hacocoon/read-heads/"

type Head struct {
	Ref string `json:"ref"`
	OID string `json:"oid"`
}

// Full head names are data, never refspec patterns or command options.
func validHeadRef(ref string) bool {
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

func validateHeads(heads []Head) (map[string]string, error) {
	if len(heads) == 0 || len(heads) > MaxHeads {
		return nil, fmt.Errorf("Git head count exceeds supported range")
	}
	result := make(map[string]string, len(heads))
	for _, head := range heads {
		if !validHeadRef(head.Ref) || !ValidOID(head.OID) || result[head.Ref] != "" {
			return nil, fmt.Errorf("invalid or duplicate Git head")
		}
		result[head.Ref] = head.OID
	}
	return result, nil
}

func readHeads(git func([]byte, ...string) ([]byte, error), req AgentRequest) (Response, error) {
	if req.Operation == "fetch" {
		return fetchHead(git, req)
	}
	// Discovery transfers names/OIDs only. No branch objects are fetched before
	// the broker's per-ref Policy check, even when all-head discovery is allowed.
	data, err := git(nil, "ls-remote", "--heads", "--", req.Remote)
	if err != nil {
		return Response{}, err
	}
	result := Response{Ref: "refs/heads/" + req.Branch}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		oid, ref, ok := strings.Cut(line, "\t")
		if !ok {
			return Response{}, fmt.Errorf("invalid remote head listing")
		}
		head := Head{Ref: ref, OID: oid}
		result.Heads = append(result.Heads, head)
	}
	current, err := validateHeads(result.Heads)
	if err != nil {
		return Response{}, err
	}
	result.OID = current[result.Ref]
	if result.OID == "" {
		return Response{}, fmt.Errorf("registered checkout branch is unavailable")
	}
	return result, nil
}

func fetchHead(git func([]byte, ...string) ([]byte, error), req AgentRequest) (Response, error) {
	if _, err := validateHeads(req.Heads); err != nil || len(req.Heads) != 1 {
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
	result.Pack, err = git([]byte(head.OID+"\n"), "pack-objects", "--stdout", "--revs")
	return result, err
}
