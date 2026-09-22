# Product Skills (tools.xdoubleu.com)

This directory is the long-term home for **product** skills: agent-loadable
guides for doing things *in the running product* (authoring a learning
path, managing the books library, planning a journey — whatever comes
later), driven by the product's own `/apps/mcp` tools.

It is deliberately separate from the repo's **dev-workflow** skills in
`.claude/skills/` (dependabot-triage, monitoring-sweep, postmortem, etc.),
which automate working on this codebase. A product skill is about using a
feature; a dev skill is about maintaining the repo. Keep the two disjoint.

## What lives here

Each subdirectory is one skill with an Agent-Skills-format `SKILL.md` at
its root:

```
skills/
├── README.md
└── learning-paths/
    └── SKILL.md
```

## Skills

- **`learning-paths/`** — author, refine, and track learning paths on
  tools.xdoubleu.com via the `learningpaths_*` MCP write/read tools.

## Using these with an agent

### Attach the skill to ChatGPT-style use

Agent-skills work by attaching a folder containing a `SKILL.md`. Point the
agent at **just the skill's folder** — e.g. `skills/learning-paths/` — not
at the whole repo. The repo has plenty of dev-op automation (`.claude/`,
`Makefile`, source) that a broad pointer would drag in and confuse an
end-user agent with; a single-folder attachment keeps the skill isolated.

### Wire the MCP connector

A product skill is guidance text; it *executes* by calling the
tools.xdoubleu.com MCP tools, so the agent needs the **`tools-apps` MCP
server** connected (OAuth 2.1; the api is both resource server and
authorization server, so any MCP-capable client can connect with
discovery + PKCE — no per-client server config). Claude Code connects with:

```bash
claude mcp add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp
```

Other harnesses use their own mechanism for the same endpoint (e.g.
OpenCode's `mcp.servers` block in `opencode.json`).

## Rules for authoring a new product skill

- **Self-contained.** A hosted agent can't walk this repo, so each
  `SKILL.md` must be complete on its own: inline the MCP tool names and
  argument shapes and any rubric — never reference repo-relative paths or
  "see `web/…`".
- **Laser-specific frontmatter `description`.** Match the natural end-user
  triggers ("create a learning path", "make me a curriculum") and name the
  exact MCP tools, so the skill never misfires on a dev task.
- **Confirm before mutating.** These skills drive live write tools against
  real user data; make eliciting-then-confirming a hard workflow step.
- **Match the shipped shape.** The MCP tool surface and the data model are
  the source of truth (`api/apps/learningpaths/mcp.go`, the
  `learningpaths.v1` proto). If the product changes, update the skill in
  the same change.