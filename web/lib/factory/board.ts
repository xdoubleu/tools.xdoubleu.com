import {
  Harness,
  JobStatus,
  TaskPriority,
  TaskStatus,
  type Job,
  type ListReposResponse,
  type Repo,
  type Task
} from '@/lib/gen/factory/v1/factory_pb'
import type {
  AgentAccount,
  ListAgentAccountsResponse
} from '@/lib/gen/factory/v1/agent_accounts_pb'
import type { ListRunnersResponse, Runner } from '@/lib/gen/factory/v1/runners_pb'
import { formatAge } from '@/lib/offline/formatAge'

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

/** The runners in a list response, none while it hasn't loaded. */
export function runnersOf(data: ListRunnersResponse | undefined): Runner[] {
  return data?.runners ?? []
}

/** The runner that isn't revoked, if any: only one can be active. */
export function activeRunner(runners: Runner[]): Runner | undefined {
  return runners.find((r) => r.revokedAt === '')
}

/** A runner as people read it; `now` is epoch milliseconds. */
export function runnerState(
  runner: Runner | undefined,
  now: number
): { label: string; online: boolean } {
  if (!runner) return { label: 'No runner', online: false }
  if (runner.revokedAt !== '') return { label: 'Revoked', online: false }
  if (runner.pairedAt === '') {
    const expired = Date.parse(runner.pairingExpiresAt) <= now
    return { label: expired ? 'Pairing code expired' : 'Waiting to pair', online: false }
  }
  if (runner.online) return { label: 'Online', online: true }
  if (runner.lastSeen === '') return { label: 'Offline', online: false }
  return { label: `Offline · seen ${formatAge(now - Date.parse(runner.lastSeen))}`, online: false }
}

export const RUNNER_DOWNLOAD_PATH = '/downloads/factory-runner-darwin-arm64'

const DEV_RELEASE = 'dev'
const SHORT_RELEASE_LENGTH = 7

/** A release (a commit SHA) as shown to people. */
export function shortRelease(release: string): string {
  return release.slice(0, SHORT_RELEASE_LENGTH)
}

/**
 * Whether the paired runner runs the served release: "current", "pending" (a
 * newer one is served), or "unknown" when either side has no real release.
 */
export function runnerUpdate(
  runner: Runner | undefined,
  latest: string | undefined
): 'current' | 'pending' | 'unknown' {
  const running = runner?.version ?? ''
  if (running === '' || running === DEV_RELEASE || !latest) return 'unknown'
  return running === latest ? 'current' : 'pending'
}

/** How often an open task is refreshed while its job is queued or running. */
export const JOB_POLL_MS = 5_000

const JOB_LABELS = new Map<JobStatus, string>([
  [JobStatus.QUEUED, 'Queued'],
  [JobStatus.RUNNING, 'Running'],
  [JobStatus.SUCCEEDED, 'Succeeded'],
  [JobStatus.FAILED, 'Failed'],
  [JobStatus.CANCELLED, 'Cancelled'],
  [JobStatus.LOST, 'Lost']
])

export function jobStatusLabel(status: JobStatus): string {
  return JOB_LABELS.get(status) ?? 'Unknown'
}

/** Whether the job is still waiting for or on the runner. */
export function jobActive(job: Job | undefined): boolean {
  return job?.status === JobStatus.QUEUED || job?.status === JobStatus.RUNNING
}

const PR_STATES = new Map([
  ['open', 'open'],
  ['merged', 'merged'],
  ['closed', 'closed']
])
const PR_CI = new Map([
  ['pending', 'CI running'],
  ['passing', 'CI passing'],
  ['failing', 'CI failing']
])
const PR_REVIEW = new Map([
  ['approved', 'approved'],
  ['changes_requested', 'changes requested']
])

/** The job's pull request as a short line, "" while it has none or no status yet. */
export function prStatusLine(job: Job | undefined): string {
  const state = PR_STATES.get(job?.prState ?? '')
  if (!job || !state) return ''
  const parts = [`PR ${state}`]
  // CI and review only matter while the PR can still change.
  if (state === 'open') {
    const ci = PR_CI.get(job.prCi)
    const review = PR_REVIEW.get(job.prReview)
    if (ci) parts.push(ci)
    if (review) parts.push(review)
  }
  return parts.join(' · ')
}
