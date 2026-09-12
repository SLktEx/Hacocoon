//go:build linux

package incus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

type exportTestOperation struct {
	incusclient.Operation
	value api.Operation
	err   error
}

func (o exportTestOperation) Get() api.Operation                { return o.value }
func (o exportTestOperation) WaitContext(context.Context) error { return o.err }

type exportTestServer struct {
	incusclient.InstanceServer
	url              string
	client           *http.Client
	image            api.Image
	mode, root       string
	t                *testing.T
	creates, deletes int
	deleted          bool
}

func (s *exportTestServer) Disconnect() {}
func (s *exportTestServer) GetConnectionInfo() (*incusclient.ConnectionInfo, error) {
	return &incusclient.ConnectionInfo{URL: s.url, Project: "hacocoon", SocketPath: "/fixture/incus.sock"}, nil
}
func (s *exportTestServer) GetHTTPClient() (*http.Client, error) { return s.client, nil }
func (s *exportTestServer) CreateImage(request api.ImagesPost, args *incusclient.ImageCreateArgs) (incusclient.Operation, error) {
	s.creates++
	owner := request.Properties[exportImageOwnerKey]
	files, err := filepath.Glob(filepath.Join(s.root, "rootfs-export-*.jsonl"))
	if err != nil || len(files) != 1 {
		s.t.Fatal("missing pre-publication owner receipt", err)
	}
	raw, err := os.ReadFile(files[0])
	if err != nil || !strings.Contains(string(raw), owner) || len(owner) != 32 {
		s.t.Fatal("owner not durable before creation")
	}
	if request.Source.Type != "instance" || request.Source.Name != "haco-snapshot-root-"+strings.Repeat("a", 32) || request.CompressionAlgorithm != "none" || request.Format != "unified" || request.Public || args != nil || len(request.Aliases) != 0 {
		s.t.Fatal("unexpected publication", request)
	}
	s.image.Properties = map[string]string{exportImageOwnerKey: owner}
	if s.mode == "foreign-image" {
		s.image.Properties[exportImageOwnerKey] = "foreign"
	}
	if s.mode == "publish-error" {
		return nil, errors.New("publication lost reply")
	}
	op := exportTestOperation{value: api.Operation{ID: "fixture-operation", StatusCode: api.Success, Metadata: map[string]any{"fingerprint": s.image.Fingerprint}}}
	if s.mode == "wait-error" {
		op.err = context.DeadlineExceeded
	}
	if s.mode == "bad-fingerprint" {
		op.value.Metadata["fingerprint"] = "invalid"
	}
	return op, nil
}
func (s *exportTestServer) GetImage(string) (*api.Image, string, error) {
	if s.deleted && s.mode != "cleanup-unknown" {
		return nil, "", api.StatusErrorf(404, "absent")
	}
	return &s.image, "", nil
}
func (s *exportTestServer) DeleteImage(fingerprint string) (incusclient.Operation, error) {
	if fingerprint != s.image.Fingerprint {
		s.t.Fatal("wrong deletion identity")
	}
	s.deletes++
	if s.mode == "cleanup-error" {
		return nil, errors.New("delete failed")
	}
	s.deleted = true
	return exportTestOperation{value: api.Operation{StatusCode: api.Success}}, nil
}
func TestExportSnapshotRootfsOwnershipAndCleanup(t *testing.T) {
	for _, mode := range []string{"ok", "foreign-source", "changed-source", "publish-error", "wait-error", "bad-fingerprint", "foreign-image", "download-error", "digest-mismatch", "oversize", "cleanup-error", "cleanup-unknown"} {
		t.Run(mode, func(t *testing.T) {
			p, observed := savedRuntimeFixture()
			reads := 0
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" || len(args) != 2 || args[0] != "query" {
					t.Fatal("unexpected source mutation", name, args)
				}
				reads++
				if mode == "foreign-source" || mode == "changed-source" && reads > 1 {
					observed.Config["user.hacocoon.owner"] = "foreign"
				}
				raw, _ := json.Marshal([]snapshotInstanceObservation{observed})
				return host.Result{Stdout: string(raw)}, nil
			}}
			data := []byte("synthetic native rootfs archive")
			hash := sha256.Sum256(data)
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Query().Get("project") != "hacocoon" || r.URL.Path != "/1.0/images/"+hex.EncodeToString(hash[:])+"/export" {
					t.Error("wrong native endpoint", r.URL)
				}
				if mode == "download-error" {
					w.WriteHeader(500)
					return
				}
				if mode == "digest-mismatch" {
					_, _ = w.Write([]byte("changed bytes"))
					return
				}
				_, _ = w.Write(data)
			}))
			defer httpServer.Close()
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			server := &exportTestServer{url: httpServer.URL, client: httpServer.Client(), image: api.Image{Fingerprint: hex.EncodeToString(hash[:]), Type: "container"}, root: root, mode: mode, t: t}
			runtime := New(runner)
			component, err := runtime.snapshotComponent(snapshotBinding{Version: 1, Project: "hacocoon", Rootfs: &p})
			if err != nil {
				t.Fatal(err)
			}
			component.State = "verified"
			limit := int64(1024)
			if mode == "oversize" {
				limit = 1
			}
			archive, err := runtime.exportSnapshotRootfs(context.Background(), component, root, limit, func(context.Context) (incusclient.InstanceServer, error) { return server, nil })
			if mode == "ok" {
				if err != nil {
					t.Fatal(err)
				}
				defer archive.Close()
				raw, err := io.ReadAll(archive.Reader())
				if err != nil || string(raw) != string(data) {
					t.Fatal("bad archive", err)
				}
			} else if err == nil || archive != nil {
				t.Fatal("incomplete export reported success", archive, err)
			}
			uncertain := mode == "publish-error" || mode == "wait-error" || mode == "bad-fingerprint" || mode == "foreign-image" || mode == "cleanup-error" || mode == "cleanup-unknown"
			files, _ := filepath.Glob(filepath.Join(root, "rootfs-export-*.jsonl"))
			if uncertain {
				if !errors.Is(err, core.ErrRecoveryRequired) || len(files) != 1 {
					t.Fatal("lost uncertain ownership evidence", err, files)
				}
			} else if len(files) != 0 {
				t.Fatal("unnecessary receipt retained", files)
			}
			if mode == "foreign-source" {
				if server.creates != 0 {
					t.Fatal("published foreign source")
				}
			} else if server.creates != 1 {
				t.Fatal("missing native publication")
			}
			refuseDelete := mode == "foreign-source" || mode == "publish-error" || mode == "wait-error" || mode == "bad-fingerprint" || mode == "foreign-image"
			if refuseDelete && server.deletes != 0 {
				t.Fatal("deleted unverified native image")
			}
			if !refuseDelete && server.deletes != 1 {
				t.Fatal("owned image cleanup not attempted")
			}
		})
	}
}

func TestRootfsDownloadRejectsSplitRedirectAndCancellation(t *testing.T) {
	for _, mode := range []string{"split", "redirect", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "split":
					w.Header().Set("Content-Type", "multipart/form-data; boundary=fixture")
					_, _ = w.Write([]byte("not a unified archive"))
				case "redirect":
					w.Header().Set("Location", "/unexpected")
					w.WriteHeader(302)
				case "cancel":
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
					close(started)
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			fake := &exportTestServer{url: server.URL, client: server.Client()}
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				archive, err := downloadExportImage(ctx, fake, strings.Repeat("a", 64), root, 1024)
				if archive != nil {
					archive.Close()
					done <- errors.New("unexpected archive")
					return
				}
				done <- err
			}()
			if mode == "cancel" {
				select {
				case <-started:
					cancel()
				case <-time.After(3 * time.Second):
					t.Fatal("download never started")
				}
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("unsafe download accepted")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("download did not stop")
			}
		})
	}
}

func TestImageExportReceiptRejectsReplacement(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	journal, err := newImageExportJournal(root, "hacocoon", "instance/owned", "/fixture/incus.sock")
	if err != nil {
		t.Fatal(err)
	}
	defer journal.close()
	if err := os.Rename(journal.path(), journal.path()+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journal.path(), []byte("foreign"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := journal.finish(); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("replacement removed", err)
	}
	raw, err := os.ReadFile(journal.path())
	if err != nil || string(raw) != "foreign" {
		t.Fatal("foreign receipt lost", err)
	}
}

func TestExportWaitRequiresTerminalSuccess(t *testing.T) {
	for _, status := range []api.StatusCode{api.Running, api.Failure, api.Success} {
		err := waitExportOperation(context.Background(), exportTestOperation{value: api.Operation{StatusCode: status}})
		if (err == nil) != (status == api.Success) {
			t.Fatalf("status %v: %v", status, err)
		}
	}
}
func TestExportMetadataResponseIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, strings.Repeat("x", (1<<20)+1)) }))
	defer server.Close()
	transport := &imageExportTransport{base: http.DefaultTransport.(*http.Transport).Clone()}
	defer transport.base.CloseIdleConnections()
	request, err := http.NewRequest(http.MethodGet, server.URL+"/1.0/images/metadata", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, err = io.ReadAll(response.Body)
	var oversized *http.MaxBytesError
	if !errors.As(err, &oversized) {
		t.Fatal("unbounded native metadata", err)
	}
}
