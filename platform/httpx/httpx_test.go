package httpx_test

import (
	"errors"
	"html/template"
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
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	return httpx.NewHandler(httpx.ServerDeps{
		Production: production,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, mux)
}

func TestCSPOverride_renders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	h := httpx.NewHandler(httpx.ServerDeps{
		Production: false,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		CSP: &httpx.CSP{
			DefaultSrc: []string{"'self'"},
			ScriptSrc:  []string{"'self'", "https://unpkg.com", "https://cdnjs.cloudflare.com"},
			ImgSrc:     []string{"'self'", "data:", "https://upload.wikimedia.org"},
		},
	}, mux)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	got := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{
		"default-src 'self'",
		"script-src 'self' https://unpkg.com https://cdnjs.cloudflare.com",
		"img-src 'self' data: https://upload.wikimedia.org",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("CSP missing %q in %q", want, got)
		}
	}
	// Default frame-src not present when override does not include it.
	if strings.Contains(got, "youtube-nocookie") {
		t.Errorf("override should drop default frame-src: %q", got)
	}
}

func TestCSPString_omitsEmptyDirectives(t *testing.T) {
	c := httpx.CSP{
		DefaultSrc: []string{"'self'"},
		ScriptSrc:  nil,
	}
	got := c.String()
	if got != "default-src 'self'" {
		t.Fatalf("got %q", got)
	}
}

func TestCSPStringRejectsInvalidSource(t *testing.T) {
	cases := []struct {
		name string
		csp  httpx.CSP
	}{
		{
			name: "empty source",
			csp:  httpx.CSP{DefaultSrc: []string{"'self'", ""}},
		},
		{
			name: "semicolon injection",
			csp:  httpx.CSP{ScriptSrc: []string{"'self'; report-uri https://evil.example/report"}},
		},
		{
			name: "newline injection",
			csp:  httpx.CSP{ImgSrc: []string{"'self'\nX-Evil: yes"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.csp.HeaderValue(); err == nil {
				t.Fatal("expected invalid CSP source error, got nil")
			}
		})
	}
}

func TestNewHandlerSetsSecurityHeaders(t *testing.T) {
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

func TestNewHandlerRecoversPanic(t *testing.T) {
	h := newTestRouter(t, false)
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestWithCSRFFieldInjectsField(t *testing.T) {
	var got string
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			field := func(*http.Request) template.HTML {
				return template.HTML(`<input type="hidden" name="csrf" value="test">`)
			}
			next.ServeHTTP(w, r.WithContext(httpx.WithCSRFField(r.Context(), field)))
		})
	}
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := httpx.CSRFFieldFromContext(r.Context())
		if f == nil {
			t.Fatal("CSRFField nil after injection")
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
	t.Run("weighted-json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/page", nil)
		req.Header.Set("Accept", "text/html;q=0.5, application/json;q=0.9")
		rec := httptest.NewRecorder()
		httpx.RenderError(rec, req, apperror.Conflict("dup"))
		if got := rec.Result().Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
	})
	t.Run("invalid-json-substring", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/page", nil)
		req.Header.Set("Accept", "text/application/jsonish")
		rec := httptest.NewRecorder()
		httpx.RenderError(rec, req, apperror.Conflict("dup"))
		if got := rec.Result().Header.Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
			t.Errorf("Content-Type = %q, want text/plain", got)
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

func TestRenderErrorDoesNotLeakInternalDetail(t *testing.T) {
	err := apperror.NotFound("event", "internal-id-123")

	t.Run("plain", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/page", nil)
		rec := httptest.NewRecorder()
		httpx.RenderError(rec, req, err)

		body := rec.Body.String()
		if strings.Contains(body, "internal-id-123") {
			t.Fatalf("response leaked internal detail: %q", body)
		}
		if !strings.Contains(body, "event non trovato") {
			t.Fatalf("response missing public message: %q", body)
		}
	})

	t.Run("json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/page", nil)
		rec := httptest.NewRecorder()
		httpx.RenderError(rec, req, err)

		body := rec.Body.String()
		if strings.Contains(body, "internal-id-123") {
			t.Fatalf("JSON response leaked internal detail: %q", body)
		}
		if !strings.Contains(body, "event non trovato") {
			t.Fatalf("JSON response missing public message: %q", body)
		}
	})
}
