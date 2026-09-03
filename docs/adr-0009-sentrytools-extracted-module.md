# ADR-0009: Extract the slog→Sentry glue into its own module, consumed via a local `replace`

- Status: Accepted
- Issues: #926, #1038
- Affects: `sentrytools/`, `api/go.mod`, `api/Dockerfile`, `.github/workflows/build-api.yml`, `main.yml`

## Context

The slog→Sentry `LogHandler` and `Init` were duplicated **byte-for-byte**
between `api`'s own copy and a second consumer (#926). That second consumer — the
`gateway/` module — was itself retired in #1038, but the extraction stands.

## Decision

`sentrytools/` is a tiny standalone Go module (own `go.mod`,
`module tools.xdoubleu.com/sentrytools`) with no deployable artifact of its own.

`api` pulls it in via a local directive in its own `go.mod`:

```
replace tools.xdoubleu.com/sentrytools => ../sentrytools
```

This is not a published module, just a same-repo path reference.

`sentrytools` intentionally takes an `env string` parameter and compares against
plain literals (`"development"`/`"test"`) rather than importing `api`'s own
`config` package — there is no reason to couple a tiny shared module to one
particular consumer's config shape.

## Alternatives considered

### Keeping the duplicate copies

That was the status quo #926 fixed. Two byte-identical copies drift the moment
one is edited.

### Publishing it as a real module

Rejected: it has exactly one consumer in one repo. A published module would add
versioning and release overhead for no isolation benefit.

### Importing `api`'s `config` package for the environment check

Rejected — it would couple the shared module to one consumer's config shape,
which is the opposite of why it was extracted.

## Consequences

- **A change to `sentrytools/` can break `api` without touching `api/`'s own
  subtree.** The `changes` job's `sentrytools` path filter is therefore OR'd into
  `api`'s own build/lint/test gate in `main.yml`, on top of
  `sentrytools-lint.yml`/`sentrytools-test.yml` linting and testing it
  standalone.
- **It changed `build-api.yml`'s Docker build context.** It used to build with
  `context: ./api` (the Dockerfile's own directory), but `../sentrytools` falls
  outside that context entirely, so it now builds with `context: .` (repo root)
  plus an explicit `file: ./api/Dockerfile`. `api/Dockerfile` itself
  `COPY sentrytools ./sentrytools` before its own module's `COPY`, then
  `WORKDIR`s into `api/` to build.

  **Never change that Dockerfile's context back to a subdirectory** without also
  re-homing `sentrytools`.

## Revisit when

A second repo needs this code, at which point publishing it properly becomes
worth the overhead.
