package spork

import "context"

type idempotencyKeyCtxKey struct{}

// WithIdempotencyKey attaches an Idempotency-Key to ctx. The SDK sends the
// value as the "Idempotency-Key" HTTP header on every request made with
// this context, allowing a POST or PATCH to be safely retried without
// creating duplicate resources.
//
// Keys should be unique per logical operation (e.g., a UUID per "create
// this one monitor" call). The server replays the original response for
// up to a bounded time window when the same key is re-submitted.
//
// Example:
//
//	ctx = spork.WithIdempotencyKey(ctx, uuid.NewString())
//	monitor, err := client.CreateMonitor(ctx, &spork.Monitor{...})
//
// See the API reference for the current replay window.
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	if key == "" {
		return ctx
	}
	return context.WithValue(ctx, idempotencyKeyCtxKey{}, key)
}

// IdempotencyKeyFromContext returns the key previously attached by
// WithIdempotencyKey, or "" if none is present. Primarily useful to
// middleware that wants to inspect the key before the HTTP call.
func IdempotencyKeyFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(idempotencyKeyCtxKey{}).(string); ok {
		return v
	}
	return ""
}
