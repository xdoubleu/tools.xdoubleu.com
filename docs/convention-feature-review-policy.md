# Convention: feature-track work skips human review behind an automated quality gate

- Enforced by: the `feature` label + `finish-task`'s track branch (#1634); the
  gate itself is `code-review` + Go `depguard` (#1629) + web
  `dependency-cruiser` (#1630) + generated Mermaid diagrams (#1630, #1631) +
  diff-scoped mutation testing (#1632)
- Issues: #1627, #1628, #1629, #1630, #1631, #1632, #1633, #1634, #1635

## Rule

Two tracks, told apart by the `feature` label (`refine-feature` applies it to
the epic and every sub-issue; `finish-task` reads it back):

- **Maintenance** (`enhancement`/`bug`/`chore`) — `finish-task`'s tiered
  rule: small code-only PRs auto-merge; larger or architectural ones wait for
  human review.
- **Feature** (`feature`) — no human reviews the PR, regardless of size. An
  automated quality gate replaces the reviewer, then the PR auto-merges.

`ci-pass` is required on both tracks; this removes human *review*, not CI.

When the last sub-issue of a feature epic closes, `finish-task` sends one
Slack summary via the `notify_slack` MCP tool. That is separate from the
Grafana→Slack alert path ([`adr-0022`](adr-0022-prometheus-grafana-metrics.md)).

## The gate

Each check stands in for something a reviewer would catch:

1. **`code-review` skill**, findings applied — design problems.
2. **Architecture-boundary lints** — Go `depguard` in root `.golangci.yml`
   denies importing `api/apps/<name>/internal/*` from outside that app (the
   `dashboard`/`learningpaths` carve-out per
   [`adr-0007`](adr-0007-dashboard-app-owns-public-sharing.md)); web
   `dependency-cruiser` enforces the server/client boundary
   ([`convention-ui-standards`](convention-ui-standards.md)) and `web/app/*`
   route segregation — imports reaching where they shouldn't.
3. **Generated Mermaid dependency diagrams** (`godepgraph`,
   `dependency-cruiser --output-type mermaid`) — the architectural shape of
   a change. Generated rather than hand-maintained so they can't drift.
4. **Diff-scoped mutation testing** (`gremlins`, StrykerJS), scoped like
   [`adr-0013`](adr-0013-diff-scoped-coverage.md)'s coverage — tests that
   don't assert anything.

## Why

The user wants zero PR-level review for feature work. Their checkpoints are
an exhaustive grilling interview during refinement and a Slack message when
the epic finishes — not GitHub. The `feature` label was chosen as the
discriminator because it leaves existing `enhancement` maintenance work on
the tiered rule.
