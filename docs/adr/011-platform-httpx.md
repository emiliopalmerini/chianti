# ADR-011: Estrazione di `platform/httpx` (con evoluzione)

## Status

Accepted

## Context

`internal/platform/httpx/httpx.go` di ITdG (247 righe, 31
importer) è il pacchetto più sostanzioso ancora fuori dal kit. È
HTTP plumbing puramente generico (chi router, middleware stack,
rate limiter, CSRF, error rendering per `*apperror.Error`), ma è
stato saltato nei round "translocation" perché ha tre punti di
accoppiamento al sito ITdG:

1. **Importa `internal/platform/config`**: `NewRouter` riceve
   `ServerDeps{Cfg *config.Config, ...}` e legge `Cfg.IsProduction()`;
   `CSRFMiddleware` riceve `*config.Config` e legge `Cfg.CSRFKey` +
   `Cfg.IsProduction()`. Chianti non può importare `config` (sarebbe
   ITdG → chianti → ITdG, ciclico).
2. **`csrf.CookieName("turno_csrf")`** hardcoded: stesso problema di
   `session` (ADR-009), il cookie name è site-specific.
3. **CSP hardcoded** che ammette `youtube-nocookie.com` come unico
   `frame-src` esterno: generico per HTMX + Alpine, rigido se un
   sito vuole iframe diversi.

Il campo `ServerDeps.CSRFKey []byte` esiste in ITdG ma non viene
mai letto da `NewRouter`: era un residuo del wiring originale
(`CSRFMiddleware` legge la chiave da `Cfg.CSRFKey`, non da
`ServerDeps`).

`BucketLimiter` è un token-bucket globale (single bucket per tutti i
request che attraversano il middleware) e il doc string già nota
che il per-IP è una "slice concern". Lo trasloco as-is: utile
default, niente cambi di firma.

`TestCSRFBypass` è zucchero test-only esportato dal package di
produzione. In ITdG sta lì per pragmatismo (un solo file). In
chianti lo separo in un subpackage dedicato così il package di
produzione non ha export test-only.

## Decision

### Cosa entra in chianti

Tutto il file ITdG, con tre evoluzioni di firma e un piccolo
re-package del bypass test.

### API esposta

```go
// chianti/platform/httpx/httpx.go
package httpx

type ServerDeps struct {
    Production bool
    Logger     *slog.Logger
}

// NewRouter builds the base chi router with the platform middleware
// stack: RequestID, RealIP (production only), request logger,
// Recoverer, Compress(5), security headers (with HSTS in
// production). Slice HTTP adapters mount onto the returned router.
func NewRouter(deps ServerDeps) chi.Router

type CSRFField = func(*http.Request) template.HTML

// CSRFMiddleware returns gorilla/csrf configured for this app:
// SameSite=Lax, Path="/", Secure when production, the given
// cookieName. The csrfKeyEncoded value is accepted as raw 32 bytes,
// base64, or hex (so site config can keep its existing format).
func CSRFMiddleware(production bool, csrfKeyEncoded, cookieName string) (func(http.Handler) http.Handler, error)

// CSRFFieldFromContext returns the csrf.TemplateField function
// stashed by CSRFMiddleware, or a no-op when the middleware is
// not installed.
func CSRFFieldFromContext(ctx context.Context) CSRFField

// CSRFTokenFromRequest returns the raw CSRF token for the current
// request, suitable for embedding in a <meta name="csrf-token"> tag.
func CSRFTokenFromRequest(r *http.Request) string

// BucketLimiter returns a token-bucket rate-limit middleware at rps
// requests per second with the given burst. Limit state is global
// (single bucket for all requests through the middleware); per-IP
// buckets are a slice concern.
func BucketLimiter(rps float64, burst int) func(http.Handler) http.Handler

// RenderError translates an error (including *apperror.Error) into
// an HTTP response. Content-type negotiation: anything under /api/
// or with Accept: application/json gets a JSON body; everything
// else gets text/plain.
func RenderError(w http.ResponseWriter, r *http.Request, err error)
```

```go
// chianti/platform/httpx/httptest/bypass.go
package httptest

// CSRFBypass is a no-op middleware that stashes an empty CSRFField
// on ctx. Tests that build the real router but don't want to fetch
// tokens mount this instead of httpx.CSRFMiddleware.
func CSRFBypass() func(http.Handler) http.Handler
```

### Evoluzioni rispetto a ITdG

| ITdG                                         | chianti                                                              |
| -------------------------------------------- | -------------------------------------------------------------------- |
| `ServerDeps{Cfg, Logger, CSRFKey}`           | `ServerDeps{Production bool, Logger *slog.Logger}`                   |
| `CSRFMiddleware(cfg *config.Config)`         | `CSRFMiddleware(production bool, csrfKeyEncoded, cookieName string)` |
| `csrf.CookieName("turno_csrf")` hardcoded    | parametro                                                            |
| `httpx.TestCSRFBypass()` nel package di prod | `httpx/httptest.CSRFBypass()` in subpackage                          |
| import `internal/kernel/apperror`            | import `chianti/kernel/apperror`                                     |

`ServerDeps.CSRFKey` viene rimosso (era unused). I 30+ callsite di
`httpx.RenderError`, `httpx.CSRFFieldFromContext`,
`httpx.CSRFTokenFromRequest`, `httpx.BucketLimiter` non cambiano.

### Content Security Policy

Resta come default HTMX-friendly:

```
default-src 'self';
img-src 'self' data:;
style-src 'self' 'unsafe-inline';
script-src 'self' 'unsafe-inline' 'unsafe-eval';
frame-src https://www.youtube-nocookie.com
```

Se un sito (Giada / T&D) chiede iframe da un host diverso,
parametrizziamo in un ADR successivo. Default-yes-CSP > opzione
prematura.

### Test

Suite shippata da chianti (ITdG ha 0 test su httpx oggi):

1. `TestNewRouterSetsSecurityHeaders` — risposta ha
   `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
   `Referrer-Policy: strict-origin-when-cross-origin`, CSP atteso.
   Production=false → niente HSTS; production=true → HSTS presente.
2. `TestNewRouterRecoversPanic` — handler che panica → 500, server
   non muore.
3. `TestCSRFMiddlewareDecodesKey` — table-driven: raw 32-byte,
   base64 (32B encoded), hex (32B encoded), errore per chiavi non
   conformi.
4. `TestCSRFMiddlewareInjectsField` — dopo il middleware,
   `httpx.CSRFFieldFromContext(r.Context())` ritorna una funzione
   non-nil che produce HTML non vuoto.
5. `TestCSRFFieldFromContextNoOpWhenMissing` — senza middleware,
   ritorna funzione che produce stringa vuota.
6. `TestBucketLimiterRejectsOverBudget` — `rps=1, burst=1`: prima
   request 200, seconda 429.
7. `TestRenderError` — table-driven sui kind di apperror:
   NotFound→404, Validation→422, Conflict→409, Unauthorized→401,
   Forbidden→403, plain `errors.New`→500.
8. `TestRenderErrorJSON` — branch JSON: path `/api/foo` → JSON
   body; `Accept: application/json` su path qualsiasi → JSON body;
   altrimenti text/plain.
9. `httptest.TestCSRFBypassInjectsNoOpField` — il bypass piazza in
   ctx una `CSRFField` che ritorna stringa vuota, mountabile come
   middleware al posto di quello reale.

### Dipendenze

Nuove in chianti `go.mod`:

- `github.com/go-chi/chi/v5`
- `github.com/gorilla/csrf`
- `golang.org/x/time/rate`

Tutte già presenti in ITdG `go.sum`, niente versioni nuove.

## Consequences

### Positive

- I 31 importer ITdG di httpx ottengono il kit testato (oggi 0
  test).
- Sblocca il punto più sostanzioso del round evolutiva (Tier 2
  completo dopo questo + `config`).
- Wiring ITdG perde la dipendenza implicita "httpx conosce config":
  callsite passa primitivi.

### Negative

- ITdG aggiorna 2 callsite di wiring (uno in `cmd/turno/wire.go`
  per `NewRouter`, uno per `CSRFMiddleware`) e 1-2 import di test
  (`httpx.TestCSRFBypass` → `chiantihttptest.CSRFBypass`).
- Chianti acquista 3 dipendenze esterne in un colpo solo (chi,
  gorilla/csrf, x/time/rate). Inevitabile per un router HTTP.
- Conflitto previsto su `chianti vX.Y.Z` di go.mod ITdG al merge
  con altri branch refactor: si risolve prendendo la versione più
  alta (0.8.0).

### Neutral

- CSP resta una decisione del kit (HTMX + Alpine + youtube). Se
  Giada/T&D chiedono altro, ADR dedicato.
- `BucketLimiter` shared globale resta un default semplice; chi
  vuole per-IP lo costruisce nello slice, come oggi in ITdG.
