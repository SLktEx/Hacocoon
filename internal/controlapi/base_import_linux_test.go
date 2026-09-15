//go:build linux

package controlapi

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestBaseImportTransportUsesBytesAndKeepsUncertainBuilder(t *testing.T) {
	for _, failure := range []bool{false, true} {
		calls := 0
		socket := doctorTestSocket(t, func(s *control.Server) {
			if err := RegisterBaseImport(s, func(_ context.Context, r io.Reader, req basebuild.ImportRequest) (basebuild.Result, error) {
				calls++
				raw, err := io.ReadAll(r)
				if err != nil {
					return basebuild.Result{}, err
				}
				if string(raw) != "image bytes" {
					t.Error("wrong bytes")
				}
				result := basebuild.Result{Base: core.BaseInfo{Name: req.Name, Revision: core.BaseRevision("sha256:" + strings.Repeat("a", 64))}, State: "ready"}
				if failure {
					result.State = "publication-unconfirmed"
					result.Builder = "build-" + strings.Repeat("b", 32)
					return result, core.ErrRecoveryRequired
				}
				return result, nil
			}); err != nil {
				t.Fatal(err)
			}
		})
		c, err := NewClient(socket)
		if err != nil {
			t.Fatal(err)
		}
		result, err := c.ImportBase(context.Background(), strings.NewReader("image bytes"), basebuild.ImportRequest{Name: "imported"})
		if (err != nil) != failure || calls != 1 || result.Base.Name != "imported" || (failure && result.Builder == "") {
			t.Fatal(result, err, calls)
		}
		for _, raw := range []string{`{"name":"../base"}`, `{"name":"ok","path":"/etc/passwd"}`, `{"name":"ok","privileged":true}`, `{"name":"--public"}`} {
			conn, err := c.wire.OpenStream(context.Background(), MethodBaseImport, json.RawMessage(raw))
			if conn != nil {
				_ = conn.Close()
			}
			if err == nil {
				t.Fatal("unsafe request accepted", raw)
			}
		}
		if calls != 1 {
			t.Fatal("invalid request reached builder")
		}
	}
}
func TestBaseImportRefusesMalformedResult(t *testing.T) {
	for _, r := range []basebuild.Result{{State: "unknown"}, {State: "ready", Builder: "../owned"}, {State: "ready", Base: core.BaseInfo{Revision: "mutable"}}, {State: "ready", Execution: &core.ExecutionResult{Stdout: "private"}}} {
		if validBaseImportResult(r) {
			t.Fatal("malformed result accepted", r)
		}
	}
}
