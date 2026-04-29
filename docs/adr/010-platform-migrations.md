# ADR-010: Estrazione di `platform/migrations` (con evoluzione)

## Status

Accepted

## Context

`internal/platform/migrations` di ITdG (139 righe + ~220 di test)
applica file `*.up.sql` numerati in ordine lessicografico, in
transazione per file, registrando ogni versione in
`schema_migrations`. Re-running è no-op.

Il pacchetto è generico per natura (apply ordered SQL, track
applied versions, idempotent). MA ha una sola coupling
problematica: il `//go:embed sql/*.sql` accoppia il runner ai file
specifici di ITdG. Estraendolo as-is significherebbe portare nel
kit i 7 file `.up.sql` di ITdG e i loro corrispondenti `.down.sql`,
costringendo Giada e T&D a inserire i propri migration nel
package del kit. Inaccettabile.

L'API attuale ITdG ha due entrypoint:

```go
func Run(path string) error           // apre il DB, applica, chiude
func RunDB(db *sql.DB) error          // applica su DB già aperto
```

Entrambi usano l'`embed.FS` interno.

Per estrarre serve **piccola evoluzione**: il runner accetta `fs.FS`

- `dir` come parametri, lasciando al sito l'embed dei propri file
  SQL.

## Decision

### Cosa entra in chianti

Solo il **runner**. Niente file `.up.sql`/`.down.sql` (sono dei
siti).

```go
// chianti/platform/migrations/migrations.go
package migrations

func Run(db *sql.DB, fsys fs.FS, dir string) error
```

Niente overload `Run(path)` (apertura del DB è scelta del consumer,
non vincolare). I siti consumer scrivono il proprio thin adapter se
vogliono la convenience di "open + run + close" in un solo
chiamata.

### API esposta (un solo entrypoint)

```go
package migrations

// Run applies pending .up.sql files from fsys[dir] to db, in
// lexicographic order, in a transaction per file. Each applied
// version is recorded in schema_migrations. Re-running is
// idempotent. The version is parsed as the prefix before the first
// underscore: e.g. "000003_documents.up.sql" -> version 3.
func Run(db *sql.DB, fsys fs.FS, dir string) error
```

Il nome del file deve essere `NNN_*.up.sql` (la parte prima del
primo underscore deve essere un intero parsabile). I file `.down.sql`
sono ignorati dal runner.

### Test

Chianti shippa una suite con `testdata/sql/`:

- `000001_first.up.sql` — `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`
- `000002_second.up.sql` — `CREATE TABLE t2 (id INTEGER PRIMARY KEY)`
- `000003_alter.up.sql` — `ALTER TABLE t1 ADD COLUMN n TEXT`

Test:

1. `TestRunCreatesSchemaMigrations`: dopo Run, la tabella
   `schema_migrations` esiste con 3 versioni.
2. `TestRunIsIdempotent`: Run + Run non duplica righe né
   altera lo schema.
3. `TestRunSkipsAppliedMigrations`: Run pre-popolando
   `schema_migrations` salta versioni già marcate.
4. `TestRunFailsOnMalformedFilename`: file senza prefisso numerico
   ritorna errore.
5. `TestRunAppliesInLexicographicOrder`: l'ordine è 1,2,3 anche se
   `fs.ReadDir` ritorna in altro ordine (uso un fs in-memory con
   ordine inverso per provarlo).

### Niente cambia nel comportamento

- Schema della tabella `schema_migrations` invariato (`version
INTEGER PRIMARY KEY, applied_at TEXT NOT NULL`).
- `applied_at` resta in formato `time.RFC3339` (ITdG usa così;
  potremmo allineare al `TimeFormat` del pacchetto database in un
  ADR separato, non oggi).
- Una transazione per file (rollback isolato in caso di errore).
- Nessun supporto a `.down.sql` reverse (rimasto fuori in ITdG e
  fuori in chianti; se serve in futuro, ADR dedicato).
- Niente locking distribuito (ITdG ha un singolo writer per file
  SQLite, non serve).

### Dipendenze

Solo stdlib. Niente nuove dipendenze esterne in chianti
(`database/sql`, `fmt`, `io/fs`, `sort`, `strconv`, `strings`,
`time`).

## Consequences

### Positive

- I siti consumer ottengono lo stesso runner testato e identico,
  niente drift su transazione/idempotenza/ordering.
- Schema migration shipping resta una scelta esplicita del sito
  (ogni sito embed-a i propri SQL).
- Chianti's Tier 3 (auth, audit, email slice — futuri) potrà
  esporre il proprio schema via embed.FS, lasciando al sito di
  decidere se applicarlo via `chiantimigrations.Run` insieme ai
  suoi SQL o in altro modo.

### Negative

- Il consumer perde l'overload `Run(path)`: ogni sito che vuole la
  convenience scrive un thin wrapper di 5 righe. Costo trascurabile.
- Conflitto previsto sulla riga `chianti vX.Y.Z` di go.mod al merge
  con altri branch refactor: si risolve prendendo la versione più
  alta (0.7.0).

### Neutral

- I test ITdG su `drop_personalizzato`/`seed_documenti` (che
  verificano che SPECIFICHE migration ITdG facciano cose
  specifiche) restano in ITdG: testano dominio, non runner.
