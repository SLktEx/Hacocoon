//go:build linux

package controlapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
)

func transferWireBundle(t *testing.T) *environmenttransfer.Staged {
	t.Helper()
	var bundle bytes.Buffer
	manifest := environmenttransfer.Manifest{Version: 1, Source: "dev"}
	var readers []io.Reader
	for _, role := range []string{"rootfs", "workspace"} {
		data := bytes.Repeat([]byte(role), 20000)
		digest := sha256.Sum256(data)
		manifest.Components = append(manifest.Components, environmenttransfer.Component{Role: role, Bytes: int64(len(data)), SHA256: hex.EncodeToString(digest[:])})
		readers = append(readers, bytes.NewReader(data))
	}
	if err := environmenttransfer.Write(&bundle, manifest, readers, 1<<20); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	staged, err := environmenttransfer.Stage(context.Background(), root, &bundle, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return staged
}
func TestEnvironmentExportStreamVerifiesCompleteBundle(t *testing.T) {
	staged := transferWireBundle(t)
	expected, err := io.ReadAll(staged.Reader())
	if err != nil {
		t.Fatal(err)
	}
	socket := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterEnvironmentExport(server, func(ctx context.Context, source string) (environmenttransfer.ExportResult, error) {
			if source != "dev" {
				t.Error(source)
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Error("missing deadline")
			}
			return environmenttransfer.ExportResult{Bundle: staged}, nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(socket)
	var output bytes.Buffer
	result, err := client.ExportEnvironment(context.Background(), "dev", &output)
	if err != nil || result.Bytes != int64(len(expected)) || !bytes.Equal(output.Bytes(), expected) {
		t.Fatal(result, err)
	}
	if _, err := environmenttransfer.Inspect(bytes.NewReader(output.Bytes()), 1<<20); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(staged.Reader()); err == nil {
		t.Fatal("server leaked bundle handle")
	}
}
func TestEnvironmentExportStreamRetainsCleanupIdentity(t *testing.T) {
	snapshot := "snap-" + strings.Repeat("a", 32)
	socket := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterEnvironmentExport(server, func(context.Context, string) (environmenttransfer.ExportResult, error) {
			return environmenttransfer.ExportResult{TemporarySnapshot: snapshot}, core.ErrRecoveryRequired
		}); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(socket)
	var output bytes.Buffer
	result, err := client.ExportEnvironment(context.Background(), "dev", &output)
	if err == nil || result.TemporarySnapshot != snapshot || output.Len() != 0 {
		t.Fatal(result, err, output.Len())
	}
}
func TestEnvironmentExportStreamRejectsMalformedCompletion(t *testing.T) {
	for _, which := range []string{"eof", "digest", "count", "both", "extra", "duplicate", "large", "cleanup", "empty"} {
		t.Run(which, func(t *testing.T) {
			socket := doctorTestSocket(t, func(server *control.Server) {
				server.RegisterStream(MethodEnvironmentExport, func(context.Context, json.RawMessage) (control.Stream, error) {
					return func(_ context.Context, conn net.Conn) error {
						enc := json.NewEncoder(conn)
						data := []byte("archive bytes")
						sum := sha256.Sum256(data)
						result := &EnvironmentExportResult{Bytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
						switch which {
						case "duplicate":
							_, err := io.WriteString(conn, `{"data":"eA==","data":"eQ=="}`+"\n")
							return err
						case "large":
							return enc.Encode(environmentExportFrame{Data: bytes.Repeat([]byte("x"), 65<<10)})
						case "empty":
							return enc.Encode(environmentExportFrame{})
						}
						if err := enc.Encode(environmentExportFrame{Data: data}); err != nil {
							return err
						}
						if which == "eof" {
							return nil
						}
						frame := environmentExportFrame{Result: result}
						switch which {
						case "digest":
							result.SHA256 = strings.Repeat("0", 64)
						case "count":
							result.Bytes++
						case "both":
							frame.Data = data
						case "cleanup":
							result.TemporarySnapshot = "snap-" + strings.Repeat("a", 32)
						}
						if err := enc.Encode(frame); err != nil {
							return err
						}
						if which == "extra" {
							return enc.Encode(frame)
						}
						return nil
					}, nil
				})
			})
			client, _ := NewClient(socket)
			var output bytes.Buffer
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if _, err := client.ExportEnvironment(ctx, "dev", &output); err == nil {
				t.Fatal("malformed completion succeeded")
			}
		})
	}
}
func TestEnvironmentExportDisconnectCancelsCapture(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	socket := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterEnvironmentExport(server, func(ctx context.Context, _ string) (environmenttransfer.ExportResult, error) {
			close(started)
			<-ctx.Done()
			defer close(stopped)
			return environmenttransfer.ExportResult{}, ctx.Err()
		}); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(socket)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := client.ExportEnvironment(ctx, "dev", io.Discard); done <- err }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("capture not started")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancel succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("client did not stop")
	}
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("server did not cancel capture")
	}
}
func TestEnvironmentExportRequestRejectsHostPathsAndDuplicateKeys(t *testing.T) {
	called := false
	socket := doctorTestSocket(t, func(server *control.Server) {
		RegisterEnvironmentExport(server, func(context.Context, string) (environmenttransfer.ExportResult, error) {
			called = true
			return environmenttransfer.ExportResult{}, errors.New("unexpected capture")
		})
	})
	wire, _ := control.NewClient(control.UnixDialer(socket))
	for _, raw := range []string{`{"source":"dev","path":"/host"}`, `{"source":"dev","source":"other"}`, `{"source":"--all"}`, `{}`} {
		conn, err := wire.OpenStream(context.Background(), MethodEnvironmentExport, json.RawMessage(raw))
		if conn != nil {
			conn.Close()
		}
		if err == nil {
			t.Fatal("invalid request accepted", raw)
		}
	}
	if called {
		t.Fatal("invalid request mutated source")
	}
}
