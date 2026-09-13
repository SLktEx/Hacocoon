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
