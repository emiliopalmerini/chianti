// Package httptest provides test-only helpers for code that builds the real
// httpx router but doesn't want real CSRF token plumbing.
package httptest

import (
	"html/template"
	"net/http"

	"github.com/emiliopalmerini/chianti/platform/httpx"
)

// CSRFBypass is a no-op middleware that stashes an empty CSRFField on ctx.
// Tests mount this in place of httpx.CSRFMiddleware.
func CSRFBypass() func(http.Handler) http.Handler {
	noop := httpx.CSRFField(func(*http.Request) template.HTML { return "" })
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := httpx.WithCSRFField(r.Context(), noop)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
