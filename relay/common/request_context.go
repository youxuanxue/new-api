package common

import "context"

type upstreamRequestContextKey struct{}

// WithUpstreamRequestContext opts a relay into caller-owned transport cancellation
// and joining stream workers before its context or writer can be reused.
func WithUpstreamRequestContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, upstreamRequestContextKey{}, true)
}

func UpstreamRequestContextEnabled(ctx context.Context) bool {
	enabled, _ := ctx.Value(upstreamRequestContextKey{}).(bool)
	return enabled
}
