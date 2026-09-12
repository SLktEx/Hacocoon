package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"testing"
)

func TestGitSavedChoiceRejectsOldPeerAndMissingReceipt(t *testing.T) {
	for _, supported := range []bool{false, true} {
		calls := 0
		path := doctorTestSocket(t, func(s *control.Server) {
			_ = s.Register(MethodGitPending, func(context.Context, json.RawMessage) (any, error) {
				proposal := gitrepo.Proposal{ID: "pending"}
				if supported {
					proposal.SavedScope = &core.CapabilityRequest{Capability: "git.repository", Action: "push", Resource: "target"}
				}
				return []gitrepo.Proposal{proposal}, nil
			})
			_ = s.Register(MethodGitDecide, func(context.Context, json.RawMessage) (any, error) { calls++; return core.CapabilityResult{}, nil })
		})
		client, err := NewClient(path)
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.DecideGitWithSavedChoice(context.Background(), "pending", true, capabilityapp.AllowEnvironment)
		if !supported && (!errors.Is(err, core.ErrUnsupported) || calls != 0) {
			t.Fatalf("old peer received save: %d %v", calls, err)
		}
		if supported && (!errors.Is(err, core.ErrIncompatibleState) || calls != 1) {
			t.Fatalf("missing receipt accepted: %d %v", calls, err)
		}
	}
}
