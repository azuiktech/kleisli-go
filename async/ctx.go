package async

import "context"

// Ctx[T] couples a value with its execution context.
// A value type — create per-request, discard after.
// Useful for types without a native WithContext method (e.g. *http.Client).
// Types that do support WithContext (e.g. *gorm.DB) should use that directly.
type Ctx[T any] struct {
	Context context.Context
	Val     T
}

// InCtx creates a Ctx[T].
func InCtx[T any](ctx context.Context, val T) Ctx[T] {
	return Ctx[T]{Context: ctx, Val: val}
}
