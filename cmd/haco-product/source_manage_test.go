package main

import (
	"bytes"
	"context"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
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
func TestSourceCommandReviewsIdentityAndRefusesUnsafeDeletion(t *testing.T) {
	for _, mode := range []string{"yes", "no", "eof", "oversize", "busy", "creating", "duplicate", "missing", "failure", "json"} {
		t.Run(mode, func(t *testing.T) {
			o := gitrepo.Object{Kind: "repo", ID: "source", Owner: strings.Repeat("a", 32), State: "ready"}
			use := gitrepo.SourceUse{Source: o}
			input := "yes\n"
			args := []string{"delete", "source"}
			switch mode {
			case "no":
				input = "no\n"
			case "eof":
				input = "yes"
			case "oversize":
				input = strings.Repeat(" ", 128) + "yes\n"
			case "busy":
				use.Workspaces = []string{"work"}
			case "creating":
				use.Source.State = "creating"
			case "failure":
				args = []string{"delete", "--yes", "source"}
			case "json":
				args = []string{"list", "--json"}
			}
			c := &sourceClientFake{all: controlapi.RepositoryManageResponse{Sources: []gitrepo.SourceUse{use}}}
			if mode == "duplicate" {
				c.all.Sources = append(c.all.Sources, use)
			}
			if mode == "missing" {
				c.all.Sources = nil
			}
			if mode == "failure" {
				c.err = core.ErrRecoveryRequired
			}
			var out, diagnostic bytes.Buffer
			code := sourceManageCommand(context.Background(), c, args, strings.NewReader(input), &out, &diagnostic)
			deleted := len(c.calls) == 2
			if deleted != (mode == "yes" || mode == "failure") {
				t.Fatal(c.calls, code, diagnostic.String())
			}
			if deleted && c.calls[1].Owner != o.Owner {
				t.Fatal("review owner lost")
			}
			if (code == 0) != (mode == "yes" || mode == "json") {
				t.Fatal(code, diagnostic.String())
			}
			if mode == "failure" && strings.Contains(out.String(), "Source repository deleted") {
				t.Fatal("failure reported successful")
			}
		})
	}
}
