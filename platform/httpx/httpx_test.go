package httpx_test

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emiliopalmerini/chianti/kernel/apperror"
	"github.com/emiliopalmerini/chianti/platform/httpx"
)

func newTestRouter(t *testing.T, production bool) http.Handler {
	t.Helper()
	r := httpx.NewRouter(httpx.ServerDeps{
		Production: production,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	r.Get("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	return r
}

func TestNewRouterSetsSecurityHeaders(t *testing.T) {
	for _, prod := range []bool{false, true} {
		t.Run(map[bool]string{false: "dev", true: "prod"}[prod], func(t *testing.T) {
			h := newTestRouter(t, prod)
			req := httptest.NewRequest(http.MethodGet, "/ok", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			hdr := rec.Result().Header
			if got := hdr.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q", got)
			}
			if got := hdr.Get("X-Frame-Options"); got != "DENY" {
				t.Errorf("X-Frame-Options = %q", got)
			}
			if got := hdr.Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
				t.Errorf("Referrer-Policy = %q", got)
			}
			csp := hdr.Get("Content-Security-Policy")
			if !strings.Contains(csp, "default-src 'self'") {
				t.Errorf("CSP missing default-src 'self': %q", csp)
			}
			if !strings.Contains(csp, "youtube-nocookie.com") {
				t.Errorf("CSP missing youtube-nocookie frame-src: %q", csp)
			}
			hsts := hdr.Get("Strict-Transport-Security")
			if prod && hsts == "" {
				t.Errorf("HSTS missing in production")
			}
			if !prod && hsts != "" {
				t.Errorf("HSTS present in dev: %q", hsts)
			}
		})
	}
}

func TestNewRouterRecoversPanic(t *testing.T) {
	h := newTestRouter(t, false)
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestCSRFMiddlewareDecodesKey(t *testing.T) {
	raw := strings.Repeat("k", 32)
	cases := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"raw32", raw, false},
		{"base64", base64.StdEncoding.EncodeToString([]byte(raw)), false},
		{"hex", hex.EncodeToString([]byte(raw)), false},
		{"too-short", "short", true},
		{"garbage", "!!!not-base64-not-hex-and-not-32!!!", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := httpx.CSRFMiddleware(false, tc.key, "test_csrf")
			if tc.wantErr && err == nil {
				t.Fatalf("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCSRFMiddlewareInjectsField(t *testing.T) {
	mw, err := httpx.CSRFMiddleware(false, strings.Repeat("k", 32), "test_csrf")
	if err != nil {
		t.Fatal(err)
	}
	var got string
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := httpx.CSRFFieldFromContext(r.Context())
		if f == nil {
			t.Fatal("CSRFField nil after middleware")
		}
		got = string(f(r))
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(got, "<input") {
		t.Fatalf("CSRFField produced %q, want hidden input", got)
	}
}

func TestCSRFFieldFromContextNoOpWhenMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	f := httpx.CSRFFieldFromContext(req.Context())
	if f == nil {
		t.Fatal("CSRFField nil")
	}
	if got := string(f(req)); got != "" {
		t.Fatalf("CSRFField = %q, want empty", got)
	}
}

func TestBucketLimiterRejectsOverBudget(t *testing.T) {
	mw := httpx.BucketLimiter(1, 1)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first call status = %d, want 200", rec1.Code)
	}
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second call status = %d, want 429", rec2.Code)
	}
}

func TestRenderError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"NotFound", apperror.NotFound("event", "abc"), http.StatusNotFound},
		{"Validation", apperror.Validation("bad", nil), http.StatusUnprocessableEntity},
		{"Conflict", apperror.Conflict("dup"), http.StatusConflict},
		{"Unauthorized", apperror.Unauthorized("no"), http.StatusUnauthorized},
		{"Forbidden", apperror.Forbidden("no"), http.StatusForbidden},
		{"Plain", errors.New("oops"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/page", nil)
			rec := httptest.NewRecorder()
			httpx.RenderError(rec, req, tc.err)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if ct := rec.Result().Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
				t.Errorf("Content-Type = %q, want text/plain", ct)
			}
		})
	}
}

func TestRenderErrorJSON(t *testing.T) {
	t.Run("api-path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/foo", nil)
		rec := httptest.NewRecorder()
		httpx.RenderError(rec, req, apperror.NotFound("x", "y"))
		if got := rec.Result().Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		if !strings.Contains(rec.Body.String(), `"error"`) {
			t.Errorf("body = %q, want JSON error", rec.Body.String())
		}
	})
	t.Run("accept-json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/page", nil)
		req.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()
		httpx.RenderError(rec, req, apperror.Conflict("dup"))
		if got := rec.Result().Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
	})
	t.Run("plain-default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/page", nil)
		rec := httptest.NewRecorder()
		httpx.RenderError(rec, req, apperror.Conflict("dup"))
		if got := rec.Result().Header.Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
			t.Errorf("Content-Type = %q, want text/plain", got)
		}
	})
}
