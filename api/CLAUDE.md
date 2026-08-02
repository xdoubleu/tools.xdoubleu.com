# api/ — Backend

Go 1.26 backend for tools.xdoubleu.com. Run all `make` commands from this directory.

## Common Commands

```bash
# Local DB — always start before running tests, stop after
docker-compose up -d        # Start local PostgreSQL 18
docker-compose down         # Stop after tests are done

# Build
make build                  # Build ./bin/api (main server binary)
# The kobo-gateway macOS menu-bar app is a separate Go module — see gateway/CLAUDE.md

# Testing (requires DB running — docker-compose up -d first)
make test                   # Run all tests
make test/v                 # Verbose output
make test/race              # With race detector
make test/cov/report        # Coverage report (HTML, excludes mocks)
make test/cov/per-pkg       # Per-package coverage with merged report

# Single test
go test ./apps/reading/... -run TestFunctionName

# Linting
make lint                   # Run all linters (Go + SQL)
make lint/fix               # Auto-fix issues
make lint/pkg PKG=apps/recipes  # Lint a single package

# Proto code generation (also run `npm run generate` from web/ — they're paired)
make proto/generate
```

## Docker Image

The api and web components are built and deployed together as one image from
the root `Dockerfile` (issue #558 — merged into a single DigitalOcean App
Platform component to avoid paying for two instances). The final stage is
`node:24-alpine`, **not** distroless — the image now has to carry the Node
runtime for the bundled Next.js standalone server regardless of what the Go
binary needs, so distroless no longer buys anything at the image-size level.
The Go binary itself is still `CGO_ENABLED=0` static with no Calibre — that
win now shows up as a **memory** win (no Qt/Python peak competing with the
Node child for RAM inside the shared 512 MB instance) rather than an
image-size one. See root `CLAUDE.md`'s "Deploy shape" note,
`api/cmd/api/web_process.go` (spawns `node server.js` as a supervised child)
and `api/cmd/api/frontend_proxy.go` (the reverse proxy front door). All
reading conversion paths are pure Go:

- Article/RSS HTML→EPUB: `conversion_epubbuild.go`.
- PDF→EPUB: `conversion_pdfextract.go` and friends, built on
  [`go-pdfium`](https://github.com/klippa-app/go-pdfium)'s WebAssembly backend
  (PDFium compiled to wasm, embedded via `go:embed`, run under
  [wazero](https://wazero.io/) — no CGO, no external runtime files). PDF text is
  reconstructed into reading-order HTML (line grouping, column-gutter detection,
  paragraph/heading/hyphenation heuristics — see `conversion_pdfextract_*.go`)
  and figures are extracted per-page, then both go through the same
  `goHTMLConverter` EPUB assembly as articles.

Wazero doesn't return memory to the OS until an instance is closed, so a
package-level `pdfSem` (mirroring the old `calibreSem`) limits PDF extraction to
one conversion at a time, and each conversion borrows a fresh pool instance and
closes it when done rather than holding one long-lived.

`make pdf/check` converts every PDF dropped into
`apps/reading/internal/services/testdata/pdf.local/` (gitignored — never commit
a PDF there) to EPUB in a scratch subdirectory, for manually inspecting
real-world conversion quality (reading order, figure placement, headings)
against actual documents that synthetic test fixtures can't reproduce.

Before the merge, this made the standalone api image ~20 MB (distroless)
instead of the ~700 MB the Calibre-based `debian:13-slim` stage used to be.
Removing Calibre is what made the merged image's memory footprint small
enough to consider fitting both api and web in the same 512 MB instance in
the first place — see the "Docker Image" section above.

## R2 Bucket CORS

The in-browser EPUB/KEPUB preview reads bytes client-side via epub.js (XHR), so the R2
bucket must allow cross-origin GET requests from the web frontend's origin. Apply this CORS
policy to **each** bucket (adjust `AllowedOrigins` per environment):

```json
[{
  "AllowedOrigins": ["http://localhost:3000"],
  "AllowedMethods": ["GET", "HEAD"],
  "AllowedHeaders": ["*"],
  "ExposeHeaders": ["Content-Length", "Content-Range", "Accept-Ranges", "ETag"],
  "MaxAgeSeconds": 3600
}]
```

Set it via the Cloudflare R2 dashboard (bucket → Settings → CORS Policy) or `aws s3api
put-bucket-cors` against the R2 endpoint. Production origin: `https://tools.xdoubleu.com`.
PDF preview (iframe navigation) is unaffected by this rule. Recreating a bucket requires
re-applying the rule — it is not stored in this repo.

## Architecture

A Go monorepo that serves multiple web apps from a single binary. All apps are registered in `cmd/api/apps.go` and share a single HTTP mux routed by URL prefix; `main.go` wraps the shared pgx pool in `postgres.NewSpanDB` once so every app's queries emit tracing spans (migrations use the raw pool). Registration order matters for migrations: `reading` registers before `games` because games' final migration drops the leftover `backlog` schema after both apps have adopted their tables. Apps expose ConnectRPC endpoints consumed by the Next.js frontend in `web/`.

### App Structure

Each app lives in `apps/<name>/` and follows a consistent layout:

```
apps/<name>/
├── app.go              # App struct embedding app.Base, implements App interface
│                       # Apps whose integration tests seed data through the
│                       # service layer (games, reading, watchparty) export a
│                       # Services field; the rest keep services private
├── routes.go           # HTTP route registration
├── handlers.go         # HTTP handlers (shared middleware/error helpers)
│                       # Large apps split handler code across focused files,
│                       # e.g. tasks_crud.go, tasks_list.go, tasks_subtasks.go
├── connect*.go         # ConnectRPC service implementations
├── internal/
│   ├── dtos/           # Request/response serialization
│   ├── models/         # Domain models
│   ├── repositories/   # DB access layer (pgx/v5)
│   ├── services/       # Business logic
│   ├── jobs/           # Background jobs (if any)
│   ├── helper/         # App-specific utilities
│   └── mocks/          # Mock implementations for testing
├── pkg/                # Reusable packages (external client integrations)
└── migrations/         # Goose SQL migrations (per-app schema)
```

### ConnectRPC Auth Handler Conventions

The `GetCurrentUser` handler (in `cmd/api/connect_auth_handlers.go`) uses a two-layer role resolution pattern:

1. Call `h.app.auth.GetUser(ctx, token)` to validate the session and get the GoTrue user (including its `Role` field).
2. Call `h.app.appUsersRepo.GetByID(ctx, user.ID)` to retrieve the DB-enriched user record. If found, prefer the DB role over the GoTrue role. If not found, fall back to the GoTrue role.

Any Connect handler that needs DB-enriched user attributes must follow this same fallback pattern rather than relying solely on the GoTrue response.

### Auth (`internal/auth`)

The `Service` interface and its `GoTrueService` implementation (Supabase, via `supabase-community/auth-go`) live together in `internal/auth`. Conventions:

- Every auth method doing I/O takes a `context.Context` first. auth-go v1.5.0 has no context support, so propagation stops at the GoTrue boundary; the DB enrichment queries and the cache do consume it.
- The middleware (`Access`/`TemplateAccess`/…) resolves users through a **per-token TTL cache** (`AUTH_CACHE_TTL` seconds, default 60, `0` disables — tests use 0 via `testhelper.NewTestConfig`). A cache hit skips the GoTrue round-trip and both enrichment queries, so role/app-access changes and the `last_seen` upsert can lag by up to the TTL.
- `enrichUser` overlays the DB-managed `Role`/`AppAccess` onto the raw GoTrue user (which always resolves to `RoleUser` with no app access on its own, per `models.UserFromTypesUser`). A DB failure there is returned, not swallowed (issue #673): silently falling back to the unenriched user would be indistinguishable from "this user has no access" to `AdminAccess`/`AppAccess`, and `resolveUser`/`refreshTokens` would cache that wrong identity for the rest of the TTL instead of retrying on the next request — worst right after a deploy, when a cold cache forces every session through enrichment at once.
- Tokens are evicted on SignOut, UpdatePassword, VerifyMFA, and UnenrollTOTP. Anything that mutates roles or app access for *other* sessions (admin `SetRole`/`SetAppAccess`) must call `InvalidateUserCache()` (clear-all) afterwards.
- `SignInRenderer` is injected post-construction from `cmd/api` (the templ sign-in page lives there).

### Shared Internal Packages (`internal/`)

- **`app.Base`** — Embedded struct providing logger, config, and auth service to every app
- **`app.HTTPError`** — Shared HTTP error type (`Status int`, `Message string`); import as `iapp "tools.xdoubleu.com/internal/app"` in handler files to avoid collision with the app struct
- **`app.ScrubInternalErrors(logger)`** — Connect handler option that logs CodeInternal/CodeUnknown errors and replaces the client-facing message with a generic one; every `New*ServiceHandler` call must pass it
- **`auth/`** — Auth interface + `GoTrueService` implementation, middleware, and per-token user cache (see "Auth" above)
- **`config/`** — Centralized config loaded from `.env` via `xdoubleu/essentia/v4`
- **`constants/`** — Shared constants
- **`contacts/`** — Contact management service with editable display names (used by recipes, shopping list, and meal-plan sharing). `AddByEmail` emails the recipient via `mailer.Client.SendTo` after the request is persisted (issue #383); a send failure never fails the request itself — only a non-`ErrNotConfigured` error is logged.
- **`crypto/`** — AES-256-GCM secret-at-rest encryption (`Sealer`), used to store OAuth tokens (see `oauthconn/` below)
- **`mailer/`** — `mailer.Client` sends email via the [Resend](https://resend.com) HTTP API (`net/http` + `encoding/json`, no SDK). `Send(ctx, subject, body)` uses the fixed recipient passed to `New(apiKey, from, to)`; `SendTo(ctx, to, subject, body)` sends to an arbitrary recipient instead, sharing the same `apiKey`/`from` and `ErrNotConfigured` degrade-gracefully semantics (returned when `apiKey`/`from`/the resolved `to` is empty), mirroring the `sentryapi`/`digitalocean` convention. `Send` is used by `observability/jobs.IssueNotifierJob` (issue #561) to alert a fixed admin address; `SendTo` is used by `contacts.Service.AddByEmail` (issue #383) to email a contact request's recipient, a link back to `WebURL + "/contacts"`. Plaintext only — no `html/template` infra exists in `api/`.
- **`github/`, `sentryapi/`, `digitalocean/`** — Read-only external observability clients (failing pull requests, unresolved Sentry issues, latest DigitalOcean deployment and its build/deploy/runtime logs). `github.ListFailingPullRequests` lists open PRs then fetches each head commit's check runs, keeping only PRs with a completed, non-passing (`failure`/`timed_out`/`cancelled`/`action_required`) check. `digitalocean.Client.DeploymentLogs(ctx, deploymentID, tailLines)` (issue #549) reads a deployment's service component names off its top-level `services` list (not `spec.services` — DO omits the spec on the deployment-detail endpoint), then fetches BUILD, DEPLOY, RUN, and RUN_RESTARTED log text per component from DO's `/logs` endpoint. `DeploymentLogsStream` (issue #672, second pass) is the same fetch, incremental: instead of collecting every component/type pair before returning, it calls a `yield func(ComponentLog) error` as each one resolves — `collectLogsStream` runs the same `collectLogsConcurrency`-bounded concurrent fetch as `collectLogs`, serializing `yield` calls with a mutex since its usual destination (a Connect server stream) isn't safe for concurrent `Send`s. Used by the `GetDeployLogs` Connect handler; `DeploymentLogs` (and the MCP tool built on it) stay unary. That endpoint returns **two** sources and both are read (issue #632): `historic_urls` are pre-signed plain-GET URLs holding *archived* text, and `live_url` is a websocket carrying a still-running component's tail — for a component that has been running since its deploy nothing is archived yet, so `live_url` is the only place its actual runtime output exists. The live read (`logs_live.go`, `coder/websocket`, scheme rewritten http(s)→ws(s) exactly as `doctl` does, token already in the query string) is bounded by `liveLogDeadline` and **never fails the call** — a dial/read error, the deadline, or the cap all return what was collected with `Truncated` set, so a flaky socket can't cost the caller the build/deploy logs. `tail_lines` must be sent explicitly (`0` → 1000, ceiling 10000) or DO replays no backlog at all and a quiet component returns nothing; `follow=false` is what makes DO close the socket. Each component/type's combined content is capped at 200 KB. Runtime logs are sourced from the deployment that is actually **serving traffic**: when the requested deployment isn't the app's `active_deployment` (`GET /v2/apps/{id}`) — the failed-deploy-on-top-of-a-running-one case — RUN/RUN_RESTARTED are additionally fetched from the app-scoped, deployment-less `/v2/apps/{id}/logs` path and tagged with the active deployment's ID in `ComponentLog.DeploymentID`; that extra lookup degrades to nothing on error. A component/type with no text from either source (e.g. no DEPLOY logs for a build-only failure) is omitted, not an error. Style B (mirrors `apps/reading/pkg/hardcover`): exported `Client` interface + `New(...)`, injected `*http.Client` with timeout, `doWithRetry` backoff, `SetBaseURL`/`SetBackoffBase` test seams, a ~45s in-memory cache, and an `ErrNotConfigured` sentinel returned when the provider isn't connected or its identifier(s) aren't picked yet. Consumed by the observability handlers, which degrade each source independently. Each client resolves its admin-picked identifier(s) — GitHub `repo`, Sentry `org`/`projects`, DigitalOcean `app_id` — fresh on every call from `global.oauth_connections.config` (via a narrow `configStore` interface over `repositories.OAuthConnectionsRepository.Get`), not a static value baked in at boot; Sentry loops its fetch across every configured project, tags each `Issue.Project`, and merges/sorts the results by `LastSeen` before caching. Each package also exposes a discovery method for the admin picker — `github.ListRepos`, `sentryapi.ListOrgs`/`ListProjects`, `digitalocean.ListApps` — which returns `oauthconn.ErrNotConnected` (not `ErrNotConfigured`) when no token is set, since discovery must work *before* any identifier is picked.
- **`oauthconn/`** — Shared plumbing for the admin-configurable OAuth connections to GitHub/Sentry/DigitalOcean (issue #440): `TokenFunc`/`NewTokenFunc` resolve a live bearer token from `repositories.OAuthConnectionsRepository`, transparently refreshing via `golang.org/x/oauth2.Config.TokenSource` and persisting rotated tokens; `StateStore` is an in-memory, single-use CSRF-state map for the browser redirect leg. Each provider package exposes its own `OAuthConfig(clientID, clientSecret, apiURL)` (`github/oauth.go`, `sentryapi/oauth.go`, `digitalocean/oauth.go`). The browser-facing `GET /admin/oauth/{provider}/start` and `/callback` routes (`cmd/api/oauth_admin.go`, admin-cookie-gated via `auth.AdminAccess`) drive the connect flow; `observability.v1.ObservabilityService.ListOAuthConnections`/`DisconnectOAuthConnection` (`cmd/api/connect_observability_oauth.go`) back the admin UI panel on `/monitoring`. Two more RPCs in the same file back the identifier picker: `GetProviderOptions` (admin-gated, dispatches to the matching client's discovery method(s) — Sentry disambiguates by an empty vs. set `sentry_org` request field) and `SetProviderConfig` (admin-gated, marshals the request's `ProviderConfig` oneof to JSON and calls `OAuthConnectionsRepository.SetConfig`). Tokens are stored encrypted (`internal/crypto`, AES-256-GCM, key from `ENCRYPTION_KEY`) in `global.oauth_connections`; the nullable `config JSONB` column holds the picked identifier(s) as opaque JSON — `NULL` means "connected but not yet configured", a distinct degraded state from "not connected" that each provider client's fetch path also checks. There is no static-token or static-identifier fallback — an admin must connect *and* configure each provider via `/monitoring` for its dashboard section to populate. If a provider's client id/secret env vars are unset, `newObservabilityClients` (`cmd/api/main.go`) logs a startup warning; see root [`README.md`](../README.md)'s "Deploy Notes" for the OAuth App registration and secret-provisioning steps.
- **`models/`** — Shared domain models
- **`repositories/`** — Shared DB repositories over the `global` schema (users, contacts, the observability tables: `JobRunsRepository`, `UsageRepository`, `StorageSnapshotsRepository`, `DBStatsRepository`, `NotifiedIssuesRepository` (dedup keys for `IssueNotifierJob`, issue #561), and `ProfileSharesRepository` for the public-profile share tokens)
- **`observability/`** — Cross-cutting instrumentation. `TrackedJob` decorates any `threading.Job` so every run is timed and recorded in `global.job_runs`, panics are recovered, and failures log at Error level (so they reach Sentry); wrap jobs at registration (see `apps/{todos,games,reading}/app.go`). `UsageRecorder` counts requests per `(day, app, endpoint)` in memory and flushes to `global.usage_daily`; the counting `usageMiddleware` sits in the `cmd/api` alice chain after `domainMiddleware`. `observability/jobs/IssueNotifierJob` (issue #561) is a cross-app job — registered directly in `cmd/api/main.go`'s own `threading.JobQueue` rather than an app's, since it isn't scoped to one app — that polls the existing `sentryapi`/`digitalocean` clients every 5 minutes and emails an admin (via `mailer.Client`) the first time a Sentry issue or a DigitalOcean deployment with `Phase == "ERROR"` is seen; dedup keys persist in `global.notified_issues` (`repositories.NotifiedIssuesRepository`) so restarts don't re-send. Either provider's `ErrNotConfigured` is skipped silently rather than failing the run.
- **`progressws/`** — WebSocket service broadcasting background-job progress (start/stop state, live "X of N" counts) keyed by job-ID topics
- **`progresshistory/`** — Generic cumulative-progress storage with carry-forward reads (used by games and reading progress graphs)
- **`mocks/`** — Shared mock implementations
- **`testhelper/`** — Test utilities: `ConnectTestDB(dsn)` wraps `postgres.Connect` for integration tests; `BuildMux(Routable)` constructs a test `http.Handler` from any app that implements `Routes`/`GetName`

### Key Libraries

| Concern | Library |
| --- | --- |
| HTTP | `net/http` + `justinas/alice` (middleware chaining) |
| RPC | `connectrpc.com/connect` — HTTP/1.1 RPC framework |
| Database | `jackc/pgx/v5` + `pressly/goose/v3` (migrations) |
| Auth | `supabase-community/auth-go` |
| WebSocket | `coder/websocket` |
| Error tracking | `getsentry/sentry-go` |
| Job queue | `xdoubleu/essentia/v4` threading.JobQueue |
| Code generation | `buf` / `protoc-gen-go` / `protoc-gen-connect-go` |
| Testing | `stretchr/testify` |

### Apps

- **games** — Steam backlog tracker: library sync, achievements, completion rate progress/distribution, user-set favourites, and the user's Steam integration settings. External client package lives in `pkg/steam/`. Has a background sync job (1 worker) and WebSocket live updates. The `steam_games.favourite` flag is user-set state: `UpsertGames` deliberately never writes it, so it survives every sync. Uses `games` DB schema (adopted from the former `backlog` schema).
- **reading** (formerly **books** — Go package `apps/reading/`, URL prefix `/reading`, schema `reading`, proto package `reading.v1`; entity types like `Book`/`BookService` keep their names) — Reading library and e-reader companion for books, arXiv papers, web articles, and RSS posts. Every catalog row has a fixed `category` (`book`/`paper`/`article`/`rss`) and non-book items carry a canonical `source_url` (dedup key, partial unique index). Ingestion paths: `LibraryService.AddBookByURL` routes arXiv URLs (`pkg/arxiv/`, Atom API) to paper ingestion — metadata from the API, then content preferentially from arXiv's LaTeXML HTML rendition (`arxiv.Client.GetHTML`, readability-extracted and built into an EPUB via the same pure-Go path as articles), falling back to a downloaded PDF when no HTML rendition exists or the HTML path fails for any reason (`IngestService.addPaperFromHTML`/`addPaperFromPDF` in `internal/services/ingest.go`; `ensurePaperPDF` stays the PDF-only repair path for a paper whose file went missing) — and everything else to readability extraction (`go-shiori/go-readability`) + article-EPUB building via a pure-Go EPUB assembler (`conversion_epubbuild.go`/`conversion_epubbuild_xhtml.go` — no Calibre subprocess; article images are downloaded and localized first, `ingest_images.go`); `FeedService` manages RSS/Atom subscriptions (`reading.feeds` + per-feed seen-set `reading.feed_items`, parsed with `mmcdole/gofeed`), imported on subscribe and polled hourly by the `poll-feeds` job with conditional GETs; RSS-ingested items never sync to Kobo devices (issue #640) — `ListKoboSyncBooks`/`GetKoboSyncBook` (`internal/repositories/books.go`) filter out `category='rss'` regardless of tags, and `BookService.EnableKoboSync` rejects RSS books outright (`ErrKoboSyncNotAllowedForRSS`, mapped to `CodeFailedPrecondition`), so no EPUB is ever eagerly built for a feed item; feed items whose link/GUID is an arXiv id are ingested as `paper`s (HTML-preferred, PDF fallback) via `IngestService.IngestArxivByID`, not `rss` articles. A `Feed` also carries `source_type` (`rss`/`email`, migration `00012_email_feeds.sql`): email-relay feeds (issue #595, for paywalled/feedless newsletters like Substack) are minted via `FeedService.CreateEmail`, which generates a token (SHA-256 hashed before storage as `inbound_token`, mirroring the Kobo device-token pattern), inserts a feed row with `url` left `NULL`, and returns the plaintext `<token>@<EMAIL_INBOUND_DOMAIN>` address once — it's never re-displayable (issue #667: no `+` required, since some newsletter signup forms reject one; the older `reading+<token>@…` form still resolves for aliases issued before the change). Inbound mail hits the raw `POST /reading/email/inbound` route (`email_routes.go`, same `AppAccess`-bypassing pattern as `kobo_routes.go`), verifies the Resend/Svix webhook signature against `EMAIL_INBOUND_SECRET`, resolves the feed by hashing the token out of the `to` address (`inboundTokenFromAddress`, tolerant of a legacy `+`), fetches the email body from Resend's receiving API (the `email.received` webhook payload carries metadata only, not HTML), and calls `FeedService.IngestEmail` to run it through the same `IngestArticleContent` tail as RSS items, tagged `category='rss'` (no separate category). `FeedService.Refresh` is a no-op for `source_type='email'` feeds and `ListAll` excludes them from the hourly `poll-feeds` job — they're push-only. Deleting a feed (`FeedService.Delete`) also removes the library items it ingested **except** any the user read or favourited (`FeedsRepository.ListRemovableBookIDs` → `BookService.RemoveFromLibrary`). RSS items are treated as an auto-pulled firehose distinct from deliberately-added reading: `buildLibraryData` keeps `category='rss'` items out of the reading-state shelves (returning them in `LibraryResponse.rss`) and the read-progress graph (`GetFinishedDates`) excludes them, so books/papers/articles count separately from RSS. All external web content goes through the size-capped `pkg/webfetch/` client. Book metadata enrichment queries two independent providers, concurrently per book (`fetchByISBN`/`searchProviders` in `book_resync.go`, via `errgroup`): UniCat (Belgian SRU/MARC catalog, no key) and Hardcover (GraphQL; set `HARDCOVER_API_KEY` — a free Bearer JWT from the account settings page that expires ~yearly and must be refreshed; left disabled/nil when unset. No daily quota, its 1 req/s limiter is the resync throughput floor). ISBN-less books are matched by title+author; resync/duplicate scans skip non-book categories. External client packages live in `pkg/unicat/`, `pkg/hardcover/`, `pkg/arxiv/`, `pkg/webfetch/`; the resync orchestration and per-source scan-status cache (`*_found` columns) live in `internal/services/book_resync.go`. Serves the raw Kobo sync protocol under `/reading/kobo/{token}/…` and a public cover proxy (`routes.go`); devices set up under the pre-rename `/books/kobo/…` prefix must re-run the gateway setup flow. Per-device debug logging (endpoint + request/response bodies) can be toggled from the Reading settings page; captured requests live in an in-memory `KoboLogStore` (`apps/reading/internal/services/kobo_log.go`), not the DB, and reset on restart. Has background jobs (2 workers) and WebSocket live updates, including a daily R2 bucket scan (`books-storage-scan`) that writes a `global.storage_snapshots` row for the admin dashboard. The object-store `Client` (`pkg/objectstore/`) exposes a paginated `List` used by that scan. Uses the `reading` DB schema — renamed in place from `books` by a pre-migration bootstrap in Go (`renameLegacyBooksSchema` in `app.go`: goose's version table lives inside the schema, so `ALTER SCHEMA … RENAME` carries the migration history along; historical migration files were rewritten to `reading.` for fresh installs, and R2 storage keys keep their `books/` prefix). The `books`→`reading` app identifier is also rewritten in `global` data by `cmd/api/migrations/00008`.
- **watchparty** — WebRTC screen sharing with draggable camera overlays. No DB, no background jobs.
- **icsproxy** — ICS calendar feed filtering and proxying. Uses `icsproxy` DB schema.
- **recipes** — Recipe management with fraction parsing, iCal export, shopping lists, and whole-recipe-book sharing with contacts (`recipebook_access`, view-only or edit). Uses `recipes` DB schema.
- **shoppinglist** — Custom items plus meal-plan ingredient aggregation, with user-defined categories, a name→category catalog, and per-store category ordering that drives a store-ordered (Apple Notes) export. The whole list is shareable with contacts (`shoppinglist_access`, view-only or edit); most data RPCs accept an `owner_user_id` so a recipient can act on a shared owner's list. **Stores are the exception — they are private to each user:** the store RPCs take no `owner_user_id` and always act on the caller's own stores, so a share recipient orders an export by their own stores and never gains access to the owner's. Uses `shoppinglist` DB schema.
- **mealplans** — Weekly meal planning with per-plan iCal feeds and plan sharing with contacts. Uses `mealplans` DB schema (its `plans` tables were adopted from the `recipes` schema — the same `ALTER TABLE … SET SCHEMA` pattern later used for the games/books split).
- **todos** — Task management with sections, workspaces, subtasks, policies, archive, search, and background archive jobs. Uses `todos` DB schema.

### Database Conventions

- Each app uses its own PostgreSQL schema (e.g., `reading`, `icsproxy`)
- Cross-cutting tables live in the `global` schema with migrations in `cmd/api/migrations/` (users, contacts, `profile_shares`, observability: `job_runs`, `usage_daily`, `storage_snapshots`, `notified_issues` (dedup keys for the issue-notifier job, issue #561), and `oauth_connections` — one encrypted-token row per external provider, see `oauthconn/` above). The `observability.v1.ObservabilityService` DB-backed RPCs (`GetJobStats`/`GetUsageStats`/`GetStorageStats`/`GetDatabaseStats` in `cmd/api/connect_observability.go`) read these plus live `pg_*` size queries. The same service also exposes four external-signal RPCs (`GetFailingPullRequests`/`GetSentryIssues`/`GetDeployStatus`/`GetDeployLogs`) plus a `GetHealthOverview` rollup (Sentry + deploy only) in `cmd/api/connect_observability_external.go`, backed by the `internal/{github,sentryapi,digitalocean}` clients; each source degrades to an empty `configured=false` section when its OAuth connection is missing or the upstream call fails, so one broken source never fails the response. `GetDeployLogs` (issue #549) fetches BUILD/DEPLOY/RUN/RUN_RESTARTED log text per service component for one deployment (an empty `deployment_id` resolves to the latest, via `LatestDeployment`) — a component/type pair with no logs yet is omitted, not an error. Its `tail_lines` request field (0 = the client default) bounds the live backlog replayed per component, and each `DeployComponentLog` carries the `deployment_id` it came from, since runtime blocks are sourced from the active deployment (issue #632). Unlike every other RPC here, its **Connect wire type is server-streaming** (issue #672, second pass): the handler calls `digitalocean.Client.DeploymentLogsStream`, which yields each component/type pair to the stream as soon as it resolves instead of assembling one response after every component finishes — the first byte then reaches the client well under DigitalOcean App Platform's ~25s edge request timeout regardless of how long the slowest component takes (see `deployLogsCtxTimeout`'s comment in `cmd/api/routes.go`). `deployLogs`/`digitalocean.Client.DeploymentLogs` — the older unary, fully-collected form — still exists for the `get_deploy_logs` MCP tool, which needs one JSON result rather than a stream; the two share `collectLogsStream`'s fetch logic underneath (`internal/digitalocean/logs.go`). The Connect handlers and their query bodies are split: each handler is a thin `requireAdmin` + connect wrapper over an internal method (`jobStats`/`usageStats`/`storageStats`/`databaseStats`, `failingPullRequests`/`sentryIssues`/`deployStatus`/`deployLogs`, and `deployLogsStream` for the RPC itself) so the MCP tools reuse the exact same read logic.
- Every app's read RPCs, plus the admin observability read methods above, are exposed to a local Claude CLI over a single **read-only MCP server** at `/apps/mcp` (`cmd/api/mcp_apps.go`, streamable-HTTP, `github.com/modelcontextprotocol/go-sdk`). Apps that implement `MCPToolProvider` (interface in `cmd/api/apps.go`) register `<app>_<rpc>` tools from an `apps/<app>/mcp.go` file that wraps their own Connect **read** handlers — only read RPCs are wrapped, so nothing mutating is reachable (e.g. mealplans `ListPlans` is served via `services.Plans.List` to skip its default-plan creation). `registerObservabilityMCPTools` (same file) registers the 8 unprefixed observability tools (`get_job_stats`/`get_usage_stats`/`get_storage_stats`/`get_database_stats`/`get_failing_pull_requests`/`get_sentry_issues`/`get_deploy_status`/`get_deploy_logs`) wrapping the internal methods above; each calls `requireAdmin` instead of the per-app gate. The shared `internal/mcptools` package provides `RequireAppAccess` (mirrors `auth.AppAccess`: admin **or** the app in the user's app-access — the gate for app tools, not `requireAdmin`), the generic `AddReadTool`, and `Unwrap`/`Result`. Auth is **MCP OAuth 2.1**: the api is the resource server (`auth.RequireBearerToken` verifies a Supabase access token via `GoTrueService.ResolveToken`; `auth.ProtectedResourceMetadataHandler` serves `/.well-known/oauth-protected-resource…` pointing at Supabase as the authorization server), and the `web` `/oauth/consent` page drives the Supabase approval — this Bearer plumbing (`mcpBearerRoute`/`mcpResourceMetadataFor`) lives in `cmd/api/mcp.go` and is shared by every tool. Register locally with `claude mcp add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp` (see root `README.md` for the one-time Supabase OAuth-server setup).
- Migrations live in `apps/<name>/migrations/` and follow Goose SQL format
- `updated_at` columns are managed via PostgreSQL triggers
- CI runs tests against a real PostgreSQL 18 instance — no DB mocking

### Cross-Schema Reads

Apps share one binary and one database, so downstream apps may **read** an
upstream app's schema directly in SQL instead of going through an internal API.
The allowed dependency direction is acyclic:

```
recipes ← mealplans ← shoppinglist
```

- `mealplans` joins `recipes.recipes` (meals reference recipes); its proto
  embeds `recipes.v1.Recipe`.
- `shoppinglist` is by design a read-side aggregator: its export and item-name
  catalog features join `mealplans.plan_meals`/`plans`/`plan_access` and
  `recipes.recipes`/`ingredients`.

Rules: reads only (never write another app's schema), never add a dependency
in the reverse direction, and each app's migrations touch only its own schema.
Upstream schema changes (recipes, mealplans) must grep downstream repositories
for affected columns.

### Public Profile Sharing

Reading and games expose read-only shareable-profile RPCs
(`reading.v1.PublicLibraryService`, `games.v1.PublicGamesService`, in each app's
`connect_public.go`). These are registered in `routes.go` **without** any
auth middleware: every request carries an opaque share token that resolves to
the owning user via the shared `ProfileSharesRepository`
(`global.profile_shares`, plaintext token, keyed by `(user_id, app)` — read-only
data, so the owner can copy the link anytime; unknown tokens, and tokens
resolved against the wrong app, return `CodeNotFound`). Reading and games each
have their own independent share link — disabling one never touches the
other. The owner manages both tokens through `profile.v1.ProfileService`,
handled in `cmd/api/connect_profile.go` behind `Access`; every RPC takes a
`ProfileApp` argument, and regenerating replaces that app's row, instantly
invalidating its old link. Public handlers must never read
`constants.UserContextKey` — no auth middleware runs, so it is never set.

`global.app_users` carries a nullable `display_name` column (the user's public
profile name, set via `ProfileService.SetDisplayName`). `CreateProfileShare`
requires it to be non-empty first (`CodeFailedPrecondition` otherwise) — a
share link is worthless without a name to attribute it to. The public RPCs
resolve it alongside the owning user ID (`ProfileSharesRepository.ResolveToken`,
a `LEFT JOIN` against `app_users`) and return it on `GetSharedLibraryResponse`/
`GetSharedSteamResponse` for the frontend to display.

## Linting

Strict linting is enforced via `golangci-lint` (40+ linters). Key constraints:

- Max line length: 88 characters (`golines`)
- Import order: standard → default → custom (`gci`)
- Max function length: 100 lines / 50 statements
- Max cyclomatic complexity: 30

Always run `make lint/fix` as the final step before committing. Manually fix anything the auto-fixer cannot resolve.

## Testing Notes

- Use mock injection for unit tests; place mocks in `internal/mocks/` or app-level `internal/<name>/mocks/`
- Integration tests hit a real database — start `docker-compose up -d` from the repo root before running tests locally
- Target ≥80% coverage on changed code; check with `make test/cov/report`
- Generated files (`gen/`, `_mock.go`) are excluded from coverage
- When fixing bugs, write a failing test first before implementing the fix

## File Size & Splits

Go files projected over ~300 lines need a split plan before adding more code:

- `*_test.go` — split by feature or handler group (e.g. `tasks_crud_test.go`, `tasks_search_test.go`)
- `.go` source — split by concern; extract large JS/TS string constants to a companion `.go` file
- `.templ` — split by UI concern (e.g. `views_list.templ`, `views_form.templ`)
