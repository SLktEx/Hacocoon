package gitrepo

import (
	"strings"
	"testing"
)

func TestPushReceiptRequiresExactConfirmedMutation(t *testing.T) {
	next, old := strings.Repeat("a", 40), strings.Repeat("b", 40)
	ref := "refs/heads/feature/work"
	for _, tc := range []struct {
		name, before, receipt string
		want                  bool
	}{
		{"created", ZeroOID, "*\t" + next + ":" + ref + "\t[new branch]\n", true},
		{"updated", old, " \t" + next + ":" + ref + "\told..new\n", true},
		{"unchanged", next, "=\t" + next + ":" + ref + "\t[up to date]\n", true},
		{"competing identical creation", ZeroOID, "=\t" + next + ":" + ref + "\t[up to date]\n", false},
		{"forced", old, "+\t" + next + ":" + ref + "\tforced\n", false},
		{"wrong ref", ZeroOID, "*\t" + next + ":refs/heads/main\t[new branch]\n", false},
		{"wrong commit", ZeroOID, "*\t" + old + ":" + ref + "\t[new branch]\n", false},
		{"missing", ZeroOID, "Done\n", false},
		{"duplicate", ZeroOID, strings.Repeat("*\t"+next+":"+ref+"\t[new branch]\n", 2), false},
		{"malformed", ZeroOID, "*\t" + next + ":" + ref + "\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := confirmedPush([]byte("To remote\n"+tc.receipt+"Done\n"), AgentRequest{Ref: ref, OldOID: tc.before, NewOID: next}); got != tc.want {
				t.Fatalf("confirmed=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestPushRejectsMalformedTargetsBeforeRunningGit(t *testing.T) {
	good := AgentRequest{Operation: "prepare", Ref: "refs/heads/feature/work", OldOID: ZeroOID, NewOID: strings.Repeat("a", 40), Pack: []byte("pack")}
	for _, change := range []func(*AgentRequest){
		func(r *AgentRequest) { r.Ref = "--upload-pack=evil" },
		func(r *AgentRequest) { r.Ref = "refs/tags/release" },
		func(r *AgentRequest) { r.Ref = "refs/heads/../main" },
		func(r *AgentRequest) { r.Ref = "refs/heads/main\nrefs/heads/other" },
		func(r *AgentRequest) { r.NewOID = ZeroOID },
		func(r *AgentRequest) { r.OldOID = "" },
		func(r *AgentRequest) { r.Operation = "push" },
	} {
		req := good
		change(&req)
		_, err := pushOperation(func([]byte, ...string) ([]byte, error) { t.Fatal("invalid request reached Git"); return nil, nil }, req)
		if err == nil {
			t.Fatalf("accepted %+v", req)
		}
	}
	if _, err := validateHeads([]Head{{Ref: good.Ref, OID: ZeroOID}}); err == nil {
		t.Fatal("accepted nonexistent advertised head")
	}
}
