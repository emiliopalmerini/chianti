# ADR-003: Estrazione di `kernel/clock`

## Status

Accepted

## Context

`internal/kernel/clock` di ITdG è un'astrazione minima del tempo
corrente (21 righe), usata da domain e application code per restare
testabili. Espone un'interfaccia `Clock` con `Now()` e due
implementazioni: `System()` (UTC reale) e `Fixed(t)` (deterministica
nei test).

È puro Go, zero dipendenze esterne, semantica già consolidata in
ITdG. Insieme ad apperror è uno dei mattoncini più trasversali: ogni
slice che genera o legge timestamp dipende da una `Clock`.

## Decision

### Cosa entra in chianti

`chianti/kernel/clock/clock.go` riproduce identico il pacchetto di
ITdG. Nessun rename, nessuna firma cambiata.

### API esposta

```go
package clock

type Clock interface {
    Now() time.Time
}

func System() Clock         // ritorna UTC time.Now()
func Fixed(t time.Time) Clock
```

`System()` ritorna sempre UTC (scelta di ITdG che si tiene). Tutti i
siti consumer pianificati gestiscono Italia ma persistono UTC, quindi
la convenzione regge.

### Test

ITdG non ha test su `clock` (è troppo banale). In chianti aggiungiamo
3 micro-test:

1. `System().Now()` ritorna in UTC.
2. `Fixed(t).Now() == t` esattamente.
3. `Fixed(t)` due chiamate ritornano lo stesso valore (immutabile).

Costo zero, copertura del 100% di un pacchetto pubblico.

### Niente cambia nel comportamento

- Nessun metodo nuovo.
- Nessuna implementazione nuova (`MutableClock`, `Tick`, ecc.).
- Niente fuso orario non-UTC.

## Consequences

### Positive

- Testabilità mantenuta: i test ITdG che usano `Fixed` continuano a
  funzionare senza modifiche.
- Pacchetto pubblico minuscolo, API stabile.

### Negative

- Trascurabili. È letteralmente il pacchetto più piccolo del kit.

### Neutral

- I 3 nuovi test sono debito di copertura saldato, non comportamento
  nuovo.
