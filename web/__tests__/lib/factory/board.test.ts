import { create } from '@bufbuild/protobuf'
import { TaskPriority, TaskSchema, TaskStatus } from '@/lib/gen/factory/v1/factory_pb'
import {
  COLUMNS,
  groupByStatus,
  parseLabels,
  priorityLabel,
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
})
