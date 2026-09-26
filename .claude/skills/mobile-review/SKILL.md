---
name: mobile-review
description: Review freshly-written frontend code in web/ for mobile usability and design problems — measuring horizontal overflow, tap-target size and spacing, and iOS input zoom in a real 375x667 Playwright viewport, then reading the source for what a browser can't see (hover-only affordances, safe areas, themes) — and report ranked, fix-shaped suggestions. Use after implementing or changing any page or component under web/, or when the user asks to "check this on mobile", "review the mobile design", "is this responsive", "does this work on a phone". Complements `npm run lint`, which enforces the primitives but checks nothing about layout at small widths.
---

# Mobile review

[`docs/convention-ui-standards.md`](../../../docs/convention-ui-standards.md)'s
mobile-first rule is review-only; this skill is that review. Runs before
`finish-task`; it opens no issues or PRs (deferred findings go on the
tracking issue or a `refine-issue` follow-up).

## Scope

Only `web/`: what the user named, else `git diff --stat main...HEAD -- web/`
plus unstaged changes. Read changed `.tsx` files in full, including their
containing layout — mobile failures are usually a parent's constraints
meeting a child's intrinsic width.

Reference viewport: **375 × 667, one thumb, no hover, no keyboard**. Add a
320px pass only for tables or wide numeric readouts.

## Measure first

```bash
cd web
npx playwright install chromium      # once per machine
npm run dev &
npm run mobile:audit -- /trains /books/library
```

`web/scripts/mobile-audit.mjs` checks both themes and reports, with real
pixel sizes:

- `horizontal-overflow` — outermost elements wider than the viewport.
- `small-tap-target` — interactive elements under 44 × 44px (not inside a
  ≥44px tappable ancestor).
- `crowded-tap-targets` — targets under 8px apart.
- `ios-focus-zoom` — fields under 16px font-size.
- `no-viewport-meta`. Pinch-zoom is deliberately disabled.

Exits 1 on anything `broken`. Flags: `--json`, `--theme dark`, `--base-url`,
`--width`, `--storage-state <file>` (most routes need login).
`CHROMIUM_EXECUTABLE_PATH` reuses an installed Chromium.

**Page needs live backend data** (renders only `Loading…`/error here): add a
temporary route under `app/` rendering the client component with mock props,
audit it, then delete it. Don't prefix the folder with `_` (Next.js 404s
private folders). After deleting, `rm -rf .next` before `npm run lint`/`npm
run build`, or a stale `routes.d.ts` entry fails `tsc`.

If the app can't start, do the static checks alone and mark findings
unmeasured.

## Static checks

Greps are leads; confirm by reading the component and its container.

**Layout**

- Fixed widths: `w-[`, `min-w-[`, `max-w-[`, `h-[` with `px` — a
  `min-w-[600px]` in a `flex` row is the usual sideways scroll.
- `flex` rows of ≥3 items without `flex-wrap`; bare `grid-cols-N` (N ≥ 2)
  without an `sm:`/`md:` prefix (`grid-cols-1 sm:grid-cols-3` is right).
- `<table>` needs a scroll container or card-per-row fallback below `sm:`.
- Long unbroken strings (IDs, URLs, titles, stations) need
  `break-words`/`truncate` plus `min-w-0` on the flex parent.
- `h-screen`/`100vh` → `100dvh`.

**Touch**

- For each small target, check whether the `components/ui/` primitive or the
  call site (`size="sm"`, `h-6`) shrank it. Icon-only, close `×`, and dense
  inline links are typical.
- `hover:`/`group-hover:` with no non-hover equivalent doesn't exist on a
  phone. Clickable cards use `interactiveCardClass`
  (`components/ui/card.tsx`).
- Native `title` tooltips never show on touch.

**Forms**

- `text-sm`/`text-xs` on inputs triggers iOS zoom.
- Numeric/email/search fields need `inputMode`/`type`.
- The on-screen keyboard hides bottom-pinned submits and `fixed bottom-0`
  bars.
- Dialogs taller than 667px must scroll internally with reachable actions.

**Visual**

- Both themes (`:root[data-theme='dark']`); watch `text-muted` on cards.
- `fixed` top/bottom elements need `env(safe-area-inset-*)`.
- Sticky headers over 15% of 667px are too tall.
- Async states use `Loading…`/`Saving…`; slow taps need feedback
  (`CardLinkStatus`).

## Steps

1. Establish scope, run `npm run mobile:audit`, read the changed components.
2. Run the static checks; for a large diff delegate the grep sweep to a
   subagent returning only `file:line` hits.
3. Screenshot at 375 × 667 in both themes when a finding's cause isn't
   obvious from source.
4. Report ranked — **Broken** (sideways scroll, untappable, hover-only),
   **Degraded** (cramped, truncation, zoom), **Polish** — each with
   `file:line`, the concrete failure with measured numbers, and the exact
   class or primitive change. No finding without a fix.
5. Close the enforcement gap: a class-string or element pattern gets a
   `ui/*` rule (with RuleTester cases) in `web/eslint-rules/` in the same
   change; measurement-only problems get a check in
   `scripts/mobile-audit.mjs`. Only design judgement stays review-only.
6. Apply fixes when asked, or when small and local; otherwise report. Then
   `finish-task`.
