import { create } from '@bufbuild/protobuf'
import {
  Harness,
  ListReposResponseSchema,
  RepoSchema,
  TaskPriority,
  TaskSchema,
  TaskStatus
} from '@/lib/gen/factory/v1/factory_pb'
import { ListRunnersResponseSchema, RunnerSchema } from '@/lib/gen/factory/v1/runners_pb'
import {
  AgentAccountSchema,
  ListAgentAccountsResponseSchema
} from '@/lib/gen/factory/v1/agent_accounts_pb'
import {
  COLUMNS,
  accountsOf,
  activeRunner,
  actorLabel,
  HARNESSES,
  filterByRepo,
  groupByStatus,
  harnessLabel,
  parseLabels,
  priorityLabel,
  repoLabel,
  reposOf,
  runnerState,
  runnersOf,
  statusLabel
} from '@/lib/factory/board'

describe('board', () => {
  it('lists the six columns in board order', () => {
    expect(COLUMNS.map((c) => c.label)).toEqual([
      'Backlog',
      'Ready',
      'In progress',
      'In review',
      'Needs you',
      'Done'
    ])
  })

  it('labels statuses and priorities, with a fallback for unknown ones', () => {
    expect(statusLabel(TaskStatus.NEEDS_YOU)).toBe('Needs you')
    expect(statusLabel(TaskStatus.UNSPECIFIED)).toBe('Unknown')
    expect(priorityLabel(TaskPriority.P0)).toBe('P0')
    expect(priorityLabel(TaskPriority.P1)).toBe('P1')
    expect(priorityLabel(TaskPriority.P2)).toBe('P2')
    expect(priorityLabel(TaskPriority.UNSPECIFIED)).toBe('')
  })

  it('groups tasks by column, keeping empty columns and the given order', () => {
    const tasks = [
      create(TaskSchema, { id: 'a', status: TaskStatus.DONE }),
      create(TaskSchema, { id: 'b', status: TaskStatus.BACKLOG }),
      create(TaskSchema, { id: 'c', status: TaskStatus.DONE })
    ]
    const groups = groupByStatus(tasks)
    expect(groups).toHaveLength(6)
    expect(groups[0].tasks.map((t) => t.id)).toEqual(['b'])
    expect(groups[1].tasks).toEqual([])
    expect(groups[5].tasks.map((t) => t.id)).toEqual(['a', 'c'])
  })

  it('splits labels on commas, trimming and dropping blanks', () => {
    expect(parseLabels(' bug, ui ,, ')).toEqual(['bug', 'ui'])
    expect(parseLabels('')).toEqual([])
  })

  it('labels harnesses, with a fallback for unknown ones', () => {
    expect(HARNESSES.map((h) => h.label)).toEqual(['Claude Code', 'OpenCode'])
    expect(harnessLabel(Harness.CLAUDE_CODE)).toBe('Claude Code')
    expect(harnessLabel(Harness.OPENCODE)).toBe('OpenCode')
    expect(harnessLabel(Harness.UNSPECIFIED)).toBe('Unknown')
  })

  it('names a repo owner/name', () => {
    expect(repoLabel(create(RepoSchema, { owner: 'octo', name: 'cat' }))).toBe('octo/cat')
  })

  it('filters tasks by repo, keeping all for an empty id', () => {
    const tasks = [
      create(TaskSchema, { id: 'a', repoId: 'r1' }),
      create(TaskSchema, { id: 'b', repoId: 'r2' })
    ]
    expect(filterByRepo(tasks, '')).toBe(tasks)
    expect(filterByRepo(tasks, 'r2').map((t) => t.id)).toEqual(['b'])
    expect(filterByRepo(tasks, 'nope')).toEqual([])
  })

  it('reads the repos of a response, and none before it loads', () => {
    const repo = create(RepoSchema, { id: 'r1' })
    expect(reposOf(create(ListReposResponseSchema, { repos: [repo] }))).toEqual([repo])
    expect(reposOf(undefined)).toEqual([])
  })

  it('reads the accounts of a response, and none before it loads', () => {
    const account = create(AgentAccountSchema, { id: 'a1' })
    expect(accountsOf(create(ListAgentAccountsResponseSchema, { accounts: [account] }))).toEqual([
      account
    ])
    expect(accountsOf(undefined)).toEqual([])
  })

  it('shows an agent actor by its account name and leaves other actors alone', () => {
    const agents = [create(AgentAccountSchema, { userId: 'u-1', name: 'Reviewer' })]
    expect(actorLabel('agent:u-1', agents)).toBe('Reviewer')
    expect(actorLabel('agent:u-9', agents)).toBe('agent:u-9')
    expect(actorLabel('human:u-1', agents)).toBe('human:u-1')
    expect(actorLabel('human:agent:u-1', agents)).toBe('human:agent:u-1')
    expect(actorLabel('agent:u-1', [])).toBe('agent:u-1')
  })

  it('reads the runners of a response, and none before it loads', () => {
    const runner = create(RunnerSchema, { id: 'r1' })
    expect(runnersOf(create(ListRunnersResponseSchema, { runners: [runner] }))).toEqual([runner])
    expect(runnersOf(undefined)).toEqual([])
  })

  it('finds the one runner that is not revoked', () => {
    const revoked = create(RunnerSchema, { id: 'old', revokedAt: '2026-10-01T00:00:00Z' })
    const active = create(RunnerSchema, { id: 'new' })
    expect(activeRunner([revoked, active])).toBe(active)
    expect(activeRunner([revoked])).toBeUndefined()
    expect(activeRunner([])).toBeUndefined()
  })

  it('describes a runner in words', () => {
    const now = Date.parse('2026-11-01T12:00:00Z')
    const paired = { pairedAt: '2026-11-01T10:00:00Z' }

    expect(runnerState(undefined, now)).toEqual({ label: 'No runner', online: false })
    expect(runnerState(create(RunnerSchema, { revokedAt: 'x', ...paired }), now)).toEqual({
      label: 'Revoked',
      online: false
    })
    expect(runnerState(create(RunnerSchema, {}), now)).toEqual({
      label: 'Waiting to pair',
      online: false
    })
    expect(
      runnerState(create(RunnerSchema, { pairingExpiresAt: '2026-11-01T11:00:00Z' }), now)
    ).toEqual({ label: 'Pairing code expired', online: false })
    expect(
      runnerState(create(RunnerSchema, { pairingExpiresAt: '2026-11-01T13:00:00Z' }), now)
    ).toEqual({ label: 'Waiting to pair', online: false })
    expect(
      runnerState(create(RunnerSchema, { ...paired, online: true, lastSeen: 'x' }), now)
    ).toEqual({ label: 'Online', online: true })
    expect(runnerState(create(RunnerSchema, paired), now)).toEqual({
      label: 'Offline',
      online: false
    })
    expect(
      runnerState(create(RunnerSchema, { ...paired, lastSeen: '2026-11-01T11:55:00Z' }), now)
    ).toEqual({ label: 'Offline · seen 5 min ago', online: false })
  })
})
