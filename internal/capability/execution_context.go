package capability

import "context"

type executionRequestKey struct{}

// ExecutionRequestID correlates provider-specific durable receipts with the
// common service's request. It is metadata, never a grant of authority.
func ExecutionRequestID(ctx context.Context) string {
	id, _ := ctx.Value(executionRequestKey{}).(string)
	return id
}
