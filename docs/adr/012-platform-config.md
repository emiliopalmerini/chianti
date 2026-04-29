# ADR-012: Estrazione di `platform/config` (solo primitivi)

## Status

Accepted

## Context

`internal/platform/config` di ITdG (177 righe) fa due cose:

1. **Definisce la `Config` struct di ITdG**, con campi e default
   site-specific: `FromEmail = "noreply@ilturnodiguardia.com"`,
   `DBPath = "./data/turno.db"`, `DocumentiReturnAddress =
   "info@ilturnodiguardia.com"`, `UploadDir = "./data/uploads"`.
   `Load()` legge env, `Validate()` ha regole "required in
   production" specifiche di ITdG.

2. **Implementa quattro primitivi generici**: `getEnv`,
   `randomKey`, `AdminSeed`, `parseAdminSeeds`. Niente di
   site-specific qui.

Estrarre la `Config` struct as-is significherebbe baked-in delle
stringhe ITdG nel kit; non funziona per Giada/T&D, che hanno
default e required diversi. Questo era il motivo per cui
`config` era stato saltato nei round translocation.

I quattro primitivi invece sono identici cross-sito: un
`getEnv(key, default)` non ha sapore ITdG, una funzione che genera
32 byte random base64-encoded nemmeno, e il formato `AdminSeed`
("user:email:password") è una scelta di formato che possiamo
fissare nel kit (i siti consumer adottano lo stesso formato per
`ADMIN_SEEDS` env var).

## Decision

### Cosa entra in chianti

Solo i quattro primitivi. La `Config` struct, `Load`, `Validate`,
`IsProduction`, `fillDevSecrets` restano in ogni sito consumer.

### API esposta

```go
// chianti/platform/config/config.go
package config

// GetEnv returns the value of key from the environment, or def if
// the variable is unset or set to the empty string.
func GetEnv(key, def string) string

// RandomKey returns a 32-byte random key, base64-encoded with
// stdlib StdEncoding (44 chars). Suitable as a CSRF key, JWT
// signing key, or session secret. Sites that need other key
// sizes should not use this helper.
func RandomKey() string

// AdminSeed is one entry of the ADMIN_SEEDS env var. The
// expected env format is "user1:email1:pw1,user2:email2:pw2".
type AdminSeed struct {
    Username string
    Email    string
    Password string
}

// ParseAdminSeeds parses raw into AdminSeed entries. Empty raw
// returns nil. Malformed entries (not exactly 3 colon-separated
// parts) are silently skipped; the caller's Validate is
// responsible for failing fast if zero seeds is unacceptable
// (e.g. ITdG requires at least one admin in production).
func ParseAdminSeeds(raw string) []AdminSeed
```

### Decisioni di firma

- **`RandomKey()` senza parametro `n int`**: ITdG usa 32 byte per
  tre cose diverse (CSRF, JWT, session). 32 è la convenzione del
  kit. Se in futuro un sito chiede un'altra size, ADR dedicato.
- **`ParseAdminSeeds` silently-skip**: preservato il comportamento
  ITdG attuale. Il `Validate()` di ogni sito decide se zero seed
  è accettabile (ITdG: no in production). Un parser strict
  `(seeds, err)` sarebbe un cambio di comportamento; lo lasciamo
  fuori.
- **Niente `IsProduction(env string) bool`**: sarebbe un'unica
  comparazione, non vale un export. I siti scrivono
  `cfg.Environment == "production"` da soli.
- **Niente `Validate` parametrico**: le regole di required field
  sono site-specific (ITdG richiede 4 secret + ADMIN_SEEDS in
  production; Giada richiederà altri campi). Un helper generico
  diluirebbe la chiarezza del messaggio di errore senza guadagno
  reale.

### Test

Suite shippata da chianti su `testdata`-free (env mocked via
`t.Setenv`):

1. `TestGetEnv` — table-driven: variabile unset → default,
   variabile set a stringa non-vuota → valore, variabile set a
   stringa vuota → default (preservato il comportamento ITdG di
   trattare `""` come "non set").
2. `TestRandomKey` — chiamata 1 produce 44 caratteri base64; il
   decode produce 32 byte; chiamata 2 produce un valore diverso
   (entropia non zero).
3. `TestParseAdminSeeds` — table-driven:
   - input vuoto → nil
   - una entry valida → 1 seed con i 3 campi
   - tre entry valide → 3 seed in ordine
   - entry malformata (2 parti) → skippata, le altre passano
   - entry malformata (4 parti) → skippata, le altre passano
   - tutte malformate → []AdminSeed{} oppure nil (caller decide se
     è errore)

### Dipendenze

Solo stdlib (`crypto/rand`, `encoding/base64`, `os`, `strings`).
Niente `github.com/joho/godotenv`: caricare `.env` è una scelta
del sito (entrypoint), non del kit. Ogni sito chiama `godotenv.Load()`
prima di costruire la propria `Config`, come oggi.

### Layout

Il package si chiama `config`, mappato sull'import path
`github.com/emiliopalmerini/chianti/platform/config`. Niente
helper packages né subpackage.

## Consequences

### Positive

- Ogni sito consumer (Giada, T&D, ITdG) ottiene gli stessi
  primitivi testati: parser admin seed, env helper, generatore di
  chiavi.
- La `Config` struct di ogni sito resta esplicita e leggibile —
  non c'è una struct "generica" da estendere o configurare.
- Round evolutiva 4/4 chiuso: dopo questo, ogni pacchetto Tier 2
  ITdG ha una controparte chianti.

### Negative

- Pacchetto piccolo (~50 righe + test). Il rapporto codice/test è
  alto, ma i primitivi sono il minimo comune denominatore reale —
  estrarre meno significherebbe drift fra siti.

### Neutral

- Nome del package nel kit (`config`) è generico. I siti consumer
  importano con alias se vogliono evitare collisione con il
  proprio package `config`:
  ```go
  import chianticonfig "github.com/emiliopalmerini/chianti/platform/config"
  ```
- `splitCSV` / `splitColon` / `split` di ITdG non vengono
  esportati. Sono dettagli implementativi di
  `parseAdminSeeds`; il kit usa `strings.Split` standard.
