import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { TASK_DATA_TYPE } from '@/lib/factory/board'
import TaskCard from '@/components/factory/TaskCard'
import { RepoSchema, TaskPriority, TaskSchema, TaskStatus } from '@/lib/gen/factory/v1/factory_pb'

const mockMove = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryActions: () => ({ move: mockMove })
}))

const task = create(TaskSchema, {
  id: 't-1',
  title: 'Fix login',
  status: TaskStatus.BACKLOG,
  priority: TaskPriority.P0,
  labels: ['bug'],
  commentCount: 2
})

const props = {
  task,
  onOpen: vi.fn(),
  onDragStart: vi.fn(),
  onDragEnd: vi.fn()
}

beforeEach(() => {
  vi.clearAllMocks()
  mockMove.mockResolvedValue(undefined)
})

describe('TaskCard', () => {
  it('shows the short task id', () => {
    const long = create(TaskSchema, { ...task, id: '12345678-aaaa-bbbb-cccc-ddddeeeeffff' })
    render(<TaskCard {...props} task={long} />)
    expect(screen.getByTestId('task-id')).toHaveTextContent('12345678')
  })

  it('shows title, priority, labels and comment count', () => {
    render(<TaskCard {...props} />)
    expect(screen.getByText('Fix login')).toBeInTheDocument()
    expect(screen.getByText('P0')).toBeInTheDocument()
    expect(screen.getByText('bug')).toBeInTheDocument()
    expect(screen.getByText('2 comments')).toBeInTheDocument()
  })

  it('offers Close epic only when the board says the epic can be closed', () => {
    const onCloseEpic = vi.fn()
    const { rerender } = render(<TaskCard {...props} />)
    expect(screen.queryByRole('button', { name: 'Close epic' })).not.toBeInTheDocument()

    rerender(<TaskCard {...props} onCloseEpic={onCloseEpic} />)
    fireEvent.click(screen.getByRole('button', { name: 'Close epic' }))
    expect(onCloseEpic).toHaveBeenCalledTimes(1)
  })

  it('shows where the pull request stands, only when there is one', () => {
    const { rerender } = render(<TaskCard {...props} />)
    expect(screen.queryByTestId('pr-badge')).not.toBeInTheDocument()

    const failing = create(TaskSchema, { ...task, prState: 'open', prCi: 'failing' })
    rerender(<TaskCard {...props} task={failing} />)
    expect(screen.getByTestId('pr-badge')).toHaveTextContent('CI failing')
  })

  it('links the pull request on GitHub, only when there is one', () => {
    const { rerender } = render(<TaskCard {...props} />)
    expect(screen.queryByTestId('pr-link')).not.toBeInTheDocument()

    const withPr = create(TaskSchema, { ...task, prUrl: 'https://github.com/o/n/pull/7' })
    rerender(<TaskCard {...props} task={withPr} />)
    expect(screen.getByTestId('pr-link')).toHaveAttribute('href', 'https://github.com/o/n/pull/7')
    expect(screen.getByTestId('pr-link')).toHaveTextContent('#7')
  })

  it('says when a Ready task is waiting for its text to be approved', () => {
    const { rerender } = render(<TaskCard {...props} />)
    expect(screen.queryByTestId('approval-hint')).not.toBeInTheDocument()

    rerender(<TaskCard {...props} awaitingApproval />)
    expect(screen.getByTestId('approval-hint')).toHaveTextContent('Not approved for auto pickup')
  })

  it('omits the comment count when there are no comments', () => {
    render(<TaskCard {...props} task={create(TaskSchema, { ...task, commentCount: 0 })} />)
    expect(screen.queryByText(/comments/)).not.toBeInTheDocument()
  })

  it('shows why a Ready task was not picked up, and retries on a fresh Ready move', async () => {
    const blocked = create(TaskSchema, {
      ...task,
      status: TaskStatus.READY,
      pickupBlockReason: 'OpenCode needs an OpenRouter model'
    })
    const { rerender } = render(<TaskCard {...props} />)
    expect(screen.queryByTestId('pickup-block')).not.toBeInTheDocument()

    rerender(<TaskCard {...props} task={blocked} />)
    expect(screen.getByTestId('pickup-block')).toHaveTextContent(
      'OpenCode needs an OpenRouter model'
    )

    fireEvent.click(screen.getByRole('button', { name: 'Retry pickup' }))
    await waitFor(() => expect(mockMove).toHaveBeenCalledWith('t-1', TaskStatus.READY))
  })

  it('says so when retrying pickup fails', async () => {
    mockMove.mockRejectedValueOnce(new Error('nope'))
    const blocked = create(TaskSchema, {
      ...task,
      status: TaskStatus.READY,
      pickupBlockReason: 'OpenCode needs an OpenRouter model'
    })
    render(<TaskCard {...props} task={blocked} />)
    fireEvent.click(screen.getByRole('button', { name: 'Retry pickup' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/Couldn.t retry pickup/)
  })

  it('marks predefined product labels distinctly from free-form ones', () => {
    const mixed = create(TaskSchema, { ...task, labels: ['books', 'bug'] })
    render(<TaskCard {...props} task={mixed} />)
    expect(screen.getByTestId('product-label')).toHaveTextContent('books')
    expect(screen.getAllByText('bug').length).toBeGreaterThan(0)
  })

  it('opens the task from its title', () => {
    const onOpen = vi.fn()
    render(<TaskCard {...props} onOpen={onOpen} />)
    fireEvent.click(screen.getByRole('button', { name: 'Fix login' }))
    expect(onOpen).toHaveBeenCalledWith('t-1')
  })

  it('is draggable and announces its task on drag start', () => {
    render(<TaskCard {...props} />)
    expect(screen.getByText('Fix login').closest('[draggable]')).not.toBeNull()
    const dt = { setData: vi.fn(), effectAllowed: '' }
    const card = screen.getByText('Fix login').closest('div[draggable]')
    if (!card) throw new Error('no draggable card')
    fireEvent.dragStart(card, { dataTransfer: dt })
    expect(props.onDragStart).toHaveBeenCalledWith(expect.anything(), task)
  })

  it('dims the card while it is being dragged', () => {
    const { rerender } = render(<TaskCard {...props} />)
    expect(screen.getByText('Fix login').closest('div[draggable]')?.className).not.toContain(
      'opacity-40'
    )
    rerender(<TaskCard {...props} dragging />)
    expect(screen.getByText('Fix login').closest('div[draggable]')?.className).toContain(
      'opacity-40'
    )
  })

  it('exposes the data type the board reads dropped task ids from', () => {
    expect(TASK_DATA_TYPE).toBe('text/plain')
  })

  it('colours the priority badge by level', () => {
    const badgeClass = (priority: TaskPriority, label: string) => {
      const { unmount } = render(
        <TaskCard {...props} task={create(TaskSchema, { ...task, priority, labels: [] })} />
      )
      const cls = screen.getByText(label).className
      unmount()
      return cls
    }
    expect(badgeClass(TaskPriority.P0, 'P0')).toContain('text-danger')
    expect(badgeClass(TaskPriority.P1, 'P1')).toContain('text-warn')
    expect(badgeClass(TaskPriority.P2, 'P2')).toContain('text-subtle')
  })

  it('names the repo only when one is given', () => {
    const { rerender } = render(<TaskCard {...props} />)
    expect(screen.queryByText('octo/cat')).not.toBeInTheDocument()

    rerender(<TaskCard {...props} repo={create(RepoSchema, { owner: 'octo', name: 'cat' })} />)
    expect(screen.getByText('octo/cat')).toBeInTheDocument()
    expect(screen.getByTestId('task-id').parentElement).toHaveTextContent('t-1 · octo/cat')
  })
})
