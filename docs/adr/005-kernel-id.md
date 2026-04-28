# ADR-005: Estrazione di `kernel/id` (UUID, Slug, BookingCode)

## Status

Accepted

## Context

`internal/kernel/id` di ITdG è un pacchetto eterogeneo che raccoglie
tre generatori usati ovunque nel codebase:

- **`NewUUID()` / `NewUUIDAt(now, r)`**: UUIDv7 in forma canonica
  8-4-4-4-12 lowercase. Il prefisso 48-bit di unix-millis garantisce
  ordinamento monotono, utile per gli inserimenti SQLite B-tree
  locali.
- **`Slug(s)`**: trasliterazione italiana (caratteri accentati →
  ascii) + lowercase + collassamento dei separatori in `-`. Tabella
  esplicita di transliteration, zero dipendenze.
- **`BookingCode()` / `BookingCodeWithSource(r)`**: codice 8 char da
  un alfabeto privo di caratteri visivamente ambigui (no 0/O, 1/I/L,
  5/S). Pensato per essere comunicato a voce o stampato senza
  ambiguità.

I tre vivono nello stesso pacchetto in ITdG; ADR-001 di chianti
suggeriva uno `slug` separato, ma il codebase li ha tenuti insieme
per pragmatismo. Manteniamo la stessa struttura per rispettare
"translocation, not evolution".

ADR-001 elencava `kernel/slug` come pacchetto separato; questa ADR
emenda quella nota: lo slug resta dentro `id`. Se in futuro avremo
ragioni concrete per splittare (per esempio un consumer non-italiano
che vuole `Slug` senza la tabella italiana), apriremo un ADR
dedicato.

## Decision

### Cosa entra in chianti

Tutto il contenuto di `internal/kernel/id/` di ITdG viene riprodotto
identico in `chianti/kernel/id/`:

- `uuid.go` + `uuid_test.go`
- `id.go` (Slug + BookingCode + tabella italiana di transliteration)
- `id_test.go`

### API esposta

```go
package id

// UUIDv7 canonico, monotonico per ordering sqlite-friendly.
func NewUUID() string
func NewUUIDAt(now time.Time, r io.Reader) string

// Slug italianizzante (à→a, è→e, ...) + lowercase + dash.
func Slug(s string) string

// Codice 8-char con alfabeto non-ambiguo.
func BookingCode() string
func BookingCodeWithSource(r io.Reader) string
```

### Nota sul nome `BookingCode`

Il nome ha un sapore di dominio ("booking"), ma la funzione è
puramente un generatore di codice. La regola della rule-of-three
applicata in modo invertito: tre consumer pianificati (ITdG/Giada/
T&D) hanno tutti bisogno di un codice generabile, comunicabile a
voce, e non ambiguo. Il nome resta come è oggi per minimizzare il
diff della migrazione.

Se in futuro chianti avrà un consumer non-prenotazioni che vuole la
stessa funzione con un nome neutro, apriremo un ADR per esporre un
alias `Code()` (o simile) lasciando `BookingCode` deprecated. Niente
preventiva.

### Test

I 4 test esistenti vengono copiati identici:

1. `TestSlugItalianAccents`
2. `TestBookingCodeFormat`
3. `TestBookingCodeWithSourceDeterministic`
4. `TestNewUUIDFormat`
5. `TestUUIDMonotonic`

Niente da aggiungere: la copertura è già esaustiva sui tre
generatori.

### Niente cambia nel comportamento

- Nessuna nuova versione UUID.
- Nessun cambio della tabella di transliteration.
- Nessun cambio dell'alfabeto BookingCode.
- Nessun helper aggiuntivo (no `MustNewUUID`, no `SlugWithMaxLen`).

## Consequences

### Positive

- I tre generatori più diffusi del codebase diventano disponibili a
  tutti i consumer del kit.
- Stessa API, zero learning curve.
- ADR-001 emendato in modo esplicito sull'unione `id+slug`.

### Negative

- Il pacchetto `id` resta eterogeneo (tre concern in un pacchetto).
  Accettabile finché coerente con ITdG; un futuro split richiede ADR
  proprio.
- Il nome `BookingCode` mantiene un'eco di dominio. Vivibile per ora
  (tutti i consumer lo usano per booking-like codes); rivedibile via
  ADR.

### Neutral

- ADR-001 è stata scritta prima di leggere il codice; questa è la
  prima emenda formale. Non serve riscriverla, basta che ADR-005
  emerga come fonte autoritativa per il caso `id+slug`.
