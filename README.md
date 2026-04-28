# chianti

Kit Go condiviso per i siti di `~/src/people-site/`. Contiene
esclusivamente mattoncini infrastrutturali e di kernel: nessuna logica
di dominio.

## Status

Pre-1.0. Breaking change ammessi senza procedura formale, comunicati
nel CHANGELOG quando ci sarà.

## Scope

Vedi [`docs/adr/001-scope-e-strategia-multisito.md`](docs/adr/001-scope-e-strategia-multisito.md)
per il confine, la rule of three, e la roadmap di estrazione.

## Dev locale cross-repo

Crea un `go.work` (non committato) in `~/src/people-site/`:

```
go 1.26.1

use (
    ./chianti
    ./ilturnodiguardia
)
```

Aggiungi gli altri siti alla `use` list quando lavori cross-repo.
