# Convention: no PR auto-merges except Renovate's; feature work adds an automated gate

- Enforced by: the `feature` label + `finish-task`'s track branch (#1634); the
  gate itself is `code-review` + Go `depguard` (#1629) + web
  `dependency-cruiser` (#1630) + diff-scoped mutation testing (#1632)
- Issues: #1627, #1629, #1630, #1631, #1632, #1633, #1634, #1635, #1922, #2774

## Rule

Every PR waits for the owner's review and merge. The only exception is
Renovate's dependency PRs, which auto-merge through `renovate.json5`.
`finish-task` never arms auto-merge, on either track or unattended.

The `feature` label (`refine-feature` applies it to the epic and every
sub-issue; `finish-task` reads it back) selects the gate, not the merge path:

- **Maintenance** (`enhancement`/`bug`/`chore`) — no gate; large or
  architectural changes get a `## Manual review needed` section.
- **Feature** (`feature`) — the automated quality gate below runs before the
  PR opens, so review starts from a checked diff.

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

Auto-merged agent PRs deploy on merge with no human in the chain. The owner
reviews every PR instead, and the gate keeps that review short by catching
design, boundary and weak-test problems first.
