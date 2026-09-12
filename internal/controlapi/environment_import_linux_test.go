//go:build linux

package controlapi

import (
	"bufio"
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

func TestEnvironmentImportStreamStagesBeforeActivation(t *testing.T) {
	staged := transferWireBundle(t)
	defer staged.Close()
	input, err := io.ReadAll(staged.Reader())
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "running", true: "retained"}[failure], func(t *testing.T) {
			root := t.TempDir()
			os.Chmod(root, 0700)
			socket := doctorTestSocket(t, func(s *control.Server) {
				if err := RegisterEnvironmentImport(s, func(ctx context.Context, r io.Reader, name string) (environmenttransfer.ImportResult, error) {
					if name != "destination" {
						t.Error(name)
					}
					copy, err := environmenttransfer.Stage(ctx, root, r, 1<<20)
					if err != nil {
						return environmenttransfer.ImportResult{}, err
					}
					defer copy.Close()
					data, err := io.ReadAll(copy.Reader())
					if err != nil || !bytes.Equal(data, input) {
						t.Error("upload bytes changed", err)
					}
					result := environmenttransfer.ImportResult{Environment: name, Workspace: "import-0123456789abcdef", State: "running"}
					if failure {
						result.State = "cleanup-required"
						return result, core.ErrRecoveryRequired
					}
					return result, nil
				}); err != nil {
					t.Fatal(err)
				}
			})
			client, _ := NewClient(socket)
			result, err := client.ImportEnvironment(context.Background(), bytes.NewReader(input), "destination")
			if failure {
				var status *control.StatusError
				if !errors.As(err, &status) || status.Code != "recovery_required" || result.State != "cleanup-required" || result.Workspace == "" {
					t.Fatal(result, err)
				}
			} else if err != nil || result.State != "running" {
				t.Fatal(result, err)
			}
		})
	}
}
func TestEnvironmentImportRejectsMalformedUploadBeforeActivation(t *testing.T) {
	bundle := transferWireBundle(t)
	defer bundle.Close()
	validBytes, err := io.ReadAll(bundle.Reader())
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"empty", "missing-end", "digest", "count", "mixed", "duplicate", "oversize", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			os.Chmod(root, 0700)
			activated := make(chan struct{}, 1)
			finished := make(chan struct{})
			socket := doctorTestSocket(t, func(s *control.Server) {
				RegisterEnvironmentImport(s, func(ctx context.Context, r io.Reader, _ string) (environmenttransfer.ImportResult, error) {
					defer close(finished)
					staged, err := environmenttransfer.Stage(ctx, root, r, 1<<20)
					if err == nil {
						defer staged.Close()
						activated <- struct{}{}
					}
					return environmenttransfer.ImportResult{}, err
				})
			})
			wire, _ := control.NewClient(control.UnixDialer(socket))
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			conn, err := wire.OpenStream(ctx, MethodEnvironmentImport, EnvironmentImportRequest{})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			sendBundle := func() {
				encoder := json.NewEncoder(conn)
				for at := 0; at < len(validBytes); {
					end := min(at+(64<<10), len(validBytes))
					if err := encoder.Encode(environmentImportUpload{Data: validBytes[at:end]}); err != nil {
						t.Fatal(err)
					}
					at = end
				}
			}
			switch mode {
			case "empty":
				io.WriteString(conn, "{}\n")
			case "missing-end":
				sendBundle()
				conn.Close()
			case "digest", "count", "mixed":
				data := validBytes
				sum := sha256.Sum256(data)
				end := &environmentImportEnd{Bytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
				sendBundle()
				if mode == "digest" {
					end.SHA256 = strings.Repeat("0", 64)
				}
				if mode == "count" {
					end.Bytes++
				}
				frame := environmentImportUpload{End: end}
				if mode == "mixed" {
					frame.Data = []byte("a")
				}
				json.NewEncoder(conn).Encode(frame)
			case "duplicate":
				io.WriteString(conn, "{\"data\":\"YQ==\",\"data\":\"YQ==\"}\n")
			case "oversize":
				json.NewEncoder(conn).Encode(environmentImportUpload{Data: make([]byte, 65537)})
			case "unknown":
				io.WriteString(conn, "{\"path\":\"/etc/shadow\"}\n")
			}
			if mode != "missing-end" {
				var response environmentImportResponse
				if err := json.NewDecoder(conn).Decode(&response); err != nil {
					t.Fatal(err)
				}
				if response.Error == nil {
					t.Fatal("accepted invalid upload")
				}
			}
			select {
			case <-finished:
			case <-ctx.Done():
				t.Fatal("handler did not complete")
			}
			select {
			case <-activated:
				t.Fatal("native activation before valid upload")
			default:
			}
		})
	}
}
func TestEnvironmentImportDisconnectCancelsActivation(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	socket := doctorTestSocket(t, func(s *control.Server) {
		RegisterEnvironmentImport(s, func(ctx context.Context, r io.Reader, _ string) (environmenttransfer.ImportResult, error) {
			if _, err := io.Copy(io.Discard, r); err != nil {
				return environmenttransfer.ImportResult{}, err
			}
			close(started)
			<-ctx.Done()
			close(stopped)
			return environmenttransfer.ImportResult{}, ctx.Err()
		})
	})
	wire, _ := control.NewClient(control.UnixDialer(socket))
	conn, err := wire.OpenStream(context.Background(), MethodEnvironmentImport, EnvironmentImportRequest{})
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("a")
	sum := sha256.Sum256(data)
	encoder := json.NewEncoder(conn)
	encoder.Encode(environmentImportUpload{Data: data})
	encoder.Encode(environmentImportUpload{End: &environmentImportEnd{Bytes: 1, SHA256: hex.EncodeToString(sum[:])}})
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("activation not reached")
	}
	conn.Close()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("disconnect did not cancel")
	}
}
func TestEnvironmentImportRequestRejectsAuthorityAndDuplicateName(t *testing.T) {
	socket := doctorTestSocket(t, func(s *control.Server) {
		RegisterEnvironmentImport(s, func(context.Context, io.Reader, string) (environmenttransfer.ImportResult, error) {
			t.Error("invalid request invoked import")
			return environmenttransfer.ImportResult{}, nil
		})
	})
	wire, _ := control.NewClient(control.UnixDialer(socket))
	for _, raw := range []string{`{"environment":"x","path":"/etc/shadow"}`, `{"environment":"x","environment":"y"}`, `{"environment":"../x"}`, `{"limit":1}`, `{"owner":"x"}`} {
		conn, err := wire.OpenStream(context.Background(), MethodEnvironmentImport, json.RawMessage(raw))
		if conn != nil {
			conn.Close()
		}
		if err == nil {
			t.Fatal(raw)
		}
	}
}
func TestEnvironmentImportClientRejectsMissingOrFalseCompletion(t *testing.T) {
	for _, response := range []string{"", `{"result":{"environment":"dev","workspace":"work","state":"failed"}}`, `{"result":{"environment":"other","workspace":"work","state":"running"}}`} {
		socket := doctorTestSocket(t, func(s *control.Server) {
			s.RegisterStream(MethodEnvironmentImport, func(context.Context, json.RawMessage) (control.Stream, error) {
				return func(_ context.Context, c net.Conn) error {
					reader := &environmentImportReader{reader: bufio.NewReaderSize(c, 128<<10), hash: sha256.New(), conn: c, limit: environmentExportWireLimit}
					io.Copy(io.Discard, reader)
					if response != "" {
						io.WriteString(c, response+"\n")
					}
					return nil
				}, nil
			})
		})
		client, _ := NewClient(socket)
		if _, err := client.ImportEnvironment(context.Background(), strings.NewReader("a"), "dev"); err == nil {
			t.Fatal("accepted", response)
		}
	}
}

func TestEnvironmentImportClientPreservesEarlyRejection(t *testing.T) {
	socket := doctorTestSocket(t, func(s *control.Server) {
		RegisterEnvironmentImport(s, func(context.Context, io.Reader, string) (environmenttransfer.ImportResult, error) {
			return environmenttransfer.ImportResult{}, core.ErrUnsupported
		})
	})
	client, _ := NewClient(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.ImportEnvironment(ctx, bytes.NewReader(make([]byte, 1<<20)), "dev")
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("early rejection deadlocked", err)
	}
}
func TestEnvironmentImportResultRejectsHostData(t *testing.T) {
	for _, r := range []environmenttransfer.ImportResult{
		{Environment: "/etc/shadow"}, {Workspace: "../work"}, {OCI: "/var/lib/incus"}, {State: "arbitrary"}, {Offline: []string{"\x1b[31m"}},
	} {
		if validImportResult(r) {
			t.Fatal(r)
		}
	}
}
