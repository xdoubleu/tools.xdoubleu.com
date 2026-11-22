# Wardcase handoff format, version 1

When a harness stops at its usage limit, wardcase writes one Markdown file for the next harness (`wardcase-handoff.md` in its session directory, `/session/` in the VM). Any tool can read or write it; this file is the contract, not the Go code.

## Layout

```markdown
---
wardcase-handoff: 1
from: claude
to: opencode
base: <commit sha, or empty>
---

# Handoff from claude to opencode

<one paragraph: the git state is the truth, the conversation is context>

## Git state
## Open todos            (omitted when none)
## Conversation so far (compacted)   (omitted when empty)
```

- **Front matter:** `key: value` lines between `---` fences, first in the file. `wardcase-handoff` is the format version; `from`/`to` are harness names; `base` is the commit the run began at. Readers ignore unknown keys and reject a version newer than they know. A change that keeps old readers working stays at the same version.
- **Git state:** fenced blocks titled `Branch`, `Commits since the run began` (only with a base), `Uncommitted changes` and `Diff summary`, each `(none)` when empty.
- **Open todos:** a checklist of the agent's unfinished items, `- [ ]` pending and `- [~]` in progress; completed ones are left out. Each item is cut at 300 bytes.
- **Conversation:** turns as `**User:** …`, `**Agent:** …` and `**Summary of the earlier conversation:** …`, in order, with tool calls and their output left out. A harness's own compaction summary replaces what came before it.

## Size caps

- A turn is cut at 2000 bytes (on a character boundary) and ends with ` …(cut)`.
- The conversation is kept to 30000 bytes by dropping the oldest whole turns, never the newest; when any are dropped it starts with `…(earlier part left out)`.

## Readable sessions

| Harness | Where | Notes |
|---|---|---|
| Claude Code | `projects/*/*.jsonl` under `CLAUDE_CONFIG_DIR` | todos from the last `TodoWrite` |
| OpenCode | `opencode/storage/{session,message,part,todo}` under `XDG_DATA_HOME` | JSON storage only; text parts, no tool calls |

Other harnesses hand over git state alone.
