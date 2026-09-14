//go:build linux

package incus

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestEnvironmentDataPathObservationDoesNotFollowLinksOrHideFiles(t *testing.T) {
	for _, scenario := range []string{"empty", "missing-parent", "symlink-parent", "symlink-target", "file", "nonempty", "oversized", "malformed", "null", "error", "header-missing", "changed-after-head"} {
		t.Run(scenario, func(t *testing.T) {
			gets, heads := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/1.0/instances/haco-demo/files" || r.URL.Query().Get("project") != "hacocoon" {
					t.Error("wrong observation", r.URL)
					w.WriteHeader(500)
					return
				}
				p := r.URL.Query().Get("path")
				kind := "directory"
				if r.Method == http.MethodHead {
					heads++
					if scenario == "missing-parent" && p == "/root/.cache" {
						w.WriteHeader(404)
						return
					}
					if scenario == "error" {
						w.WriteHeader(403)
						return
					}
					if (scenario == "symlink-parent" && p == "/root") || (scenario == "symlink-target" && p == "/root/.cache/go-build") {
						kind = "symlink"
					}
					if scenario == "file" {
						kind = "file"
					}
					if scenario != "header-missing" {
						w.Header().Set("X-Incus-type", kind)
					}
					return
				}
				gets++
				if p != "/root/.cache/go-build" {
					t.Error("listed ancestor")
				}
				if scenario == "changed-after-head" {
					kind = "symlink"
				}
				w.Header().Set("X-Incus-type", kind)
				body := `{"type":"sync","status_code":200,"metadata":[]}`
				switch scenario {
				case "nonempty":
					body = `{"type":"sync","status_code":200,"metadata":["keep"]}`
				case "oversized":
					body = strings.Repeat("x", 33<<10)
				case "malformed":
					body = "oops"
				case "null":
					body = `{"type":"sync","status_code":200,"metadata":null}`
				}
				io.WriteString(w, body)
			}))
			defer server.Close()
			err := verifyRootfsDataPath(context.Background(), server.URL, "hacocoon", "haco-demo", "/root/.cache/go-build", server.Client().Do)
			if (err == nil) != (scenario == "empty" || scenario == "missing-parent") {
				t.Fatal(err)
			}
			if scenario == "nonempty" && !errors.Is(err, core.ErrStorageBusy) {
				t.Fatal(err)
			}
			if scenario == "symlink-parent" && (heads != 1 || gets != 0) {
				t.Fatal("followed parent link")
			}
			if scenario == "missing-parent" && (heads != 2 || gets != 0) {
				t.Fatal("observed below missing parent")
			}
		})
	}
}

func TestEnvironmentDataPathCancellationReachesDaemon(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := verifyRootfsDataPath(ctx, "http://unix.socket", "hacocoon", "haco-demo", "/root/cache", func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestEnvironmentWorkspacePathsObserveTheVolumeRatherThanRootfs(t *testing.T) {
	for _, scenario := range []string{"empty", "missing", "link-parent", "link-target", "nonempty", "unsupported", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			_, _, mount := environmentWorkspaceFixture(t)
			mount.Device, mount.Path = "workspace-repo-a", "/workspace/repo-a"
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/1.0/storage-pools/pool/volumes/custom/haco-work-work-a/files" || r.URL.Query().Get("project") != "hacocoon" || len(r.URL.Query()) != 2 {
					t.Error("wrong storage observation", r.URL)
					w.WriteHeader(500)
					return
				}
				p := r.URL.Query().Get("path")
				paths = append(paths, p)
				if strings.Contains(p, "workspace") {
					t.Error("did not resolve relative volume path")
				}
				if scenario == "unsupported" {
					w.WriteHeader(405)
					return
				}
				if scenario == "missing" && p == "/build" {
					w.WriteHeader(404)
					return
				}
				kind := "directory"
				if (scenario == "link-parent" && p == "/build") || (scenario == "link-target" && p == "/build/cache") {
					kind = "symlink"
				}
				w.Header().Set("X-Incus-type", kind)
				if r.Method == http.MethodGet {
					if p != "/build/cache" {
						t.Error("listed ancestor")
					}
					if scenario == "nonempty" {
						io.WriteString(w, `{"type":"sync","status_code":200,"metadata":["keep"]}`)
					} else {
						io.WriteString(w, `{"type":"sync","status_code":200,"metadata":[]}`)
					}
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "cancelled" {
				cancel()
			}
			err := verifyWorkspaceDataPath(ctx, server.URL, "hacocoon", mount, "/workspace/repo-a/build/cache", server.Client().Do)
			if (err == nil) != (scenario == "empty" || scenario == "missing") {
				t.Fatal(scenario, err)
			}
			if (scenario == "link-parent" || scenario == "missing") && len(paths) != 1 {
				t.Fatal("followed unsafe ancestor", paths)
			}
			if scenario == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}

func TestEnvironmentWorkspacePathsRefuseWrongMountAndRootfsFallback(t *testing.T) {
	_, _, mount := environmentWorkspaceFixture(t)
	mount.Device, mount.Path = "workspace-repo-a", "/workspace/repo-a"
	do := func(*http.Request) (*http.Response, error) { t.Fatal("invalid target reached daemon"); return nil, nil }
	for _, target := range []string{"/workspace/repo-a", "/workspace/repo-b/cache", "/workspace/repo-a/../repo-b/cache", "/workspace/repo-a/.git/objects"} {
		if err := verifyWorkspaceDataPath(context.Background(), "http://unix.socket", "hacocoon", mount, target, do); err == nil {
			t.Fatal("unsafe target accepted", target)
		}
	}
	if err := verifyRootfsDataPath(context.Background(), "http://unix.socket", "hacocoon", "haco-demo", "/workspace/repo-a/cache", do); err == nil {
		t.Fatal("used rootfs for Workspace")
	}
}
