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
  { status: TaskStatus.TESTING, label: 'Testing' },
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

/**
 * Tasks per column, in column order; the server already orders each by recency.
 * Done tasks stay off the board unless `withDone`.
 */
export function groupByStatus(
  tasks: Task[],
  withDone = false
): { status: TaskStatus; label: string; tasks: Task[] }[] {
  return COLUMNS.filter((c) => withDone || c.status !== TaskStatus.DONE).map((c) => ({
    ...c,
    tasks: tasks.filter((t) => t.status === c.status)
  }))
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

/** The first 8 characters of a task's uuid, enough to tell tasks apart on the board. */
export function shortTaskId(id: string): string {
  return id.slice(0, 8)
}

/** How a dragged card announces its task to the board's drop targets. */
export const TASK_DATA_TYPE = 'text/plain'

/** Tasks of one repo; an empty id keeps them all. */
export function filterByRepo(tasks: Task[], repoId: string): Task[] {
  return repoId === '' ? tasks : tasks.filter((t) => t.repoId === repoId)
}

/**
 * The product a task belongs to, from a fixed set so the board can focus on one
 * app at a time. "platform" covers cross-cutting work (auth, deploy, runners).
 * Lower-case; the label check ignores case. Kept alongside free-form labels.
 */
export const PRODUCT_LABELS: string[] = [
  'books',
  'games',
  'feeds',
  'watchparty',
  'recipes',
  'mealplans',
  'shoppinglist',
  'dashboard',
  'trains',
  'learningpaths',
  'movies',
  'podcasts',
  'taste',
  'factory',
  'platform'
]

/** Whether a label is one of the predefined products, so it renders distinctly. */
export function isProductLabel(label: string): boolean {
  const l = label.trim().toLowerCase()
  return PRODUCT_LABELS.some((p) => p === l)
}

const normLabel = (label: string) => label.trim().toLowerCase()

/** The tasks that carry every one of the labels (AND, ignoring case); none keeps them all. */
export function filterByLabels(tasks: Task[], labels: string[]): Task[] {
  const wanted = labels.map(normLabel).filter((l) => l !== '')
  if (wanted.length === 0) return tasks
  return tasks.filter((t) => {
    const have = t.labels.map(normLabel)
    return wanted.every((l) => have.includes(l))
  })
}

/** `refine-feature` tags an epic's parent and sub-tasks `epic:<slug>`. */
const EPIC_PREFIX = 'epic:'

/** Whether a label names an epic. */
export function isEpicLabel(label: string): boolean {
  return normLabel(label).startsWith(EPIC_PREFIX)
}

/** The epic's slug, "" for a label that isn't an epic. */
export function epicName(label: string): string {
  return isEpicLabel(label) ? label.trim().slice(EPIC_PREFIX.length) : ''
}

/** The plain `epic` label that marks an epic's parent task. */
const EPIC_PARENT_LABEL = 'epic'

/** Whether a task is an epic's parent, not one of its sub-tasks. */
export function isEpicParent(task: Task): boolean {
  return task.labels.some((l) => normLabel(l) === EPIC_PARENT_LABEL)
}

/** How many of an epic's sub-tasks (every other task with its `epic:<slug>` label) are Done. */
export function epicProgress(epicLabel: string, tasks: Task[]): { done: number; total: number } {
  const children = filterByLabels(tasks, [epicLabel]).filter((t) => !isEpicParent(t))
  return {
    done: children.filter((t) => t.status === TaskStatus.DONE).length,
    total: children.length
  }
}

/** The parent task of an epic label, if there is one. */
export function epicParent(epicLabel: string, tasks: Task[]): Task | undefined {
  return filterByLabels(tasks, [epicLabel]).find((t) => isEpicParent(t))
}

/**
 * Whether the epic can be closed: its parent is open and it has sub-tasks, all Done.
 * `label` picks which of the parent's `epic:<slug>` labels counts; the first by default.
 */
export function canCloseEpic(
  parent: Task,
  tasks: Task[],
  label: string | undefined = parent.labels.find(isEpicLabel)
): boolean {
  if (label === undefined || parent.status === TaskStatus.DONE) return false
  const { done, total } = epicProgress(label, tasks)
  return total > 0 && done === total
}

/** The filter labels with one group's pick swapped: the group's labels go, `next` ("" for none) comes. */
export function withGroupLabel(
  labels: string[],
  inGroup: (label: string) => boolean,
  next: string
): string[] {
  const rest = labels.filter((l) => !inGroup(l))
  return next === '' ? rest : [...rest, next]
}

/** The filter labels with `label` added (once, ignoring case) or removed. */
export function toggleLabel(labels: string[], label: string, on: boolean): string[] {
  const rest = labels.filter((l) => normLabel(l) !== normLabel(label))
  return on ? [...rest, label] : rest
}

/** The distinct labels across tasks, sorted. */
export function distinctLabels(tasks: Task[]): string[] {
  return [...new Set(tasks.flatMap((t) => t.labels))].sort()
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

export const RUNNER_DOWNLOAD_PATH = '/downloads/factory-runner.dmg'

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

/** The pull request status a Job or a Task carries. */
type PullRequestFields = Pick<Job, 'prState' | 'prCi' | 'prReview'>

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
export function prStatusLine(job: PullRequestFields | undefined): string {
  if (!job) return ''
  const state = PR_STATES.get(job.prState)
  if (!state) return ''
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

/** What "Reply: fix CI" sends. */
export const FIX_CI_REPLY =
  'CI is red on the pull request. Find out why the checks fail, fix it, and push.'

/** Whether the job's pull request is open with failing checks, so a Reply can fix it. */
export function canFixCi(job: PullRequestFields | undefined): boolean {
  return job?.prState === 'open' && job.prCi === 'failing'
}

/** What a board card says about the task's pull request, most pressing first. */
export function prBadge(
  task: PullRequestFields
): { label: string; variant: 'danger' | 'warn' | 'success' | 'secondary' } | undefined {
  switch (task.prState) {
    case 'merged':
      return { label: 'PR merged', variant: 'secondary' }
    case 'closed':
      return { label: 'PR closed', variant: 'secondary' }
    case 'open':
      break
    default:
      return undefined
  }
  if (task.prCi === 'failing') return { label: 'CI failing', variant: 'danger' }
  if (task.prReview === 'changes_requested') {
    return { label: 'Changes requested', variant: 'warn' }
  }
  if (task.prCi === 'pending') return { label: 'CI running', variant: 'warn' }
  if (task.prReview === 'approved') return { label: 'Approved', variant: 'success' }
  if (task.prCi === 'passing') return { label: 'CI passing', variant: 'success' }
  return { label: 'PR open', variant: 'secondary' }
}

/** A reset time as "Fri 7 Nov, 14:30" in the viewer's zone; "" for an empty or bad one. */
export function formatLimitTime(iso: string, timeZone?: string, locale?: string): string {
  const ms = Date.parse(iso)
  if (Number.isNaN(ms)) return ''
  return new Intl.DateTimeFormat(locale, {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
    timeZone
  }).format(ms)
}

/**
 * The board notice while the Claude usage limit is still to lift ("Limited until Fri
 * 7 Nov, 14:30"); "" when there is none or it has already lifted.
 */
export function limitNotice(
  until: string,
  now: number,
  timeZone?: string,
  locale?: string
): string {
  const ms = Date.parse(until)
  if (!until || Number.isNaN(ms) || ms <= now) return ''
  return `Limited until ${formatLimitTime(until, timeZone, locale)}`
}

/** Why a job stopped, when the Claude usage limit did it; "" otherwise. */
export function jobLimitNote(job: Pick<Job, 'limited' | 'limitResetsAt'>): string {
  if (!job.limited) return ''
  const when = formatLimitTime(job.limitResetsAt)
  return when ? `stopped by the Claude limit, resets ${when}` : 'stopped by the Claude limit'
}

/** What ran a job, "OpenCode · vendor/model"; "" while the server didn't say. */
export function jobRunner(job: Pick<Job, 'harness' | 'model'>): string {
  if (job.harness === Harness.UNSPECIFIED) return ''
  const label = harnessLabel(job.harness)
  return job.model ? `${label} · ${job.model}` : label
}

/**
 * Whether a Ready task of a repo with auto pickup is waiting for a human to approve
 * its text: it was last edited by an agent, or is from before approvals existed.
 */
export function awaitingPickupApproval(task: Task, repos: Repo[]): boolean {
  if (task.status !== TaskStatus.READY || task.pickupApproved) return false
  return repos.find((r) => r.id === task.repoId)?.autoPickup === true
}
