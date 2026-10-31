import { create } from '@bufbuild/protobuf'
import {
  Harness,
  ListReposResponseSchema,
  RepoSchema,
  TaskPriority,
  TaskSchema,
  TaskStatus
} from '@/lib/gen/factory/v1/factory_pb'
import {
  COLUMNS,
  HARNESSES,
  filterByRepo,
  groupByStatus,
  harnessLabel,
  parseLabels,
  priorityLabel,
  repoLabel,
  reposOf,
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
})
