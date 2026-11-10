package gitrepo

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type metadataGitRequester func(context.Context, core.CapabilityRequest) (core.CapabilityResult, error)

func (f metadataGitRequester) RequestWithApproval(ctx context.Context, request core.CapabilityRequest, _ func(context.Context, core.ApprovalRequest) (bool, error)) (core.CapabilityResult, error) {
	return f(ctx, request)
}

func TestPolicyDescriptorMatchesPreparedGitRequestsWithoutExecution(t *testing.T) {
	descriptor := (*Broker)(nil).PolicyDescriptor()
	if descriptor.Capability != (*Broker)(nil).Capability() || len(descriptor.Actions) != 2 {
		t.Fatalf("unexpected descriptor: %#v", descriptor)
	}
	for _, tc := range []struct {
		name, action, ref, old, next string
	}{
		{name: "discovery", action: "fetch", ref: gitadapter.AllHeadsRef},
		{name: "fetch", action: "fetch", ref: "refs/heads/main", next: strings.Repeat("b", 40)},
		{name: "push-existing", action: "push", ref: "refs/heads/main", old: strings.Repeat("a", 40), next: strings.Repeat("b", 40)},
		{name: "push-create", action: "push", ref: "refs/heads/topic", old: gitadapter.ZeroOID, next: strings.Repeat("b", 40)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			broker := NewBroker(nil, nil, "")
			called := false
			broker.Capabilities = metadataGitRequester(func(ctx context.Context, request core.CapabilityRequest) (core.CapabilityResult, error) {
				called = true
				if request.Capability != descriptor.Capability || request.Resource != "https://github.com/example/project.git" || request.Action != tc.action {
					t.Fatalf("unexpected prepared authority: %#v", request)
				}
				matched := false
				for _, action := range descriptor.Actions {
					if action.Action != request.Action {
						continue
					}
					matched = true
					var names, keys []string
					for _, attribute := range action.Attributes {
						names = append(names, attribute.Name)
						if len(attribute.Values) != 0 && !slices.Contains(attribute.Values, request.Attributes[attribute.Name]) {
							t.Fatalf("request value missing from %s enum", attribute.Name)
						}
					}
					for key := range request.Attributes {
						keys = append(keys, key)
					}
					sort.Strings(names)
					sort.Strings(keys)
					if !reflect.DeepEqual(names, keys) {
						t.Fatalf("metadata attributes %v differ from request attributes %v", names, keys)
					}
				}
				if !matched {
					t.Fatalf("prepared action missing from metadata: %s", request.Action)
				}
				scope, err := broker.SavedApprovalScope(ctx, request)
				if err != nil {
					t.Fatal(err)
				}
				for key, value := range request.Attributes {
					if key == "operation_id" || key == "old_oid" || key == "new_oid" {
						value = "*"
					}
					if scope.Attributes[key] != value {
						t.Fatalf("saved scope widening differs from metadata for %s", key)
					}
				}
				return core.CapabilityResult{}, core.ErrPolicyDenied
			})
			proposal := Proposal{Environment: "dev", Repository: "project", Remote: "https://github.com/example/project.git", Ref: tc.ref, Operation: tc.action, OldOID: tc.old, NewOID: tc.next}
			_, err := broker.perform(context.Background(), binding{}, proposal, func(context.Context) (gitadapter.Response, error) {
				t.Fatal("metadata parity test executed Git")
				return gitadapter.Response{}, nil
			})
			if !called || !errors.Is(err, core.ErrPolicyDenied) {
				t.Fatalf("request not captured without execution: called=%v error=%v", called, err)
			}
		})
	}
}

func TestPolicyDescriptorReturnsIndependentGitMetadata(t *testing.T) {
	first := (*Broker)(nil).PolicyDescriptor()
	first.Actions[0].Attributes[0].Description = "changed"
	first.Actions[0].Notes[0] = "changed"
	if first.Actions[1].Attributes[0].Description == "changed" || first.Actions[1].Notes[0] == "changed" {
		t.Fatal("Git actions share mutable metadata")
	}
	second := (*Broker)(nil).PolicyDescriptor()
	if second.Actions[0].Attributes[0].Description == "changed" || second.Actions[0].Notes[0] == "changed" {
		t.Fatal("descriptor calls share mutable metadata")
	}
}
