# Convention: every PR auto-merges on green CI; feature work adds an automated gate

- Enforced by: the `feature` label + `finish-task`'s track branch (#1634); the
  gate itself is `code-review` + Go `depguard` (#1629) + web
  `dependency-cruiser` (#1630) + diff-scoped mutation testing (#1632)
- Issues: #1627, #1629, #1630, #1631, #1632, #1633, #1634, #1635, #1922, #2774

## Rule

Every PR merges itself once `ci-pass` is green; the owner reviews afterwards
and checks the task in Testing. `finish-task` arms `gh pr merge --auto
--squash` on both tracks and unattended. Renovate's PRs auto-merge through
`renovate.json5`. Never merge on red CI.

The `feature` label (`refine-feature` applies it to the epic and every
sub-issue; `finish-task` reads it back) selects the gate, not the merge path:

- **Maintenance** (`enhancement`/`bug`/`chore`) — no gate; large or
  architectural changes get a `## Manual review needed` section for the
  owner's later review.
- **Feature** (`feature`) — the automated quality gate below runs before the
  PR opens, since nothing else checks the diff before it merges.

`ci-pass` is required on both tracks.

**Feature work is never completed headless.** A human drives the
implementation and tests the output when the epic completes; no background
agent picks up refined feature items or finishes them.

## The gate

Each check catches something a reviewer would otherwise have to (also running, when
the shape of a change needs one, the generated Mermaid `arch:diagram` targets
as a manual aid):

1. **`code-review` skill**, findings applied — design problems.
2. **Architecture-boundary lints** — Go `depguard` in root `.golangci.yml`
   denies importing `api/apps/<name>/internal/*` from outside that app (the
   `dashboard`/`learningpaths` carve-out per
   [`adr-0007`](adr-0007-dashboard-app-owns-public-sharing.md)); web
   `dependency-cruiser` forbids imports across `app/`,
   `components/` and `lib/` domains — imports reaching where they shouldn't.
3. **Diff-scoped mutation testing** (`gremlins`, StrykerJS), scoped like
   [`adr-0013`](adr-0013-diff-scoped-coverage.md)'s coverage — tests that
   don't assert anything. gremlins only runs a package's own tests, so code
   reached solely by app-root integration tests is listed NOT COVERED, never
   run: keep branching logic in pure functions unit-tested in their own
   package.

## Why

Merges deploy with no human in the chain: the owner chose speed over a
pre-merge review and reviews retroactively. Green CI is the only merge gate, so
the feature gate catches design, boundary and weak-test problems first, and
the human Ready move stays the approval for unattended work
([`convention-unattended-agent-trust`](convention-unattended-agent-trust.md)).
