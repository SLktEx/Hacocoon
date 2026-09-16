//go:build linux

package incus

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"gopkg.in/yaml.v2"
)

type importImageServer struct {
	incusclient.InstanceServer
	t                *testing.T
	root, mode       string
	image            api.Image
	created, deleted bool
	uploads, deletes int
}

func (s *importImageServer) Disconnect() {}
func (s *importImageServer) GetConnectionInfo() (*incusclient.ConnectionInfo, error) {
	return &incusclient.ConnectionInfo{Project: "hacocoon", SocketPath: "/fixture/incus.sock"}, nil
}
func (s *importImageServer) GetImage(fp string) (*api.Image, string, error) {
	if s.mode == "exists" && !s.created {
		return &api.Image{Fingerprint: fp}, "", nil
	}
	if !s.created || s.deleted && s.mode != "cleanup-unknown" {
		return nil, "", api.StatusErrorf(404, "absent")
	}
	return &s.image, "", nil
}
func (s *importImageServer) RawOperation(method, path string, data any, etag string) (incusclient.Operation, string, error) {
	s.uploads++
	if method != "POST" || path != "/images" || etag != "" {
		s.t.Fatal("unexpected image request")
	}
	r, ok := data.(io.Reader)
	if !ok {
		s.t.Fatal("not a native stream")
	}
	body, err := io.ReadAll(r)
	if err != nil {
		s.t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	fp := hex.EncodeToString(sum[:])
	tr := tar.NewReader(bytes.NewReader(body))
	if _, err := tr.Next(); err != nil {
		s.t.Fatal(err)
	}
	raw, err := io.ReadAll(tr)
	if err != nil {
		s.t.Fatal(err)
	}
	var m rootfsImportMetadata
	if yaml.UnmarshalStrict(raw, &m) != nil {
		s.t.Fatal("bad metadata")
	}
	owner := m.Properties[importImageOwnerKey]
	files, err := filepath.Glob(filepath.Join(s.root, "rootfs-import-*.jsonl"))
	if err != nil || len(files) != 1 {
		s.t.Fatal("ownership receipt missing", err)
	}
	receipt, err := os.ReadFile(files[0])
	if err != nil || len(owner) != 32 || !bytes.Contains(receipt, []byte(owner)) || !bytes.Contains(receipt, []byte(fp)) {
		s.t.Fatal("ownership not durable before upload")
	}
	s.created = true
	s.image = api.Image{Fingerprint: fp, Type: "container", ImagePut: api.ImagePut{Properties: m.Properties}}
	if s.mode == "foreign" {
		s.image.Properties[importImageOwnerKey] = "foreign"
	}
	if s.mode == "upload-error" {
		return nil, "", errors.New("lost reply")
	}
	if s.mode == "nil-operation" {
		return nil, "", nil
	}
	op := exportTestOperation{value: api.Operation{ID: "owned-import", StatusCode: api.Success, Metadata: map[string]any{"fingerprint": fp}}}
	if s.mode == "wait-error" {
		op.err = context.DeadlineExceeded
	}
	if s.mode == "bad-fingerprint" {
		op.value.Metadata["fingerprint"] = strings.Repeat("c", 64)
	}
	return op, "", nil
}
func (s *importImageServer) DeleteImage(fp string) (incusclient.Operation, error) {
	if fp != s.image.Fingerprint {
		s.t.Fatal("foreign image selected")
	}
	s.deletes++
	if s.mode == "cleanup-error" {
		return nil, errors.New("delete failed")
	}
	s.deleted = true
	return exportTestOperation{value: api.Operation{StatusCode: api.Success}}, nil
}
func TestNativeRootfsImportOwnershipAndCleanup(t *testing.T) {
	for _, mode := range []string{"ok", "exists", "upload-error", "nil-operation", "wait-error", "bad-fingerprint", "foreign", "consumer-error", "canceled-consumer", "cleanup-error", "cleanup-unknown"} {
		t.Run(mode, func(t *testing.T) {
			root := privateRootfsImportDir(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := &importImageServer{t: t, root: root, mode: mode}
			connects := 0
			connect := func(c context.Context) (incusclient.InstanceServer, error) {
				connects++
				if c.Err() != nil {
					t.Fatal("cleanup inherited cancellation")
				}
				return server, nil
			}
			consumed := false
			err := New(nil).withImportedRootfs(ctx, bytes.NewReader(rootfsImportFixture(t, rootfsMetadataFixture, nil)), root, 1<<20, func(fp string) error {
				consumed = true
				if fp != server.image.Fingerprint || server.image.Properties[importImageOwnerKey] == "" {
					t.Fatal("unverified image consumed")
				}
				if mode == "consumer-error" {
					return errors.New("consumer failed")
				}
				if mode == "canceled-consumer" {
					cancel()
					return context.Canceled
				}
				return nil
			}, connect)
			files, readErr := filepath.Glob(filepath.Join(root, "rootfs-import-*.jsonl"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			retain := mode == "upload-error" || mode == "nil-operation" || mode == "wait-error" || mode == "bad-fingerprint" || mode == "foreign" || mode == "cleanup-error" || mode == "cleanup-unknown"
			if (len(files) == 1) != retain {
				t.Fatal("incorrect residue evidence", mode, len(files), err)
			}
			if mode == "ok" {
				if err != nil || !consumed || !server.deleted {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("failure hidden", mode)
			}
			if retain && !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal("unknown result not retained", err)
			}
			if mode == "exists" && (server.uploads != 0 || server.deletes != 0 || consumed) {
				t.Fatal("existing image adopted")
			}
			if mode == "upload-error" || mode == "nil-operation" || mode == "wait-error" || mode == "bad-fingerprint" || mode == "foreign" {
				if consumed || server.deletes != 0 {
					t.Fatal("unconfirmed image consumed/deleted")
				}
			}
			if mode == "consumer-error" || mode == "canceled-consumer" {
				if !server.deleted || connects != 2 || errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("completed image cleanup failed", err)
				}
			}
		})
	}
}
