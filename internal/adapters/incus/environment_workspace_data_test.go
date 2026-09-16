package incus

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func environmentWorkspaceFixture(t *testing.T) (*SandboxProvider, core.EnvironmentResourceBinding, WorkspaceAttachment) {
	t.Helper()
	p, err := NewSandboxProvider(New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		t.Fatal("unexpected native operation")
		return host.Result{}, core.ErrUnsupported
	}}))
	if err != nil {
		t.Fatal(err)
	}
	instance, areas := environmentPlacementFixture()
	areas[0].Attachment.Target = "/workspace/build/cache"
	request := core.EnvironmentResourceBinding{InstanceID: instance, WorkspacePath: "managed:work-a", Attachments: areas}
	m := WorkspaceAttachment{Device: "workspace", Pool: "pool", Volume: "haco-work-work-a", Path: "/workspace", Owner: strings.Repeat("f", 32), Repository: "repo-a"}
	p.ConfigureManagedWorkspaces(func(_ context.Context, source string) ([]WorkspaceAttachment, error) {
		if source != request.WorkspacePath {
			t.Fatal("wrong leased Workspace", source)
		}
		return []WorkspaceAttachment{m}, nil
	})
	return p, request, m
}

func TestEnvironmentWorkspacePlacementRequiresLeasedManagedData(t *testing.T) {
	for _, scenario := range []string{"valid", "external", "read-only", "no-resolver", "missing", "duplicate", "collection-root", "wrong-repository", "wrong-kind", "owner", "protected"} {
		t.Run(scenario, func(t *testing.T) {
			p, request, m := environmentWorkspaceFixture(t)
			mounts := []WorkspaceAttachment{m}
			switch scenario {
			case "external":
				request.WorkspacePath = "/home/user/work"
			case "read-only":
				request.ReadOnly = true
			case "missing":
				mounts = nil
			case "duplicate":
				mounts = append(mounts, m)
			case "collection-root":
				mounts[0].Device, mounts[0].Path = "workspace-repo-a", "/workspace/repo-a"
				request.Attachments[0].Attachment.Target = "/workspace/repo-a"
			case "wrong-repository":
				mounts[0].Device, mounts[0].Path = "workspace-repo-b", "/workspace/repo-b"
			case "wrong-kind":
				mounts[0].Volume = "haco-repo-work-a"
			case "owner":
				mounts[0].Owner = strings.Repeat("z", 32)
			case "protected":
				request.Attachments[0].Attachment.Target = "/workspace/.git/objects"
			}
			p.ConfigureManagedWorkspaces(func(context.Context, string) ([]WorkspaceAttachment, error) { return mounts, nil })
			if scenario == "no-resolver" {
				p.ConfigureManagedWorkspaces(nil)
			}
			digest, bound, err := p.environmentPlacementBinding(context.Background(), request)
			if (err == nil) != (scenario == "valid") {
				t.Fatal(scenario, err)
			}
			if err == nil && (len(digest) != 64 || len(bound) != 1 || bound[0] != m) {
				t.Fatal("lost Workspace binding")
			}
		})
	}
}

func TestEnvironmentWorkspaceBindingPinsOwnershipButNotGitPresentation(t *testing.T) {
	p, request, m := environmentWorkspaceFixture(t)
	original, _, err := p.environmentPlacementBinding(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"branch", "owner", "volume", "workspace"} {
		t.Run(scenario, func(t *testing.T) {
			changed, binding := m, request
			switch scenario {
			case "branch":
				changed.Branch, changed.Remote = "new-branch", "new-remote"
			case "owner":
				changed.Owner = strings.Repeat("e", 32)
			case "volume":
				changed.Volume = "haco-work-work-b"
			case "workspace":
				binding.WorkspacePath = "managed:work-b"
			}
			p.ConfigureManagedWorkspaces(func(context.Context, string) ([]WorkspaceAttachment, error) {
				return []WorkspaceAttachment{changed}, nil
			})
			digest, _, err := p.environmentPlacementBinding(context.Background(), binding)
			if err != nil || (digest == original) != (scenario == "branch") {
				t.Fatal("wrong identity boundary", err)
			}
		})
	}
}

func TestEnvironmentWorkspaceDevicesRejectDriftAndOverlaps(t *testing.T) {
	for _, scenario := range []string{"valid", "missing", "inherited", "source", "readonly", "shift", "alias", "nested", "digest"} {
		t.Run(scenario, func(t *testing.T) {
			p, request, m := environmentWorkspaceFixture(t)
			digest, mounts, err := p.environmentPlacementBinding(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			devices := map[string]map[string]string{m.Device: environmentWorkspaceDevice(m), environmentDataDevicePrefix + "go": environmentDataDevice(request.Attachments[0])}
			config := environmentDataConfiguration{Config: map[string]string{environmentInstanceKey: request.InstanceID, environmentDataKey: digest}, Devices: devices, ExplicitDevices: maps.Clone(devices)}
			switch scenario {
			case "missing":
				delete(devices, m.Device)
			case "inherited":
				delete(config.ExplicitDevices, m.Device)
			case "source":
				devices[m.Device]["source"] = "haco-work-work-b"
			case "readonly":
				devices[m.Device]["readonly"] = "true"
			case "shift":
				devices[m.Device]["shift"] = "true"
			case "alias":
				devices["unbound-alias"] = environmentWorkspaceDevice(m)
			case "nested":
				devices["nested"] = map[string]string{"type": "disk", "path": "/workspace/build/cache/other"}
			case "digest":
				config.Config[environmentDataKey], _ = environmentDataBinding(request.InstanceID, request.Attachments)
			}
			err = verifyEnvironmentDataDevices(config, request.InstanceID, digest, request.Attachments, mounts, true)
			if (err == nil) != (scenario == "valid") {
				t.Fatal(scenario, err)
			}
		})
	}
}

func TestEnvironmentWorkspaceVolumeRequiresExactExclusiveUse(t *testing.T) {
	for _, scenario := range []string{"valid", "detached", "other-env", "other-project", "shared", "owner", "role", "repository", "exit"} {
		t.Run(scenario, func(t *testing.T) {
			p, _, m := environmentWorkspaceFixture(t)
			object, err := environmentWorkspaceObject(m)
			if err != nil {
				t.Fatal(err)
			}
			volume := repositoryVolumeObservation{Name: m.Volume, Type: "custom", ContentType: "filesystem", Config: volumeConfig(object), UsedBy: []string{"/1.0/instances/haco-demo?project=hacocoon"}}
			exit := 0
			switch scenario {
			case "detached":
				volume.UsedBy = nil
			case "other-env":
				volume.UsedBy[0] = "/1.0/instances/haco-other?project=hacocoon"
			case "other-project":
				volume.UsedBy[0] = "/1.0/instances/haco-demo?project=other"
			case "shared":
				volume.UsedBy = append(volume.UsedBy, volume.UsedBy[0])
			case "owner":
				volume.Config["user.hacocoon.owner"] = strings.Repeat("e", 32)
			case "role":
				volume.Config["user.hacocoon.role"] = "repo"
			case "repository":
				volume.Config["user.hacocoon.repository"] = "other"
			case "exit":
				exit = 1
			}
			data, _ := json.Marshal(volume)
			p.runner = &fakeRunner{run: func(_ context.Context, _ int, command string, args []string) (host.Result, error) {
				if command != "incus" || strings.Join(args, " ") != "query /1.0/storage-pools/pool/volumes/custom/"+m.Volume+"?project=hacocoon" {
					t.Fatal("unexpected native operation", command, args)
				}
				return host.Result{Stdout: string(data), ExitCode: exit}, nil
			}}
			err = p.verifyEnvironmentWorkspaceData(context.Background(), "haco-demo", []WorkspaceAttachment{m})
			if (err == nil) != (scenario == "valid") {
				t.Fatal(scenario, err)
			}
			if scenario == "shared" && !errors.Is(err, core.ErrStorageBusy) {
				t.Fatal(err)
			}
		})
	}
}
