# ADR-0009: Extract the slog→Sentry glue into its own module, consumed via a local `replace`

- Status: Accepted
- Issues: #926, #1038
- Affects: `sentrytools/`, `api/go.mod`, `api/Dockerfile`, `.github/workflows/build-api.yml`, `main.yml`

## Context

The slog→Sentry `LogHandler`/`Init` was duplicated byte-for-byte in two modules
(#926).

## Decision

`sentrytools/` is a standalone module (`tools.xdoubleu.com/sentrytools`), no
deployable artifact, pulled into `api` by:

```
replace tools.xdoubleu.com/sentrytools => ../sentrytools
```

It takes an `env string` and compares literals (`"development"`/`"test"`)
rather than importing `api`'s `config`.

## Alternatives considered

- **Duplicate copies** — drift on first edit.
- **Publish it** — versioning overhead for one in-repo consumer.
- **Import `api/config`** — couples the shared module to one consumer.

## Consequences

- A `sentrytools/` change can break `api`, so the `sentrytools` path filter is
  OR'd into `api`'s build/lint/test gate, alongside its own lint/test workflows.
- `build-api.yml` uses `context: .` with `file: ./api/Dockerfile`, which copies
  `sentrytools` before `api`. **Never narrow the context to a subdirectory**
  without re-homing `sentrytools`.

## Revisit when

A second repo needs it.
