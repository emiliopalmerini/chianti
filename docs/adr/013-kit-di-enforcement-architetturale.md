# ADR-013: Chianti come kit di enforcement architetturale

## Status

Accepted

## Context

Chianti nasce per evitare drift fra più siti Go indipendenti. Le prime
estrazioni hanno privilegiato la condivisione dell'infrastruttura concreta:
router `chi`, sessioni `scs`, CSRF `gorilla/csrf`, driver SQLite
`mattn/go-sqlite3`, rate limiter `golang.org/x/time/rate`.

Questa scelta riduce boilerplate nei consumer, ma cambia la natura del kit:
ogni adapter concreto porta dipendenze, transitive upgrade, vincoli di build e
decisioni operative dentro il modulo condiviso. Il risultato è comodo, ma meno
adatto al ruolo più importante di `chianti`: rendere esplicita
l'architettura che voglio applicare a ogni sito.

Il bisogno principale non è nascondere ogni integrazione ripetuta. Il bisogno
principale è impedire che i consumer divergano sui pattern fondamentali:
errori tipati, clock testabili, ID, event bus, confini fra dominio e adapter,
transazioni, rendering HTTP degli errori, validazione condivisa e convenzioni
di migrazione.

## Decision

Chianti è un kit di enforcement architetturale.

Il criterio primario per includere codice non è "riduce boilerplate", ma
"rende più coerente e verificabile l'architettura dei consumer".

Chianti può contenere:

- primitivi di kernel puri o quasi puri;
- contratti piccoli usati dai consumer;
- helper basati sulla standard library;
- convenzioni testate che non incorporano scelte di dominio;
- adapter HTTP verso servizi esterni solo quando non richiedono moduli Go
  terzi e il confine resta stretto;
- runner e utility che ricevono interfacce standard (`database/sql`,
  `net/http`, `io/fs`, `context`) invece di imporre implementazioni concrete.

Chianti non dovrebbe contenere per default:

- router concreti di terze parti;
- driver database concreti;
- session manager concreti;
- middleware CSRF concreti;
- slice di dominio o admin-product behavior;
- schemi SQL dei consumer;
- wiring applicativo di un sito;
- adapter introdotti solo per evitare poche righe ripetute nei consumer.

Le integrazioni concrete vivono nei consumer. Per esempio, un consumer può
continuare a usare `chi`, `scs`, `gorilla/csrf` e `mattn/go-sqlite3`, ma li
importa direttamente nel proprio `internal/platform/*`.

Una dipendenza esterna può comunque entrare in `chianti`, ma solo con ADR
esplicita che documenti:

1. perché un contratto o helper stdlib non basta;
2. quali consumer reali hanno lo stesso bisogno;
3. quale costo di build, sicurezza e upgrade viene accettato;
4. perché il confine resta architetturale e non diventa wiring di un sito.

Quando questa ADR entra in conflitto con ADR precedenti che promuovevano
adapter concreti, questa ADR guida la direzione futura. Le ADR storiche
restano utili per capire perché il codice esiste oggi, ma non obbligano a
mantenere dipendenze concrete nel kit.

## Consequences

### Positive

- Il public surface resta più piccolo e più stabile.
- I consumer restano liberi di cambiare router, session manager o driver senza
  trascinare tutto il kit.
- Il modulo condiviso diventa più semplice da aggiornare, testare e ragionare.
- Le dipendenze terze diventano scelte esplicite dei consumer, non del kernel
  architetturale comune.
- La regola di promozione diventa più severa: non basta che due siti copino lo
  stesso adapter, deve esserci valore architetturale nel condividerlo.

### Negative

- Alcuni adapter piccoli verranno duplicati nei consumer.
- La migrazione verso questa direzione può rompere API pre-1.0 già estratte.
- Alcuni test di integrazione diventano responsabilità dei consumer, soprattutto
  quando richiedono driver o middleware concreti.
- La comodità iniziale per un nuovo sito diminuisce leggermente.

### Neutral

- I consumer possono continuare a usare le librerie terze preferite.
- Chianti può ancora contenere infrastruttura, ma nella forma di contratti,
  helper stdlib e runner driver-agnostic.
- Le decisioni precedenti restano documentazione storica; il codice può essere
  semplificato in passaggi successivi.
