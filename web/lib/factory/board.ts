import {
  Harness,
  TaskPriority,
  TaskStatus,
  type ListReposResponse,
  type Repo,
  type Task
} from '@/lib/gen/factory/v1/factory_pb'
import type {
  AgentAccount,
  ListAgentAccountsResponse
} from '@/lib/gen/factory/v1/agent_accounts_pb'

export const COLUMNS: { status: TaskStatus; label: string }[] = [
  { status: TaskStatus.BACKLOG, label: 'Backlog' },
  { status: TaskStatus.READY, label: 'Ready' },
  { status: TaskStatus.IN_PROGRESS, label: 'In progress' },
  { status: TaskStatus.IN_REVIEW, label: 'In review' },
  { status: TaskStatus.NEEDS_YOU, label: 'Needs you' },
  { status: TaskStatus.DONE, label: 'Done' }
]

export const PRIORITIES: { priority: TaskPriority; label: string }[] = [
  { priority: TaskPriority.P0, label: 'P0' },
  { priority: TaskPriority.P1, label: 'P1' },
  { priority: TaskPriority.P2, label: 'P2' }
]

export function statusLabel(status: TaskStatus): string {
  return COLUMNS.find((c) => c.status === status)?.label ?? 'Unknown'
}

export function priorityLabel(priority: TaskPriority): string {
  return PRIORITIES.find((p) => p.priority === priority)?.label ?? ''
}

/** Tasks per column, in column order; the server already orders each by recency. */
export function groupByStatus(
  tasks: Task[]
): { status: TaskStatus; label: string; tasks: Task[] }[] {
  return COLUMNS.map((c) => ({ ...c, tasks: tasks.filter((t) => t.status === c.status) }))
}

/** Splits a comma-separated label field into trimmed, non-empty labels. */
export function parseLabels(input: string): string[] {
  return input
    .split(',')
    .map((l) => l.trim())
    .filter((l) => l !== '')
}

export const HARNESSES: { harness: Harness; label: string }[] = [
  { harness: Harness.CLAUDE_CODE, label: 'Claude Code' },
  { harness: Harness.OPENCODE, label: 'OpenCode' }
]

export function harnessLabel(harness: Harness): string {
  return HARNESSES.find((h) => h.harness === harness)?.label ?? 'Unknown'
}

export function repoLabel(repo: Repo): string {
  return `${repo.owner}/${repo.name}`
}

/** Tasks of one repo; an empty id keeps them all. */
export function filterByRepo(tasks: Task[], repoId: string): Task[] {
  return repoId === '' ? tasks : tasks.filter((t) => t.repoId === repoId)
}

/** The repos in a list response, none while it hasn't loaded. */
export function reposOf(data: ListReposResponse | undefined): Repo[] {
  return data?.repos ?? []
}

/** The accounts in a list response, none while it hasn't loaded. */
export function accountsOf(data: ListAgentAccountsResponse | undefined): AgentAccount[] {
  return data?.accounts ?? []
}

const AGENT_ACTOR_PREFIX = 'agent:'

/** An actor ("<kind>:<id>") as shown to people: an agent by its account name. */
export function actorLabel(actor: string, agents: AgentAccount[]): string {
  if (!actor.startsWith(AGENT_ACTOR_PREFIX)) return actor
  const userId = actor.slice(AGENT_ACTOR_PREFIX.length)
  return agents.find((a) => a.userId === userId)?.name ?? actor
}
