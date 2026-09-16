package incus

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
	"time"
)

func TestHostImageExecutionRejectsArbitraryCommands(t *testing.T) {
	source := core.PersistentResource{ID: "oci-source:host", Kind: OCIStoreKind, Owner: strings.Repeat("a", 32), SourceOnly: true, State: "ready"}
	for _, args := range [][]string{
		{"image", "rm", "--force", "sha256:" + strings.Repeat("a", 64)},
		{"image", "rm", "app:latest"},
		{"image", "inspect", "--format", `{{json .Config.Env}}`, "sha256:" + strings.Repeat("a", 64)},
		{"image", "inspect", "--format", `{{json .RepoDigests}}`, "--", "-option"},
		{"run", "--privileged", "image"},
		{"image", "ls", "--quiet", "--no-trunc", "--host", "tcp://remote"},
	} {
		backend := &PersistentResourceBackend{Runtime: New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
			t.Fatal("invalid command reached native runtime")
			return host.Result{}, nil
		}})}
		if _, err := backend.ExecHostImage(context.Background(), source, "docker", args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
func TestHostImageExecutionRequiresExactUnprivilegedSource(t *testing.T) {
	for _, mode := range []string{"ok", "pending", "foreign-role", "inherited-role", "privileged", "wrong-owner", "wrong-mount", "extra-user", "paused", "layout", "truncated", "failed"} {
		t.Run(mode, func(t *testing.T) {
			source := core.PersistentResource{ID: "oci-source:host", Kind: OCIStoreKind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32), SourceOnly: true, State: "ready"}
			volume := persistentVolumeObservation{Name: "haco-persistent-" + source.Owner, Type: "custom", ContentType: "filesystem", Config: map[string]string{"user.hacocoon.owner": source.Owner, "user.hacocoon.resource": source.ID, "user.hacocoon.kind": source.Kind, "user.hacocoon.source-only": "true"}, UsedBy: []string{"/1.0/instances/haco-host?project=hacocoon"}}
			i := hostOCICopyInstance{Name: trustedHostName, Type: "container", Profiles: []string{}, StatusCode: 103, Config: map[string]string{trustedHostRoleKey: trustedHostRoleValue, hostOCIStoreKey: source.Owner}, LocalConfig: map[string]string{trustedHostRoleKey: trustedHostRoleValue, hostOCIStoreKey: source.Owner}, Devices: map[string]map[string]string{"oci": {"type": "disk", "pool": "pool", "source": volume.Name, "path": OCIStorePath}}}
			switch mode {
			case "foreign-role":
				i.Config[trustedHostRoleKey] = "foreign"
			case "inherited-role":
				delete(i.LocalConfig, trustedHostRoleKey)
			case "privileged":
				i.Config["security.privileged"] = "true"
			case "wrong-owner":
				volume.Config["user.hacocoon.owner"] = "foreign"
			case "wrong-mount":
				i.Devices["oci"]["path"] = "/other"
			case "extra-user":
				volume.UsedBy = append(volume.UsedBy, "/1.0/instances/guest?project=hacocoon")
			case "paused":
				i.StatusCode = 110
			}
			called := false
			encode := func(v any) host.Result { raw, _ := json.Marshal(v); return host.Result{Stdout: string(raw)} }
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" {
					t.Fatal(name)
				}
				switch {
				case args[0] == "config":
					if mode == "pending" {
						return host.Result{Stdout: "pending"}, nil
					}
					return host.Result{}, nil
				case args[0] == "query" && strings.Contains(args[1], "/instances/"):
					return encode(i), nil
				case args[0] == "query" && strings.Contains(args[1], "/volumes/custom?"):
					return encode([]persistentVolumeObservation{volume}), nil
				case args[0] == "exec" && args[len(args)-1] == hostOCILayoutVerify:
					if mode == "layout" {
						return host.Result{ExitCode: 40}, nil
					}
					return host.Result{}, nil
				case args[0] == "exec":
					called = true
					if !strings.Contains(strings.Join(args, " "), "/usr/bin/env -i PATH=/usr/local/bin:/usr/bin:/bin HOME=/nonexistent docker --host unix:///run/docker.sock image ls --quiet --no-trunc") {
						t.Fatal("unexpected native authority", args)
					}
					if mode == "truncated" {
						return host.Result{StdoutTruncated: true}, nil
					}
					if mode == "failed" {
						return host.Result{ExitCode: 1}, nil
					}
					return host.Result{}, nil
				default:
					t.Fatal("unexpected command", args)
					return host.Result{}, nil
				}
			}}
			_, err := (&PersistentResourceBackend{Runtime: New(runner)}).ExecHostImage(context.Background(), source, "docker", []string{"image", "ls", "--quiet", "--no-trunc"})
			if mode == "ok" {
				if err != nil || !called {
					t.Fatalf("valid source: %v", err)
				}
			} else if err == nil || (called && mode != "truncated" && mode != "failed") {
				t.Fatalf("unsafe source execution: %v called=%v", err, called)
			}
		})
	}
}

func TestHostImageExecutionUsesCopyOperationLock(t *testing.T) {
	source := core.PersistentResource{ID: "oci-source:host", Kind: OCIStoreKind, Owner: strings.Repeat("a", 32), SourceOnly: true, State: "ready"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	unlock, err := lockHostOperation(ctx, "host-image-lock-test")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	runtime := New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		t.Fatal("native command ran during another Host operation")
		return host.Result{}, nil
	}})
	runtime.project = "host-image-lock-test"
	waiting, stop := context.WithTimeout(ctx, 50*time.Millisecond)
	defer stop()
	_, err = (&PersistentResourceBackend{Runtime: runtime}).ExecHostImage(waiting, source, "docker", []string{"image", "ls", "--quiet", "--no-trunc"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("copy lock not shared: %v", err)
	}
}
