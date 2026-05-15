# chianti

Kit Go condiviso per piccoli e medi siti di professionisti italiani. Il suo
scopo è rendere esplicita e riusabile un'architettura basata su contratti
piccoli, primitivi di kernel, helper stdlib-friendly e pattern testati.

`chianti` non è un bundle di integrazioni. I siti consumer restano
responsabili dei loro adapter concreti: router, driver SQLite, sessioni,
CSRF, UI, asset, schemi SQL e wiring applicativo.

## Status

Pre-1.0. Breaking change ammessi senza procedura formale, comunicati
nel [`CHANGELOG.md`](CHANGELOG.md).

## Scope

Vedi [`docs/adr/001-scope-e-strategia-multisito.md`](docs/adr/001-scope-e-strategia-multisito.md)
per il confine multi-sito originale e
[`docs/adr/013-kit-di-enforcement-architetturale.md`](docs/adr/013-kit-di-enforcement-architetturale.md)
per la direzione attuale: `chianti` deve enforceare architettura, non
massimizzare la riduzione del boilerplate.

In pratica:

- `kernel/*` contiene building block puri o quasi puri.
- `platform/*` contiene helper e contratti infrastrutturali che non
  incorporano scelte di dominio o dipendenze concrete non necessarie.
- Le integrazioni con librerie terze vivono nei consumer, salvo ADR
  esplicita che accetti il costo e il confine.
- Le slice di dominio, le UI, gli asset e gli schemi SQL restano sempre
  nei consumer.

## Dev locale cross-repo

Crea un `go.work` locale e non committato fuori dal repo quando devi lavorare
insieme a uno o più consumer:

```
go 1.26.1

use (
    ./chianti
    ./ilturnodiguardia
)
```

Aggiungi gli altri siti alla `use` list quando lavori cross-repo.
