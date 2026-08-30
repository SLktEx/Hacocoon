package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAgentRemoteFolderURI(t *testing.T) {
	alias := "haco-agent-0123456789abcdef"
	want := "vscode-remote://ssh-remote+haco-agent-0123456789abcdef/workspace"
	if got := agentRemoteFolderURI(alias); got != want {
		t.Fatalf("folder URI = %q, want %q", got, want)
	}
}

func TestAgentsLaunchArgsTargetsRemoteWorkspace(t *testing.T) {
	folderURI := "vscode-remote://ssh-remote+haco-agent-abcd/workspace"
	want := []string{"--agents", "--folder-uri", folderURI}
	if got := agentsLaunchArgs(folderURI); !reflect.DeepEqual(got, want) {
		t.Fatalf("launch args = %#v, want %#v", got, want)
	}
}

func TestWriteAgentSessionDescriptorJSON(t *testing.T) {
	descriptor := agentSessionDescriptor{
		SessionID:       "session-sensitive",
		Environment:     "agent-0123456789abcdef0123",
		WorkspacePath:   "/home/user/worktrees/task-a",
		RemoteWorkspace: "/workspace",
		SSHAlias:        "haco-agent-0123456789abcdef",
		HostPort:        2222,
		FolderURI:       "vscode-remote://ssh-remote+haco-agent-0123456789abcdef/workspace",
	}
	var output bytes.Buffer
	if err := writeAgentSessionDescriptor(&output, descriptor, true); err != nil {
		t.Fatal(err)
	}
	var got agentSessionDescriptor
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("decode descriptor: %v\n%s", err, output.String())
	}
	if !reflect.DeepEqual(got, descriptor) {
		t.Fatalf("descriptor = %+v, want %+v", got, descriptor)
	}
}

func TestWriteAgentSessionDescriptorTextDoesNotExposeSessionID(t *testing.T) {
	descriptor := agentSessionDescriptor{
		SessionID:       "sensitive-session-id",
		Environment:     "agent-0123456789abcdef0123",
		WorkspacePath:   "/home/user/worktrees/task-a",
		RemoteWorkspace: "/workspace",
		SSHAlias:        "haco-agent-0123456789abcdef",
		FolderURI:       "vscode-remote://ssh-remote+haco-agent-0123456789abcdef/workspace",
	}
	var output bytes.Buffer
	if err := writeAgentSessionDescriptor(&output, descriptor, false); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"environment: agent-0123456789abcdef0123",
		"ssh: haco-agent-0123456789abcdef",
		"workspace: /workspace",
		"folder-uri: vscode-remote://ssh-remote+haco-agent-0123456789abcdef/workspace",
	} {
		if !strings.Contains(output.String(), required) {
			t.Fatalf("text descriptor missing %q: %s", required, output.String())
		}
	}
	if strings.Contains(output.String(), descriptor.SessionID) {
		t.Fatalf("human output exposes raw session id: %s", output.String())
	}
}

func TestWriteLegacyCompatibleDescriptorRemovesManualRemoteHandoff(t *testing.T) {
	legacy := "environment: agent-abc\nssh: haco-agent-abc\nworkspace: /workspace\nagents: New -> Remote -> SSH -> haco-agent-abc\n"
	descriptor := agentSessionDescriptor{
		Environment:     "agent-abc",
		RemoteWorkspace: "/workspace",
		SSHAlias:        "haco-agent-abc",
		FolderURI:       "vscode-remote://ssh-remote+haco-agent-abc/workspace",
	}
	var output bytes.Buffer
	if err := writeLegacyCompatibleDescriptor(&output, legacy, descriptor); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "New -> Remote -> SSH") {
		t.Fatalf("legacy manual handoff remains in output: %s", output.String())
	}
	if !strings.Contains(output.String(), "folder-uri: "+descriptor.FolderURI) {
		t.Fatalf("folder URI missing from output: %s", output.String())
	}
}

func TestNormalizeOrchestrationPrepareArgs(t *testing.T) {
	options, passthrough, err := normalizeOrchestrationPrepareArgs([]string{
		"--session", "session-a",
		"--json",
		"--code", "code-insiders",
		"--host-port", "2222",
		"/tmp/worktree-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.SessionID != "session-a" || !options.JSON || options.NoLaunch || options.CodeCommand != "code-insiders" {
		t.Fatalf("unexpected options: %+v", options)
	}
	want := []string{
		"--no-launch",
		"--session", "session-a",
		"--code", "code-insiders",
		"--host-port", "2222",
		"/tmp/worktree-a",
	}
	if !reflect.DeepEqual(passthrough, want) {
		t.Fatalf("passthrough = %#v, want %#v", passthrough, want)
	}
}

func TestNormalizeOrchestrationPrepareArgsPreservesNoLaunchIntent(t *testing.T) {
	options, passthrough, err := normalizeOrchestrationPrepareArgs([]string{
		"--session=session-a",
		"--no-launch",
		".",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !options.NoLaunch {
		t.Fatalf("no-launch intent was lost: %+v", options)
	}
	if len(passthrough) == 0 || passthrough[0] != "--no-launch" {
		t.Fatalf("legacy prepare was not forced to no-launch: %#v", passthrough)
	}
}
