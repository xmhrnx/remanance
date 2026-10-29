package fsys

import "context"

type deepKey struct{}

// WithDeep marks a parse as part of a deep scan: parsers may then read the
// whole volume looking for orphaned metadata, which takes as long as
// reading the device.
func WithDeep(ctx context.Context) context.Context {
	return context.WithValue(ctx, deepKey{}, true)
}

// IsDeep reports whether ctx asks for a deep parse.
func IsDeep(ctx context.Context) bool {
	v, _ := ctx.Value(deepKey{}).(bool)
	return v
}
