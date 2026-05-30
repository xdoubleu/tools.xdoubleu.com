import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { TaskCard } from '@/components/todos/TaskCard'
import { TaskSchema, SubtaskSchema } from '@/lib/gen/todos/v1/tasks_pb'

const mockToggleSubtask = jest.fn().mockResolvedValue({})

jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({
    toggleSubtask: mockToggleSubtask
  })
}))

function makeTask(overrides: Parameters<typeof create<typeof TaskSchema>>[1] = {}) {
  return create(TaskSchema, {
    id: 'task-1',
    ownerUserId: 'user-1',
    title: 'Test Task',
    status: 'open',
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
    ...overrides
  })
}

function makeSubtask(overrides: Parameters<typeof create<typeof SubtaskSchema>>[1] = {}) {
  return create(SubtaskSchema, {
    id: 'sub-1',
    taskId: 'task-1',
    title: 'A subtask',
    done: false,
    parentSubtaskId: '',
    ...overrides
  })
}

describe('TaskCard', () => {
  beforeEach(() => mockToggleSubtask.mockClear())

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

  it('renders top-level subtask titles', () => {
    const subtasks = [
      makeSubtask({ id: 'sub-1', title: 'First subtask', parentSubtaskId: '' }),
      makeSubtask({ id: 'sub-2', title: 'Second subtask', parentSubtaskId: '' })
    ]
    render(<TaskCard task={makeTask({ subtasks })} />)
    expect(screen.getByText('First subtask')).toBeInTheDocument()
    expect(screen.getByText('Second subtask')).toBeInTheDocument()
  })

  it('does not render subtask list when no subtasks', () => {
    render(<TaskCard task={makeTask({ subtasks: [] })} />)
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('calls toggleSubtask and onChanged when subtask checkbox clicked', async () => {
    const onChanged = jest.fn()
    const subtasks = [makeSubtask({ id: 'sub-1', title: 'Do thing', parentSubtaskId: '' })]
    render(<TaskCard task={makeTask({ subtasks })} onChanged={onChanged} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark done' }))

    await waitFor(() => {
      expect(mockToggleSubtask).toHaveBeenCalledWith({ subtaskId: 'sub-1', taskId: 'task-1' })
      expect(onChanged).toHaveBeenCalled()
    })
  })

  it('calls onClick when the card is clicked', () => {
    const onClick = jest.fn()
    render(<TaskCard task={makeTask()} onClick={onClick} />)
    fireEvent.click(screen.getByRole('listitem'))
    expect(onClick).toHaveBeenCalledTimes(1)
  })
})
