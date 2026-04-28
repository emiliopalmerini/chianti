# ADR-001: Scope di chianti e strategia multi-sito

## Status

Accepted

## Context

Esistono o sono pianificati tre siti web indipendenti, tutti scritti
in Go come monoliti single-binary, con SQLite come storage e con la
stessa architettura a slice esagonale che è già consolidata in
"Il Turno di Guardia" (di seguito ITdG):

- **ITdG**: già in produzione. Eventi con prenotazioni, capacità,
  threshold di conferma, opzioni, documenti legali, audit, mail
  templated. Admin team: Giada, Giulia, Emilio.
- **Giada (giadataribelli)**: in pianificazione. Esiste oggi una
  versione Astro statica che sarà riscritta in Go. Dominio:
  prenotazioni di servizi in partita IVA con dati di fatturazione.
  Niente documenti legali per minori. Admin team diverso da ITdG.
- **Travels & Dragons**: futuro. Dominio: viaggi con bonifico, stato
  pagamento, ricevute, opzioni. Admin team diverso da ITdG.

Ogni sito ha:

- Il proprio deploy.
- Il proprio file SQLite (`data/<sito>.db`).
- I propri admin (seed via `ADMIN_SEEDS`).
- Il proprio dominio di prenotazione (Evento ≠ Servizio ≠ Viaggio).

Quello che invece tutti i siti condividono per natura tecnica:

- Stack: Go 1.25+, chi router, scs sessions, html/template, TailwindCSS
  v4, HTMX, Alpine.js.
- Pattern di errore tipato (`apperror`).
- Bus eventi in-process per decoupling fra slice.
- Sessioni admin sqlite-backed + sessioni form in-memory.
- Rate limiter, CSRF helper, request logger.
- Validatori italiani (CF, CAP, telefono).
- Loader env con validazione production.
- Runner migrations numerate.
- Client mail (Resend + noop) e template engine.
- Schema admin_users + sessioni admin.
- Schema audit_entries + recorder che ascolta il bus.
- Schema template_email + dispatcher.

Senza un meccanismo di condivisione esplicito, le opzioni sono due:
copia-incolla (drift garantito a 6-12 mesi) oppure forzare un
monolite multi-tenant (accoppia il ciclo di vita di domini scorrelati).
Entrambe sono peggio di un kit estratto bene.

## Decision

### Cos'è chianti

Chianti è un **modulo Go separato** che contiene esclusivamente i
mattoncini infrastrutturali e di kernel condivisi fra i siti. Vive in
un repo git dedicato (`github.com/emiliopalmerini/chianti`, privato).
Non contiene logica di dominio.

I siti consumer (ITdG, giadataribelli, travels-and-dragons) lo
importano come dipendenza Go normale. Chianti non conosce i suoi
consumer.

### Layout dei repo

Repo separati, non monorepo. Lo sviluppo locale cross-repo è abilitato
da un `go.work` in `~/src/people-site/` che NON viene committato in
nessuno dei repo (è ambiente locale dello sviluppatore).

```
~/src/people-site/
  go.work                    # locale, .gitignored, mai committato
  chianti/                   # repo: emiliopalmerini/chianti
  ilturnodiguardia/          # repo esistente (ITdG)
  giadataribelli/            # repo futuro (riscrittura Go)
  travels-and-dragons/       # repo futuro
```

Esempio `go.work` locale:

```
go 1.25.2

use (
    ./chianti
    ./ilturnodiguardia
    ./giadataribelli
)
```

Quando un sito non ha bisogno di lavorare cross-repo, si rimuove dalla
`use` list e Go risolve `chianti` dal `go.sum` come dipendenza
normale.

### Versionamento

- Tag semver. Pre-1.0 (`v0.x.y`) sono ammessi breaking change senza
  procedure formali, comunicati nel CHANGELOG.
- I siti pinnano una versione esatta in `go.mod` e aggiornano quando
  vogliono.
- Promozione a `v1.0.0` solo dopo che almeno due siti girano stabili
  in produzione su una stessa minor del kit.

### Scope iniziale (primo round di estrazione)

Il primo round include solo i moduli **chiaramente generici e a basso
rischio**, in due livelli:

**Tier 1 (utility pure, zero I/O):**

- `kernel/apperror`: errori tipati (Kind, Wrap, Is).
- `kernel/clock`: astrazione tempo per test deterministici.
- `kernel/id`: generatore UUIDv7 string.
- `kernel/slug`: helper per URL slug.
- `kernel/eventbus`: pub/sub in-process.

**Tier 2 (infrastruttura HTTP, mail, config, sessioni):**

- `platform/italy`: validatori CF, CAP, telefono.
- `platform/httpx`: router builder, rate limiter, CSRF helper,
  request logger.
- `platform/session`: costruttori scs (admin sqlite-backed, form
  in-memory).
- `platform/email`: client Resend, noop mailer, interfaccia template
  service.
- `platform/config`: loader env con validazione produzione.
- `platform/migrations`: runner che applica file `.up.sql`/`.down.sql`
  numerati. I file SQL restano nei singoli siti.

### Fuori scope ora (Tier 3, deferred)

I seguenti slice **non entrano** in chianti nel primo round, anche se
sono candidati naturali. Vengono valutati dopo lo spike Giada, perché
solo allora abbiamo un secondo punto dati reale per confermare che la
forma è davvero condivisa:

- `auth` slice (admin login, sessioni, seeding).
- `audit` slice (audit_entries + recorder + reader).
- `email` slice (template_email + dispatcher + fallimenti).

Se lo spike conferma identità di forma, vengono promossi al kit con
ADR dedicati (uno per slice).

### Fuori scope sempre

- Slice di dominio (`evento`, `prenotazione`, `opzione`, `documento`,
  `prenotazionepubblica`, equivalenti dei nuovi siti). Questi sono
  per natura specifici al sito.
- UI pubblica (home, about, contatti) e UI admin (sidebar, topbar,
  pagine). Branding e navigazione sono per sito.
- Asset statici (CSS compilato, immagini, video, JS). Ogni sito
  spedisce i suoi.
- File `.up.sql` / `.down.sql` di migrazione. Solo il runner è
  condiviso, non gli schemi.

### Regola di promozione (rule of three)

Niente entra in chianti solo perché "sembra generico" guardando
ITdG. Una cosa diventa candidata al kit quando:

1. È implementata in modo **funzionalmente identico** in almeno due
   siti.
2. La sua API non incorpora decisioni di dominio specifico.
3. C'è un terzo consumer plausibile (anche solo "il prossimo sito")
   che la userebbe senza modifiche.

Questo riduce il rischio di astrarre con un campione di uno camuffato
da due.

### Strategia di migrazione di ITdG verso chianti

Una volta che chianti contiene il primo round:

1. ITdG aggiunge `chianti` come dipendenza in `go.mod`.
2. Per ogni modulo nel kit (uno alla volta, non in bulk), ITdG:
   - Rimuove l'implementazione locale.
   - Sostituisce gli import path.
   - Esegue `make test` finché tutto è verde.
   - Commit atomico con riferimento all'ADR specifico.
3. Ogni migrazione di slice/modulo ha un ADR dedicato in
   `ilturnodiguardia/docs/adr/` (ADR-047, ADR-048, ...).

Lo spike Giada parte dopo che ITdG importa con successo almeno
Tier 1. Non serve aspettare l'intera migrazione di Tier 2.

## Consequences

### Positive

- Bugfix infrastrutturali si propagano a tutti i siti aggiornando
  una sola dipendenza.
- I nuovi siti partono con una baseline solida invece che con un
  copia-incolla di ITdG.
- Il confine fra dominio e infrastruttura diventa esplicito e
  difendibile (se è in chianti, è infrastruttura).
- Lo sviluppo locale cross-repo resta fluido grazie a `go.work`
  senza dover pubblicare versioni per testare.

### Negative

- Cambi cross-cutting richiedono PR su due o tre repo. Mitigato dal
  `go.work` locale.
- Overhead di versionamento e CHANGELOG per chianti. Mitigato
  dall'essere pre-1.0 con breaking change ammessi.
- Rischio di estrarre prematuramente. Mitigato dalla rule of three
  e dall'esclusione esplicita del Tier 3 dal primo round.
- Rischio di drift se i siti non aggiornano la versione di chianti.
  Mitigato dall'essere pre-1.0 e dall'avere pochi consumer (3) tutti
  gestiti dalla stessa persona.

### Neutral

- ITdG continua a funzionare invariato durante la migrazione. Ogni
  step è atomico e reversibile.
- I file SQL di migrazione restano nei siti consumer; chianti non
  prescrive schemi, solo come applicarli.
