//go:build linux

package controlapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
)

// RegisterEnvironmentExport accepts no client-selected host path or archive budget.
// Its callback uses trusted controller configuration and canonical source ownership.
func RegisterEnvironmentExport(server *control.Server, export func(context.Context, string) (environmenttransfer.ExportResult, error)) error {
	if export == nil {
		return core.ErrInvalidArgument
	}
	return server.RegisterStream(MethodEnvironmentExport, func(_ context.Context, payload json.RawMessage) (control.Stream, error) {
		var req EnvironmentExportRequest
		if decodeExportJSON(payload, &req) != nil || !exportSourcePattern.MatchString(req.Source) {
			return nil, control.ErrInvalidArgument
		}
		return func(ctx context.Context, conn net.Conn) error {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
			defer cancel()
			disconnected := make(chan struct{})
			go func() { defer close(disconnected); var b [1]byte; _, _ = conn.Read(b[:]); cancel() }()
			defer func() { _ = conn.Close(); <-disconnected }()
			encoder := json.NewEncoder(conn)
			send := func(frame environmentExportFrame) error {
				if err := conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
					return err
				}
				return encoder.Encode(frame)
			}
			saved, err := export(ctx, req.Source)
			result := EnvironmentExportResult{TemporarySnapshot: saved.TemporarySnapshot}
			if err == nil && (saved.Bundle == nil || saved.TemporarySnapshot != "") {
				err = core.ErrRecoveryRequired
			}
			if err != nil {
				if saved.Bundle != nil {
					err = errors.Join(err, saved.Bundle.Close())
				}
				return send(environmentExportFrame{Result: &result, Error: statusFromError(err)})
			}
			// Only the completed private bundle is streamed. Closing its handle is part of
			// success; neither source cleanup failure nor a late close can produce success.
			hash := sha256.New()
			reader := saved.Bundle.Reader()
			buffer := make([]byte, 64<<10)
			for {
				if err = ctx.Err(); err != nil {
					break
				}
				var n int
				n, err = reader.Read(buffer)
				if n > 0 {
					if int64(n) > environmentExportWireLimit-result.Bytes {
						err = core.ErrInvalidArgument
						break
					}
					if writeErr := send(environmentExportFrame{Data: buffer[:n]}); writeErr != nil {
						err = writeErr
						break
					}
					hash.Write(buffer[:n])
					result.Bytes += int64(n)
				}
				if err == io.EOF {
					err = nil
					break
				}
				if err != nil {
					break
				}
			}
			err = errors.Join(err, saved.Bundle.Close())
			if err == nil {
				result.SHA256 = hex.EncodeToString(hash.Sum(nil))
			}
			return send(environmentExportFrame{Result: &result, Error: statusFromError(err)})
		}, nil
	})
}
