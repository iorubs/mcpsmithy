package server

import (
	"context"
	"net/http"
)

// mergedCtx falls back to a second context for value lookups while
// taking deadline and cancellation from the first. Used to propagate
// caller-side slog context values (smithy-cli's service/kind) into
// per-request handler chains that synthesize their own contexts.
type mergedCtx struct {
	context.Context
	values context.Context
}

func (m mergedCtx) Value(key any) any {
	if v := m.Context.Value(key); v != nil {
		return v
	}
	return m.values.Value(key)
}

// withCtxValues wraps an http.Handler so that every request context
// inherits values from base while keeping its own cancellation and
// deadline.
func withCtxValues(base context.Context, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(mergedCtx{Context: r.Context(), values: base})
		h.ServeHTTP(w, r)
	})
}
