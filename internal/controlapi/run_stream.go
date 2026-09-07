package controlapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
)

// Match the control envelope bound while keeping execution cancellation separate
// from ordinary lifecycle RPCs, which may intentionally finish after disconnect.
const maxRunResultBytes = 16 << 20

func registerRun(server *control.Server, runner runService) error {
	return server.RegisterStream(MethodRun, func(_ context.Context, payload json.RawMessage) (control.Stream, error) {
		var request runapp.Spec
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.WorkspacePath) == "" || len(request.Argv) == 0 {
			return nil, control.NewStatusError("invalid_argument", "workspace_path and argv are required")
		}
		return func(ctx context.Context, conn net.Conn) error {
			runCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			stopped := make(chan struct{})
			go func() {
				defer close(stopped)
				// No input frames are defined. EOF, an error, or unsolicited bytes all
				// terminate the caller's ownership of this execution.
				var b [1]byte
				_, _ = conn.Read(b[:])
				cancel()
			}()
			defer func() { _ = conn.Close(); <-stopped }()
			result, err := runner.Run(runCtx, request)
			// Run performs canonical, bounded cleanup using an uncancelled context.
			// Disconnection cannot turn unknown cleanup into a successful result.
			data, marshalErr := json.Marshal(runResponse{Result: result, Error: statusFromError(err)})
			if marshalErr != nil {
				return marshalErr
			}
			if len(data)+1 > maxRunResultBytes {
				return fmt.Errorf("run result exceeds size limit: %w", control.ErrProtocol)
			}
			if err := conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
				return err
			}
			_, err = conn.Write(append(data, '\n'))
			return err
		}, nil
	})
}
