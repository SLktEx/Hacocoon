package cli

import (
	"bytes"
	"context"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/git"
	"strings"
	"testing"
)

type sourceClientFake struct {
	all   controlapi.RepositoryManageResponse
	calls []controlapi.RepositoryManageRequest
	err   error
}

func (c *sourceClientFake) RepositoryManage(_ context.Context, r controlapi.RepositoryManageRequest) (controlapi.RepositoryManageResponse, error) {
	c.calls = append(c.calls, r)
	if r.Operation == "delete" {
		return controlapi.RepositoryManageResponse{}, c.err
	}
	return c.all, nil
}
func TestSourceCommandDeletionAndForceRecovery(t *testing.T) {
	for _, mode := range []string{"yes", "no", "eof", "oversize", "busy", "creating", "force-busy", "force-creating", "force-long", "duplicate", "missing", "failure", "force-failure", "json"} {
		t.Run(mode, func(t *testing.T) {
			o := gitrepo.Object{Kind: "repo", ID: "source", Owner: strings.Repeat("a", 32), State: "ready"}
			use := gitrepo.SourceUse{Source: o}
			input := "yes\n"
			args := []string{"delete", "source"}
			wantDelete := true
			wantForce := false
			wantCode := 0
			switch mode {
			case "no":
				input, wantDelete, wantCode = "no\n", false, 1
			case "eof":
				input, wantDelete, wantCode = "yes", false, 1
			case "oversize":
				input, wantDelete, wantCode = strings.Repeat(" ", 128)+"yes\n", false, 1
			case "busy":
				use.Workspaces = []string{"work"}
			case "creating":
				use.Source.State = "creating"
			case "force-busy":
				use.Workspaces = []string{"work"}
				args, input, wantForce = []string{"delete", "-f", "source"}, "", true
			case "force-creating":
				use.Source.State = "creating"
				args, input, wantForce = []string{"delete", "-f", "source"}, "", true
			case "force-long":
				args, input, wantForce = []string{"delete", "--force", "source"}, "", true
			case "duplicate":
				wantDelete, wantCode = false, 1
			case "missing":
				wantDelete, wantCode = false, 1
			case "failure":
				args, wantCode = []string{"delete", "--yes", "source"}, 1
			case "force-failure":
				args, input, wantForce, wantCode = []string{"delete", "-f", "source"}, "", true, 1
			case "json":
				args, input, wantDelete, wantCode = []string{"list", "--json"}, "", false, 0
			}
			c := &sourceClientFake{all: controlapi.RepositoryManageResponse{Sources: []gitrepo.SourceUse{use}}}
			if mode == "duplicate" {
				c.all.Sources = append(c.all.Sources, use)
			}
			if mode == "missing" {
				c.all.Sources = nil
			}
			if mode == "failure" || mode == "force-failure" {
				c.err = core.ErrRecoveryRequired
			}
			var out, diagnostic bytes.Buffer
			code := sourceManageCommand(context.Background(), c, args, strings.NewReader(input), &out, &diagnostic)
			var deletes []controlapi.RepositoryManageRequest
			for _, call := range c.calls {
				if call.Operation == "delete" {
					deletes = append(deletes, call)
				}
			}
			if (len(deletes) == 1) != wantDelete {
				t.Fatalf("calls=%+v code=%d diagnostic=%s", c.calls, code, diagnostic.String())
			}
			if code != wantCode {
				t.Fatalf("code=%d want=%d diagnostic=%s", code, wantCode, diagnostic.String())
			}
			if wantDelete {
				if deletes[0].Force != wantForce {
					t.Fatalf("force=%t want=%t request=%+v", deletes[0].Force, wantForce, deletes[0])
				}
				if wantForce {
					if deletes[0].Owner != "" || len(c.calls) != 1 {
						t.Fatalf("forced cleanup performed review/list round trip: %+v", c.calls)
					}
				} else if deletes[0].Owner != o.Owner || len(c.calls) != 2 {
					t.Fatalf("reviewed owner/list lost: %+v", c.calls)
				}
			}
			if (mode == "failure" || mode == "force-failure") && strings.Contains(out.String(), "Source repository deleted") {
				t.Fatal("failure reported successful")
			}
		})
	}
}
