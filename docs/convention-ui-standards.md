# Convention: web UI standards

- Enforced by: `npm run lint` — ESLint `no-restricted-syntax` blocks raw
  `<button>`/`<input>`/`<select>`/`<textarea>` in `components/**` and `app/**`,
  and `generate:ui-catalog:check` fails when the primitive catalog is stale.
  `npm run build` catches the server/client boundary trap below. The remaining
  rules (mobile-first, async-state wording, `interactiveCardClass`) are review.
- Issues: #1412

## Rule

- **Mobile-first and responsive** — Tailwind breakpoints (`sm:`, `md:`, `lg:`)
  and relative units. No fixed-pixel widths.
- **Server Components by default** — Client Components only where interactivity
  (`useState`, `useEffect`, event handlers) is required.
- **shadcn/ui primitives** — every interactive control uses a `components/ui/`
  primitive. Don't hand-style raw
  `<button>`/`<input>`/`<select>`/`<textarea>`; ESLint rejects them outside
  `components/ui/` itself. **The inventory is
  [`spec-ui-primitives.md`](spec-ui-primitives.md)** — generated from the
  source, so consult it rather than a list written down here. If nothing there
  fits, add a primitive; don't style a raw element at the call site.
  The rare legitimate exception (a hidden `<input type="file">`, which no
  primitive can wrap) takes an `eslint-disable-next-line no-restricted-syntax`
  with the reason spelled out.
- **Class overrides** — merge with `cn()` from `lib/cn.ts` (`clsx` +
  `tailwind-merge`).
- **Clickable cards** — use `interactiveCardClass` from `components/ui/card.tsx`.
  Don't hand-roll per-card `hover:shadow`/`hover:bg` variants.
- **Async states** — page-level loading is
  `<p className="text-muted">Loading…</p>`; page-level errors are
  `<p className="text-danger">Failed to load X.</p>`. Buttons swap their label to
  a `…`-suffixed present participle while a mutation is pending (`Saving…`,
  `Updating…`). Always the typographic ellipsis `…`, never `...`.

## Why

- **`cn()`** is what makes a `className` prop reliably override a primitive's
  defaults; plain concatenation loses to Tailwind's class ordering.
- **`interactiveCardClass`** shows its accent ring **at rest**, not just on
  hover, so cards read as interactive on touch devices, which have no hover
  state.
- **The Server Component default** has a hard trap behind it — see below.

## Worked examples

### Tailwind v4, CSS-first theme

There is **no `tailwind.config.ts` anywhere**. `app/globals.css` defines RGB-triple
CSS variables per light/dark, exposed as utilities via `@theme inline`, plus a
separate `@theme` block for shadow/radius tokens. Colors track the active scheme
at runtime.

### Light/dark

Dark tokens key off `:root[data-theme='dark']`, **not** `prefers-color-scheme`.
`lib/theme.ts` owns the per-device preference (`auto`/`light`/`dark` in
localStorage) and exports `themeInitScript`, inlined in `app/layout.tsx`'s
`<head>` so the resolved scheme is on `<html>` before first paint (no flash);
`auto` stays live via a `matchMedia` listener. The Appearance section on
`/settings` is the toggle.

`lib/theme.ts` deliberately has **no React imports**, because both the server
layout and a client component import it.

## What violating it looked like

**A Server Component must never import from a file that pulls in client-only
hooks** — even for an unrelated shared constant. Next's build-time server/client
boundary check rejects it, and that check is enforced **only by `next build`**,
not by `tsc`, ESLint, or Jest. So it passes every local check and fails the
Docker build.

Put constants shared across the boundary in a plain `lib/` module with no React
imports — which is exactly why `lib/theme.ts` has none.

### A documented rule that nothing checked (#1412)

Every rule above was written down before it was enforced, and the codebase
drifted away from all of them while this document claimed ESLint had it covered.
By the time anyone measured: raw `<button>` in 9 component files, raw `<input>`
in **19** places, `interactiveCardClass` on 10 of ~24 card files, and
`components/ui/card.tsx` itself joining class arrays instead of using `cn()` —
the primitive this convention cites most, breaking the rule it cites it for.

Two things caused it, and both are now fixed rather than restated:

- **Nothing failed.** `eslint.config.mjs` had a `no-restricted-syntax` block
  that only covered protobuf `satisfies`. Writing "don't hand-style raw
  `<button>`" changed nothing a contributor or agent would ever trip over. The
  JSX selectors now make it a failing check.
- **The inventory was prose, and stale.** This file used to hardcode the
  primitive list, which had already fallen behind (`Breadcrumb`, `Label`,
  `PageContainer` were missing). Someone reading it got a *wrong* answer to
  "what already exists" — worse than no answer, because it reads as
  authoritative. Hence a generated catalog with a CI staleness check.

The general lesson: a convention that can only be violated silently will be.
When adding a rule here, say what fails when it's broken — and if the answer is
"nothing", build that check first.
