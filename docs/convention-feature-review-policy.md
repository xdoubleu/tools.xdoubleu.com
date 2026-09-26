# Convention: feature-track work skips human review behind an automated quality gate

- Enforced by: the `feature` label + `finish-task`'s track branch (#1634); the
  gate itself is `code-review` + Go `depguard` (#1629) + web
  `dependency-cruiser` (#1630) + diff-scoped mutation testing (#1632)
- Issues: #1627, #1629, #1630, #1631, #1632, #1633, #1634, #1635, #1922

## Rule

Two tracks, told apart by the `feature` label (`refine-feature` applies it to
the epic and every sub-issue; `finish-task` reads it back):

- **Maintenance** (`enhancement`/`bug`/`chore`) — `finish-task`'s tiered
  rule: small code-only PRs auto-merge; larger or architectural ones wait for
  human review.
- **Feature** (`feature`) — no human reviews the PR, regardless of size. An
  automated quality gate replaces the reviewer, then the PR auto-merges.

`ci-pass` is required on both tracks; this removes human *review*, not CI.

**Feature work is never completed headless** (#1922). A human drives the
implementation: they instruct the code changes and **test the output when the
epic completes**. No background agent factory picks up refined feature items
or auto-finishes them. The `feature` label still means "PRs auto-merge
unreviewed", but the person who started the epic is responsible for verifying
the result.

## The gate

Each check stands in for something a reviewer would catch (also running, when
the shape of a change needs one, the generated Mermaid `arch:diagram` targets
as a manual aid):

1. **`code-review` skill**, findings applied — design problems.
2. **Architecture-boundary lints** — Go `depguard` in root `.golangci.yml`
   denies importing `api/apps/<name>/internal/*` from outside that app (the
   `dashboard`/`learningpaths` carve-out per
   [`adr-0007`](adr-0007-dashboard-app-owns-public-sharing.md)); web
   `dependency-cruiser` enforces the server/client boundary
   ([`convention-ui-standards`](convention-ui-standards.md)) and `web/app/*`
   route segregation — imports reaching where they shouldn't.
3. **Diff-scoped mutation testing** (`gremlins`, StrykerJS), scoped like
   [`adr-0013`](adr-0013-diff-scoped-coverage.md)'s coverage — tests that
   don't assert anything.

## Why

The user wants zero PR-level review for feature work. Their checkpoints are
an exhaustive grilling interview during refinement and hands-on testing when
the epic finishes — not GitHub code review. The `feature` label was chosen as
the discriminator because it leaves existing `enhancement` maintenance work on
the tiered rule.