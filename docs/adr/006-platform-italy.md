# ADR-006: Estrazione di `platform/italy`

## Status

Accepted

## Context

`internal/platform/italy` di ITdG raccoglie validatori e formattatori
italiani usati da public form, admin, riepiloghi PDF, e documenti
generati. Il pacchetto è puro Go (solo `fmt`/`strings`/`time`), senza
stato globale, senza dipendenze esterne. Apre il Tier 2 di chianti
perché:

- È platform-level (non kernel) ma tecnicamente indistinguibile da
  un kernel package: zero I/O, zero side effect.
- Tutti i siti consumer pianificati sono italiani: CF, CAP,
  province, telefono, formattazione date, formattazione euro sono
  servizi che useranno tutti.
- Apre la strada alle estrazioni Tier 2 più articolate (httpx,
  session, email infra) che invece hanno dipendenze e composizione.

## Decision

### Cosa entra in chianti

`chianti/platform/italy/validation.go` riproduce identico il
pacchetto di ITdG:

- `FormatDate(t)`: "2 gennaio 2026, ore 15:04".
- `FormatDateOnly(t)`: "2 gennaio 2026".
- `ValidFiscalCode(s)`: 16 alfanum, case-insensitive, sintattico.
- `ValidCAP(s)`: 5 cifre.
- `ValidProvince(s)`: 110 codici provincia ISO 3166-2:IT.
- `FormatEuroCents(c)`: cents → "€ N,NN".
- `ValidPhone(s)`: 6-20 char, plus opzionale iniziale, glifi di
  presentazione (spazio, punto, dash, parentesi).

### API esposta

Identica all'attuale ITdG. Nessun rename, nessun cambio di firma,
nessuna nuova funzione.

### Test

ITdG ha 3 test (CF, CAP, Province). Aggiungiamo 4 test che oggi
mancano:

1. `TestFormatDate`: una data di riferimento ritorna la stringa attesa.
2. `TestFormatDateOnly`: stessa data, senza componente ora.
3. `TestFormatEuroCents`: tabella di casi inclusi 0, 150, 4200, 99.
4. `TestValidPhone`: tabella di casi accettati e rifiutati (numero
   senza prefisso, internazionale, troppo corto, troppo lungo, plus
   non in prima posizione, char invalido).

Costo zero, il pacchetto pubblico ha copertura completa.

### Sintatticità rivendicata

Il package doc resta esplicito: i validatori sono sintattici
(`ValidFiscalCode` accetta una stringa di 16 alfanumerici, non
verifica che corrisponda a una persona reale). È una scelta esplicita
di ADR-008 di ITdG e si tiene.

### Niente cambia nel comportamento

- Nessun checksum nuovo (ADR-008 di ITdG ha esplicitamente escluso
  validazione strong del CF; se domani serve, ADR dedicato).
- Nessuna geolocalizzazione (CAP→provincia non si fa qui).
- Nessuna formattazione regionale alternativa.

## Consequences

### Positive

- Tutti i siti consumer ottengono validatori italiani identici e
  testati senza copia-incolla.
- ADR-001 di chianti elencava `platform/italy` come Tier 2; questa
  ADR onora quel commit.
- Apre il pattern "pacchetto puro Tier 2" che useremo come
  riferimento per i prossimi (httpx, ecc. saranno più complicati ma
  partiranno da qui).

### Negative

- Trascurabili. Stdlib only, niente nuove dipendenze.

### Neutral

- I 110 codici provincia sono codificati come tabella esplicita.
  Aggiornamenti futuri (riforme amministrative) richiedono modifica
  del kit; accettabile vista la stabilità del set.
