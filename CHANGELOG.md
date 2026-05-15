# Changelog

All notable changes to this pre-1.0 module are documented here.

## Unreleased

### Breaking

- `platform/config.RandomKey` now returns `(string, error)` instead of
  silently ignoring entropy source failures.

### Changed

- `kernel/id.NewUUID` preserves lexicographic monotonicity for rapid calls in
  the same process.
- `platform/httpx.RenderError` renders only public `apperror.Msg` values and
  does not expose internal details in HTTP responses.
- `platform/database` SQLite constraint helpers prefer typed driver errors
  before falling back to legacy string matching.
- `platform/migrations` reports duplicate pending migration versions before
  applying any migration.
- `platform/httpx` parses `Accept` headers by media type and ignores invalid
  JSON-like substrings.
- `platform/httpx.CSP` validates source expressions before rendering header
  values.
- `platform/italy.FormatEuroCents` preserves the sign for negative amounts.
- `platform/migrations` requires six-digit migration version prefixes such as
  `000001_name.up.sql`.
