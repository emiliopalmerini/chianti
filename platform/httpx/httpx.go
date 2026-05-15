// Package httpx provides net/http helpers: middleware composition, request
// logging, panic recovery, security headers, rate limiting, CSRF field
// contracts, and error rendering for *apperror.Error. Concrete routers and
// CSRF middleware live in consumer sites.
package httpx

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"mime"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emiliopalmerini/chianti/kernel/apperror"
)

type Middleware func(http.Handler) http.Handler

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

// BaseMiddleware returns the platform middleware stack in request order:
// request logging, panic recovery, and security headers. Consumers apply it to
// their router of choice.
func BaseMiddleware(deps ServerDeps) []Middleware {
	return []Middleware{
		RequestLogger(deps.Logger),
		Recoverer(deps.Logger),
		SecurityHeaders(deps.Production, deps.CSP),
	}
}

// Wrap applies middleware to h in the same order as net/http requests pass
// through it: Wrap(h, a, b) runs a before b before h.
func Wrap(h http.Handler, middleware ...Middleware) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}

// NewHandler applies BaseMiddleware to h.
func NewHandler(deps ServerDeps, h http.Handler) http.Handler {
	return Wrap(h, BaseMiddleware(deps)...)
}

// RequestLogger logs method, path, response status, bytes written, and
// duration after the wrapped handler completes.
func RequestLogger(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r)
			logger.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
	wrote  bool
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.wrote {
		return
	}
	r.wrote = true
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if !r.wrote {
		r.wrote = true
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += n
	return n, err
}

func (r *responseRecorder) Status() int { return r.status }

func (r *responseRecorder) BytesWritten() int { return r.bytes }

// Recoverer converts panics into HTTP 500 responses and logs the stack. It
// must wrap handlers that have not already committed a response.
func Recoverer(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					logger.Error("http panic", "panic", v, "stack", string(debug.Stack()))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
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

// SecurityHeaders sets the shared defensive response headers. HSTS is only
// enabled for production because it is sticky in browsers.
func SecurityHeaders(production bool, override *CSP) Middleware {
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

// CSRFField is the template hook consumer CSRF middleware can expose to views.
type CSRFField = func(*http.Request) template.HTML

type csrfFieldKey struct{}

// WithCSRFField returns a copy of ctx carrying f as the CSRFField that
// CSRFFieldFromContext will return. Consumer CSRF middleware can use this to
// expose template fields without tying chianti to a concrete CSRF library.
func WithCSRFField(ctx context.Context, f CSRFField) context.Context {
	return context.WithValue(ctx, csrfFieldKey{}, f)
}

// CSRFFieldFromContext returns the csrf.TemplateField function stashed by
// consumer middleware, or a no-op when no field is installed.
func CSRFFieldFromContext(ctx context.Context) CSRFField {
	if v, ok := ctx.Value(csrfFieldKey{}).(CSRFField); ok && v != nil {
		return v
	}
	return func(*http.Request) template.HTML { return "" }
}

// BucketLimiter returns a token-bucket rate-limit middleware at rps requests
// per second with the given burst. Limit state is global (single bucket for
// all requests through the middleware); per-IP buckets are a slice concern.
func BucketLimiter(rps float64, burst int) Middleware {
	lim := newTokenBucket(rps, burst)
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

type tokenBucket struct {
	mu       sync.Mutex
	rps      float64
	capacity float64
	tokens   float64
	last     time.Time
}

func newTokenBucket(rps float64, burst int) *tokenBucket {
	if rps <= 0 {
		rps = 1
	}
	if burst <= 0 {
		burst = 1
	}
	now := time.Now()
	return &tokenBucket{
		rps:      rps,
		capacity: float64(burst),
		tokens:   float64(burst),
		last:     now,
	}
}

func (b *tokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * b.rps
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
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
