package core

import "context"

// TerminalMetadata describes the narrow terminal identity/capability metadata
// that may be propagated from a Hacocoon client to a managed interactive PTY.
// It intentionally does not carry arbitrary environment variables.
type TerminalMetadata struct {
	Term      string
	ColorTerm string
	Columns   int
	Rows      int
}

type terminalMetadataContextKey struct{}

// WithTerminalMetadata associates validated terminal metadata with one
// controller request/session context.
func WithTerminalMetadata(ctx context.Context, metadata TerminalMetadata) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, terminalMetadataContextKey{}, metadata)
}

// TerminalMetadataFromContext returns request-scoped terminal metadata when
// present. The zero value means no terminal metadata was supplied.
func TerminalMetadataFromContext(ctx context.Context) TerminalMetadata {
	if ctx == nil {
		return TerminalMetadata{}
	}
	metadata, _ := ctx.Value(terminalMetadataContextKey{}).(TerminalMetadata)
	return metadata
}
