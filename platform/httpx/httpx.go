// Package httpx provides shared HTTP plumbing: chi router constructor,
// middleware chain, rate limiter, CSRF wiring, and error rendering for
// *apperror.Error. Slice HTTP adapters mount onto the returned router.
package httpx

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
	"golang.org/x/time/rate"

	"github.com/emiliopalmerini/chianti/kernel/apperror"
)

type ServerDeps struct {
	Production bool
	Logger     *slog.Logger
}

// NewRouter builds the base chi router with the platform middleware
// stack: RequestID, RealIP (production only), request logger, Recoverer,
// Compress(5), security headers (with HSTS in production).
func NewRouter(deps ServerDeps) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	if deps.Production {
		r.Use(middleware.RealIP)
	}
	r.Use(requestLogger(deps.Logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(securityHeaders(deps.Production))
	return r
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}

func securityHeaders(production bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Content-Security-Policy",
				"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; frame-src https://www.youtube-nocookie.com")
			if production {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CSRFField matches the signature gorilla/csrf.TemplateField produces, so
// real middleware and tests plug in interchangeably.
type CSRFField = func(*http.Request) template.HTML

type csrfFieldKey struct{}

// WithCSRFField returns a copy of ctx carrying f as the CSRFField that
// CSRFFieldFromContext will return. CSRFMiddleware uses this internally;
// adjacent packages (test helpers) can use it to install a stub field
// without depending on the context key type.
func WithCSRFField(ctx context.Context, f CSRFField) context.Context {
	return context.WithValue(ctx, csrfFieldKey{}, f)
}

// CSRFMiddleware returns gorilla/csrf configured for this app: SameSite=Lax,
// Path="/", Secure when production, with the given cookieName. The
// csrfKeyEncoded value is accepted as raw 32 bytes, base64, or hex.
func CSRFMiddleware(production bool, csrfKeyEncoded, cookieName string) (func(http.Handler) http.Handler, error) {
	key, err := decodeCSRFKey(csrfKeyEncoded)
	if err != nil {
		return nil, err
	}
	mw := csrf.Protect(key,
		csrf.Secure(production),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.Path("/"),
		csrf.CookieName(cookieName),
	)
	plaintext := !production
	return func(next http.Handler) http.Handler {
		wrapped := mw(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithCSRFField(r.Context(), CSRFField(csrf.TemplateField))
			if plaintext {
				ctx = context.WithValue(ctx, csrf.PlaintextHTTPContextKey, true)
			}
			wrapped.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// CSRFFieldFromContext returns the csrf.TemplateField function stashed by
// CSRFMiddleware, or a no-op when the middleware is not installed.
func CSRFFieldFromContext(ctx context.Context) CSRFField {
	if v, ok := ctx.Value(csrfFieldKey{}).(CSRFField); ok && v != nil {
		return v
	}
	return func(*http.Request) template.HTML { return "" }
}

// CSRFTokenFromRequest returns the raw CSRF token for the current request,
// suitable for embedding in a <meta name="csrf-token"> tag.
func CSRFTokenFromRequest(r *http.Request) string {
	return csrf.Token(r)
}

func decodeCSRFKey(s string) ([]byte, error) {
	if len(s) == 32 {
		return []byte(s), nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	return nil, errors.New("CSRF key must decode to 32 bytes (raw, base64, or hex)")
}

// BucketLimiter returns a token-bucket rate-limit middleware at rps requests
// per second with the given burst. Limit state is global (single bucket for
// all requests through the middleware); per-IP buckets are a slice concern.
func BucketLimiter(rps float64, burst int) func(http.Handler) http.Handler {
	lim := rate.NewLimiter(rate.Limit(rps), burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !lim.Allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RenderError translates an error (including *apperror.Error) into an HTTP
// response. Content-type negotiation: anything under /api/ or with
// Accept: application/json gets a JSON body; everything else gets text/plain.
func RenderError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	msg := "errore interno"
	switch {
	case apperror.Is(err, apperror.KindNotFound):
		status = http.StatusNotFound
		msg = err.Error()
	case apperror.Is(err, apperror.KindValidation):
		status = http.StatusUnprocessableEntity
		msg = err.Error()
	case apperror.Is(err, apperror.KindConflict):
		status = http.StatusConflict
		msg = err.Error()
	case apperror.Is(err, apperror.KindUnauthorized):
		status = http.StatusUnauthorized
		msg = err.Error()
	case apperror.Is(err, apperror.KindForbidden):
		status = http.StatusForbidden
		msg = err.Error()
	}

	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":{"message":` + jsonString(msg) + `}}`))
		return
	}
	http.Error(w, msg, status)
}

func wantsJSON(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		return true
	}
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
