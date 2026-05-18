import { render, screen, fireEvent } from '@testing-library/react'
import { TaskCard } from '@/components/todos/TaskCard'
import type { Task } from '@/lib/gen/todos/v1/tasks_pb'

function makeTask(overrides: Partial<Task> = {}): Task {
  return ({
    id: 'task-1',
    ownerUserId: 'user-1',
    title: 'Test Task',
    description: '',
    labels: [],
    status: 'open',
    priority: 0,
    sortOrder: 0,
    completedAt: '',
    archivedAt: '',
    dueDate: '',
    deadline: '',
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
    sectionId: '',
    workspaceId: '',
    recurDays: 0,
    recurRule: '',
    links: [],
    subtasks: [],
    subtaskDone: 0,
    subtaskTotal: 0,
    ...overrides,
  }) as Task
}

describe('TaskCard', () => {
  it('renders the task title', () => {
    render(<TaskCard task={makeTask({ title: 'My Task' })} />)
    expect(screen.getByText('My Task')).toBeInTheDocument()
  })

  it('renders the listitem role', () => {
    render(<TaskCard task={makeTask()} />)
    expect(screen.getByRole('listitem')).toBeInTheDocument()
  })

  it('renders priority badge when priority > 0', () => {
    render(<TaskCard task={makeTask({ priority: 1 })} />)
    expect(screen.getByText('P1')).toBeInTheDocument()
  })

  it('renders P2/P3/P4 badges correctly', () => {
    const { rerender } = render(<TaskCard task={makeTask({ priority: 2 })} />)
    expect(screen.getByText('P2')).toBeInTheDocument()
    rerender(<TaskCard task={makeTask({ priority: 3 })} />)
    expect(screen.getByText('P3')).toBeInTheDocument()
    rerender(<TaskCard task={makeTask({ priority: 4 })} />)
    expect(screen.getByText('P4')).toBeInTheDocument()
  })

  it('does not render priority badge when priority is 0', () => {
    render(<TaskCard task={makeTask({ priority: 0 })} />)
    expect(screen.queryByText(/^P\d$/)).not.toBeInTheDocument()
  })

  it('renders labels', () => {
    render(<TaskCard task={makeTask({ labels: ['bug', 'feature'] })} />)
    expect(screen.getByText('bug')).toBeInTheDocument()
    expect(screen.getByText('feature')).toBeInTheDocument()
  })

  it('renders due date in relative format', () => {
    render(<TaskCard task={makeTask({ dueDate: '2024-01-15' })} />)
    expect(screen.getByText('Jan 15')).toBeInTheDocument()
  })

  it('applies line-through for done tasks', () => {
    render(<TaskCard task={makeTask({ status: 'done' })} />)
    const title = screen.getByText('Test Task')
    expect(title.className).toContain('line-through')
  })

  it('renders subtask progress when subtasks exist', () => {
    render(<TaskCard task={makeTask({ subtaskDone: 2, subtaskTotal: 5 })} />)
    expect(screen.getByText('2/5 subtasks')).toBeInTheDocument()
  })

  it('does not render subtask count when total is 0', () => {
    render(<TaskCard task={makeTask({ subtaskDone: 0, subtaskTotal: 0 })} />)
    expect(screen.queryByText(/subtasks/)).not.toBeInTheDocument()
  })

  it('calls onClick when the card is clicked', () => {
    const onClick = jest.fn()
    render(<TaskCard task={makeTask()} onClick={onClick} />)
    fireEvent.click(screen.getByRole('listitem'))
    expect(onClick).toHaveBeenCalledTimes(1)
  })
})
