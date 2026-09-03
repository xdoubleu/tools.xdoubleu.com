# ADR-0015: kobo-gateway is its own Go module, pinned to `GOTOOLCHAIN=go1.24.13`

- Status: Accepted
- Issues: [progrium/darwinkit#286](https://github.com/progrium/darwinkit/issues/286)
- Affects: `kobo-gateway/go.mod`, `kobo-gateway/Makefile`

## Context

kobo-gateway's menu bar needs cgo and the real AppKit/Xcode SDK
([`darwinkit`](https://github.com/progrium/darwinkit)).

Separately, darwinkit's AppKit bridge **`SIGABRT`s on launch under Go 1.25+**
(upstream issue #286, open and unfixed).

## Decision

### A separate module

`module tools.xdoubleu.com/kobo-gateway`, with its own `go.sum`. Pulling
darwinkit into the root/`api` module would force every Linux build/lint/test of
the main server to carry an Objective-C dependency it never uses.

### An explicit toolchain pin

The Makefile pins `GOTOOLCHAIN=go1.24.13` even though `go.mod` only states that
as a minimum. **A bare `go` directive can't downgrade a newer ambient Go on
`PATH`**, so every `make` target forces the exact toolchain regardless of what's
installed locally or in CI.

## Alternatives considered

### Keeping kobo-gateway in the root/`api` module

Rejected: it would impose an Objective-C dependency on every Linux build of the
server, which never uses it.

### Relying on `go.mod`'s minimum version alone

Doesn't work — a `go` directive sets a floor, not a ceiling, and cannot downgrade
a newer ambient toolchain. The crash would reappear for anyone with Go 1.25+
installed.

## Consequences

- **Never bump past 1.24.x alone, or the crash silently reappears** for anyone
  with a newer Go installed. Bump the pin and the `go.mod` minimum together, and
  only once upstream #286 is fixed.
- kobo-gateway needs its own lint/test workflows on a `macos-14` runner; it
  shares the root `.golangci.yml` (golangci-lint's config search walks up from
  the working directory).
- `make dist` needs `sips`/`iconutil`/`hdiutil` (standard macOS tools) to build
  `AppIcon.icns` and pack the `.dmg`.
- **macOS only** — cgo + Xcode command line tools; it will not cross-compile from
  Linux. This is also why it can't be a Docker build stage (ADR-0002).

## Revisit when

progrium/darwinkit#286 is fixed upstream — then bump the pin and the `go.mod`
minimum together.
