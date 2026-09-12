package gitrepo

import (
	"fmt"
	"strings"
)

// Preparation and external mutation receive separate Policy checks. The target
// is a controller-selected literal ref, not authority derived from checkout.
func pushOperation(git func([]byte, ...string) ([]byte, error), req AgentRequest) (Response, error) {
	if !validHeadRef(req.Ref) || !ValidOID(req.OldOID) || !ValidOID(req.NewOID) || req.NewOID == ZeroOID {
		return Response{}, fmt.Errorf("invalid push ref or OIDs")
	}
	if req.Operation == "push" {
		if len(req.Pack) != 0 {
			return Response{}, fmt.Errorf("approved push cannot supply another pack")
		}
		if err := validatePushCommits(git, req.OldOID, req.NewOID); err != nil {
			return Response{}, err
		}
		expected := req.OldOID
		if expected == ZeroOID {
			expected = ""
		}
		// Empty lease expectation requires the ref to remain absent. Concurrent
		// creation cannot be overwritten, even after a human approved this push.
		output, err := git(nil, "push", "--porcelain", "--no-verify", "--force-with-lease="+req.Ref+":"+expected, "--", req.Remote, req.NewOID+":"+req.Ref)
		if err != nil {
			return Response{}, fmt.Errorf("approved push failed or remote changed; inspect the remote before retrying")
		}
		if !confirmedPush(output, req) {
			return Response{}, fmt.Errorf("push result is unconfirmed; inspect the remote before retrying")
		}
		return Response{OID: req.NewOID, Ref: req.Ref}, nil
	}
	if req.Operation != "prepare" || len(req.Pack) == 0 {
		return Response{}, fmt.Errorf("invalid push preparation")
	}
	data, err := git(nil, "ls-remote", "--heads", "--", req.Remote, req.Ref)
	if err != nil {
		return Response{}, err
	}
	old := ZeroOID
	if len(data) != 0 {
		oid, ref, ok := strings.Cut(strings.TrimSuffix(string(data), "\n"), "\t")
		if !ok || ref != req.Ref || !ValidOID(oid) || oid == ZeroOID {
			return Response{}, fmt.Errorf("invalid remote push target")
		}
		old = oid
	}
	if old != req.OldOID {
		return Response{}, fmt.Errorf("push does not match the listed remote commit or absence")
	}
	if old != ZeroOID {
		tracking := readCachePrefix + strings.TrimPrefix(req.Ref, "refs/heads/")
		if _, err := git(nil, "fetch", "--no-tags", "--no-recurse-submodules", "--", req.Remote, "+"+req.Ref+":"+tracking); err != nil {
			return Response{}, err
		}
		value, err := git(nil, "rev-parse", "--verify", tracking+"^{commit}")
		if err != nil || strings.TrimSpace(string(value)) != old {
			return Response{}, fmt.Errorf("remote changed while preparing push")
		}
	}
	// Only object bytes are imported, never guest .git/config, hooks or remotes.
	if _, err := git(req.Pack, "index-pack", "--stdin", "--strict", "--max-input-size=33554432"); err != nil {
		return Response{}, fmt.Errorf("invalid Git object pack")
	}
	if err := validatePushCommits(git, old, req.NewOID); err != nil {
		return Response{}, err
	}
	base := old
	if old == ZeroOID {
		// Creation displays the complete new tree, not just its last commit.
		empty, err := git(nil, "hash-object", "-t", "tree", "-w", "--stdin")
		if err != nil {
			return Response{}, err
		}
		base = strings.TrimSpace(string(empty))
		if !ValidOID(base) {
			return Response{}, fmt.Errorf("invalid empty tree identity")
		}
	}
	summary, err := git(nil, "diff", "--no-ext-diff", "--no-textconv", "--stat", base, req.NewOID, "--")
	if err != nil {
		return Response{}, err
	}
	if len(summary) > 8192 {
		summary = summary[:8192]
	}
	return Response{OID: old, Ref: req.Ref, Summary: string(summary)}, nil
}

func confirmedPush(output []byte, req AgentRequest) bool {
	want := " "
	if req.OldOID == ZeroOID {
		want = "*"
	} else if req.OldOID == req.NewOID {
		want = "="
	}
	count := 0
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) == 1 {
			continue
		}
		if len(fields) != 3 || fields[0] != want || fields[1] != req.NewOID+":"+req.Ref {
			return false
		}
		count++
	}
	// Git can return success/up-to-date for a concurrently created identical
	// ref. That is not confirmation that our expected-absent creation happened.
	return count == 1
}

func validatePushCommits(git func([]byte, ...string) ([]byte, error), old, next string) error {
	kind, err := git(nil, "cat-file", "-t", next)
	if err != nil || string(kind) != "commit\n" {
		return fmt.Errorf("new OID is not an available commit")
	}
	if old != ZeroOID {
		if _, err := git(nil, "merge-base", "--is-ancestor", old, next); err != nil {
			return fmt.Errorf("non-fast-forward push is unsupported")
		}
	}
	return nil
}
