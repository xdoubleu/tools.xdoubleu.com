# docs/

The "why" behind the codebase. The code is the spec for current behavior; nothing here describes how a subsystem works today.

- **ADR** — a decision, the alternatives considered, and its consequences.
- **Convention** — a rule contributors and agents follow.
- **Spec** — a record of something outside the repo's code (e.g. a claude.ai routine's prompt text).

Create an ADR with `make adr/new name=<slug> [label=<index label>]` (next free number vs `origin/main`, index line added; the index is one ADR per line so concurrent branches don't conflict). Start a convention from `TEMPLATE-convention.md`. Keep it short ([convention-concise-docs-and-comments](convention-concise-docs-and-comments.md)). The index is the "Docs" section of the root [`AGENTS.md`](../AGENTS.md); register new documents there. Each document lists its own issues.
