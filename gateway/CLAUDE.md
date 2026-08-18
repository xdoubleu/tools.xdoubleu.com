# gateway/ — routing + process supervision

The merged single-container deploy shape (issue #558; split into its own
binary in #904) puts three processes in one Docker image / one container:
this `gateway` binary is PID 1, and spawns `api` and the
Next.js standalone `web` server as supervised children, reverse-proxying
every request between them. It's a **separate Go module**
(`module tools.xdoubleu.com/gateway`, its own `go.sum`) — deliberately not
part of the `api` module — so it doesn't pull in `api`'s 70-field config or
any of its app-specific dependencies for what's fundamentally a thin routing
layer.

Not to be confused with `kobo-gateway/` at the repo root — an unrelated
macOS menu-bar app for Kobo device setup (own `CLAUDE.md`).

`cmd/gateway/main.go`'s logger/Sentry init (`sentrytools.NewLogHandler`,
`sentrytools.Init`) comes from the repo-root `sentrytools/` module — a
separate, tiny Go module (own `go.mod`) pulled in via a local `replace`
directive, not `api`'s config/dependency graph. It used to be a
byte-for-byte duplicate of `api`'s own copy; see root `CLAUDE.md`'s CI
section for how the two modules share it without either depending on the
other's `internal/` packages.

## Structure

```text
gateway/
├── cmd/gateway/main.go     # entrypoint: config, logger/Sentry init, starts both children, serves
└── internal/gateway/
    ├── config.go            # env-parsed Config — only what routing/supervision needs
    ├── child_process.go     # ChildProcess: generic supervised-child type (SIGTERM→SIGKILL, shutdown-on-crash)
    ├── children.go          # NewAPICmd/NewWebCmd — builds each child's *exec.Cmd + env
    └── proxy.go             # NewHandler — the /health, /api/*, everything-else routing switch
```

## Process supervision (`child_process.go`)

`ChildProcess` is used identically for both children: `Stop` sends SIGTERM,
escalating to SIGKILL after `childProcessStopTimeout` (10s) if the child
doesn't exit. If a child exits on its own without `Stop` having been called,
`shutdownSignal` (SIGTERM to this process) brings the whole gateway down —
serving requests against a dead child forever is worse than a container
restart, and DO's own restart policy handles the recovery.

## Child environments (`children.go`)

- **api**: our own trusted binary, gets the **full** parent environment
  (`os.Environ()`) — it needs DB/Supabase/OAuth/etc. secrets — with only
  `PORT` overridden to `cfg.APIPort` so it doesn't collide with gateway's own
  listener. `cmd/api/main.go` has no idea it's being proxied; it just reads
  `PORT` like it always has. This also means `api` inherits `RELEASE`
  straight from the container env with no code here needed — see below.
- **web**: an explicit narrow allowlist, not the full environment — it's a
  Node dependency, not code we own. `RELEASE` must always reach it: without
  it `getRelease()` returns `'dev'` in the browser and `gatewayNeedsUpdate`
  (`web/lib/books/gatewayClient.ts`) silently stops detecting installed
  kobo-gateway updates for every user. `KOBO_GATEWAY_RELEASE` is forwarded
  the same way — it's the release actually baked into the bundled
  kobo-gateway `.dmg`/binary, which can lag behind `RELEASE` (its own build
  can be cache-skipped when `kobo-gateway/` is unchanged, see root
  `CLAUDE.md`'s CI section); `gatewayNeedsUpdate` compares against this, not
  `RELEASE`, so it detects a genuinely newer kobo-gateway build rather than
  every deploy.

`Config.Release`/`Config.KoboGatewayRelease` (`config.go`) are plain
env-parsed fields, not compiled-in ldflags globals like `api`'s used to be —
nothing in this module needs to recompile just because the deploying
commit's SHA changed. `gateway`'s own build is cacheable for the same
reason (`build-gateway.yml`).

## Routing (`proxy.go`)

Replicates the two ingress rules the separate api/web components got for free
before #558 merged them into one (they were DO App Platform components then;
kamal-proxy fronts the single container now):

- `GET /health` → api child, unstripped (the health check hits the container
  port directly — `proxy.healthcheck.path` in `config/deploy.yml`; web has no
  health route).
- `GET /gateway/version` → answered directly by gateway itself (not
  proxied), `{"release": cfg.Release}` — same unproxied shape as `/health`.
  Consumed by `web/components/Footer.tsx` to show gateway's own release
  alongside `web`'s and `api`'s, since the three can now differ.
- `/api` and `/api/*` → api child, `/api` prefix stripped (so `API_URL`
  stays an absolute `https://.../api` URL and web needs no code changes).
  Preserves an existing quirk on purpose: `GET /api/version` is registered
  on the api mux itself, so its external path is `/api/api/version`.
- everything else → web child.

Both proxy targets can legitimately be "not ready yet" (api may still be
running migrations, web may still be starting) — a dead/unready upstream
gets a logged 503 (`upstreamResponseHeaderTimeout`, 15s), not a hang.

## Building

```bash
make run    # go run ./cmd/gateway — spawns real api/web children per your local env vars
make build  # ./bin/gateway
make test / make test/cov/report
make lint / make lint/fix   # golangci-lint, shared root .golangci.yml config
```

No cgo/AppKit constraint here (unlike `kobo-gateway/`) — builds and tests
run on ordinary Linux CI runners, `build-gateway.yml`/`gateway-lint.yml`/
`gateway-test.yml`, same shape as `api`'s. `gateway/Dockerfile` compiles the
binary and is built + pushed to its own GHCR repo by `build-gateway.yml`
(BuildKit-cached, see root `CLAUDE.md`'s CI section); the root `Dockerfile`
never compiles it, just pulls that image and `COPY --from=`s the binary in.
