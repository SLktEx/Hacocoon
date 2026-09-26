package incus

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/env"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestRepositoryGitConnectionRequiresOwnedWorkspaceAndExactProxy(t *testing.T) {
	for _, mode := range []string{"new", "existing", "name", "route", "provider", "foreign-volume", "query-fails", "malformed", "unmanaged", "disk-type", "disk-pool", "disk-source", "disk-path", "foreign-proxy", "add-fails", "missing-client", "unsafe-client", "push-fails"} {
		t.Run(mode, func(t *testing.T) {
			work := repositoryObject("work", "task", "demo")
			env := core.Environment{Name: "dev", RuntimeRef: "haco-runtime-v1:" + environmentapp.ProviderIncus + ":" + base64.RawURLEncoding.EncodeToString([]byte("haco-dev"))}
			const socket = "/run/hacocoon/git/dev.sock"
			proxy := map[string]string{"type": "proxy", "bind": "instance", "listen": "unix:/var/lib/hacocoon-git.sock", "connect": "unix:" + socket, "mode": "0600", "uid": "0", "gid": "0"}
			disk := map[string]string{"type": "disk", "pool": "haco-local-default", "source": "haco-work-task", "path": "/workspace"}
			devices := map[string]map[string]string{"workspace": disk}
			instance := map[string]any{"config": map[string]string{managedEnvironmentMarkerKey: managedEnvironmentMarkerValue}, "devices": devices}
			client := writeTrustedClientFixture(t, 0755)
			failure := errors.New("provider failure")
			switch mode {
			case "existing":
				devices["git-broker"] = proxy
			case "name":
				env.Name = "../dev"
			case "route":
				env.RuntimeRef = "haco-runtime-v1:" + environmentapp.ProviderIncus + ":" + base64.RawURLEncoding.EncodeToString([]byte("haco-other"))
			case "provider":
				env.RuntimeRef = "haco-runtime-v1:another:" + base64.RawURLEncoding.EncodeToString([]byte("haco-dev"))
			case "unmanaged":
				instance["config"] = map[string]string{}
			case "disk-type", "disk-pool", "disk-source", "disk-path":
				disk[mode[len("disk-"):]] = "other"
			case "foreign-proxy":
				devices["git-broker"] = map[string]string{"type": "proxy", "connect": "unix:/run/other.sock"}
			case "missing-client":
				client += "-absent"
			case "unsafe-client":
				if err := os.Chmod(client, 0777); err != nil {
					t.Fatal(err)
				}
			}
			added, pushed := 0, 0
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" {
					t.Fatal(name)
				}
				switch {
				case reflect.DeepEqual(args, []string{"query", "/1.0/storage-pools/haco-local-default/volumes/custom/haco-work-task?project=hacocoon"}):
					volume := repositoryVolume(work)
					if mode == "foreign-volume" {
						volume["config"].(map[string]string)["user.hacocoon.owner"] = "other"
					}
					return repositoryJSON(t, volume), nil
				case reflect.DeepEqual(args, []string{"query", "/1.0/instances/haco-dev?project=hacocoon"}):
					if mode == "query-fails" {
						return host.Result{}, failure
					}
					if mode == "malformed" {
						return host.Result{Stdout: "{"}, nil
					}
					return repositoryJSON(t, instance), nil
				case len(args) > 2 && args[0] == "config":
					want := []string{"config", "device", "add", "haco-dev", "git-broker", "proxy", "bind=instance", "listen=unix:/var/lib/hacocoon-git.sock", "connect=unix:" + socket, "mode=0600", "uid=0", "gid=0", "--project", "hacocoon"}
					if !reflect.DeepEqual(args, want) || devices["git-broker"] != nil {
						t.Fatal("proxy overwritten or broadened", args)
					}
					added++
					if mode == "add-fails" {
						return host.Result{}, failure
					}
					devices["git-broker"] = proxy
					return host.Result{}, nil
				case len(args) > 2 && args[0] == "file":
					want := []string{"file", "push", client, "haco-dev/usr/local/bin/git-remote-haco", "--project", "hacocoon", "--uid", "0", "--gid", "0", "--mode", "0755"}
					if !reflect.DeepEqual(args, want) || !reflect.DeepEqual(devices["git-broker"], proxy) {
						t.Fatal("helper installed outside verified Git endpoint", args)
					}
					payload, err := os.ReadFile(client)
					if err != nil || string(payload) != "test-client-binary\n" {
						t.Fatal("unverified helper input", err)
					}
					pushed++
					if mode == "push-fails" {
						return host.Result{}, failure
					}
					return host.Result{}, nil
				default:
					t.Fatal("unexpected provider operation", args)
					return host.Result{}, failure
				}
			}}
			backend := &RepositoryBackend{Runtime: New(runner), ProductBinary: client}
			if mode == "new" || mode == "existing" || mode == "foreign-proxy" {
				ready, inspectErr := backend.InspectGitConnection(context.Background(), env, work, socket)
				if ready != (mode == "existing") || ((inspectErr != nil) != (mode == "foreign-proxy")) || added != 0 || pushed != 0 {
					t.Fatal("inspection mutated or accepted different wiring", ready, inspectErr, added, pushed)
				}
			}
			err := backend.ConnectGit(context.Background(), env, work, socket)
			if mode == "new" || mode == "existing" {
				if err != nil || pushed != 1 || (mode == "existing" && added != 0) || (mode == "new" && added != 1) {
					t.Fatal("valid endpoint failed", err, added, pushed)
				}
				if err := backend.ConnectGit(context.Background(), env, work, socket); err != nil || pushed != 2 || added > 1 {
					t.Fatal("repeated connection replaced the existing proxy", err, added, pushed)
				}
				return
			}
			if err == nil || (mode != "push-fails" && pushed != 0) {
				t.Fatal("unverified Git endpoint accepted", err, added, pushed)
			}
			switch mode {
			case "name", "route", "provider":
				if !errors.Is(err, core.ErrInvalidArgument) || len(runner.calls) != 0 {
					t.Fatal("invalid route reached provider", err, runner.calls)
				}
			case "add-fails", "push-fails", "query-fails":
				if !errors.Is(err, failure) {
					t.Fatal("provider failure lost", err)
				}
			case "missing-client", "unsafe-client":
				// A failed install must not publish an unverified executable.
			default:
				if added != 0 {
					t.Fatal("unverified target mutated")
				}
			}
		})
	}
}
