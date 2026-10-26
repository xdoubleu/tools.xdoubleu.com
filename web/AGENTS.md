# web/ — Frontend

Vite 8 SPA: React 19, React Router (data router), TypeScript strict, Vitest. The api serves the static build → [`adr-0034`](../docs/adr-0034-vite-spa-served-by-the-api.md). Run `npm` commands from this directory; shared commands are in the root `AGENTS.md`.

## Layout

- `src/` — entry (`main.tsx`), `routes.tsx`, root layout, route error/not-found states, Sentry/PostHog init, Web Vitals.
- `app/` — one route folder per app/domain; each `page.tsx` is a route (`[id]` → `:id`), loaded lazily by `import.meta.glob`. `dashboard/{games,reading}/` holds both owner and public (token-shared) dashboards → [`adr-0007`](../docs/adr-0007-dashboard-app-owns-public-sharing.md).
- `components/` — shared components at the root, one subfolder per domain, primitives in `components/ui/`.
- `lib/` — `client.ts` (browser transport), `swrKeys.ts`, `env.ts`, `cn.ts`, per-domain subfolders; `gen/` is generated (read the `.proto` instead).
- `hooks/` — SWR hooks per domain, client-only WebSocket hooks, and `navigation.ts`.
- `public/` — static files; `scripts/generate-icons.mjs` renders the gitignored PNG icons (dev/build run it).

## Data flow (SWR)

Pages render client components that fetch through SWR hooks over `lib/client.ts` (binary, `credentials: 'include'`). Nothing prefetches on a server, so a first visit shows loading states until the API answers.

- **Every SWR key goes through `lib/swrKeys.ts`**, for both hooks and `mutate()`; an inline literal splits the cache from its invalidator.
- **Routing goes through `@/hooks/navigation` and `components/ui/link`** (`Link` takes `href`; its pending state drives `CardLinkStatus`). Don't import `react-router` hooks in components.
- Route params come from `useRouteParams<{ id: string }>()`; a page never receives props.
- `getApiUrl()` (`lib/env.ts`) reads `window.__ENV__`, which the api serves as `/env.js` (Vite's dev server serves the same from the shell env or `.env`). One build serves every environment.

## Offline → [`adr-0027`](../docs/adr-0027-offline-service-worker-and-persisted-swr.md), [`adr-0028`](../docs/adr-0028-offline-write-outbox.md)

- `lib/offline/serviceWorker.ts` is built into `dist/sw.js` with the precache list; it must stay self-contained. The api fills in `__OFFLINE_ENABLED__`.
- Add live-only or admin SWR keys to `NOT_PERSISTED` (`lib/offline/persist.ts`).
- Offline writes: `enqueueWrite` with a `defineOfflineWrite` registered in `lib/offline/outbox.ts`.

## Commands

```bash
npm ci                                  # first, in a fresh worktree
npm run dev                             # Vite on :3000; set API_URL (e.g. http://localhost:8000) in .env.local
npm run test:cov:diff                   # coverage on lines changed vs origin/main — run before pushing (matches codecov/patch)
npm run mobile:audit -- /trains /books  # 375x667 Playwright audit; needs `npm run dev` + `npx playwright install chromium`
npm run generate:ui-catalog             # regenerate components/ui/README.md (lint fails if stale)
rm -f tsconfig.tsbuildinfo              # before `npx tsc --noEmit` after a lockfile change
```

## UI rules → [`convention-ui-standards`](../docs/convention-ui-standards.md)

- **ESLint rejects raw controls and hand-rolled forms of existing primitives** (`<h1>`, cards, banners…) via `ui/*` rules in `eslint-rules/`. Check [`components/ui/README.md`](components/ui/README.md) first; add a primitive, not a raw element. A new class-pattern check needs a RuleTester case.
- Merge classes with `cn()`; navigating cards use `LinkCard`, with controls in its `actions` footer.
- Pending buttons use a present participle with the typographic `…`, never `...`.
- Tailwind v4 CSS-first (no JS config); dark tokens key off `:root[data-theme='dark']`, owned by `lib/theme.ts` and `public/theme-init.js`.

## kobo-gateway client (`lib/books/gatewayClient.ts`)

Talks to the local helper at `https://127.0.0.1:41132` (see `kobo-gateway/AGENTS.md`; the CSP's `connect-src` in `api/internal/spa/csp.go` allows it). The browser makes all authenticated API calls and hands the gateway only a sync URL.

- Bump `REQUIRED_GATEWAY_VERSION` (with Go's `GatewayVersion`) **only for protocol breaks**; routine updates flow through `gatewayNeedsUpdate` comparing against `getKoboGatewayRelease()` → [`adr-0004`](../docs/adr-0004-runtime-release-env-vs-compile-stamp.md).
- `public/downloads/` is assembled in CI (`build-api.yml` stages the kobo-gateway artifacts), **so the download 404s under `npm run dev`** → [`adr-0002`](../docs/adr-0002-kobo-gateway-ci-cache-split.md).

## OAuth consent (`app/oauth/consent/`)

Drives the api's embedded fosite AS from the browser (`lib/oauth2as/consentClient.ts`); the pending request's query params are echoed verbatim both ways — don't normalize or default them here.

## Files and tests

- Split TS/TSX files over ~300 lines before adding code (components by UI concern, hooks by data domain).
- Vitest + React Testing Library; `vitest.setup.tsx` mocks `Link` and `@/hooks/navigation` for every test (a test of either calls `vi.unmock`). A `vi.mock` factory returns `{ default: … }` for a default export; use `vi.hoisted` for values it reads eagerly. ≥80% coverage on `components/`, `lib/`, `hooks/` → [`adr-0013`](../docs/adr-0013-diff-scoped-coverage.md).
