# ADR-009: Estrazione di `platform/session` (con evoluzione)

## Status

Accepted

## Context

`internal/platform/session` di ITdG (51 righe + 29 di test) wrappa
`alexedwards/scs` per fornire due session manager distinti:

- **Admin**: sqlite-backed, lifetime 24h, idle 2h. Per il login
  staff.
- **Form**: in-memory, lifetime 2h, idle 30min. Per il wizard di
  prenotazione pubblica.

Ogni manager imposta lo stesso set di cookie defaults (HttpOnly,
SameSite=Lax, Path=/, Secure parametrico).

Il pacchetto è quasi tutto generico **tranne** due costanti
hardcoded:

```go
const (
    AdminCookieName = "turno_admin_session"
    FormCookieName  = "turno_form_session"
)
```

Il prefisso `turno_` è il binario ITdG. Non valido per Giada o T&D.

A differenza dei pacchetti già estratti (apperror, clock, eventbus,
id, italy, database, email), `session` non si presta a pura
traslocazione: serve **piccola evoluzione** delle firme per non
incarnare nel kit le scelte di naming di un sito.

## Decision

### Cosa entra in chianti

`chianti/platform/session/session.go` riproduce la struttura del
pacchetto ITdG ma con due cambi di firma e la rimozione delle
costanti site-specific. Wiring scs invariato (sqlite3store per
admin, memstore per form), defaults invariati (lifetime e idle).

### Costanti rimosse

`AdminCookieName` e `FormCookieName` **non entrano** in chianti.
Restano nei consumer site, generalmente a livello di
`cmd/<binary>/wire.go` o un piccolo file locale, perché il nome del
cookie è una scelta del sito.

### API esposta (evoluzione delle firme)

```go
package session

// Era: NewAdminManager(db *sql.DB, secure bool) *scs.SessionManager
func NewAdminManager(db *sql.DB, cookieName string, secure bool) *scs.SessionManager

// Era: NewFormManager(secure bool) *scs.SessionManager
func NewFormManager(cookieName string, secure bool) *scs.SessionManager
```

Defaults ereditati invariati: 24h/2h admin, 2h/30min form. Cookie
options invariate (HttpOnly true, SameSite Lax, Path /).

### Test

In ITdG c'è un solo test
(`TestFormManagerCookieIsolated`) che verifica che admin/form
abbiano cookie diversi. In chianti riscrivo coerentemente con la
nuova API + aggiungo qualche micro-test mancante:

1. `TestAdminManagerSetsCookieName`: nome ricevuto come parametro
   diventa `Cookie.Name`.
2. `TestFormManagerSetsCookieName`: idem per form.
3. `TestManagersHaveExpectedDefaults`: admin lifetime 24h / idle 2h,
   form lifetime 2h / idle 30min, entrambi HttpOnly + SameSiteLax +
   Path "/".
4. `TestCookieIsolation`: passando nomi diversi i cookie non si
   sovrappongono (porta avanti il test originale di ITdG).

Coverage del pacchetto pubblico: completa.

### Niente cambia nel comportamento (oltre alla firma)

- Wiring scs invariato.
- Lifetime / idle defaults invariati.
- Cookie options invariate.
- Nessun helper nuovo (`NewSecureManager`, opzioni funzionali, ecc.).

### Dipendenze

`alexedwards/scs/v2`, `alexedwards/scs/sqlite3store`,
`alexedwards/scs/v2/memstore`. Sono già dipendenze indirette
dell'unico consumer reale (ITdG); estraendo `session` chianti le
prende come dirette. Niente CGO aggiuntivo (memstore è puro Go,
sqlite3store usa il driver già presente via `platform/database`).

## Consequences

### Positive

- I siti consumer ottengono lo stesso wiring scs identico, niente
  drift su lifetime/idle/cookie options.
- Il prefisso del cookie diventa scelta esplicita del sito: pulito,
  leggibile in `wire.go`, niente "magia" nel kit.
- Pattern "kit takes parameters where sites differ, kit hardcodes
  where sites agree" applicato in modo chiaro: cookie name varia,
  scs wiring no.

### Negative

- Prima ADR di chianti che rompe la regola "translocation pura": le
  firme cambiano. Costo basso (2 callsite ITdG da aggiornare) ma
  segna l'inizio della fase "evolution" del kit, dove ogni
  estrazione può richiedere piccoli cambi di API.

### Neutral

- `AdminCookieName` / `FormCookieName` migrano lato consumer,
  inline o come costanti locali. ITdG si aggiorna in ADR-052.
- I siti non-italiani futuri (oggi non previsti) non hanno vincoli
  sul naming del cookie da chianti.
