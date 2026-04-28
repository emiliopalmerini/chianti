# ADR-007: Estrazione di `platform/database`

## Status

Accepted

## Context

`internal/platform/database` di ITdG (147 righe + 26 di test)
contiene:

- **`Open(path)`**: apre un handle SQLite tunato (single-writer,
  WAL, foreign keys ON, busy_timeout 5000, synchronous NORMAL) e
  ritorna un wrapper `*DB`.
- **`TimeFormat`**: il layout time canonico per le colonne TEXT.
  `2006-01-02T15:04:05.000000000Z07:00`. Fixed-width così
  l'ordinamento lessicografico SQL coincide con l'ordinamento
  cronologico (RFC3339Nano non basta perché taglia gli zeri
  finali).
- **`Scanner`**: interfaccia comune fra `*sql.Row` e `*sql.Rows` per
  helper di scan riusabili.
- **Helper di nullable**: `PtrToNullable[T]`, `NullStringToPtr`,
  `NullInt64ToIntPtr`.
- **Error matcher SQLite**: `IsUniqueConstraint(err, needles...)`,
  `IsForeignKeyViolation(err)`.
- **Helper di query**: `ClampLimit(limit, fallback)`,
  `WithTx(ctx, db, fn)`.

Tutti i siti consumer pianificati useranno SQLite (deciso in conv.).
Il pacchetto è per natura site-agnostic: niente embed, niente
default site-specific, niente domain term. È il candidato Tier 2
più pulito dopo `italy`.

## Decision

### Cosa entra in chianti

`chianti/platform/database/` riproduce identico il pacchetto di
ITdG, conservando lo split in `database.go` (apertura) e `sqlx.go`
(helper).

### Dipendenza nuova: `mattn/go-sqlite3`

Chianti finora era 100% stdlib. Estraendo `database` introduciamo la
prima dipendenza esterna del kit: `github.com/mattn/go-sqlite3`. È
CGO-based, quindi tutti i consumer di `chianti/platform/database`
ereditano il vincolo CGO. Accettabile perché:

- Tutti i siti pianificati (ITdG, Giada, T&D) sono SQLite per
  scelta esplicita (vedi ADR-001).
- È la libreria standard de facto per SQLite in Go.
- L'alternativa (driver parametrico) è over-engineering rispetto
  alla rule of three.

I consumer che non usano `platform/database` (es. solo
`kernel/apperror`) non incorrono nel costo CGO grazie al
sub-package import: Go scarica la dep solo se il pacchetto viene
importato.

### API esposta

Identica all'attuale ITdG. Niente rename, niente cambio di firma.

```go
package database

type DB struct{ *sql.DB }
func Open(path string) (*DB, error)
func (d *DB) Close() error

const TimeFormat = "2006-01-02T15:04:05.000000000Z07:00"

type Scanner interface { Scan(dest ...any) error }

func PtrToNullable[T any](p *T) any
func NullStringToPtr(n sql.NullString) *string
func NullInt64ToIntPtr(n sql.NullInt64) *int

func IsUniqueConstraint(err error, needles ...string) bool
func IsForeignKeyViolation(err error) bool

func ClampLimit(limit, fallback int) int
func WithTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error
```

### Test

ITdG ha 1 test (`TestOpenAppliesPragmas`). Aggiungiamo unit test
sugli helper che oggi non hanno copertura diretta:

1. `TestPtrToNullable_NilReturnsNil`
2. `TestPtrToNullable_NonNilReturnsValue`
3. `TestNullStringToPtr_InvalidReturnsNil`
4. `TestNullStringToPtr_ValidReturnsValue`
5. `TestNullInt64ToIntPtr_InvalidReturnsNil`
6. `TestNullInt64ToIntPtr_ValidReturnsValue`
7. `TestIsUniqueConstraint_MatchesAndFiltersByNeedle`
8. `TestIsForeignKeyViolation_DetectsTypicalMessage`
9. `TestClampLimit_PositiveAndNonPositive`
10. `TestWithTx_CommitOnSuccess`
11. `TestWithTx_RollbackOnError`

`TestOpenAppliesPragmas` resta uguale (foreign_keys=1 su `:memory:`).

Costo: ~80 righe di test, copertura completa del pacchetto pubblico.

### Niente cambia nel comportamento

- Nessun cambio di pragmas su `Open`.
- Nessun cambio di `TimeFormat`.
- Nessun nuovo helper (no `IsCheckConstraint`, no
  `WithReadOnlyTx`).
- Nessun supporto a non-SQLite (no parametrizzazione di driver).

## Consequences

### Positive

- Tutti i siti consumer ottengono lo stesso handle SQLite tunato
  identicamente, niente drift sui pragma.
- `TimeFormat` come costante condivisa elimina il rischio che un
  futuro sito formatti diversamente i timestamp.
- Helper di nullable e error matcher centralizzati.
- Pacchetto pubblico passa dal coverage parziale a quello completo.

### Negative

- Chianti non è più 100% stdlib: `mattn/go-sqlite3` introduce CGO
  per i consumer di `platform/database`. Documentato sopra.
- Il pacchetto resta SQLite-only. Se un futuro sito sceglie
  Postgres (oggi escluso), serve refactoring. Costo accettabile a
  fronte della decisione "SQLite ovunque" presa in ADR-001.

### Neutral

- ITdG rimuove `internal/platform/database/` interamente; il
  pacchetto `migrations` di ITdG continua ad importare il database
  ma da chianti.
