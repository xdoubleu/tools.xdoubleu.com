# Convention: concise docs and comments

- Enforced by: `make lint/docs` (CI `docs-lint`, `finish-task`, Claude `PostToolUse` hook, OpenCode `repo-guard` idle nudge) for budgets and long comment blocks; review for the rest
- Issues: #1850

## Rule

Every sentence in a doc or comment costs every future reader, human or agent, and agents pay it on every session. Write less:

- **Say it once.** Each fact has one home. Link to it rather than restating it; don't repeat a rule in `AGENTS.md`, `CLAUDE.md`, a skill and a doc.
- **Don't overexplain.** State the rule or the non-obvious *why*, not the reasoning trail, alternatives considered, or examples that restate the rule.
- **No comment that restates the code.** Comment only what the code can't say: a hidden constraint, a surprising invariant, a workaround's cause.
- **Current behavior only.** Never reference removed code or frame a landed change as pending. If history explains the code, phrase it to stay true ("replicating what X used to provide").
- **No provenance in comments.** Issue numbers, "added because…", and incident narratives belong in the commit, PR, or a `docs/` ADR, not inline.
- Prefer a short list over paragraphs.

## Why

The instruction files and skills grew by accretion — each incident appended another paragraph, often restating one that already existed elsewhere. Duplicates drift apart, and long prose buries the rules that matter.
