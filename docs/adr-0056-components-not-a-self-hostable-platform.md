# ADR-0056: Reusable components, not a self-hostable platform

- Status: Accepted
- Issues: epic T-741; closes T-224, T-247, T-248, T-249, T-250
- Affects: `docs/adr-0046-platform-respike.md`, `wardcase/`, `factory-runner/`, `api/internal/platform/`, `.claude/`
- Supersedes: [adr-0046](adr-0046-platform-respike.md)'s self-hosting, enabled-apps and private-infra decisions

## Context

ADR-0046 aimed for anyone to self-host the apps: a Compose file, a Helm chart, an "enabled apps" switch and a private infra repo. The owner changed course. tools.xdoubleu.com is the owner's own AI-native platform: the apps the owner needs, where anyone with the right grants asks their agent to add one. It is a proof of concept. What others should be able to take are the components it is built from, each usable alone and swappable here, ideally as inspiration for existing ADEs and harnesses.

## Decision

- **No self-host packaging** of the platform: no Compose file, chart or app switch for others. Server config stays beside the code. ADR-0046's code tiers, migration isolation and Kamal deploy stay.
- **A component** is useful without this platform, has one small contract (a CLI, a Go API, a protocol or a file format), holds no app domain and never imports the monorepo. The monorepo consumes it as an outsider would.
- **One repo per component**, under `github.com/xdoubleu/<name>`, licensed **Apache-2.0**. The platform repo stays GPL-3.0. Modules still inside the monorepo keep their `tools.xdoubleu.com/...` path until they move.
- **When to extract:** large components once their contract is stable; small helpers only with a second consumer ([adr-0009](adr-0009-sentrytools-extracted-module.md) shows the cost of one).
- **Hexagonal apps:** Connect handlers and MCP tools are two presentation adapters over the same app services, with no logic of their own. MCP-from-Connect is therefore not a component.
- **Layers:**
  - The factory becomes a standalone local app that owns the board and jobs. An optional server syncs it and allows remote control; tools.xdoubleu.com runs one. It is the layer an ADE such as Orca replaces.
  - wardcase never depends on the factory. The factory uses wardcase's Go API, and an ADE uses its CLI.
  - wardcase owns isolation: every session it starts runs in its own worktree or clone, so sessions never collide. When the harness exits, it reports work left in that worktree.
- **Skills are generic.** No skill is specific to this repo. Each reads repo specifics (commands, labels, MCP tools) from a config file and lives in the owner's marketplace. The doc-budget check moves there as well. `ready-issues-sweep` and `subissue-sweep` retire with the GitHub-issue mode.

| Candidate | Decision |
|---|---|
| wardcase | extract |
| factory (board, runner, optional server) | extract as a standalone app (T-345) |
| diff-coverage, marketplace skills | already external |
| agent identity (OAuth 2.1 AS, agent accounts, delegation, grants) | extract after decoupling from the users model and factory protos |
| selfupdate | keep until a second consumer |
| MCP tools over Connect, observability tools, kit helpers, web offline layer, kobo-gateway | keep |

## Consequences

- The self-hosting tasks are closed. Zitadel no longer ships in a Compose file or chart ([adr-0051](adr-0051-zitadel-for-people-sign-in.md)).
- The worktree edit guard, the unshipped-work `Stop` hook and `.opencode/plugins/repo-guard/` shrink as wardcase and the generic skills take over ([adr-0014](adr-0014-start-finish-task-enforcement.md)).
- Each extraction gets its own task under T-741, with its contract named.

## Revisit when

Someone wants to run the whole platform rather than a component.
