# ADR-0013: Gate on coverage of changed lines, not whole-file percentages

- Status: Accepted
- Issues: #1301, #1364, #1376, #1632, #1868, T-300, T-301
- Affects: `tools/diff_coverage_ts.py`, `tools/diff_coverage_go.py`, `tools/merge_coverage.py`, `api/Makefile`, `.github/workflows/*-test.yml`, `web/package.json`

## Context

Whole-file percentages mislead both ways: a one-line edit to a gappy file fails,
while an untested new function hides in a well-covered file.

## Decision

Gate on **lines changed vs `origin/main`**, locally and in CI. The test workflows run the scripts on pull requests; there is no third-party coverage service (Codecov was dropped in T-300 after repeated outages blocked `ci-pass`):

- `npm run test:cov:diff` → `tools/diff_coverage_ts.py`; `make test/cov/diff` →
  `tools/diff_coverage_go.py`; `make test/cov/per-pkg` merges profiles via
  `tools/merge_coverage.py`.
- `diff_coverage_ts.py` fails a changed file under 80% line/branch on its
  changed lines, **or with no coverage data at all** (e.g. a new unimported
  file). Only files in `vite.config.ts`'s `test.coverage.include` count.
- `diff_coverage_go.py` reports a conservative block-boundary-only number beside
  the gate, and reads each module's prefix from its `go.mod`
  (api, kobo-gateway, factory-runner, selfupdate). `menubar_darwin.go` and
  `factory-runner/cmd/factory-runner/entry.go` are excluded.
- Mutation testing (`make test/mutation/diff`, `npm run test:mutation:diff`,
  #1632) reuses the same diff scoping via `tools/diff_packages_go.py` /
  `tools/diff_files_ts.py`.

The gate's logic moves into its own Go module, `xdoubleu/diff-coverage`, so
other repos (`check-in` first) can drop Codecov too. Each repo pins it with a
`tool` directive in `go.mod`, so `go tool diff-coverage` is the same version
locally and in CI. Threshold, base ref and include/exclude globs are flags; the
behaviour above is kept, and the Python scripts are deleted once this repo
uses the module.

## Alternatives considered

- **Whole-file thresholds** — misleading, as above.
- **Codecov's `codecov/patch`** — duplicated this gate, and its outages and
  deactivation blocked every merge.
- **octocov** — reads only lcov `SF`/`DA` records, so no branch coverage, and
  computes patch coverage only inside a PR through the GitHub API (100%
  elsewhere): no local gate, and a GitHub outage passes silently.
- **diff-cover** — no Go profile support, skips changed files with no coverage
  data, and folds branches into lines. Replaying a sidebar PR without its tests,
  ours failed AppShell (50% lines, 75% branches) and Sidebar (100% lines, 58%
  branches); diff-cover failed at 29% overall but could not have caught a
  Sidebar-only branch gap.

## Consequences

- A green local run is the CI verdict.
- The project-coverage status and PR comment Codecov offered are gone.
- Changing the gate means a release of `diff-coverage` and a version bump here.
