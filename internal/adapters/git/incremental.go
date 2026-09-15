package gitadapter

import (
	"context"
	"fmt"
	"io"
	"strings"
)

const maxHaves = 32

// ValidHaves bounds untrusted history hints shared by the broker and native agent.
func ValidHaves(operation string, haves []string) bool {
	if len(haves) > maxHaves || (operation != "fetch" && len(haves) != 0) {
		return false
	}
	seen := make(map[string]bool, len(haves))
	for _, oid := range haves {
		if !ValidOID(oid) || oid == ZeroOID || seen[oid] {
			return false
		}
		seen[oid] = true
	}
	return true
}

func helperHaves(ctx context.Context) ([]string, error) {
	// These are guest-side optimization hints, never authority. Enumerate only
	// a bounded set of local branch tips.
	data, err := helperGit(ctx, nil, "for-each-ref", "--count=32", "--format=%(objectname)", "refs/heads/", "refs/remotes/")
	if err != nil {
		return nil, err
	}
	var haves []string
	seen := make(map[string]bool)
	for _, oid := range strings.Fields(string(data)) {
		if !ValidOID(oid) || oid == ZeroOID {
			return nil, fmt.Errorf("invalid local Git branch tip")
		}
		if !seen[oid] {
			haves = append(haves, oid)
			seen[oid] = true
		}
	}
	return haves, nil
}

func helperPushPack(ctx context.Context, next, old string, output io.Writer) error {
	revisions := next + "\n"
	// Existing targets are fetched during preparation. A new branch's basis
	// must first pass the separate exact-ref fetch in helperNewBranchBasis.
	if old != ZeroOID {
		if _, err := helperGit(ctx, nil, "cat-file", "-e", old+"^{commit}"); err == nil {
			revisions += "^" + old + "\n"
		}
	}
	_, err := runPack(helperCommand(ctx, "pack-objects", "--stdout", "--revs"), strings.NewReader(revisions), output)
	return err
}

// helperNewBranchBasis reuses one advertised ancestor through the ordinary
// exact-ref read path. It never changes the expected-absent push target or
// converts read permission into push permission.
func helperNewBranchBasis(ctx context.Context, repo, next string, listed Response, exchange Exchange) (string, error) {
	heads, err := ValidateHeads(listed.Heads)
	if err != nil || !ValidOID(next) || next == ZeroOID || heads[listed.Ref] != listed.OID {
		return "", fmt.Errorf("invalid new-branch history selection")
	}
	// Prefer the registered checkout head, then a bounded number of alternatives.
	candidates := []Head{{Ref: listed.Ref, OID: listed.OID}}
	for _, head := range listed.Heads {
		if head.Ref != listed.Ref && len(candidates) < maxHaves {
			candidates = append(candidates, head)
		}
	}
	for _, head := range candidates {
		if _, err := helperGit(ctx, nil, "merge-base", "--is-ancestor", head.OID, next); err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			continue
		}
		// The Host fetches only this freshly authorized ref and checks its exact
		// OID. Supplying the same have yields an empty pack; no history is sent
		// back to the guest. The Host's read-cache ref retains the basis objects.
		response, err := exchange(ctx, Request{Operation: "fetch", Repository: repo, Heads: []Head{head}, Haves: []string{head.OID}, PackOutput: io.Discard})
		if err != nil {
			return "", err // A denied or moved ref never triggers another read/push.
		}
		if response.Error != "" || response.Ref != head.Ref || response.OID != head.OID || response.PackBytes <= 0 || response.PackBytes > maxTransferBytes {
			return "", fmt.Errorf("invalid new-branch history confirmation")
		}
		return head.OID, nil
	}
	return ZeroOID, nil // No advertised ancestor: retain the bounded complete pack.
}
