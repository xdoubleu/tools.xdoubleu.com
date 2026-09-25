# ADR-0015: kobo-gateway is its own Go module, pinned to `GOTOOLCHAIN=go1.24.13`

- Status: Accepted
- Issues: [progrium/darwinkit#286](https://github.com/progrium/darwinkit/issues/286)
- Affects: `kobo-gateway/go.mod`, `kobo-gateway/Makefile`

## Context

The menu bar needs cgo and AppKit via [`darwinkit`](https://github.com/progrium/darwinkit),
whose AppKit bridge **`SIGABRT`s on launch under Go 1.25+** (upstream #286,
unfixed).

## Decision

- **Separate module** `tools.xdoubleu.com/kobo-gateway` with its own `go.sum`,
  so Linux builds of `api` never carry an Objective-C dependency.
- **Makefile pins `GOTOOLCHAIN=go1.24.13`.** `go.mod`'s `go` directive is only a
  floor and can't downgrade a newer ambient Go.

## Alternatives considered

- **Keep it in `api`'s module** — imposes darwinkit on every server build.
- **Rely on `go.mod`'s minimum** — doesn't prevent Go 1.25+.

## Consequences

- **Never bump past 1.24.x alone.** Bump the pin and `go.mod` minimum together,
  only once #286 is fixed.
- macOS only (cgo + Xcode CLT); own lint/test workflows on `macos-14`, sharing
  the root `.golangci.yml`. Can't be a Docker stage (ADR-0002).
- `make dist` needs `sips`/`iconutil`/`hdiutil`.

## Revisit when

darwinkit#286 is fixed.
