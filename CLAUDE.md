# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Monorepo Overview

Go 1.26 backend (`api/`) serving multiple apps from a single binary, paired with a Next.js 16 / React 19 frontend (`web/`, standalone Node server). `api` and `web` deploy as two independent Kamal services behind one shared kamal-proxy instance, routed by kamal-proxy's own path-based routing (see Deploy shape below). Apps share a single HTTP mux and expose ConnectRPC endpoints. Each app owns its own PostgreSQL schema; shared proto definitions live in `proto/`. A separate macOS-only Go module, `kobo-gateway/`, ships as a downloadable menu-bar helper (own `CLAUDE.md`). `sentrytools/` is a tiny third Go module holding slog→Sentry glue `api` pulls in via a local `replace` directive — see the CI section below.

Apps: **games**, **books** (Go package `apps/books`, schema `books`, proto `books.v1`), **feeds**, **watchparty**, **recipes**, **mealplans**, **shoppinglist**, **todos**, **dashboard** (no schema — see below). All apps are registered in `api/cmd/api/apps.go` (implements the `App` interface: `Routes`, `ApplyMigrations`, `GetName`, `GetDisplayName`, `GetDomain`, `Start`) — migrations run sequentially in registration order, so schema dependencies between apps (e.g. books adopting the old backlog schema before games drops it) dictate the list order. **dashboard** (issue #737) centralizes the public Games and Reading (books+feeds) dashboards — both private/owner and public/shared views — plus the share-token lifecycle; it owns no schema of its own and reaches games/books/feeds only through exported Go methods on those apps' structs (e.g. `Games.BuildSharedSteam`, `Books.BuildSharedLibrary`, `Feeds.BuildSummary`), never their internal packages or schemas directly, since Go's `internal/` visibility rules block that. `games`/`books` no longer have any dashboard-shaped route or public-sharing code of their own — see `api/CLAUDE.md`'s "Public Dashboard Sharing" section.

Shared Go code lives in `api/internal/` (auth, config, encryption, contacts, observability, digitalocean, github, sentryapi, mailer, oauthconn, mcptools, repositories, safedial, testhelper). Any outbound fetch of a
**user-supplied URL** must go through `api/internal/safedial` — its dialer
refuses non-public IPs, which is what keeps books/feeds from being
turned into an SSRF pivot against the container's own network. Each app under `api/apps/<name>/` follows: `internal/{models,repositories,services,jobs,helper,mocks}`, `migrations/`, and (where relevant) `pkg/`.

**Deploy shape (issue #558, split into 3 processes in #904, split back into 2 independent services in #1038):** `api` and `web` each build their own Docker image (`api/Dockerfile`, `web/Dockerfile`) and deploy as two independent Kamal apps (`config/deploy.api.yml`, `config/deploy.web.yml`) to the Hetzner VPS (`infra/`), sharing one kamal-proxy instance and domain. `api`'s config registers `proxy.path_prefix: "/api,/.well-known"` (unstripped — kamal-proxy can't strip only one of several prefixes on a single service, so `api/cmd/api/kamal_proxy_shim.go`'s `stripAPIPathPrefix` middleware does the `/api` stripping itself, leaving `/.well-known/*` — RFC 9728/8414 OAuth discovery — untouched); `web`'s config has no `path_prefix`, so it's the catch-all for everything else. This replicates the ingress split a hand-rolled `gateway/` Go module used to provide as PID 1 in a single merged container (issue #558's original reason for merging was DigitalOcean App Platform billing per component; DO was decommissioned in #1113, removing the reason the merge — and `gateway/` — existed at all).

**Long-request handler deadlines:** a handler that outlives the edge proxy's response timeout gets its connection reset before it writes a byte — no server log, no Sentry event. This bit `GetDeployLogs` twice (issue #672) under the old DigitalOcean edge (Envoy, a hard ~25s that wasn't configurable), and the deadlines added then are still in force: `deployLogsCtxTimeout` in `api/cmd/api/routes.go` = 20s, `liveLogDeadline` in `api/internal/digitalocean/logs_live.go` = 8s. Since #1113 the edge is kamal-proxy, whose ceiling *is* configurable (`proxy.response_timeout` in `config/deploy.api.yml`, currently unset so Kamal's default applies) — so raising a handler deadline is now possible, but must be paired with raising that. Any handler must still finish (or start streaming) under whichever ceiling is in effect; a deadline set above it never fires. Enforced only by convention/comments — no test asserts these constants stay under it.

A largely read-only **MCP server** at `/apps/mcp` (MCP OAuth 2.1, entirely first-party as of issue #1039 — `api` is both resource server and, via an embedded `ory/fosite` authorization server in `api/internal/oauth2as`, the authorization server too, no external Auth provider involved) exposes each app's own read RPCs as `<app>_<rpc>` tools plus 11 unprefixed admin observability tools, so a local Claude CLI can pull production domain data and system health as context. Apps opt in by implementing `MCPToolProvider` (`api/cmd/api/apps.go`); shared gating/marshaling lives in `api/internal/mcptools`. App tools are gated by the caller's own per-app access and return only that user's data; observability tools require admin. One observability tool, `resolve_sentry_issue`, is a deliberate exception to "read-only" — it lets an admin-authenticated agent close out a Sentry issue it just filed a fix for. See the "Apps MCP server" section in `README.md` for the full auth flow and setup.

**MCP coverage gaps:** if the user describes a production issue and there's no MCP tool that surfaces it, or an existing tool returns wrong/incomplete data, fix that gap first (add/correct the tool) before investigating the issue itself — otherwise the same blind spot just recurs next time. Note the gap and fix in this file when it happens.

- *Issue #1027* — Supabase restricted the whole project for blowing its monthly egress quota, and nothing could say which endpoint caused it: `get_usage_stats` counted **requests** per endpoint but never bytes, so an endpoint returning 2 MB a call looked identical to one returning 200 B. `global.usage_daily` gained a `bytes` column and `get_usage_stats` now reports it. The database is reached over a transaction-mode pooler and billed per byte returned, so "which endpoint moves the most data" is the question that matters — see `api/CLAUDE.md`'s Database Conventions for the query rule this exists to enforce.

## Code Navigation (ast-grep)

**Prefer `ast-grep` over `grep`/`rg` for code searches** — it understands syntax trees, so results are exact (no false positives from comments/strings). Reserve `grep`/`rg` for non-code files (logs, configs, docs).

```bash
ast-grep run --pattern 'func FunctionName($$$) $$$' --lang go
ast-grep run --pattern 'const $VAR: TypeName = $$$' --lang typescript
ast-grep run --pattern '...' --lang go api/apps/recipes/   # scope to a subtree
```

`$NAME` matches one node, `$$$` matches zero or more, `$$` matches one complex expression.

**Comments must describe current behavior, not history.** Never write a comment that references removed code, superseded architecture, or frames a landed change as still-pending — a stale claim actively misleads the next reader (human or Claude). If historical context genuinely explains *why* the current code looks the way it does, phrase it so it stays true regardless of when it's read (e.g. "replicating what X used to provide" rather than "X hasn't happened yet") — see `api/cmd/api/kamal_proxy_shim.go`'s note on replicating what the now-retired `gateway/` module used to provide for the pattern to follow.

**Do not read `api/gen/`, `api/internal/mocks/`, `api/apps/*/internal/mocks/`, or `web/lib/gen/`** to discover field names, RPC signatures, or mock signatures — read the corresponding `.proto` file in `proto/` or the source interface instead; it's smaller and is the source of truth.

## Commands

```bash
# API (from api/)
docker-compose up -d       # start Postgres (needed before running/testing)
make run                   # go run ./cmd/api
make test                  # go test -p 1 ./...
make lint                  # golangci-lint + sqlfluff + buf lint
make lint/fix               # auto-fix (golines, golangci-lint --fix, gci, sqlfluff, buf lint)
make test/cov/report        # coverage report
make build                  # go build ./cmd/api
make scaffold NAME=x [DB=true] [JOBS=true]   # generate a new app skeleton
docker-compose down

# Web (from web/)
npm run dev
npm test                    # jest
npm run lint                 # eslint + tsc --noEmit + prettier --check + knip + syncpack
npm run lint:fix
npm run build                # required before finishing web tasks, see below
npm run generate             # regenerate lib/gen/ ConnectRPC clients from proto

# Kobo Gateway (from kobo-gateway/, macOS only — cgo + AppKit)
make build / make dist / make test / make lint/fix

# Proto (when any .proto file changes, run BOTH generators)
cd api && make proto/generate   # regenerates api/gen/
cd web && npm run generate      # regenerates web/lib/gen/
```

Generated stubs (`api/gen/`, `web/lib/gen/`) ARE committed; CI's proto-staleness check also runs `buf lint` (e.g. RPC response types must be named `<Method>Response`) — run `make lint/proto` locally first. **`make proto/generate` must be the last step touching `api/gen/` before committing** — `make lint/fix`'s `gci` pass runs across the whole repo (including generated files) and reorders their import groups, which CI's proto-staleness check flags as stale since it diffs a raw `buf generate` (never gci'd) against the committed files. If `make lint/fix` ran after (or in the same session as) `make proto/generate` for any reason, re-run `make proto/generate` afterward and `git diff api/gen web/lib/gen` should show nothing before committing.

Run a single Go test: `go test ./apps/books/internal/services/... -run TestName -v` (from `api/`). Single Jest test: `npx jest path/to/file.test.ts -t "test name"` (from `web/`).

## Adding a New Tool

```bash
cd api && make scaffold NAME=mytool [DB=true] [JOBS=true]
```

The scaffold does **not** auto-register the app — after scaffolding: register it in `api/cmd/api/apps.go`, implement handlers/routes in `api/apps/mytool/routes.go`, add domain logic under `api/apps/mytool/internal/`, edit the initial migration if `DB=true`, then `make build` to verify.

## Starting a Task

Before exploring or reading code for a task, always pull the latest `main` first (`git checkout main && git pull` from the repo root, or `git fetch origin main`) — don't explore against a stale checkout, since another session or the user may have merged changes since.

Before making any changes, always create a completely fresh worktree off up-to-date `main` — never edit in the main checkout or reuse an existing branch/worktree, even one from earlier in this same task; it may already be merged or based on a stale `main`. Prefer the `EnterWorktree` tool; if unavailable, fall back to:

```bash
git checkout main && git pull
git worktree add ../<descriptive-branch-name> -b <descriptive-branch-name> main
```

**After `EnterWorktree` (or the fallback) returns, every subsequent Read/Edit/Write absolute path must be rebased onto the new worktree directory it reports** — do not keep reusing an absolute path prefix from earlier in the session (e.g. the original checkout, or a prior worktree). Nothing rewrites old paths for you: a stale prefix silently edits the wrong checkout, and since a Bash `cd` doesn't persist between tool calls either, `pwd` alone won't catch it. If this happens, recover by diffing the wrongly-edited files (`git diff --cached`/`git diff`), restoring that checkout to clean, and applying the diff (`git apply`) in the correct worktree — don't just re-run the edits from memory, since that risks drift from what was actually tested.

Before editing, always create a tracking GitHub issue for the work via the `refine-issue` skill (not a bare `gh issue create`), so Priority/Status/labels get set — do this even for work that wasn't explicitly requested as an "issue", e.g. tooling/doc changes. If a finalized plan exists (from plan mode or otherwise), record it in the issue's `## Plan` section before the first edit, and move Status to "In progress" at that point.

## Finishing a Task — Required Final Steps

1. **Lint** — `cd api && make lint/fix` and/or `cd web && npm run lint` (whichever area changed); `cd kobo-gateway && make lint/fix` for kobo-gateway changes; `cd sentrytools && make lint/fix` for sentrytools changes (and re-run `go mod tidy` in `api` if its public API changed, since `api` depends on it via a local `replace`).
2. **Coverage** — target ≥80% on changed code. API: `cd api && docker-compose up -d && make test/cov/report && docker-compose down` (always stop the DB after). Web: `cd web && npm run test:cov`.
3. **Build** (web changes only) — `cd web && npm run build`. Next.js's server/client boundary check (a Server Component importing anything from a file that pulls in client-only hooks) is enforced **only** by `next build`, not `tsc --noEmit`, ESLint, or Jest — lint/coverage passing does not mean the build passes. Put constants shared across the boundary in a plain `lib/` module with no React imports.
4. **Open the PR yourself** — don't wait to be asked:
   ```bash
   git push -u origin HEAD
   gh pr view --json number >/dev/null 2>&1 || gh pr create --fill --base main
   ```
   Never push to `main` directly; never open as `--draft`. Reference the tracking issue from "Starting a Task" in the PR body using a closing keyword (e.g. `Fixes #123`, `Closes #123`) so the issue auto-closes on merge — a bare `#123` or "Related to #123" leaves the issue open even after merge (this happened with issue #727 / PR #728).
   - **Small, code-only changes**: no `CLAUDE.md`, `Makefile`/npm-script, lint config, CI workflow, or script edits, AND none of the "larger/architectural" signals below apply — enable auto-merge right away, in the same breath as creating the PR — `gh pr merge --auto --squash` only merges once checks pass, so there's no reason to wait for green first: `gh pr create --fill --base main && gh pr merge --auto --squash`.
   - **Tooling/harness changes, or larger/architectural code changes**: do **not** enable auto-merge — open a normal (non-draft) PR and wait for the user's own review. Tooling/harness means anything touching `CLAUDE.md`, Makefile targets, lint config, `.github/workflows/*`, scripts, or hooks. Larger/architectural means any of: a diff of roughly >150–200 changed lines or >8 files (check `git diff --stat` against `main` before opening the PR); a new/changed public interface, edits under a shared `api/internal/*` package, a new/modified DB migration, a new app registered in `api/cmd/api/apps.go`, a new/changed proto RPC, or changes spanning more than one app under `api/apps/*` or `web/`; or any `go.mod`/`package.json` dependency addition, removal, or version bump.
5. **Monitor CI until green, fixing it yourself if it isn't**:
   ```bash
   gh pr checks --watch
   gh pr view --json mergeable,mergeStateStatus,statusCheckRollup
   ```
   A red PR or non-`MERGEABLE` state is not "done" — diagnose the actual failure (don't just re-run blindly) and repeat from step 1. Once green + mergeable, report the PR URL (auto-merge was already armed in step 4 for small code-only changes; for tooling/harness or larger/architectural changes, stop here and wait for review).
6. **Reflect on whether this change exposed a doc/tooling gap** — once CI is green, look back at the commit range for this task and ask: (a) would a CLAUDE.md addition/correction, a new Make/npm target, a lint rule, a CI workflow tweak, or a script have made this specific change faster or safer to implement; (b) did the PR need more than one push to go green (check `gh pr checks`/`gh run list` and the commit log for fixup commits), and if so, what local check would have caught the failure before pushing. Only flag something concrete tied to what actually happened — not speculative "would be nice" additions. If nothing is worth flagging, stop here. If something is: in a **separate** fresh worktree off `main`, open a tracking issue, edit only `CLAUDE.md`/tooling files, and open an independent, non-draft PR referencing it (never stacked on the original PR) — following this same checklist for that PR too.

## CI

`.github/workflows/main.yml` orchestrates reusable workflows (`proto-check`, `build-api`, `build-web`, `build-kobo-gateway`, `api-lint`, `web-lint`, `kobo-gateway-lint`, `api-test`, `web-test`, `kobo-gateway-test`) gated by a `changes` path filter. `kobo-gateway-test`/`kobo-gateway-lint` run on a `macos-14` runner (needs to compile cgo/AppKit). The `api`/`web`/`kobo_gateway` filters exclude `**/*.md` (e.g. a `CLAUDE.md`-only change doesn't set that filter's output), so a docs-only PR triggers none of the build/lint/test jobs — `ci-pass` then has nothing to wait on and skips triggering Codecov entirely (see its "Trigger and wait for Codecov to report" step, guarded on at least one test job having actually run).

`api`/`web` no longer bake the deploying commit's SHA into their compiled artifacts (only `kobo-gateway` still does — see below) — `RELEASE` is a plain container `ENV`, read at runtime (`api/internal/config`, `web/lib/env.ts`). `api` and `web` each have their own `Dockerfile` (`api/Dockerfile`, `web/Dockerfile` — real multi-stage builds ending in a runnable final stage, deployed directly rather than assembled into a merged image) and their own public GHCR repo (`ghcr.io/xdoubleu/tools.xdoubleu.com/{api,web}`), built via `docker buildx build` in `build-api.yml`/`build-web.yml`, cached via BuildKit's own `type=gha,scope=<component>` layer cache — no hand-computed `hashFiles()` cache key is reproduced across jobs anywhere in this path (issue #948 originally tried that and it broke: `build-web.yml` computed `hashFiles('web/**')` *after* `npm ci`, folding the gitignored `node_modules` tree into the key, which only ever self-matched within that one job). `main.yml`'s `build-api`/`build-web`/`build-kobo-gateway` jobs are each gated only on their **own** subtree changing (`api`/`proto` for `build-api`; `web`/`proto`/`kobo_gateway` for `build-web`; `kobo_gateway` for `build-kobo-gateway`; any of them also run on a `.github/workflows/**` change) — not on any image-affecting change generally. Each pushes `:latest` + `:<sha>` (full commit SHA) only on push to `main`; PR runs build (validating the image actually compiles) but never push, so `docker/build-push-action@v7`'s own step already provides the PR-time build validation a separate assembly job used to (issue #900's original motivation for a merged-image `docker-check` job no longer applies once there's no merged image). `main.yml`'s `deploy-kamal` job accepts `skipped` as well as `success` for `needs.build-api.result`/`needs.build-web.result`, since a build job may legitimately not run — but only actually runs a given service's `kamal deploy` step when that service's own build job produced a fresh `:<sha>` tag (`if: needs.build-api.result == 'success'` / `== 'web'`, see below), since a skipped build means nothing changed for that service and there's no new tag to deploy.

kobo-gateway stays on the old `actions/cache`-restore-into-build-context mechanism (`build-web.yml` restores `kobo-gateway/dist/kobo-gateway` from the `build-kobo-gateway-${{ hashFiles('kobo-gateway/**') }}` cache entry `build-kobo-gateway.yml` writes, requiring the same `ref: ${{ github.head_ref || github.ref_name }}` checkout override so both sides compute the same hash) — it cannot be a Docker build stage at all: it needs cgo + the real AppKit/Xcode SDK (see `kobo-gateway/cmd/kobo-gateway/menubar_darwin.go`'s `//go:build darwin`), which no Linux container can produce, and its two output files (the raw binary + `.dmg`) are never executed inside the deploy image anyway — just served as static downloads. A cache miss here (cold cache, or eviction) fails the job loudly with instructions to re-run `build-kobo-gateway.yml` via `workflow_dispatch`.

`kobo-gateway`'s own release stays compile-time-stamped (`-X main.Release=...` in `build-kobo-gateway.yml`'s `make dist`) since it's a `.dmg`/binary that leaves the container and runs on a user's own machine — no deploy-time environment to read a version from. Its build is still cacheable (skips `make dist` when `kobo-gateway/` is unchanged), which means the release baked into the currently-bundled artifact can legitimately be older than the rest of the deploy. `build-kobo-gateway.yml` records the SHA it actually used in a `dist/kobo-gateway/RELEASE` file (survives the cache); `build-web.yml` reads that into a `KOBO_GATEWAY_RELEASE` build-arg, baked into `web/Dockerfile`'s image `ENV` directly, so `gatewayNeedsUpdate` (`web/lib/books/gatewayClient.ts`) compares a user's installed kobo-gateway against what's actually bundled, not this deploy's overall SHA. `web/components/Footer.tsx` surfaces `web`/`api`'s own (possibly differing) short release hashes for the same reason — nothing forces them to match.

- **On PRs**: full suite runs; `ci-pass` aggregates and is the required check. It also gates on Codecov's `codecov/project`/`codecov/patch` statuses — `codecov.yml` sets `notify.manual_trigger: true` so Codecov posts nothing until `ci-pass` runs `codecovcli send-notifications` after every test job uploads (upload count varies by path filter, so a fixed `after_n_builds` doesn't work).
- **On push to `main`**: `proto-check`/`api-lint`/`web-lint`/`kobo-gateway-lint` don't re-run (PR's green checks are trusted), but `build-api`/`build-web`/`build-kobo-gateway` do — they're the sole producers of the deploy artifacts, not just PR-time validation; test jobs still run to refresh Codecov's baseline but never gate deploy. `main.yml`'s `deploy-kamal` job (issue #1036, split into two independent `kamal deploy` invocations by #1038) then deploys the just-pushed `api`/`web` images to the Hetzner VPS from #1029's self-hosting migration via `bundle exec kamal deploy -c config/deploy.web.yml --skip-push --version=<sha>` and `-c config/deploy.api.yml` (web first, then api — a required order, not just a preference: kamal-proxy demands TLS be established by the root path service (web, no `path_prefix`) before a path-prefixed service (api) can register for the same host, or the api deploy gets rejected outright or corrupts the host's TLS state for web's later registration — see issue #1132) — authenticates over SSH via an `ssh-agent` the job starts itself, loading a `KAMAL_SSH_KEY` repo secret through `printf '%s\n' | ssh-add -` (issue #1106: `webfactory/ssh-agent` fed the secret to `ssh-add -` verbatim, and a secret stored without its trailing newline made OpenSSH reject the PEM with `error in libcrypto`), seeds `known_hosts` with a plain `ssh-keyscan` — never `-H` (issue #1111: SSHKit substitutes its own `known_hosts` reader for net-ssh's, and that one returns the keys of only the *first* matching hashed line, so of keyscan's three lines only ssh-rsa counted as known and the server's ed25519 key raised `Net::SSH::HostKeyMismatch`) — and needs no config render step at all: `config/deploy.api.yml`/`config/deploy.web.yml` are committed as-is, and Kamal evaluates each as ERB before parsing (`Kamal::Configuration.load_config_file`), so their only two non-committable values (`KAMAL_SERVER_IP`/`KAMAL_REGISTRY_USERNAME`) come straight from the job's env. Both are repo Secrets, not Variables: the repo is public, GitHub masks only Secrets, and `KAMAL_SERVER_IP` is echoed into an `ssh-keyscan` command. (#1113 removed the old `infra/templates/deploy.yml.tftpl` + `envsubst` indirection.) See `infra/README.md`'s "Automate Kamal deploys in CI" section for the full secrets list, which is the single source of truth for every app secret. The `deploy-kamal` job runs under a GitHub `production` Environment (`environment: production`), branch-restricted to `main` with no required reviewers, and all its secrets live as environment secrets on `production` rather than repo-level — the branch restriction is what actually matters here, since only a `main` push can populate this job's `secrets.*` context; a PR run never can. Since Cutover (#1034) `deploy-kamal` is *the* production deploy — `tools.xdoubleu.com` resolves to the VPS and both configs' `proxy.host`/`proxy.ssl` let kamal-proxy terminate TLS with its own Let's Encrypt cert (shared for the one domain across both services) — so it has no `continue-on-error` and a failure there fails the workflow. Kamal's per-service rollback (`kamal rollback -c config/deploy.api.yml`/`-c config/deploy.web.yml`) reverts either service independently without touching the other.

**#1113 decommissioned DigitalOcean**: the `deploy` job, `do-app.yaml`, and the `DO_ACCESS_TOKEN`/`DO_APP_ID` secrets are gone, as is `infra/`'s duplicate local deploy path (`null_resource.kamal_deploy`, `data.external.deployable_image`, `deployable-image.sh`, and the 25 app-secret tfvars that fed it). **Tofu now provisions the host only** — firewall, hardening, deploy keys, Postgres (GoTrue was part of this too until issue #1039 replaced it with first-party auth in `api` and removed the container entirely) — and never deploys the app; app secrets live in repo Secrets and nowhere else. `DO_OAUTH_CLIENT_ID`/`SECRET` are unrelated and stay: they belong to the app's DigitalOcean monitoring *feature* (`api/internal/digitalocean`), not to hosting.

Because `main` deploys without re-testing, never push directly to `main` — only merge PRs whose CI passed. When editing any `.github/workflows/*.yml`, ensure its own `pull_request` trigger includes `.github/workflows/**` in its `paths` filter (docker-build workflows are the deliberate exception — push-to-main only).

**`ci-pass` fails with "Timed out waiting for Codecov to report" (issue #863's PR):** Codecov's GitHub-app check-suite for the commit can get stuck permanently in `queued` — confirmed via `gh api repos/<repo>/commits/<sha>/check-suites --jq '.check_suites[] | select(.app.slug=="codecov")'` — even though Codecov's own backend finished processing the coverage report (`https://api.codecov.io/api/v2/github/<owner>/repos/<repo>/commits/<sha>/` shows `"state":"complete"`). No `codecov/patch`/`codecov/project` check-run is ever created in that state, so `ci-pass` always times out (10 min) no matter how many times the workflow job itself is rerun. There's no API to rerequest another app's check-suite with a PAT; push a new commit (an empty one is fine) to get Codecov a fresh check-suite.

The **kobo-gateway** app (`kobo-gateway/`) ships inside the `web` Docker image as a downloadable `.dmg`, built separately on `macos-14` by `build-kobo-gateway.yml` and restored into `build-web.yml`'s build context from its `actions/cache` entry (see above). The `kobo_gateway` path filter in `main.yml` feeds `build-kobo-gateway` and `build-web`'s own gate — keep it in sync if `kobo-gateway/` moves.

`golangci-lint` runs across all three Go modules (`api`, `kobo-gateway`, `sentrytools`) off one shared root `.golangci.yml` — golangci-lint's config search walks up from the working directory, so per-module `working-directory` settings in `api-lint.yml`/`kobo-gateway-lint.yml`/`sentrytools-lint.yml` all resolve to the same file. `kobo-gateway` used to run bare `go vet`/`gofmt` only; `kobo-gateway-lint.yml` (macOS, same reasoning as `kobo-gateway-test.yml`) didn't exist at all until this parity fix.

`sentrytools/` (own `go.mod`, `module tools.xdoubleu.com/sentrytools`) is a tiny standalone Go module with no deployable artifact of its own — it holds the slog→Sentry `LogHandler` and `Init` that used to be duplicated byte-for-byte between `api`'s own copy and a second consumer (issue #926; that second consumer, the now-retired `gateway/` module, was removed in #1038). `api` pulls it in via a local `replace tools.xdoubleu.com/sentrytools => ../sentrytools` directive in its own `go.mod` — not a real published module, just a same-repo path reference — so a change to `sentrytools/` can break `api` without touching `api/`'s own subtree. The `changes` job's `sentrytools` path filter is OR'd into `api`'s own build/lint/test gate in `main.yml` for exactly that reason, on top of `sentrytools-lint.yml`/`sentrytools-test.yml` linting/testing it standalone. It intentionally takes an `env string` parameter and compares against plain literals (`"development"`/`"test"`) rather than importing `api`'s own `config` package — there's no reason to couple a tiny shared module to one particular consumer's config shape.

The local `replace` also changed `build-api.yml`'s Docker build context: it used to build with `context: ./api` (the Dockerfile's own directory), but `../sentrytools` falls outside that context entirely, so it now builds with `context: .` (repo root) plus an explicit `file: ./api/Dockerfile`. `api/Dockerfile` itself `COPY sentrytools ./sentrytools` before its own module's `COPY`, then `WORKDIR`s into `api/` to build — keep this in mind before ever changing that Dockerfile's context back to a subdirectory.

## Docs Impact

When a change touches project structure, packages, Make/npm targets, shared services, or architecture conventions, update this file and `README.md` in the same change.
