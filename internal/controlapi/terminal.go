package controlapi

import (
	"context"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func shellTerminalContext(ctx context.Context, metadata core.TerminalMetadata) context.Context {
	if metadata.Columns > 0 && metadata.Rows > 0 {
		updates := make(chan core.TerminalSize, 1)
		metadata.Resizes = updates
		control.SetTerminalResizeHandler(ctx, func(columns, rows int) error {
			// The control transport serializes callbacks. A slow PTY consumes
			// the latest size without growing a queue during window dragging.
			select {
			case <-updates:
			default:
			}
			updates <- core.TerminalSize{Columns: columns, Rows: rows}
			return nil
		})
	}
	return core.WithTerminalMetadata(ctx, metadata)
}
