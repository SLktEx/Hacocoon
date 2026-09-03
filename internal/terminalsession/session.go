package terminalsession

import "context"

// Size is one terminal geometry update in character cells.
type Size struct {
	Columns int
	Rows    int
}

// ResizeSource exposes terminal resize events for one interactive session.
// Transport packages can implement this without making Core depend on their
// wire protocol.
type ResizeSource interface {
	ResizeEvents() <-chan Size
}

type resizeSourceContextKey struct{}

// WithResizeSource associates a resize event source with one prepared session.
func WithResizeSource(ctx context.Context, source ResizeSource) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if source == nil {
		return ctx
	}
	return context.WithValue(ctx, resizeSourceContextKey{}, source)
}

// ResizeSourceFromContext returns the session resize source when present.
func ResizeSourceFromContext(ctx context.Context) ResizeSource {
	if ctx == nil {
		return nil
	}
	source, _ := ctx.Value(resizeSourceContextKey{}).(ResizeSource)
	return source
}
