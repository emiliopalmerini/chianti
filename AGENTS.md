# Agent Notes

`chianti` is a shared Go architecture kit for small to medium shop sites for
Italian freelancers. Its main job is to enforce the way these sites are built:
small contracts, kernel primitives, stdlib-friendly helpers, and tested
patterns. It is not a convenience bundle for third-party infrastructure
integrations. It must not contain site domain logic, public UI, admin UI,
static assets, consumer SQL schemas, or consumer-specific wiring.

## Goals

- Keep the public surface small, stable, and boring.
- Preserve the boundary from `docs/adr/001-scope-e-strategia-multisito.md`.
- Preserve the architecture-kit direction from
  `docs/adr/013-kit-di-enforcement-architetturale.md`.
- Prefer concrete shared needs from multiple sites over generic-looking
  abstractions from a single consumer.
- Keep consumers independent. `chianti` must not know any specific site,
  business, brand, or consumer implementation.
- Optimize for architecture enforcement over boilerplate reduction. A little
  duplicated adapter wiring in consumer sites is acceptable when it keeps the
  shared kit simpler and more explicit.
- Preserve correctness before convenience. A breaking pre-1.0 change is fine
  when it makes the kit simpler or more correct, but document important changes.

## Scope Rules

- `kernel/*` packages are pure or near-pure building blocks such as typed
  errors, clocks, IDs, and the in-process event bus.
- `platform/*` packages are infrastructure contracts and helpers such as HTTP
  middleware primitives, config helpers, database/sql helpers, migration
  runners, email ports, and Italian validators.
- Consumers own concrete third-party integrations such as routers, SQLite
  drivers, session managers, CSRF libraries, authentication wiring, and
  production service adapters unless an ADR explicitly accepts that dependency
  inside `chianti`.
- Domain slices stay in consumer sites. Do not add booking, event, service,
  trip, payment, document, audit, auth, or admin-product behavior here unless
  an ADR explicitly promotes it.
- SQL migration files stay in consumer sites. The kit may provide runners, not
  site schemas.
- A candidate package belongs here only when it satisfies the rule of three in
  ADR-001: functionally identical in at least two sites, no embedded domain
  decision, and a plausible third consumer.

## Quality Rules

- Use normal Go package boundaries. Domain defines ports in consumer sites;
  `chianti` provides contracts and helpers those sites can import.
- Keep APIs narrow. Do not leak implementation details from platform packages
  into caller-facing types without a real need.
- Do not add a third-party dependency just to hide consumer boilerplate. If a
  package needs a concrete adapter, prefer defining a small interface or helper
  and keep the adapter in the consumer.
- Avoid flags, fallbacks, and variants for hypothetical future sites. Add them
  only when a real consumer needs them.
- Comment important behavior where the invariant is not obvious locally:
  ordering guarantees, failure isolation, production defaults, security policy,
  migration rules, cache or session lifetime.
- Prefer comments next to the implementation over separate explanatory files.
- Keep tests close to the package they specify. Tests are the spec for shared
  behavior because downstream sites will rely on it.
- Follow existing package style before introducing new helpers or abstractions.

## Safety

- Do not commit a `go.work`; it is local multi-project developer state.
- Treat pre-1.0 compatibility deliberately. Breaking changes are allowed, but
  make the blast radius clear for consumers.
- Do not make consumer-specific shortcuts in shared packages.
- Do not run destructive git operations unless explicitly requested.

## Layout

- `kernel/apperror`: typed application errors.
- `kernel/clock`: testable clock abstraction.
- `kernel/eventbus`: synchronous in-process pub/sub with handler isolation.
- `kernel/id`: ID types and UUID generation.
- `platform/config`: environment loading and production validation.
- `platform/database`: database/sql helpers and shared query conventions.
- `platform/email`: mailer interfaces and stdlib HTTP adapters where accepted.
- `platform/httpx`: net/http middleware primitives and HTTP error rendering.
- `platform/italy`: Italian data validators.
- `platform/migrations`: numbered SQL migration runner.
- `platform/session`: session contracts only if kept; concrete session manager
  adapters belong in consumers by default.
- `docs/adr`: architectural decisions. Read the relevant ADR before changing a
  package boundary.

## Testing

Use the project tools, currently plain Go commands:

```sh
go test ./...
```

For behavior changes, add or update the package tests first, verify they fail
for the missing behavior, then implement until the full test suite passes.
