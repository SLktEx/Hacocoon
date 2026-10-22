//go:build linux

package controlapi

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	basebuild "github.com/SLktEx/Hacocoon/internal/base/build"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestBaseImportStreamHasNoHiddenEnvironmentCapOrDeadline(t *testing.T) {
	socket := doctorTestSocket(t, func(server *control.Server) {
		err := RegisterBaseImport(server, func(ctx context.Context, source io.Reader, req basebuild.ImportRequest) (basebuild.Result, error) {
			if _, deadline := ctx.Deadline(); deadline {
				t.Error("Base inherited Environment import deadline")
			}
			reader := source.(*environmentImportReader)
			if reader.limit != basebuild.MaxArchiveLimitBytes || reader.limit <= environmentExportWireLimit {
				t.Error("Base inherited Environment transfer cap")
			}
			data, err := io.ReadAll(source)
			if err != nil || string(data) != "image" {
				return basebuild.Result{}, errors.New("bad upload")
			}
			return basebuild.Result{Base: core.BaseInfo{Name: req.Name, Revision: core.BaseRevision("sha256:" + strings.Repeat("b", 64))}, State: "ready"}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClientWithDialer(func(ctx context.Context) (net.Conn, error) {
		if _, deadline := ctx.Deadline(); deadline {
			t.Error("client inserted a fixed deadline")
		}
		return control.UnixDialer(socket)(ctx)
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.ImportBase(context.Background(), strings.NewReader("image"), basebuild.ImportRequest{Name: "tools"})
	if err != nil || result.State != "ready" {
		t.Fatal(result, err)
	}
	if _, err := client.ImportBase(context.Background(), strings.NewReader("image"), basebuild.ImportRequest{Name: "tools", MaxBytes: 4}); err == nil {
		t.Fatal("explicit client size cap ignored")
	}
}

func TestImportDeadlinePolicyPreservesOtherDomainsAndCaller(t *testing.T) {
	for _, mode := range []string{"base", "caller", "environment", "workspace"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			if mode == "caller" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Hour)
				defer cancel()
			}
			called := false
			client, err := NewClientWithDialer(func(got context.Context) (net.Conn, error) {
				called = true
				deadline, ok := got.Deadline()
				if mode == "base" {
					if ok {
						t.Error("unexpected Base deadline")
					}
				} else if !ok {
					t.Error("deadline removed from unrelated import")
				} else if mode == "caller" && time.Until(deadline) < 59*time.Minute {
					t.Error("caller deadline shortened")
				} else if mode != "caller" && (time.Until(deadline) > 30*time.Minute || time.Until(deadline) < 29*time.Minute) {
					t.Error("existing deadline changed")
				}
				return nil, core.ErrRuntimeUnavailable
			})
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "base", "caller":
				_, err = client.ImportBase(ctx, strings.NewReader("image"), basebuild.ImportRequest{Name: "tools"})
			case "environment":
				_, err = client.ImportEnvironment(ctx, strings.NewReader("image"), "dev")
			case "workspace":
				_, err = client.ImportWorkspace(ctx, strings.NewReader("image"), WorkspaceImportRequest{Name: "tools", Repository: "repo"})
			}
			if err == nil || !called {
				t.Fatal("dial policy not exercised", err)
			}
		})
	}
}

// Exercise the actual frame counter at the previous wire ceiling without
// pretending that a 64 GiB payload was transmitted in this component test.
func TestBaseImportFrameCanCrossFormerWireCap(t *testing.T) {
	for _, tc := range []struct {
		name         string
		count, limit int64
		allowed      bool
	}{
		{"base", environmentExportWireLimit, basebuild.MaxArchiveLimitBytes, true},
		{"environment", environmentExportWireLimit, environmentExportWireLimit, false},
		{"integer-boundary", basebuild.MaxArchiveLimitBytes, basebuild.MaxArchiveLimitBytes, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer func() { _ = client.Close() }()
			defer func() { _ = server.Close() }()
			sent := make(chan error, 1)
			go func() { sent <- json.NewEncoder(client).Encode(environmentImportUpload{Data: []byte("x")}) }()
			r := &environmentImportReader{reader: bufio.NewReaderSize(server, 128<<10), hash: sha256.New(), conn: server, count: tc.count, limit: tc.limit}
			var data [1]byte
			n, err := r.Read(data[:])
			if tc.allowed {
				if err != nil || n != 1 || data[0] != 'x' || r.count != tc.count+1 {
					t.Fatal(n, err, r.count)
				}
			} else if !errors.Is(err, control.ErrProtocol) {
				t.Fatal("limit not enforced", n, err)
			}
			if err := <-sent; err != nil {
				t.Fatal(err)
			}
		})
	}
}
