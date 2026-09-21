---
name: posthog-ux-discovery
description: Mine PostHog product analytics + session replay for friction signals (rage-clicks, funnel drop-off, unused features) and file proposed UX-improvement GitHub issues — never applying a UI change or opening a PR itself. Use whenever the user asks to "check PostHog for UX issues", "run the UX discovery routine", or "find friction signals" — also the skill a weekly scheduled routine (issue #1639, part of epic #1637, `docs/spec-routine-posthog-ux-discovery.md`) runs unattended.
---

# PostHog UX Discovery

Queries PostHog's own MCP server (trends, funnels, retention, paths, HogQL —
not a bespoke `tools-apps` tool; see the "No new MCP tool" note below) for
product-analytics friction signals across `web/`'s key flows, and turns each
genuine finding into a proposed UX-improvement GitHub issue via
`refine-issue`. This skill **never** applies a UI change, opens a branch, or
creates a PR — its only output is a filed or updated issue that then goes
through the normal `refine-issue`/grilling pipeline like any other proposed
feature, per epic #1637's human-reviewed-refinement checkpoint.

Depends on issue #1638 (PostHog Web SDK integration) having shipped and had
at least a full week to collect real usage data — before that, every query
below returns empty or near-empty results, which is expected, not a bug.

## No new MCP tool

PostHog ships an official MCP server with real analytics queries — this
skill attaches it as a second connector (alongside `tools-apps` and GitHub),
the same way every routine in this repo attaches GitHub directly rather than
proxying it through `tools-apps`. There is no `api/internal/mcptools`
addition here: `api/` has no PostHog credentials or client of its own, and
building one would duplicate a read surface PostHog already exposes.

## Steps

0. **Open the run record.** Call `record_action(mode: "open",
   trigger_source: "schedule", routine_name: "posthog-ux-discovery")` before
   pulling any data. Keep the returned `id` for step 6.

1. **Enumerate key flows.** A "key flow" is a named, multi-step user journey
   in one of `web/`'s apps (e.g. games library → mark achievement, recipes →
   add to meal plan → export shopping list, books → sync to Kobo). Prefer
   funnels/insights already saved in the PostHog project (check via the MCP
   connector first) over inventing new ad hoc queries each run — a saved
   insight gives a stable, shareable URL to cite as evidence in any issue
   this skill files. If no relevant insight exists yet for a flow worth
   checking, note that gap in the run's final summary (a human can create
   the insight in PostHog) rather than approximating it with a raw HogQL
   query that will drift from what a person later looks at in the UI.

2. **Pull friction signals per flow**, via the PostHog MCP connector only:
   - **Rage-clicks**: query the `$rageclick` autocapture event, grouped by
     element/page, over the trailing 7 days.
   - **Funnel drop-off**: query each key flow's funnel over the trailing 7
     days.
   - **Unused/never-triggered features**: query event volume over the
     trailing 30 days for any named custom event PostHog is already
     capturing for a feature (autocapture alone can't identify "a feature
     nobody uses" — this only works for features with their own tracked
     event). If no such custom events exist yet, note that gap in the run's
     summary instead of guessing from autocapture volume.

3. **Apply the genuine-finding thresholds** (tuned to avoid weekly board
   spam — a routine that files on every minor blip trains its own tracking
   issues to be ignored):
   - **Rage-click cluster**: the same element/page was rage-clicked by **≥5
     distinct users** in the trailing 7 days.
   - **Funnel step drop-off**: a step loses **>30 percentage points** of
     entrants, over a window of **≥20 entrants**, **and persists across two
     consecutive weekly runs** before it's filed. A one-off week is noise,
     not signal.
     - **First time a drop is seen**: file a provisional record — a plain
       GitHub issue (via `issue_write`/`gh issue create`, not
       `refine-issue`, and **not added to the project board**) titled
       `[PostHog provisional] <flow name> — <step name> drop-off`, labeled
       `posthog-provisional` (a bookkeeping label local to this skill, not
       part of `.claude/github-triage.config.json`'s taxonomy — it never
       reaches a human-facing board column), body containing the flow/step
       names, the entrant/drop numbers, and this run's date. This is
       internal state carried between runs, not a user-facing proposal.
     - **A later run sees the same flow+step still dropping**: search open
       `posthog-provisional`-labeled issues for a title match. If found,
       this confirms persistence — proceed to step 4 as a genuine finding,
       then close the provisional issue (`state_reason: completed`, comment
       linking the real proposal issue) so it doesn't accumulate forever.
     - If a provisional issue's flow/step recovers (next run's drop no
       longer exceeds the threshold), close it as `not_planned` with a
       one-line comment — it was noise.
   - **Unused/never-triggered feature**: **zero usage events** over a
     trailing 30-day window, for a feature whose tracked event has existed
     for **≥30 days** (check the event's earliest occurrence via the PostHog
     MCP connector — never flag a just-shipped feature before anyone's had
     a chance to find it).

4. **Dedup against existing GitHub issues** (open **and** closed), the same
   way `sentry-triage` dedups against Sentry permalinks — search issue
   bodies for the PostHog insight/flow name instead of a permalink:
   - Referenced in an **open** issue → already tracked; update it with this
     run's fresh numbers instead of filing a duplicate.
   - Referenced in a **closed** issue → already proposed and resolved
     (implemented, or explicitly rejected) — don't refile the same finding
     without new evidence the situation changed materially.
   - Not referenced anywhere → new finding, continue to step 5.

5. **File or update a GitHub issue via `refine-issue`** (config for this
   repo lives in `.claude/github-triage.config.json`) for each genuine,
   undeduped finding. Body must include:
   - the PostHog insight/flow name and, when one exists, its saved-insight
     URL (so it's traceable back and dedup works next run, mirroring how
     `sentry-triage` cites a Sentry permalink)
   - the threshold evidence — the actual numbers that crossed the
     threshold, not just "friction detected"
   - a concrete proposed UI fix, not just a description of the symptom
   - **Labeling**: type `enhancement` when the finding is friction in an
     existing, already-shipped flow (the common case — rage-clicks,
     drop-offs, unused features are all "something that exists but doesn't
     work well"); `feature` only if the proposed fix is a wholly new
     capability, not just a tweak to what's there.
   - **Scope**: whichever app the flow belongs to (`reading`, `games`,
     `recipes`, `mealplans`, `shoppinglist`) when unambiguous, else
     `platform` (the config's own fallback).
   - **Priority**: follow `refine-issue`'s own P0/P1/P2 rule (defined in
     `.claude/github-triage.config.json`'s `priorityRule`) rather than
     hardcoding one here, the same way `sentry-triage` defers to it — a
     finding on an already-shipped, working flow ordinarily lands P1
     ("improves existing, working functionality"); reserve P0 for the rare
     case a finding reveals something actually broken, not just suboptimal
     (that's usually `monitoring-sweep`'s territory instead).

6. **Close the run record.** Call `record_action(mode: "close", id: <the id
   from step 0>, outcome: ...)` — `"no_action_needed"` if no threshold was
   crossed this run, `"succeeded"` if it filed/updated at least one finding
   (or provisional record) without erroring, `"failed"` if the run itself
   couldn't complete (the PostHog MCP connector errored out, etc.). This is
   the last step in every run — an opened row that's never closed is itself
   a detectable problem (issue #1443's `AutomatedActionStalled` rule).

7. **Report a final summary**: one line per finding (flow/step → evidence →
   filed/updated issue link), any provisional records opened or closed this
   run, and any gaps noted in steps 1/2 (missing saved insights, missing
   custom events for feature-usage tracking) worth a human's attention.

## Mutation boundary

This skill only ever calls `refine-issue`/`issue_write` to file, update, or
close an issue (including its own `posthog-provisional` bookkeeping issues).
It never opens a branch, creates a PR, or edits a file in this repository —
enforced the same way `monitoring-sweep`'s unattended mode enforces "never
write a code fix, never open a PR" for its own detection-only findings.

## Notes

- Distinct from `monitoring-sweep` (broken things: Sentry errors, red CI,
  alerts) and `sentry-triage` (Sentry-specific). This skill only ever
  proposes improvements to things that already work, never anything P0/P1.
- The weekly routine that runs this skill unattended is documented in
  `docs/spec-routine-posthog-ux-discovery.md` — its exact prompt text and
  required connectors, for reproducing the routine setup by hand in the
  claude.ai routines UI (the trigger-creation API silently drops connectors
  beyond what the creating session holds — #1438's finding, the same
  constraint every routine in this repo works around).
