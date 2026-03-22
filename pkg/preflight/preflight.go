package preflight

import (
	"context"
	"errors"
)

// ErrWorkFound is returned when a kernel package would be processed (download +
// BTF generation) in normal mode. Used only with FromContext(ctx) == true.
var ErrWorkFound = errors.New("preflight: archive needs updates for at least one kernel package")

type ctxKey struct{}

// WithContext marks ctx so processPackage stops before enqueueing jobs and
// returns ErrWorkFound when a package would otherwise be processed.
func WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, true)
}

// FromContext reports whether preflight (no job enqueue) is active.
func FromContext(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKey{}).(bool)
	return v
}
