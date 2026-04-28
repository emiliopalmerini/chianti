# ADR-008: Estrazione di `platform/email`

## Status

Accepted

## Context

`internal/platform/email` di ITdG (155 righe + 41 + 108 di test)
fornisce l'astrazione `Sender` per l'invio mail outbound, con tre
implementazioni:

- **`resendSender`**: client HTTP per `api.resend.com`, autentica
  via `Authorization: Bearer <apiKey>` e POSTa il messaggio in JSON
  con allegati base64. Costruito via `NewResendSender(apiKey, from)`
  oppure `NewResendSenderWithEndpoint(apiKey, from, endpoint)` per i
  test (httptest fake server).
- **`devOverrideSender`**: wrapper che riscrive `msg.To` su un
  singolo indirizzo (per evitare di mandare mail reali in dev e
  staging) e logga i destinatari originali.
- **`noopSender`**: logger-only, ritorna nil. Usato in dev quando
  `RESEND_API_KEY` è vuoto.

Tipi pubblici: `Message` (To, Subject, HTML, Text, Attachments),
`Attachment` (Filename, ContentType, Body).

Il pacchetto è puro infrastruttura: niente default site-specific,
niente domain term, niente assunzione su slice `email` (che invece
in ADR-013 di ITdG vive a parte e usa questo `Sender`). È il
secondo Tier 2 più chiaro dopo `database`.

Differenza chiave da `database`: `platform/email` introdurrebbe in
chianti **zero** dipendenze esterne (l'HTTP client è stdlib, niente
SDK Resend). Il kit resta CGO-free per i consumer che usano solo
email senza database.

## Decision

### Cosa entra in chianti

`chianti/platform/email/email.go` riproduce identico il pacchetto
di ITdG. Nessun rename, nessun cambio di firma.

I file di test esistenti (`email_test.go`, `attachments_test.go`)
vengono copiati identici, adattando l'import path.

### API esposta

```go
package email

type Attachment struct{ Filename, ContentType string; Body []byte }
type Message    struct{ To []string; Subject, HTML, Text string; Attachments []Attachment }
type Sender     interface{ Send(ctx context.Context, msg Message) error }

func NewResendSender(apiKey, from string) (Sender, error)
func NewResendSenderWithEndpoint(apiKey, from, endpoint string) (Sender, error)
func NewDevOverride(inner Sender, override string) Sender
func NewNoop(logger *slog.Logger) Sender
```

`NewResendSender` valida che `apiKey` e `from` non siano vuoti (e
ritorna un errore prima di costruire); `NewResendSenderWithEndpoint`
ammette endpoint vuoto e cade su quello reale.

### Test

ITdG ha 5 test (DevOverride redirect, ResendSender required
fields, Resend encoding di attachments via fake server, Send forwards
attachments, zero attachments). Aggiungiamo 2 test che oggi mancano:

1. `TestNoopReturnsNil`: il noop sender non panica e ritorna nil.
2. `TestResendSenderErrorsOn4xx`: status code >= 400 dal server fake
   produce un errore con il body inglobato nel messaggio.

Coverage del pacchetto pubblico: completa.

### Niente cambia nel comportamento

- Nessun cambio di payload Resend (campi `from`/`to`/`subject`/
  `html`/`text`/`attachments` invariati).
- Nessun retry policy, nessun rate limit, nessun queueing. Il
  Sender è "fire e ritorna errore se fallisce"; politiche di retry
  vivono nel chiamante (slice email per ITdG).
- Nessun nuovo provider (no SES, no SendGrid). Se serve, ADR
  dedicato.
- Timeout HTTP fisso a 10s, come ITdG.

### Dipendenze

Zero nuove dipendenze esterne. Solo stdlib + `log/slog`. Chianti
resta CGO-free per chi usa solo `platform/email`.

## Consequences

### Positive

- Tutti i siti consumer ottengono lo stesso Sender testato e
  identico, niente drift sul payload Resend.
- Lo slice email per-sito (Tier 3, deferred) si appoggia a
  `chianti/platform/email.Sender` come singolo punto di astrazione.
- Resta CGO-free: il sito che non usa database non paga il costo
  di `mattn/go-sqlite3`.

### Negative

- Trascurabili. Il pacchetto è auto-contenuto e ben testato.

### Neutral

- Il `Sender` interface esposto resta identico, quindi consumer
  che fanno mocking/wrapping non subiscono cambi.
- `NewNoop` accetta un `*slog.Logger` e cade su `slog.Default()`
  se nil — comportamento conservato.
