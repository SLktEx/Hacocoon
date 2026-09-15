package core

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestTemporaryWorkspaceAndEnvironmentCreationHaveIndependentIdentities(t *testing.T) {
	seen := map[string]bool{}
	for range 16 {
		work, err := NewTemporaryWorkspace()
		if err != nil || !ValidTemporaryWorkspace(work) || !IsTemporaryWorkspacePath(work.Path) || seen[work.Path] {
			t.Fatal("temporary workspace identity is not fresh and self-contained", work, err)
		}
		seen[work.Path] = true
		instance, err := NewEnvironmentInstanceID()
		if err != nil || !ValidEnvironmentInstanceID(instance) || seen[instance] {
			t.Fatal("Environment creation identity was reused", instance, err)
		}
		seen[instance] = true
		for _, invalid := range []Workspace{
			{ID: work.ID, Path: "/tmp/" + strings.TrimPrefix(work.Path, "temporary:")},
			{ID: "workspace:other", Path: work.Path},
			{ID: work.ID, Path: work.Path + "/child"},
			{ID: work.ID, Path: "temporary:" + instance},
		} {
			if ValidTemporaryWorkspace(invalid) {
				t.Fatal("foreign path or owner accepted as a temporary workspace", invalid)
			}
		}
	}
	for _, invalid := range []string{"builder", "env-", "env-" + strings.Repeat("a", 31), "env-" + strings.Repeat("a", 32) + "/child", "env-" + strings.Repeat("A", 32)} {
		if ValidEnvironmentInstanceID(invalid) {
			t.Fatal("human name or noncanonical generation accepted", invalid)
		}
	}
}

func TestWorkspaceLeaseRoundTripPreservesActiveOwnershipAndAttachments(t *testing.T) {
	resource := PersistentResource{ID: "oci:retained", Owner: strings.Repeat("a", 32), Kind: "oci-containerd", State: "ready"}
	lease := WorkspaceLease{EnvironmentID: "builder", InstanceID: "env-" + strings.Repeat("b", 32), Owner: "builder", WorkspaceID: "workspace:retained", SourcePath: "managed:retained", RuntimeRef: "incus:builder", State: WorkspaceLeaseActive, AccessMode: WorkspaceReadWrite, PersistentResource: resource.Ref(), Attachments: []EnvironmentAttachment{validEnvironmentAttachment()}}
	environment := Environment{Name: lease.EnvironmentID, Workspace: Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath}, RuntimeRef: lease.RuntimeRef, AccessMode: lease.AccessMode, PersistentResource: resource.Ref(), Attachments: slices.Clone(lease.Attachments)}
	encoded, err := json.Marshal(lease)
	if err != nil {
		t.Fatal(err)
	}
	var decoded WorkspaceLease
	if err := json.Unmarshal(encoded, &decoded); err != nil || !lease.Equal(decoded) || !decoded.MatchesEnvironment(environment) {
		t.Fatal("persisted lease did not retain its owned Environment", decoded, err)
	}
	for _, change := range []func(*WorkspaceLease){
		func(l *WorkspaceLease) { l.Owner = "another-owner" },
		func(l *WorkspaceLease) { l.InstanceID = "env-" + strings.Repeat("c", 32) },
		func(l *WorkspaceLease) { l.Attachments[0].Resource.Owner = strings.Repeat("c", 32) },
	} {
		other := decoded
		other.Attachments = slices.Clone(decoded.Attachments)
		change(&other)
		if lease.Equal(other) {
			t.Fatal("changed ownership compared equal after decoding", other)
		}
	}
	for _, change := range []func(*WorkspaceLease){
		func(l *WorkspaceLease) { l.State = WorkspaceLeaseCleanupRequired },
		func(l *WorkspaceLease) { l.RuntimeAbsent = true },
		func(l *WorkspaceLease) { l.RuntimeRef = "incus:other" },
		func(l *WorkspaceLease) { l.WorkspaceID = "workspace:other" },
		func(l *WorkspaceLease) { l.Attachments = nil },
		func(l *WorkspaceLease) { l.PersistentResource.Owner = strings.Repeat("c", 32) },
	} {
		other := decoded
		change(&other)
		if other.MatchesEnvironment(environment) {
			t.Fatal("inactive or foreign lease matched the Environment", other)
		}
	}
	if !lease.Equal(decoded) || !lease.MatchesEnvironment(environment) {
		t.Fatal("comparison mutated the original ownership values")
	}
}

func TestTerminalMetadataIsScopedToOneExecutionContext(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	resize := make(chan TerminalSize, 1)
	first := TerminalMetadata{Term: "xterm-256color", ColorTerm: "truecolor", Columns: 120, Rows: 40, DisplayLanguage: "ja", Resizes: resize}
	second := TerminalMetadata{Term: "dumb", Columns: 80, Rows: 24}
	one := WithTerminalMetadata(parent, first)
	two := WithTerminalMetadata(parent, second)
	if TerminalMetadataFromContext(one) != first || TerminalMetadataFromContext(two) != second || TerminalMetadataFromContext(parent) != (TerminalMetadata{}) {
		t.Fatal("metadata escaped its execution context")
	}
	resize <- TerminalSize{Columns: 90, Rows: 30}
	if got := <-TerminalMetadataFromContext(one).Resizes; got != (TerminalSize{Columns: 90, Rows: 30}) || TerminalMetadataFromContext(two).Resizes != nil {
		t.Fatal("resize update was routed to a different execution")
	}
	cancel()
	if one.Err() != context.Canceled || two.Err() != context.Canceled {
		t.Fatal("metadata wrapping lost execution cancellation")
	}
}

func TestDNSModeDefaultsToHostWithoutNormalizingUnknownPolicies(t *testing.T) {
	for _, test := range []struct {
		mode, effective DNSMode
		valid           bool
	}{
		{"", DNSHost, true}, {DNSHost, DNSHost, true}, {DNSBackend, DNSBackend, true}, {DNSDisabled, DNSDisabled, true},
		{"HOST", "HOST", false}, {"disabled ", "disabled ", false}, {"automatic", "automatic", false},
	} {
		if test.mode.Effective() != test.effective || test.mode.Valid() != test.valid {
			t.Fatal("DNS selection silently changed an explicit policy", test)
		}
	}
}
