# ADR-0013: Gate on coverage of changed lines, not whole-file percentages

- Status: Accepted
- Issues: #1301, #1364, #1376, #1632, #1868, T-300
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

## Alternatives considered

- **Whole-file thresholds** — misleading, as above.
- **Codecov's `codecov/patch`** — duplicated this gate, and its outages and
  deactivation blocked every merge.

## Consequences

- A green local run is the CI verdict.
- The project-coverage status and PR comment Codecov offered are gone.
