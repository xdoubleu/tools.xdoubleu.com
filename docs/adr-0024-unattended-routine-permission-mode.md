# ADR-0024: Unattended routines run with permissions bypassed at the trigger, not via a broader repo allowlist

- Status: Accepted
- Issues: #1745. Related: #1706, #1707 (the Plan Mode half of the same "no
  human present" problem), #1444/#1441 (the `automated_actions` run-history
  mechanism these routines already use to make an unattended run
  observable).
- Affects: the manual routine setup steps in `docs/spec-routine-nightly-
  maintenance-sweep.md`, `docs/spec-routine-ready-issues-executor.md`,
  `docs/spec-routine-red-pr-repair.md`, and any future routine created the
  same way — never `.claude/settings.json` itself.

## Context

#1706/#1707 fixed unattended scheduled routines (ready-issues-sweep,
nightly-maintenance-sweep, red-pr-repair, and the routines the Grafana
`routine-fire` webhook fires — `api/internal/routines.Client`, issue #1444)
getting trapped in Plan Mode forever, by removing the repo-wide
`permissions.defaultMode: plan` default (`docs/adr-0014-start-finish-task-
enforcement.md`). That solved only half of "no human is present to approve
anything": outside Plan Mode, the *default* permission mode still prompts
for approval on every tool call not already covered by
`.claude/settings.json`'s `permissions.allow` list — and an unattended
routine has no one to answer that prompt either. A routine dispatching
`start-task`/`finish-task` subagents (`ready-issues-executor`), pushing
fixing commits (`red-pr-repair`), or calling a mutating MCP tool
(`resolve_sentry_issue`, `dismiss_security_alert`,
`record_action` — `nightly-maintenance-sweep`) stalls on its first call
outside that allowlist exactly the way it used to stall on Plan Mode.

## Decision

Each routine's trigger, configured by hand in the claude.ai routines UI per
its own `docs/spec-routine-*.md` (the trigger-creation API silently drops
connectors, issue #1438, so these are already hand-created), is set to run
with permissions bypassed (`permissionMode: bypassPermissions` — skip the
per-tool-call approval prompt entirely) instead of the default interactive
mode. This is a property of the *trigger*, scoped to exactly that routine's
invocations, not a change to `.claude/settings.json`. Interactive sessions
in this repo — a human at the keyboard, or a human-initiated Claude Code
Remote/web session — are completely unaffected: they keep the existing
default permission mode and the existing `permissions.allow` list exactly
as documented in root `CLAUDE.md` and `docs/adr-0014`.

Each of the three `docs/spec-routine-*.md` files now lists "set the
trigger's permission mode to bypass permissions" as one of its manual setup
steps, alongside the schedule and connectors — the same kind of
hand-entered, non-API-reproducible configuration those docs already record
for exactly this reason (issue #1438).

## Alternatives considered

### Broaden `.claude/settings.json`'s `permissions.allow` list

Adding the write/bash actions these routines need (`git commit`/`push`,
`gh pr create`, `Edit`/`Write` inside a worktree, the mutating MCP tools)
to the repo-wide allowlist would unblock routines without touching the
claude.ai routines UI at all. Rejected: `.claude/settings.json` has no way
to scope an allowlist entry to "only when nobody is present to approve it"
— every entry added there also silently pre-approves the same action for
every interactive session, including a human working through `start-task`
who might actually want to be asked before, say, a destructive `git`
command runs. The issue this ADR resolves explicitly calls out avoiding
exactly this blanket loosening.

### Detect an unattended session and switch settings profiles conditionally

Have the routine's launch path set something (an env var, a launch flag)
that a hook or a conditional settings merge reads to apply a wider
allowlist only for that session. Rejected as unnecessary complexity: the
routines platform already has a first-class per-trigger permission-mode
setting that does exactly this — no in-repo detection mechanism needs
inventing, and there is nothing to keep in sync between this repo's
settings and routine-trigger configuration beyond the one field each spec
doc now documents.

## Consequences

- A routine created without this step set (or a new routine added later
  without following the pattern) reproduces the exact same silent stall
  this ADR fixes — it looks identical to the Plan Mode trap #1706 fixed,
  just one layer further in. There is no code-level guard against this;
  it's a manual-setup checklist item the same way connectors and schedule
  already are, and equally unrecoverable by API (issue #1438).
- Because the routine's own trigger, not repo config, carries this
  setting, nothing in `.claude/settings.json`, `make hooks/test`, or CI
  verifies it's actually set — the only verification is a routine
  actually completing an unattended run past its first
  Edit/Write/Bash/mutating-tool call, the same way #1625's push-access
  question was only ever confirmed by watching a real dispatch succeed.
- Each `automated_actions` row a routine writes (`record_action`,
  `AutomatedActionStalled`/`AutomatedRoutineMissed` in
  `docs/adr-0022-prometheus-grafana-metrics.md`'s Phase 15/17) stays the
  detection mechanism for a routine that silently stalls for *any* reason,
  permission-mode misconfiguration included — this ADR doesn't add a
  distinct alert for this specific failure mode, since the existing one
  already covers "a known routine didn't run/didn't close."

## Revisit when

The routines platform gains a per-repository or per-connector permission
policy that could replace per-trigger configuration — at that point,
re-evaluate whether three hand-set trigger fields are still the right
place for this, or whether it collapses into one repo-level setting. Also
revisit if a routine ever needs to run with *narrower* permissions than
`bypassPermissions` grants (e.g. a future routine that should mutate data
but never push code) — that would need a mode this ADR doesn't cover.
