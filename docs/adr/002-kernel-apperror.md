# ADR-002: Estrazione di `kernel/apperror`

## Status

Accepted

## Context

Il pacchetto `internal/kernel/apperror` di ITdG è un wrapper di errore
tipato con `Kind`, messaggio user-safe, dettaglio interno, errori
field-level, e wrapping standard Go. È usato in 33 file di ITdG: ogni
slice di dominio lo usa per dichiarare invarianti, ogni adapter HTTP
lo legge tramite `httpx.RenderError` per mappare `Kind` a status code.

Il pacchetto è il candidato ideale per il **primo modulo di chianti**
perché:

- **Zero dipendenze esterne**: solo `errors` e `fmt` da stdlib.
- **API piccola e stabile**: 6 costruttori, 1 tipo, 1 enum, 2 helper.
  Non sono cambiati significativamente in nessuna ADR successiva.
- **Generico per natura**: nessuna decisione legata al dominio
  prenotazione (l'unico residuo italianeggiante è il messaggio
  "non trovato" / "errore interno", ma sono stringhe utente, non
  vincoli architetturali).
- **Semantica già consolidata**: i test esistenti coprono i casi
  d'uso reali (NotFound, Validation con fields, wrapping con
  `errors.Join`, Internal che preserva l'errore originale).

Estraendolo come primo modulo validiamo l'intero workflow
chianti↔consumer (import via `go.work`, sostituzione path, test verdi)
con il rischio più basso possibile.

## Decision

### Cosa entra in chianti

Tutto il contenuto attuale di `internal/kernel/apperror/` di ITdG
viene riprodotto identico in `chianti/kernel/apperror/`:

- `apperror.go`: `Kind`, `Error`, costruttori (`NotFound`,
  `Validation`, `Conflict`, `Unauthorized`, `Forbidden`, `Internal`),
  helper (`Is`, `FieldsOf`).
- `apperror_test.go`: i 4 test esistenti, adattati al nuovo import
  path (`github.com/emiliopalmerini/chianti/kernel/apperror`).

### API esposta

Identica all'attuale. Nessun cambio di firma, nessun rename, nessuna
aggiunta. Lo scopo di questa ADR è pura **traslocazione**, non
evoluzione.

```go
package apperror

type Kind int
const (
    KindInternal Kind = iota
    KindNotFound
    KindValidation
    KindConflict
    KindUnauthorized
    KindForbidden
)
func (k Kind) String() string

type Error struct {
    Kind    Kind
    Msg     string
    Detail  string
    Fields  map[string]string
    // wrapped (unexported)
}
func (e *Error) Error() string
func (e *Error) Unwrap() error

func NotFound(resource, id string) *Error
func Validation(msg string, fields map[string]string) *Error
func Conflict(msg string) *Error
func Unauthorized(msg string) *Error
func Forbidden(msg string) *Error
func Internal(err error) *Error

func Is(err error, kind Kind) bool
func FieldsOf(err error) map[string]string
```

### Messaggi utente in italiano

I costruttori `NotFound` e `Internal` producono messaggi in italiano
("non trovato", "errore interno"). Si tiene così, coerente con
ADR-015 di ITdG (Italian ubiquitous language) e applicabile a tutti
i siti consumer pianificati (tutti italiani, conferma utente).

Se un futuro sito non-italiano usa chianti, verrà aperta una nuova
ADR per parametrizzare i messaggi (probabile: tornare stringhe `Kind`
e lasciare al chiamante la traduzione). Non lo facciamo oggi perché
non c'è il caso d'uso (regola: niente generalizzazione preventiva).

### Test

Si copiano i 4 test esistenti adattando l'import path:

1. `TestNotFound`: kind e detail coerenti.
2. `TestValidationFields`: `FieldsOf` ritorna i field error.
3. `TestIsWithWrapped`: `Is` traversa `errors.Join`.
4. `TestInternalPreservesUnderlying`: `errors.Is` trova l'errore
   wrapped.

Si aggiunge **un test in più** che oggi manca: `Kind.String()` per
ogni valore enum. Costo zero, copertura della tabella `switch`.

### Niente cambia nel comportamento

- Nessuna nuova `Kind` aggiunta.
- Nessuna firma cambiata.
- Nessun campo nuovo in `Error`.
- Nessun helper nuovo (no `Wrap`, `WithField`, ecc.).

### Migrazione consumer (ITdG)

ITdG documenta la sua parte di migrazione con un ADR proprio
(ITdG ADR-047). In sintesi: aggiunge `chianti` come dep, sostituisce
gli import path nei 33 file, elimina il pacchetto locale, run dei
test.

## Consequences

### Positive

- Primo passo concreto verso il kit condiviso, con rischio minimo.
- Workflow `go.work` validato end-to-end prima di affrontare moduli
  più grossi.
- ITdG resta verde e funzionante; la migrazione è meccanica e
  reversibile.
- Stessa API, zero learning curve per chi conosce già ITdG.

### Negative

- 33 file di ITdG cambiano import path in un singolo commit.
  Mitigato: cambio puramente meccanico (find/replace), nessuna
  modifica semantica.
- Apperror in chianti diventa pubblico; futuri cambi richiedono
  comunque coordinare i consumer. Mitigato: l'API è stabile e
  pre-1.0 ammette breaking change.

### Neutral

- I messaggi italiani restano nel kit. È una scelta esplicita,
  rivedibile via ADR se arriva un consumer non-italiano.
