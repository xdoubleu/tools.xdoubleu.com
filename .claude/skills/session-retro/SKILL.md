---
name: session-retro
description: Analyze the current session's own tool calls, retries, and CI runs for concrete inefficiencies (redundant reads, avoidable back-and-forth, a missing or under-triggered skill/MCP tool, a doc gap, an inherited workaround rule that's gone stale) and, only when something real turns up, verify the actual root cause and ship the smallest fix as its own tracking issue and independent PR. Always run as the mandatory last step of finish-task; also use standalone when the user asks to "reflect on this session", "run a retro", or "how could this have been done more efficiently".
---

# Session Retro

Reflects on the work just completed in this session and looks for concrete
ways future sessions could do the same kind of work with fewer tool calls,
less token usage, and less CI back-and-forth — then ships any real finding
as its own tracking issue and independent PR. The analysis itself is
mandatory every time a work item finishes; only findings backed by
something that actually happened this session turn into an issue/PR.

## What counts as a "concrete" finding

Only flag something you can point to in *this session's own history* — a
specific tool call, a specific retry, a specific CI failure. Never propose a
change on pure speculation ("this would probably help") or because a
hypothetical future task might benefit. If nothing concrete happened, say so
and stop — an empty retro is a normal, healthy outcome, not a failure to
find something.

## What to look for

Review the session's own tool-call and commit history (not the wider
codebase, not other sessions) for these patterns:

1. **Redundant or serial tool calls that should've been fewer/parallel**
   - The same file `Read` more than once, or read in full when a
     `grep`/`ast-grep` search would have found the one relevant section.
   - Several independent Bash/Read/Grep calls issued one at a time across
     separate turns when CLAUDE.md already says to batch independent calls
     in parallel.
   - A multi-step `gh`/`git` sequence that recurs across sessions and isn't
     yet a Make/npm target or script.

2. **Token-heavy reads**
   - A large file read start-to-finish to find one function/constant — an
     `ast-grep`/`grep` pass first would have scoped it.
   - `api/gen/`, `web/lib/gen/`, or a mocks directory read directly instead
     of the `.proto`/source interface CLAUDE.md says to prefer.
   - Verbose command output (e.g. an unfiltered `git log`, a `gh pr checks`
     polled before it had settled) that could've been piped/filtered.

3. **Avoidable CI back-and-forth**
   - Did the PR need more than one push to go green? For each red run, what
     actually failed, and would a local command (`make lint`, `make
     test/cov/report`, `npm run build`) have caught it before pushing?

4. **Missing or under-triggered skills / MCP tools**
   - A procedure was reconstructed from CLAUDE.md prose or memory that
     matches an existing skill's territory, but the skill wasn't invoked —
     is its `description` too narrow or ambiguous to have matched this
     phrasing?
   - A production-data question needed manual multi-step investigation
     (reading source, cross-referencing IDs) that a new or corrected MCP
     tool would answer in one call — see root CLAUDE.md's "MCP coverage
     gaps" convention, which this generalizes beyond just MCP tools.
   - A multi-step procedure with no skill at all that recurred this session
     (e.g. run more than once) or that CLAUDE.md itself documents as a
     repeated gotcha.

5. **Doc gaps**
   - A convention, gotcha, or ordering rule had to be rediscovered this
     session (by trial and error, or by asking) that isn't written down
     anywhere it would've been found first.

6. **Inherited workaround rules worth re-verifying**
   - CLAUDE.md, a skill, or a code comment states an "always do X in this
     order" / "always avoid Y" rule that exists to work around some other
     tool's behavior. Before treating that rule as settled, check whether
     it's still *true* — has the underlying tool/config since changed to
     make the workaround unnecessary? A rule that was correct when written
     but never re-verified against the current config is exactly the stale
     claim CLAUDE.md's own "Comments must describe current behavior, not
     history" rule warns about — test it (run the actual commands, read the
     actual config) rather than propagating it forward unverified. Example
     from this repo: a documented rule said `make proto/generate` must run
     *after* `make lint/fix`, because `gci`/`golangci-lint`/prettier would
     supposedly reformat generated files and make CI's proto-staleness check
     fail. Running a full `make lint/fix` on a clean tree and checking `git
     status api/gen` afterward showed zero diff — `--skip-generated`, the
     `formatters.exclusions.paths` config, `.prettierignore`, and eslint's
     `ignores` already fully exclude generated code from every lint/fix
     tool. The ordering rule (and the skill built around it) was deleted
     entirely rather than carried forward, since the constraint it worked
     around didn't exist anymore.

## Steps

1. **Scan this session's own history** — the tool calls actually made, the
   commits on this branch, and (if a PR was opened) `gh pr checks`/`gh run
   list` for that PR — against the six categories above.
2. **Decide if anything is concrete enough to act on.** A single minor
   inefficiency with no clear fix isn't worth an issue. A repeated pattern,
   a CI failure with an obvious local check that would've caught it, or a
   skill/tool gap that clearly would have shortened this exact session —
   those are.
3. **If nothing concrete: report a one-line "no retro findings this
   session" and stop.** Do not open an issue or PR for a null result.
4. **If something concrete: verify the actual root cause before picking a
   fix.** Don't accept an existing rule's stated reason at face value, and
   don't encode a workaround as a new procedure/skill/doc rule without first
   checking — empirically, if you can (run the command, read the config) —
   whether a tool/config change could eliminate the problem outright. A fix
   that removes the need for a rule always beats a fix that documents the
   rule more thoroughly or automates following it. Once the root cause is
   confirmed, pick the smallest fix that addresses it — a new/edited skill,
   a Make/npm target, a lint rule, a CLAUDE.md correction, an MCP tool
   addition/fix. Don't bundle unrelated findings into one fix if they'd
   naturally ship as separate PRs (same judgment call `issue-triage` uses
   for splitting issues).
5. **Ship each fix exactly like any other task, in its own lane — never
   stacked on the work item's own branch/PR**:
   - Use `start-task` for a completely fresh worktree off `main` and a new
     tracking issue (type `chore`, describing the inefficiency observed and
     the fix) — not a comment tacked onto the original issue.
   - Make the change.
   - Use `finish-task` to ship it — lint/coverage/build as applicable, PR
     opened and referencing the new issue, CI watched to green.
     `finish-task`'s own auto-merge rule applies normally (a skill/CLAUDE.md/
     tooling edit means no auto-merge, wait for review).
6. **Report back**: what was scanned, what (if anything) was found, and the
   issue/PR link(s) for anything shipped.

## Notes

- This is the mandatory last step of `finish-task` — run it every time a
  work item finishes, not just when something feels obviously wasteful.
  Most runs should find nothing; that's expected, not a sign to lower the
  bar for what counts as "concrete."
- Never fold a retro fix into the work item's own PR, even a tiny one-line
  CLAUDE.md tweak — a separate issue/PR keeps the (possibly already-merged
  or auto-merging) original PR reviewable on its own terms and gives the
  retro fix its own CI run.
- If a retro finding is about `start-task`/`finish-task`/`session-retro`
  itself, edit those skill files directly — the same rules (fresh worktree,
  own issue, own PR) apply to editing a skill as to any other tooling
  change.
