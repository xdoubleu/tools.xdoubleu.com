# Convention: web UI standards

- Enforced by: `npm run lint` (ESLint `no-restricted-syntax` blocks raw `<button>`/`<input>`/`<select>`/`<textarea>` in `components/**` and `app/**`; `generate:ui-catalog:check` fails on a stale catalog); `npm run build` (server/client boundary); `mobile-review` skill, whose `npm run mobile:audit` fails on overflow, sub-44px or crowded tap targets, and sub-16px inputs at 375x667. Everything else is review.
- Issues: #1412

## Rule

- **Mobile-first** — Tailwind breakpoints and relative units, no fixed-pixel widths.
- **Server Components by default** — Client Components only for interactivity.
- **shadcn/ui primitives** for every interactive control; the inventory is
  [`web/components/ui/README.md`](../web/components/ui/README.md) (generated).
  If nothing fits, add a primitive. A hidden `<input type="file">` may use
  `eslint-disable-next-line no-restricted-syntax` with a reason.
- **Class overrides** via `cn()` (`lib/cn.ts`).
- **Clickable cards** use `interactiveCardClass` (`components/ui/card.tsx`).
- **Async states** — loading `<p className="text-muted">Loading…</p>`, errors
  `<p className="text-danger">Failed to load X.</p>`, pending buttons `Saving…`.
  Always `…`, never `...`.

## Why

- `cn()` lets a `className` prop reliably override defaults; concatenation loses
  to Tailwind ordering.
- `interactiveCardClass` shows its ring at rest, since touch has no hover.

## Worked examples

- **Tailwind v4, CSS-first**: no `tailwind.config.ts`; `app/globals.css`
  defines RGB-triple variables per scheme via `@theme inline`, plus `@theme`
  for shadow/radius tokens.
- **Light/dark** keys off `:root[data-theme='dark']`, not
  `prefers-color-scheme`. `lib/theme.ts` stores `auto`/`light`/`dark` in
  localStorage and exports `themeInitScript`, inlined in `app/layout.tsx`'s
  `<head>` to avoid a flash. The toggle is on `/settings`.

## What violating it looked like

- **Server/client trap:** a Server Component importing any file that pulls in
  client hooks fails only at `next build`, not `tsc`/ESLint/Jest. Put shared
  constants in a React-free `lib/` module — why `lib/theme.ts` has no React
  imports.
- **#1412:** these rules were documented but unchecked, and the code drifted
  (raw `<input>` in 19 places, `interactiveCardClass` on 10 of ~24 cards). The
  ESLint selectors and generated catalog fixed it. When adding a rule, say what
  fails when it's broken; if nothing does, build that check first.
