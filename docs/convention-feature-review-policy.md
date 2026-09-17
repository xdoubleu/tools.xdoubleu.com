# Convention: feature-track work skips human review behind an automated quality gate

- Enforced by: the `feature` label + `finish-task`'s track branch (#1634); the
  gate itself is `code-review` (existing skill) + Go `depguard` (#1629) + web
  `dependency-cruiser` (#1630) + generated Mermaid diagrams (#1630, #1631) +
  diff-scoped mutation testing (#1632)
- Issues: #1627, #1628, #1629, #1630, #1631, #1632, #1633, #1634, #1635

## Rule

There are two tracks for shipping a change in this repo, told apart by one
label:

- **Maintenance** (`enhancement`/`bug`/`chore`, epic #1338) — the
  self-healing pipeline. `finish-task`'s existing tiered rule is unchanged: a
  small, code-only PR auto-merges; a larger or architecturally significant
  one (migrations, shared `api/internal/*`, `CLAUDE.md`/Makefile/workflow
  edits, >150–200 lines or >8 files) waits for a human to review it.
- **Feature** (`feature` label, epic #1627) — **no human reviews the PR at
  all**, regardless of size. What a reviewer would otherwise have caught is
  instead caught by an automated quality gate that runs before every
  feature-track PR ships. `ci-pass` is still required on both tracks — this
  removes human *review*, not CI *verification*.

`refine-feature` (#1633) is what applies the `feature` label, and applies it
to the epic and to every sub-issue it produces — `finish-task` (#1634) is
what reads that label back to decide which of the two behaviors above
applies to a given PR.

## Why

Epic #1627 records the user's own stated preference: for feature work they
want zero PR-level human review, and they will not use GitHub to track it —
their two checkpoints become (1) an exhaustive, zero-assumption interview
during issue refinement, where they *do* want to be heavily involved, and
(2) a Slack message once a feature epic finishes, not a GitHub-based
checkpoint. Removing the reviewer's usual job — catching design problems,
architectural drift, and tests that don't actually assert anything — means
something else has to do that job instead, which is what the four-part gate
below is for.

**Track discriminator: the `feature` label**, not a new taxonomy. It was
chosen specifically because it doesn't misclassify #1338's own sub-issues,
which are labeled `enhancement` and must keep going through the existing
tiered rule unchanged.

## What replaces review

Four checks, each standing in for a specific thing a human reviewer would
otherwise have caught:

1. **The `code-review` skill**, run automatically, with its findings applied
   before the PR ships — the same skill a human would otherwise have been
   asked to run by hand.
2. **Deterministic architecture-boundary lints** — Go `depguard` rules in
   the shared root `.golangci.yml` denying an app's
   `api/apps/<name>/internal/*` from being imported outside that app, with
   the `dashboard` (and `learningpaths`) carve-out
   [`adr-0007`](adr-0007-dashboard-app-owns-public-sharing.md) already
   documents — reaching another app only through its exported struct
   methods, never its `internal/` package (#1629); web
   `dependency-cruiser` rules for the server/client import boundary
   ([`convention-ui-standards`](convention-ui-standards.md)) and
   `web/app/*` route segregation (#1630). Both replace a reviewer eyeballing
   "does this import reach somewhere it shouldn't."
3. **Auto-generated Mermaid diagrams of the real dependency graph** —
   `godepgraph -format mermaid` for Go (#1631), `dependency-cruiser
   --output-type mermaid` for web (#1630) — so a change's architectural
   shape can be read at a glance, the way a reviewer would otherwise have
   built a mental model from the diff. Both GitHub and Claude Code render
   Mermaid natively, so no image-conversion step is needed. A hand-authored
   C4/Structurizr-style model was considered and rejected during planning:
   the two boundary linters above already generate a model of the real
   imports, and a parallel hand-maintained one would drift from it.
4. **Diff-scoped mutation testing** — `gremlins` for Go, `StrykerJS` for
   web, scoped to changed packages/files the same way
   [`adr-0013`](adr-0013-diff-scoped-coverage.md)'s coverage gate is scoped
   to changed lines, rather than a whole-repo run on every PR (#1632).
   Coverage percentage alone doesn't prove a new test asserts anything
   meaningful, which matters more once no human reads the diff to notice a
   vacuous test.

Each of the four, when it was added, was also run once against the **whole
existing codebase** (not diff-scoped) to produce a baseline of what already
violated the new rule or had weak mutation coverage, rather than silently
grandfathering existing code in — see each sub-issue above for its own
baseline result.

## What stays constant

`ci-pass` is still the required check on a `feature` PR exactly as on any
other — nothing here changes CI, only who (or what) reads the diff before it
merges. The maintenance track's own decisions, sub-issues, and auto-merge
thresholds (#1338) are untouched.

## Notification: Slack, not GitHub, once an epic finishes

The user does not use GitHub to track feature work, so the checkpoint after
the up-front grilling interview is a Slack message once every sub-issue
under a feature epic has closed — not a GitHub notification, and not a
review request. `finish-task` (#1634) sends it via the mutating `notify_slack`
MCP tool (#1628), summarizing what was built and linking the generated
Mermaid diagrams. The maintenance track's existing Grafana→Slack alert path
([`adr-0022`](adr-0022-prometheus-grafana-metrics.md)) is a separate,
unrelated channel and is unchanged by this.

## Worked examples

Epic #1627 itself is labeled `feature` and is the first case this policy
governs — it dogfoods its own rule rather than being reviewed by hand.

## What violating it looked like

None yet — this is a new convention, introduced by #1627 rather than in
response to an incident.
