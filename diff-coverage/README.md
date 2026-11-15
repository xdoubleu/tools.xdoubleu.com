# diff-coverage

Gate the coverage of **changed lines**, from Go coverage profiles and lcov
reports. No SaaS, no Python.

```bash
go run github.com/xdoubleu/diff-coverage@v0.1.0 go coverage.out
go run github.com/xdoubleu/diff-coverage@v0.1.0 lcov coverage/lcov.info
go run github.com/xdoubleu/diff-coverage@v0.1.0 merge -o merged.out a.out b.out
```

To pin the version, add `tool github.com/xdoubleu/diff-coverage` to `go.mod`
and run `go tool diff-coverage`.

Run it in the directory the report's paths are relative to (the Go module, or
the web project). Changed lines come from `git diff` against `--base`;
untracked files count in full.

| Flag | Default |
|---|---|
| `--threshold N` | `80` |
| `--base REF` | merge-base of `origin/main` and `HEAD` |
| `--include GLOB` | `**/*.go`; `**/*.ts`, `**/*.tsx` for lcov |
| `--exclude GLOB` | `**/*_mock.go`, `**/gen/**`; `*.d.ts` and test files for lcov |

`--include` and `--exclude` repeat and replace their defaults; `*_test.go` is
always skipped. A glob without a slash matches at any depth; `**` crosses
directories.

- **go**: a line is covered when every block touching it was hit. A
  conservative figure (block start/end lines only) is printed beside the gate.
  The module prefix comes from `go.mod`. Files with no instrumented changed
  line are skipped.
- **lcov**: line and branch coverage must each meet the threshold. A changed
  file with no coverage record fails.
- **merge**: max count per block across profiles, then a per-directory summary;
  `-o` writes the merged profile.

Exit status is 1 when the gate fails.
