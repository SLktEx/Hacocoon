package incus

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	gitrepo "github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func repositoryObject(kind, id, repository string) gitrepo.Object {
	return gitrepo.Object{Kind: kind, ID: id, Repository: repository, NativeRef: "haco-local-default/haco-" + kind + "-" + id, Owner: strings.Repeat("a", 32), State: "ready", Remote: "https://github.com/example/" + repository + ".git", Branch: "main"}
}

// These fields are Incus's custom filesystem-volume API response, including the
// ownership written at creation. Guest Git configuration is not an input.
func repositoryVolume(object gitrepo.Object) map[string]any {
	return map[string]any{
		"name": "haco-" + object.Kind + "-" + object.ID, "type": "custom", "content_type": "filesystem",
		"config":  map[string]string{"user.hacocoon.owner": object.Owner, "user.hacocoon.role": object.Kind, "user.hacocoon.repository": object.Repository},
		"used_by": []string{},
	}
}

func repositoryJSON(t *testing.T, value any) host.Result {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return host.Result{Stdout: string(data)}
}

func TestRepositoryPlanUsesSelectedStorageAndRejectsInvalidNames(t *testing.T) {
	for _, mode := range []string{"repo", "work", "kind", "option", "path", "storage-failure"} {
		t.Run(mode, func(t *testing.T) {
			failure := errors.New("selected pool unavailable")
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" || !reflect.DeepEqual(args, []string{"storage", "show", "haco-local-default", "--project", "hacocoon"}) {
					t.Fatal("unselected storage accessed", name, args)
				}
				if mode == "storage-failure" {
					return host.Result{}, failure
				}
				return host.Result{}, nil
			}}
			runtime := New(runner)
			runtime.setRootPool("haco-local-default")
			kind, id := "work", "task"
			switch mode {
			case "repo":
				kind = "repo"
			case "kind":
				kind = "environment"
			case "option":
				id = "--all"
			case "path":
				id = "../task"
			}
			got, err := (&RepositoryBackend{Runtime: runtime}).Plan(context.Background(), kind, id)
			switch mode {
			case "repo", "work":
				if err != nil || got != "haco-local-default/haco-"+kind+"-task" || len(runner.calls) != 1 {
					t.Fatal(got, err, runner.calls)
				}
			case "storage-failure":
				if got != "" || !errors.Is(err, failure) || len(runner.calls) != 1 {
					t.Fatal("failed selection fell back to another pool", got, err)
				}
			default:
				if got != "" || !errors.Is(err, core.ErrInvalidArgument) || len(runner.calls) != 0 {
					t.Fatal("invalid plan reached storage", got, err)
				}
			}
		})
	}
}

func TestRepositoryCreationPreservesCopyIDsAndAssignsFreshOwnership(t *testing.T) {
	idmap := `[{"Isuid":true,"Isgid":false,"Hostid":1000000,"Nsid":0,"Maprange":1000000000},{"Isuid":false,"Isgid":true,"Hostid":1000000,"Nsid":0,"Maprange":1000000000}]`
	for _, mode := range []string{"fresh", "copy", "invalid-target", "missing-idmap", "empty-idmap", "malformed-idmap", "cross-pool", "foreign-source", "source-failure", "lost-reply"} {
		t.Run(mode, func(t *testing.T) {
			source := repositoryObject("repo", "demo", "demo")
			target := repositoryObject("work", "task", "demo")
			target.Owner = strings.Repeat("b", 32)
			observation := repositoryVolume(source)
			config := observation["config"].(map[string]string)
			config["volatile.idmap.last"], config["volatile.idmap.next"] = idmap, idmap
			config["security.shifted"], config["user.private"] = "true", "source-private"
			switch mode {
			case "invalid-target":
				target.NativeRef = "haco-local-default/haco-work-other"
			case "missing-idmap":
				delete(config, "volatile.idmap.next")
			case "empty-idmap":
				config["volatile.idmap.last"] = "[]"
			case "malformed-idmap":
				config["volatile.idmap.next"] = `{"not":"a mapping"}`
			case "cross-pool":
				source.NativeRef = "another-pool/haco-repo-demo"
			case "foreign-source":
				config["user.hacocoon.owner"] = strings.Repeat("c", 32)
			}
			failure := errors.New("provider reply lost")
			var created map[string]any
			posts := 0
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" {
					t.Fatal(name)
				}
				if len(args) == 2 && args[0] == "query" {
					if args[1] != "/1.0/storage-pools/"+strings.Split(source.NativeRef, "/")[0]+"/volumes/custom/haco-repo-demo?project=hacocoon" {
						t.Fatal("read another source", args)
					}
					if mode == "source-failure" {
						return host.Result{}, failure
					}
					return repositoryJSON(t, observation), nil
				}
				if len(args) != 7 || !reflect.DeepEqual(args[:6], []string{"query", "-X", "POST", "--wait", "/1.0/storage-pools/haco-local-default/volumes/custom?project=hacocoon", "--data"}) {
					t.Fatal("unexpected mutation", args)
				}
				posts++
				if err := json.Unmarshal([]byte(args[6]), &created); err != nil {
					t.Fatal(err)
				}
				if mode == "lost-reply" {
					return host.Result{}, failure
				}
				return host.Result{}, nil
			}}
			backend := &RepositoryBackend{Runtime: New(runner)}
			copySource := &source
			if mode == "fresh" {
				copySource = nil
			}
			err := backend.CreateVolume(context.Background(), target, copySource)
			if mode == "invalid-target" {
				if !errors.Is(err, core.ErrInvalidArgument) || len(runner.calls) != 0 {
					t.Fatal("invalid target reached provider", err)
				}
				return
			}
			if mode != "fresh" && mode != "copy" && mode != "lost-reply" {
				if !errors.Is(err, core.ErrIncompatibleState) || posts != 0 {
					t.Fatal("unverified copy created", err, created)
				}
				return
			}
			if (mode == "lost-reply" && !errors.Is(err, failure)) || (mode != "lost-reply" && err != nil) || posts != 1 {
				t.Fatal("creation outcome changed", err, posts)
			}
			wantConfig := map[string]any{"user.hacocoon.owner": target.Owner, "user.hacocoon.role": "work", "user.hacocoon.repository": "demo"}
			want := map[string]any{"name": "haco-work-task", "type": "custom", "content_type": "filesystem", "config": wantConfig}
			if copySource != nil {
				wantConfig["volatile.idmap.last"], wantConfig["volatile.idmap.next"] = idmap, idmap
				want["source"] = map[string]any{"type": "copy", "name": "haco-repo-demo", "pool": "haco-local-default", "project": "hacocoon", "volume_only": true}
			}
			if !reflect.DeepEqual(created, want) {
				t.Fatal("copy changed filesystem IDs or inherited source authority", created)
			}
		})
	}
}

func TestRepositoryObservationRefusesUnverifiedVolume(t *testing.T) {
	for _, mode := range []string{"valid", "pool-option", "wrong-ref", "invalid-owner", "query-fails", "nonzero", "truncated", "malformed", "name", "type", "content_type", "missing-config", "owner", "role", "repository"} {
		t.Run(mode, func(t *testing.T) {
			object := repositoryObject("work", "task", "demo")
			observed := repositoryVolume(object)
			switch mode {
			case "pool-option":
				object.NativeRef = "--all/haco-work-task"
			case "wrong-ref":
				object.NativeRef = "haco-local-default/haco-work-other"
			case "invalid-owner":
				object.Owner = ""
			case "name", "type", "content_type":
				observed[mode] = "other"
			case "missing-config":
				delete(observed, "config")
			case "owner", "role", "repository":
				observed["config"].(map[string]string)["user.hacocoon."+mode] = "other"
			}
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" || !reflect.DeepEqual(args, []string{"query", "/1.0/storage-pools/haco-local-default/volumes/custom/haco-work-task?project=hacocoon"}) {
					t.Fatal("unowned target queried", name, args)
				}
				result := repositoryJSON(t, observed)
				switch mode {
				case "query-fails":
					return result, errors.New("observation failed")
				case "nonzero":
					result.ExitCode = 1
				case "truncated":
					result.StdoutTruncated = true
				case "malformed":
					result.Stdout = "{"
				}
				return result, nil
			}}
			err := (&RepositoryBackend{Runtime: New(runner)}).InspectVolume(context.Background(), object)
			switch mode {
			case "valid":
				if err != nil {
					t.Fatal(err)
				}
			case "pool-option", "wrong-ref", "invalid-owner":
				if !errors.Is(err, core.ErrInvalidArgument) || len(runner.calls) != 0 {
					t.Fatal("invalid reference reached provider", err)
				}
			default:
				if !errors.Is(err, core.ErrIncompatibleState) {
					t.Fatal("unverified volume accepted", err)
				}
			}
		})
	}
}

func TestRepositoryWorkspaceAttachmentsKeepMembersAndRoutingSeparate(t *testing.T) {
	for _, mode := range []string{"single", "collection", "foreign-second", "invalid-path", "repo"} {
		t.Run(mode, func(t *testing.T) {
			first, second := repositoryObject("work", "task-one", "one"), repositoryObject("work", "task-two", "two")
			second.Owner, second.Remote, second.Branch = strings.Repeat("b", 32), "", ""
			object := first
			if mode == "repo" {
				object.Kind = "repo"
			} else if mode != "single" {
				object = gitrepo.Object{Kind: "work", ID: "task", Members: []gitrepo.Object{first, second}}
			}
			if mode == "invalid-path" {
				object.Members[1].Repository = "../two"
			}
			before := repositoryJSON(t, object).Stdout
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				for i, member := range object.Copies() {
					if name == "incus" && reflect.DeepEqual(args, []string{"query", "/1.0/storage-pools/haco-local-default/volumes/custom/haco-work-" + member.ID + "?project=hacocoon"}) {
						observed := repositoryVolume(member)
						if mode == "foreign-second" && i == 1 {
							observed["config"].(map[string]string)["user.hacocoon.owner"] = "replacement"
						}
						return repositoryJSON(t, observed), nil
					}
				}
				t.Fatal("unexpected provider operation", name, args)
				return host.Result{}, core.ErrIncompatibleState
			}}
			mounts, err := (&RepositoryBackend{Runtime: New(runner)}).WorkspaceAttachments(context.Background(), object)
			if repositoryJSON(t, object).Stdout != before {
				t.Fatal("saved membership changed")
			}
			if mode == "repo" || mode == "foreign-second" || mode == "invalid-path" {
				if err == nil || mounts != nil || (mode == "repo" && len(runner.calls) != 0) {
					t.Fatal("invalid collection exposed partial mounts", mounts, err)
				}
				return
			}
			want := []WorkspaceAttachment{{Device: "workspace", Pool: "haco-local-default", Volume: "haco-work-task-one", Path: "/workspace", Owner: first.Owner, Repository: "one", Remote: first.Remote, Branch: "main"}}
			if mode == "collection" {
				want[0].Device, want[0].Path = "workspace-one", "/workspace/one"
				want = append(want, WorkspaceAttachment{Device: "workspace-two", Pool: "haco-local-default", Volume: "haco-work-task-two", Path: "/workspace/two", Owner: second.Owner, Repository: "two"})
			}
			if err != nil || !reflect.DeepEqual(mounts, want) || len(runner.calls) != len(want) {
				t.Fatal("Workspace identity or offline routing changed", mounts, err)
			}
		})
	}
}
