package gitrepo

import (
	"context"
	"fmt"
	"strings"
)

const maxHaves = 32

func validHaves(operation string, haves []string) bool {
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

func helperPushPack(ctx context.Context, next, old string) ([]byte, error) {
	revisions := next + "\n"
	// The trusted agent fetches and rechecks this exact old target before
	// importing the pack. New branches retain a complete pack for now.
	if old != ZeroOID {
		if _, err := helperGit(ctx, nil, "cat-file", "-e", old+"^{commit}"); err == nil {
			revisions += "^" + old + "\n"
		}
	}
	return helperGit(ctx, []byte(revisions), "pack-objects", "--stdout", "--revs")
}
