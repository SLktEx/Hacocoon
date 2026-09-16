package gitadapter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestHeadValidationBoundsAndNames(t *testing.T) {
	oid := strings.Repeat("a", 40)
	for _, ref := range []string{"refs/heads/main", "refs/heads/topic/v1.2", "refs/heads/日本語", "refs/heads/HEAD"} {
		if _, err := ValidateHeads([]Head{{Ref: ref, OID: oid}}); err != nil {
			t.Fatal(ref, err)
		}
	}
	for _, ref := range []string{"main", "refs/tags/v1", "refs/heads/*", "refs/heads/../main", "refs/heads/a.lock", "refs/heads/.hidden", "refs/heads/a\\b", "refs/heads/a\nb", "refs/heads/a:b", "refs/heads/a@{1}", "refs/heads/a//b", "refs/heads/end.", "refs/heads/" + strings.Repeat("x", 1024), "refs/heads/\xff"} {
		if _, err := ValidateHeads([]Head{{Ref: ref, OID: oid}}); err == nil {
			t.Fatal("unsafe head accepted", ref)
		}
	}
	for _, heads := range [][]Head{nil, {{Ref: "refs/heads/main", OID: "-evil"}}, {{Ref: "refs/heads/main", OID: oid}, {Ref: "refs/heads/main", OID: oid}}, make([]Head, MaxHeads+1)} {
		if _, err := ValidateHeads(heads); err == nil {
			t.Fatal("invalid heads accepted")
		}
	}
}

func TestHelperRejectsUnlistedOrDuplicateFetchBeforeBroker(t *testing.T) {
	oid := strings.Repeat("a", 40)
	for _, batch := range []string{"fetch " + oid + " refs/heads/other\n", "fetch " + oid + " refs/heads/main\nfetch " + oid + " refs/heads/main\n", "fetch " + strings.Repeat("b", 40) + " refs/heads/main\n"} {
		calls := 0
		exchange := func(_ context.Context, req Request) (Response, error) {
			calls++
			if req.Operation != "list" {
				t.Fatal("invalid fetch reached broker")
			}
			return Response{OID: oid, Ref: "refs/heads/main", Heads: []Head{{Ref: "refs/heads/main", OID: oid}}}, nil
		}
		var out bytes.Buffer
		if err := Helper(context.Background(), []string{"origin", "haco://demo"}, strings.NewReader("list\n"+batch+"\n"), &out, &out, exchange); err == nil || calls != 1 {
			t.Fatal("invalid helper batch accepted", err, calls)
		}
	}
}

func TestReadHeadsRejectsExcessAndNonCommitBeforePack(t *testing.T) {
	for _, mode := range []string{"excess", "tree"} {
		git := func(_ []byte, args ...string) ([]byte, error) {
			switch args[0] {
			case "fetch":
				return nil, nil
			case "ls-remote":
				count := 1
				if mode == "excess" {
					count = MaxHeads + 1
				}
				lines := strings.Repeat("a", 40) + "\trefs/heads/main\n"
				for i := 1; i < count; i++ {
					lines += fmt.Sprintf("%s\trefs/heads/b%d\n", strings.Repeat("a", 40), i)
				}
				return []byte(lines), nil
			case "cat-file":
				return []byte("tree\n"), nil
			case "rev-parse":
				return []byte(strings.Repeat("a", 40) + "\n"), nil
			default:
				t.Fatal("invalid remote reached pack generation", args)
				return nil, nil
			}
		}
		req := AgentRequest{Operation: "list", Branch: "main"}
		if mode == "tree" {
			req.Operation = "fetch"
			req.PackOutput = io.Discard
			req.Heads = []Head{{Ref: "refs/heads/main", OID: strings.Repeat("a", 40)}}
		}
		if _, err := readHeads(git, req, func([]byte) (int64, error) { t.Fatal("invalid remote reached pack"); return 0, nil }); err == nil {
			t.Fatal("invalid remote accepted", mode)
		}
	}
}
