# Convention: web UI standards

- Enforced by: eslint + `npm run build` (partially); the rest is review
- Issues: —

## Rule

- **Mobile-first and responsive** — Tailwind breakpoints (`sm:`, `md:`, `lg:`)
  and relative units. No fixed-pixel widths.
- **Server Components by default** — Client Components only where interactivity
  (`useState`, `useEffect`, event handlers) is required.
- **shadcn/ui primitives** — every interactive control uses a `components/ui/`
  primitive (`Button`, `Input`, `Select`, `Textarea`, `MenuItem`, `Badge`,
  `Card`, `Dialog`, `Checkbox`, `RadioGroup`, `Combobox`, `Popover`, `DateInput`,
  `Table`). Don't hand-style raw
  `<button>`/`<input>`/`<select>`/`<textarea>`.
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
