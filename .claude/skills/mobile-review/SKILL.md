---
name: mobile-review
description: Review freshly-written frontend code in web/ for mobile usability and design problems — measuring horizontal overflow, tap-target size and spacing, and iOS input zoom in a real 375x667 Playwright viewport, then reading the source for what a browser can't see (hover-only affordances, safe areas, themes) — and report ranked, fix-shaped suggestions. Use after implementing or changing any page or component under web/, or when the user asks to "check this on mobile", "review the mobile design", "is this responsive", "does this work on a phone". Complements `npm run lint`, which enforces the primitives but checks nothing about layout at small widths.
---

# Mobile review

`docs/convention-ui-standards.md` makes mobile-first a rule and says outright
which half of it is enforced: ESLint blocks raw interactive elements and the
UI catalog staleness check keeps the inventory honest, but **"mobile-first,
relative units, no fixed-pixel widths" is review only**. Nothing fails when a
card overflows a 375px viewport. That gap is what this skill fills.

Per that document's own closing lesson — *a convention that can only be
violated silently will be* — anything found here that is mechanically
checkable belongs in `eslint.config.mjs` as a `no-restricted-syntax` rule, not
in a review comment. Step 5 is not optional garnish.

## Scope

Only `web/`. Determine what to review, in this order:

1. What the user named, if they named something.
2. Otherwise `git diff --stat main...HEAD -- web/` plus unstaged changes —
   the code just written, not the whole app.

Read the changed `.tsx` files in full. Reviewing a diff hunk in isolation
misses the mobile failures, which are almost always about a **parent's**
layout constraints meeting a child's intrinsic width.

## Reference viewport

**375 × 667, one thumb, no hover, no keyboard.** Every judgement below is
against that, not against a laptop window narrowed with a mouse. A second
pass at 320px is worth it only for pages carrying a table or a wide numeric
readout.

## Measure first, then read

`npm run mobile:audit` (`web/scripts/mobile-audit.mjs`) drives Playwright at
375 × 667 in both themes and **measures** the four things that are guesswork
from source alone:

```bash
cd web
npx playwright install chromium      # once per machine
npm run dev &                        # the audit needs the app running
npm run mobile:audit -- /trains /books/library
```

It reports, with the offending elements and their real pixel sizes:

- **`horizontal-overflow`** — `scrollWidth` exceeding the viewport, naming the
  *outermost* elements that stick out, so one wide child doesn't produce a
  finding for every ancestor containing it. This is the check the static pass
  can only guess at: a `min-w-[600px]` is a lead, a measured 640px row in a
  375px viewport is the bug.
- **`small-tap-target`** — every visible interactive element under 44 × 44px.
  A small link *inside* a ≥44px tappable ancestor is not reported, since the
  card is the real target.
- **`crowded-tap-targets`** — pairs of targets less than 8px apart.
- **`ios-focus-zoom`** — fields whose computed font-size is under 16px.
- **`no-viewport-meta` / `zoom-disabled`** — a missing viewport meta (the page
  then lays out at ~980px and scales down), or one that blocks pinch-zoom.

Exit code is 1 when anything `broken` is found, so it works as a gate. Useful
flags: `--json`, `--theme dark`, `--base-url`, `--width`, and
`--storage-state <file>` for routes behind auth — most of this app is logged
in, so capture a Playwright storage state once and reuse it rather than
auditing only the login page. `CHROMIUM_EXECUTABLE_PATH` points at an existing
Chromium where one is already installed.

Run this **before** reading the source. It turns the checklist below from
"scan the JSX and worry" into "here are three measured failures, plus the
things a browser cannot see". If the app genuinely can't be started, fall
through to the static checks alone and say in the report that the findings are
unmeasured.

## Checks

The audit covers overflow, tap targets and input zoom. Everything below is
either what a browser can't judge or what explains *why* a measured finding
happened — the static pass names the class to change.

Work through these against the changed files. Each names what to grep for,
but the grep is a lead — confirm by reading the component and its container.

### Layout and overflow

- **Fixed pixel widths.** `ast-grep run --pattern '<$C className=$CLS $$$ />' --lang tsx`
  is too broad here; grep the changed files for `w-[`, `min-w-[`, `max-w-[`,
  `h-[` with a `px` value. A `min-w-[600px]` inside a `flex` row is the single
  most common cause of a page that scrolls sideways.
- **Horizontal overflow** (measured by the audit — this is the *cause* side).
  Any `flex` row of ≥3 items with no `flex-wrap`, any
  `grid-cols-N` (N ≥ 2) without an `sm:`/`md:` prefix — those start at N
  columns on a phone. `grid-cols-1 sm:grid-cols-3` is the correct shape;
  bare `grid-cols-3` is a bug.
- **Tables.** A `<table>` in this codebase needs a scroll container or a
  card-per-row fallback below `sm:`. Check what the surrounding element does.
- **Long unbroken strings** — IDs, URLs, book titles, station names, journey
  IDs. Without `break-words`/`truncate` (plus `min-w-0` on the flex parent,
  which is the part everyone forgets) one long value pushes the whole row wide.
- **`100vh`.** Mobile browser chrome makes `h-screen` taller than the visible
  area; `100dvh` is what's meant.

### Touch

- **Target size and spacing.** The audit measures these; your job is the fix.
  For each reported element, check whether the `components/ui/` primitive
  supplies the padding and the call site shrank it (`size="sm"`, `h-6`), or
  whether the primitive itself is short. Icon-only buttons, close `×` buttons
  and inline links in dense lists are where this breaks.
- **Hover-only affordances.** `hover:` with no non-hover equivalent means the
  affordance does not exist on a phone. `group-hover:opacity-100` on a
  reveal-on-hover action is the standard offender. Clickable cards must use
  `interactiveCardClass` from `components/ui/card.tsx`, which the convention
  says shows its ring **at rest** for exactly this reason.
- **Native `title` tooltips** never appear on touch. If the `title` carries
  information not otherwise on screen, that information is missing on mobile.

### Forms and keyboard

- **iOS zoom.** An `<input>` rendering below 16px makes Safari zoom the page on
  focus and not zoom back. Check any `text-sm`/`text-xs` on an input primitive.
- **Keyboard type.** Numeric, email, and search fields should carry
  `inputMode`/`type` so the right keyboard opens.
- **The on-screen keyboard eats the bottom half of the viewport.** A submit
  button pinned to the bottom of a form, or a `fixed bottom-0` bar, needs
  checking against a focused field.
- **Dialogs and sheets** must scroll internally and stay reachable — a dialog
  taller than 667px with its actions below the fold is unusable.

### Visual

- **Both themes.** Dark tokens key off `:root[data-theme='dark']`; anything
  introducing a colour must work in both. Contrast on `text-muted` over a card
  background is the usual near-miss.
- **Safe areas.** Anything `fixed` at the top or bottom needs
  `env(safe-area-inset-*)` accounting, or it lands under the notch or the home
  indicator.
- **Sticky headers** costing >15% of a 667px viewport are too tall for a phone.
- **Async states** follow the convention's wording rules — `Loading…`,
  `Saving…`, typographic `…`. On mobile these matter more, not less: a slow
  tap with no feedback reads as a dead control, which is why
  `CardLinkStatus` exists.

## Steps

1. **Establish scope** per above, then **run `npm run mobile:audit`** on the
   affected routes. Read the changed components in full — including the layout
   that contains them — with the measured findings already in hand.

2. **Run the static checks.** For a large diff, delegate the grep sweep to a subagent
   per root `CLAUDE.md`'s "Delegating to Subagents" and have it return only
   the hits with `file:line`, not the matched files.

3. **Look at it.** Beyond the numbers the audit returns, screenshot the changed
   routes at 375 × 667 in both themes when something is reported but the cause
   isn't obvious from the source — collisions and cramped spacing read
   instantly in a picture and not at all in a class list.

4. **Report, ranked, fix-shaped.** Group as:
   - **Broken on mobile** — unusable or unreachable: sideways scroll, a
     control that cannot be tapped, an action only reachable on hover.
   - **Degraded** — usable but poor: cramped targets, truncation, avoidable
     zoom.
   - **Polish** — spacing, contrast near the line, wording.

   Each finding gets `file:line`, the concrete failure — with the audit's
   real numbers where it measured them ("this row is 640px wide in a 375px
   viewport, so the page scrolls sideways"; "this icon button is 24 × 24px")
   — and the specific Tailwind or primitive change that fixes it. No finding
   without a fix. If a class is the whole fix, write the class.

5. **Close the enforcement gap.** For every finding, ask whether ESLint could
   have caught it. Bare `grid-cols-N`, `h-screen`, sub-16px input text and
   fixed-`px` widths in `app/**`/`components/**` all can — they are class-string
   patterns, the same shape as the existing `no-restricted-syntax` JSX rules.
   Add the rule in the same change as the fix; a rule shipped later never
   ships. Anything needing real layout measurement belongs in
   `scripts/mobile-audit.mjs` as another check instead — that's the same
   "build the check" move, one level up from a class-string pattern. Only
   genuine design judgement stays review-only.

6. **Apply the fixes** when the user asked for a review-and-fix, or when the
   findings are small and local. Otherwise report and let them choose. Either
   way, `npm run lint` and `npm run build` before finishing, per
   `finish-task`.

## Relationship to the other skills

Runs **before** `finish-task`, on work that is complete but not yet shipped —
it is a review pass over new frontend code, not a step in the ship sequence.
It does not open issues or PRs itself; findings the user defers become part of
the tracking issue `start-task` already created, or a follow-up via
`refine-issue`.
