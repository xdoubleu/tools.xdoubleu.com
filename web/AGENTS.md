# web/ — Frontend

Next.js 16 App Router, React 19, TypeScript strict, standalone Node server (`output: 'standalone'`). Run `npm` commands from this directory; shared commands are in the root `AGENTS.md`.

## Layout

- `app/` — one route folder per app/domain. `dashboard/{games,reading}/` holds both owner and public (token-shared) dashboards → [`adr-0007`](../docs/adr-0007-dashboard-app-owns-public-sharing.md).
- `components/` — shared components at the root, one subfolder per domain, primitives in `components/ui/`.
- `lib/` — `client.ts` (browser transport), `server/` (RSC transport + fetchers), `swrKeys.ts`, `env.ts`, `cn.ts`, per-domain subfolders; `gen/` is generated (read the `.proto` instead).
- `hooks/` — SWR hooks per domain, plus client-only WebSocket hooks.

## Data flow (RSC + SWR)

`lib/client.ts` (browser, binary, `credentials: 'include'`) and `lib/server/client.ts` (RSC, per request via `cache()`, 10s timeout). Server components prefetch with `fetchOrNull` (`lib/server/fetchers.ts`) and pass results as SWR fallback via `<SWRFallback>`.

- **The RSC transport must never forward the refresh-token cookie** — an RSC can't persist a rotated one, so a server-side refresh kills the browser's session.
- **Every SWR key goes through `lib/swrKeys.ts`**, for both hooks and `mutate()`; an inline literal splits the cache from its invalidator. `<SWRFallback>` keys must match the hook's exactly.
- `getApiUrl()` (`lib/env.ts`) reads `window.__ENV__.API_URL` in the browser (injected in `app/layout.tsx`, since one build serves every environment) and `process.env.API_URL` on the server.

## Commands

```bash
npm ci                                  # first, in a fresh worktree
npm run test:cov:diff                   # coverage on lines changed vs origin/main — run before pushing (matches codecov/patch)
npm run mobile:audit -- /trains /books  # 375x667 Playwright audit; needs `npm run dev` + `npx playwright install chromium`
npm run generate:ui-catalog             # regenerate components/ui/README.md (lint fails if stale)
```

## UI rules → [`convention-ui-standards`](../docs/convention-ui-standards.md)

- **ESLint rejects raw controls and the hand-rolled forms of existing primitives** (`<h1>`, cards, banners, stretched links, loading text…) via the `ui/*` rules in `eslint-rules/`. Check [`components/ui/README.md`](components/ui/README.md) first; add a primitive rather than styling a raw element. A new class-pattern check goes into `eslint-rules/` with a RuleTester case.
- Merge classes with `cn()`; navigating cards use `LinkCard`, with controls in its `actions` footer.
- Pending buttons use a present participle with the typographic `…`, never `...`.
- Tailwind v4 CSS-first (no `tailwind.config.ts`); dark tokens key off `:root[data-theme='dark']`, owned by `lib/theme.ts`.
- **A Server Component must never import a file that pulls in client-only hooks**, even for a constant. Only `next build` catches it — put shared constants in a React-free `lib/` module.

## kobo-gateway client (`lib/books/gatewayClient.ts`)

Talks to the local helper at `https://127.0.0.1:41132` (see `kobo-gateway/AGENTS.md`). The browser makes all authenticated API calls and hands the gateway only a sync URL.

- Bump `REQUIRED_GATEWAY_VERSION` (with Go's `GatewayVersion`) **only for protocol breaks**; routine updates flow through `gatewayNeedsUpdate` comparing against `getKoboGatewayRelease()` → [`adr-0004`](../docs/adr-0004-runtime-release-env-vs-compile-stamp.md).
- `public/` is assembled at Docker build time (`build-web.yml` stages the kobo-gateway artifacts), **so the download route 404s under `npm run dev`** → [`adr-0002`](../docs/adr-0002-kobo-gateway-ci-cache-split.md).

## OAuth consent (`app/oauth/consent/`)

Drives the api's embedded fosite AS directly; the pending request's query params are echoed verbatim both ways — don't normalize or default them here.

## Files and tests

- Split TS/TSX files over ~300 lines before adding code (components by UI concern, hooks by data domain).
- Jest + React Testing Library; ≥80% coverage on `components/`, `lib/`, `hooks/` → [`adr-0013`](../docs/adr-0013-diff-scoped-coverage.md).

<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
