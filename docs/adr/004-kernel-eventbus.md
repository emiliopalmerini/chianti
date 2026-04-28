# ADR-004: Estrazione di `kernel/eventbus`

## Status

Accepted

## Context

`internal/kernel/eventbus` di ITdG è un pub/sub sincrono in-process
(82 righe + 130 righe di test) usato per coordinazione cross-slice.
Il bus ha tre garanzie chiave già verificate dai test esistenti:

1. **Best-effort delivery**: handler che fallisce o panica viene
   loggato ma non blocca i successivi.
2. **Registration order**: gli handler girano nell'ordine in cui sono
   stati registrati.
3. **In-flight snapshot**: un handler che si sottoscrive durante una
   `Publish` non viene invocato per l'evento corrente.

ITdG usa il bus per: capacità eventi che reagisce a prenotazioni,
audit recorder che ascolta ogni evento del catalogo, email dispatcher
che reagisce a `email.invio_richiesto`. Tutti i siti consumer
pianificati hanno bisogno della stessa primitiva.

## Decision

### Cosa entra in chianti

`chianti/kernel/eventbus/eventbus.go` + `eventbus_test.go` riprodotti
identici da ITdG. Nessun rename, nessuna firma cambiata.

### API esposta

```go
package eventbus

type Event interface {
    EventName() string
}

type Handler func(ctx context.Context, evt Event) error

type Bus interface {
    Subscribe(eventName string, h Handler)
    Publish(ctx context.Context, evt Event)
}

func New() Bus
func NewWithLogger(logger *slog.Logger) Bus
```

`Publish` non ritorna errore: il bus è "fire after commit" e il
chiamante non deve poter trattare la consegna come parte della sua
transazione (semantica esplicitata nel godoc di pacchetto).

### Test

I 7 test esistenti vengono copiati identici:

1. `TestPublishInvokesHandlers`
2. `TestPublishContinuesAfterHandlerError`
3. `TestPublishUnknownEvent`
4. `TestPublishRecoversFromHandlerPanic`
5. `TestPublishRunsHandlersInRegistrationOrder`
6. `TestSubscribeDuringPublishDoesNotRunForInFlightEvent`
7. `TestPublishIsSafeFromConcurrentGoroutines`

Coprono tutte le garanzie pubbliche. Niente da aggiungere.

### Niente cambia nel comportamento

- Nessun nuovo `Unsubscribe`.
- Nessun supporto async/queue/persisted.
- Nessun tipo `Topic` parametrico.
- Nessuna semantica "at-least-once".

Restano "non fatti" per design: chianti è un kit, non un broker.

## Consequences

### Positive

- Tutti i siti consumer ottengono lo stesso bus testato e identico,
  con le stesse garanzie su panic/error/order.
- L'`audit` slice (futuro Tier 3) potrà appoggiarsi al bus condiviso
  senza patch per-sito.

### Negative

- Trascurabili. Il bus non ha dipendenze esterne (solo stdlib).

### Neutral

- Il logger di default (`slog.Default()`) resta scelto dal consumer
  via `slog.SetDefault`, coerente con ITdG.
