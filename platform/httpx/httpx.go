// Package httpx provides shared HTTP plumbing: chi router constructor,
// middleware chain, rate limiter, CSRF wiring, and error rendering for
// *apperror.Error. Slice HTTP adapters mount onto the returned router.
package httpx

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
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
	// CSP, when non-nil, overrides the default Content-Security-Policy
	// header. Use it to allowlist external CDNs (script/style/img) without
	// forking the platform middleware. nil keeps the conservative default.
	CSP *CSP
}

// CSP is a structured Content-Security-Policy. Each field is a list of
// source expressions; empty fields are omitted from the rendered header.
// 'self' is a string literal — quote it as `"'self'"` in callers.
type CSP struct {
	DefaultSrc []string
	ScriptSrc  []string
	StyleSrc   []string
	ImgSrc     []string
	ConnectSrc []string
	FontSrc    []string
	FrameSrc   []string
	MediaSrc   []string
	ObjectSrc  []string
}

// HeaderValue renders the CSP as a single-line directive string suitable for
// the Content-Security-Policy response header.
func (c CSP) HeaderValue() (string, error) {
	parts := make([]string, 0, 9)
	add := func(name string, vals []string) error {
		if len(vals) == 0 {
			return nil
		}
		for _, v := range vals {
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("csp %s contains empty source", name)
			}
			if strings.ContainsAny(v, ";\r\n") {
				return fmt.Errorf("csp %s contains invalid source %q", name, v)
			}
		}
		parts = append(parts, name+" "+strings.Join(vals, " "))
		return nil
	}
	for _, directive := range []struct {
		name string
		vals []string
	}{
		{"default-src", c.DefaultSrc},
		{"script-src", c.ScriptSrc},
		{"style-src", c.StyleSrc},
		{"img-src", c.ImgSrc},
		{"connect-src", c.ConnectSrc},
		{"font-src", c.FontSrc},
		{"frame-src", c.FrameSrc},
		{"media-src", c.MediaSrc},
		{"object-src", c.ObjectSrc},
	} {
		if err := add(directive.name, directive.vals); err != nil {
			return "", err
		}
	}
	return strings.Join(parts, "; "), nil
}

// String renders the CSP as a single-line directive string. Invalid source
// expressions render as an empty string; use HeaderValue when errors matter.
func (c CSP) String() string {
	v, err := c.HeaderValue()
	if err != nil {
		return ""
	}
	return v
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
	r.Use(securityHeaders(deps.Production, deps.CSP))
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

// defaultCSP returns the conservative platform-default CSP.
func defaultCSP() CSP {
	return CSP{
		DefaultSrc: []string{"'self'"},
		ImgSrc:     []string{"'self'", "data:"},
		StyleSrc:   []string{"'self'", "'unsafe-inline'"},
		ScriptSrc:  []string{"'self'", "'unsafe-inline'", "'unsafe-eval'"},
		FrameSrc:   []string{"https://www.youtube-nocookie.com"},
	}
}

func securityHeaders(production bool, override *CSP) func(http.Handler) http.Handler {
	csp := defaultCSP()
	if override != nil {
		csp = *override
	}
	cspHeader, err := csp.HeaderValue()
	if err != nil {
		cspHeader, _ = defaultCSP().HeaderValue()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Content-Security-Policy", cspHeader)
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
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		msg = appErr.Msg
		switch appErr.Kind {
		case apperror.KindNotFound:
			status = http.StatusNotFound
		case apperror.KindValidation:
			status = http.StatusUnprocessableEntity
		case apperror.KindConflict:
			status = http.StatusConflict
		case apperror.KindUnauthorized:
			status = http.StatusUnauthorized
		case apperror.KindForbidden:
			status = http.StatusForbidden
		default:
			status = http.StatusInternalServerError
			msg = "errore interno"
		}
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
	for _, raw := range strings.Split(r.Header.Get("Accept"), ",") {
		mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		if q, ok := params["q"]; ok {
			weight, err := strconv.ParseFloat(q, 64)
			if err != nil || weight <= 0 {
				continue
			}
		}
		if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") {
			return true
		}
	}
	return false
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
