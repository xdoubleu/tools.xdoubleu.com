# ADR-0040: The board's MCP tools are for agent accounts, which may mark a task Ready

- Status: Accepted; the `AgentToolsOnly` gate is superseded by
  [ADR-0047](adr-0047-first-class-agent-users.md)
- Issues: #2714, #2701
- Affects: `api/internal/platform/factory/mcp/mcp.go`, `api/internal/platform/mcptools/` (`RequireAgent`,
  `AddAgentReadTool`, `AddAgentWriteTool`, `AgentGrants`)

## Context

Agents file and discuss tasks on the factory board, but a person alone decides
that work may start. Originally only a human admin in a browser could move a
task into Ready ([ADR-0039](adr-0039-agent-accounts.md), #2701 decision 5); the
owner later let agents do it too (T-424). The existing tool
registrars gate on per-app access, which admits an admin and refuses an agent,
the opposite of what the board needs.

## Decision

- Twelve tools on `/apps/mcp`: `factory_list_repos`, `factory_list_tasks`,
  `factory_get_task`, `factory_get_job` (read) and `factory_create_task`, `factory_comment_task`,
  `factory_move_task`, `factory_move_tasks`, `factory_update_task` (write), plus the routine tools
  `factory_list_routines` (read), `factory_set_routine`, `factory_run_routine` and `factory_delete_routine`
  (write; [adr-0052](adr-0052-factory-scheduled-routines.md)). A job's account may use them,
  except on the routine its own job runs for.
- `factory_get_task` lists the task's jobs with their outcome (how the job ended, or
  why it failed, was lost or was stopped; stored for every job, truncated).
  `factory_get_job(job_id, tail)` adds the last `tail` log events (default 50, at most
  200). The events have credential values removed, and the owner accepts that code and
  file contents of the job's repo reach agents with factory read access. The full log
  (`ListJobEvents`) stays browser-only. A job's own account reads only its own task's jobs.
- `factory_update_task` edits title, body, priority and labels but never the
  column. It refuses a title or body change once a task is Ready, because a
  person approved that text. `factory_move_tasks` moves many tasks and returns
  only the ids moved and failed. A job's own account can use neither.
- Each tool is gated by the `factory` grant (an admin has it implicitly), write
  tools at the write level. An agent or a person may use them; a service
  identity gets a permission error. `AgentGrants` leaves the `factory_` tools
  out of the tool list of anyone without the grant. A person acts as
  `human:<user id>` with their own role, so an admin may mark tasks Ready and
  manage routines over MCP as in the browser.
- The tools call the services as an agent actor (`agent:<user id>`), which is
  neither a browser nor an admin. `TaskService.Move` lets an agent or an admin, on
  any channel, move a task into Ready and records the actor in a comment.
- `factory_list_tasks` filters by status, repo, label (any case) and `query`,
  a case-insensitive search of titles and bodies run in SQL, so a routine finds
  the task it filed earlier by a stable key (a Sentry permalink) without
  reading every body.
- A new task always starts in Backlog. `factory_list_repos` returns id, owner,
  name and default branch only. List results leave out task bodies.

## Alternatives considered

- Gate on app access like other apps: an admin would pass and the agent fail.
- Check the status in the tool: a second path to forget; the service already owns
  the rule.

## Consequences

- An agent can create tasks, comment and move a task between every column except
  Ready, including out of Ready and into Done. Review of what it did stays with
  the person who merges.
- A person works the board through the web UI, not MCP.

## Revisit when

Agents should see jobs, or edits to a Ready task's text become necessary.
