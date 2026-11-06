import React from 'react'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'

let mockQuery = ''
vi.mock('@/hooks/navigation', () => ({
  useSearchParams: () => new URLSearchParams(mockQuery)
}))
const mockSetPaused = vi.fn()
const mockMove = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryActions: () => ({ setPickupPaused: mockSetPaused, move: mockMove }),
  useFactoryTasks: vi.fn(),
  useFactoryRepos: vi.fn(),
  useFactoryRunners: vi.fn()
}))
vi.mock('@/components/factory/TaskCard', () => ({
  __esModule: true,
  default: ({
    task,
    repo,
    awaitingApproval,
    onOpen,
    onDragStart,
    onDragEnd
  }: {
    task: { id: string; title: string }
    repo?: { name: string }
    awaitingApproval?: boolean
    onOpen: (id: string) => void
    onDragStart: (event: unknown, task: { id: string }) => void
    onDragEnd: () => void
  }) => (
    <button
      data-task-id={task.id}
      onClick={() => onOpen(task.id)}
      onDragStart={() => onDragStart({ dataTransfer: { setData: () => {} } }, task)}
      onDragEnd={() => onDragEnd()}
    >
      {task.title}
      {repo && <span data-testid="repo">{repo.name}</span>}
      {awaitingApproval && <span data-testid="awaiting">awaiting</span>}
    </button>
  )
}))
vi.mock('@/components/factory/TaskDetail', () => ({
  __esModule: true,
  default: ({ id, onClose }: { id: string; onClose: () => void }) => (
    <div data-testid="detail" onClick={onClose}>
      {id}
    </div>
  )
}))
vi.mock('@/components/factory/TaskDialog', () => ({
  __esModule: true,
  default: ({
    defaultRepoId,
    onSaved,
    onClose
  }: {
    defaultRepoId?: string
    onSaved: (id: string) => void
    onClose: () => void
  }) => (
    <div data-testid="create" data-default-repo={defaultRepoId}>
      <button onClick={() => onSaved('new-id')}>save</button>
      <button onClick={onClose}>close</button>
    </div>
  )
}))

import FactoryClient from '@/components/factory/FactoryClient'
import { useFactoryRepos, useFactoryRunners, useFactoryTasks } from '@/hooks/useFactory'
import { ListRunnersResponseSchema, RunnerSchema } from '@/lib/gen/factory/v1/runners_pb'
import {
  ListReposResponseSchema,
  ListTasksResponseSchema,
  RepoSchema,
  TaskSchema,
  TaskStatus
} from '@/lib/gen/factory/v1/factory_pb'

function mockTasks(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useFactoryTasks).mockReturnValue({ isLoading: false, ...value })
}

const board = create(ListTasksResponseSchema, {
  tasks: [
    create(TaskSchema, { id: 'a', title: 'Alpha', status: TaskStatus.BACKLOG, repoId: 'r1' }),
    create(TaskSchema, { id: 'b', title: 'Beta', status: TaskStatus.DONE, repoId: 'r2' })
  ]
})

const oneRepo = create(ListReposResponseSchema, {
  repos: [create(RepoSchema, { id: 'r1', owner: 'octo', name: 'cat' })]
})
const twoRepos = create(ListReposResponseSchema, {
  repos: [
    create(RepoSchema, { id: 'r1', owner: 'octo', name: 'cat' }),
    create(RepoSchema, { id: 'r2', owner: 'octo', name: 'dog' })
  ]
})

function mockRunners(data?: unknown) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useFactoryRunners).mockReturnValue({ data })
}

function mockRepos(data?: unknown) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useFactoryRepos).mockReturnValue({ data })
}

beforeEach(() => {
  vi.clearAllMocks()
  mockQuery = ''
  mockSetPaused.mockResolvedValue(undefined)
  mockMove.mockResolvedValue(undefined)
  mockRepos(undefined)
  mockRunners(undefined)
})

describe('FactoryClient', () => {
  it('shows every column with its tasks and counts', () => {
    mockTasks({ data: board })
    render(<FactoryClient />)
    for (const name of ['Backlog', 'Ready', 'In progress', 'In review', 'Needs you', 'Done']) {
      expect(screen.getByRole('region', { name })).toBeInTheDocument()
    }
    expect(screen.getByRole('region', { name: 'Backlog' })).toHaveTextContent('Alpha')
    expect(screen.getByRole('region', { name: 'Done' })).toHaveTextContent('Beta')
    expect(screen.getAllByText('Nothing here.')).toHaveLength(5)
  })

  it('shows when the Claude limit lifts, only while it is still in force', () => {
    const inTwoHours = new Date(Date.now() + 2 * 3600_000).toISOString()
    mockTasks({
      data: create(ListTasksResponseSchema, { tasks: [], claudeLimitedUntil: inTwoHours })
    })
    const { rerender } = render(<FactoryClient />)
    expect(screen.getByText(/^Limited until /)).toBeInTheDocument()

    const gone = new Date(Date.now() - 3600_000).toISOString()
    mockTasks({ data: create(ListTasksResponseSchema, { tasks: [], claudeLimitedUntil: gone }) })
    rerender(<FactoryClient />)
    expect(screen.queryByText(/Limited until/)).not.toBeInTheDocument()

    mockTasks({ data: board })
    rerender(<FactoryClient />)
    expect(screen.queryByText(/Limited until/)).not.toBeInTheDocument()
  })

  it('takes the notice down by itself once the limit has lifted', () => {
    vi.useFakeTimers()
    try {
      const inOneMinute = new Date(Date.now() + 60_000).toISOString()
      mockTasks({
        data: create(ListTasksResponseSchema, { tasks: [], claudeLimitedUntil: inOneMinute })
      })
      render(<FactoryClient />)
      expect(screen.getByText(/^Limited until /)).toBeInTheDocument()

      act(() => {
        vi.advanceTimersByTime(90_000)
      })
      expect(screen.queryByText(/Limited until/)).not.toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it('shows loading and error states', () => {
    mockTasks({ isLoading: true })
    const { unmount } = render(<FactoryClient />)
    expect(screen.getByText(/Loading board/)).toBeInTheDocument()
    unmount()

    mockTasks({ error: new Error('boom') })
    render(<FactoryClient />)
    expect(screen.getByText(/Failed to load the board/)).toBeInTheDocument()
  })

  it('opens a task and closes it again', () => {
    mockTasks({ data: board })
    render(<FactoryClient />)
    fireEvent.click(screen.getByRole('button', { name: 'Alpha' }))
    expect(screen.getByTestId('detail')).toHaveTextContent('a')
    fireEvent.click(screen.getByTestId('detail'))
    expect(screen.queryByTestId('detail')).not.toBeInTheDocument()
  })

  it('opens the task a link names, and nothing without one', () => {
    mockTasks({ data: board })
    const plain = render(<FactoryClient />)
    expect(screen.queryByTestId('detail')).not.toBeInTheDocument()
    plain.unmount()

    mockQuery = '?task=a'
    render(<FactoryClient />)
    expect(screen.getByTestId('detail')).toHaveTextContent('a')
    fireEvent.click(screen.getByTestId('detail'))
    expect(screen.queryByTestId('detail')).not.toBeInTheDocument()
  })

  it('creates a task and opens it', () => {
    mockTasks({ data: board })
    render(<FactoryClient />)
    fireEvent.click(screen.getByRole('button', { name: 'New task' }))
    fireEvent.click(screen.getByRole('button', { name: 'save' }))
    expect(screen.getByTestId('detail')).toHaveTextContent('new-id')
    fireEvent.click(screen.getByRole('button', { name: 'close' }))
    expect(screen.queryByTestId('create')).not.toBeInTheDocument()
  })

  it('shows no columns until the board arrives, and no error or spinner beside data', () => {
    mockTasks({ isLoading: true })
    const { unmount } = render(<FactoryClient />)
    expect(screen.queryAllByRole('region')).toHaveLength(0)
    unmount()

    mockTasks({ data: board, error: new Error('refetch failed'), isLoading: true })
    render(<FactoryClient />)
    expect(screen.queryByText(/Failed to load/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Loading board/)).not.toBeInTheDocument()
    expect(screen.getAllByRole('region')).toHaveLength(7)
  })

  it('starts without the create sheet or an open task', () => {
    mockTasks({ data: board })
    render(<FactoryClient />)
    expect(screen.queryByTestId('create')).not.toBeInTheDocument()
    expect(screen.queryByTestId('detail')).not.toBeInTheDocument()
  })

  it('does not show an error while the board has not failed', () => {
    mockTasks({ isLoading: true })
    render(<FactoryClient />)
    expect(screen.queryByText(/Failed to load/)).not.toBeInTheDocument()
  })

  it('shows whether the runner is online, and links to its page', () => {
    mockTasks({ data: board })
    const { unmount } = render(<FactoryClient />)
    expect(screen.getByText('No runner')).toBeInTheDocument()
    expect(screen.getByText('No runner').className).toContain('text-subtle')
    expect(screen.getByRole('link', { name: 'Runner' })).toHaveAttribute('href', '/factory/runners')
    unmount()

    mockRunners(
      create(ListRunnersResponseSchema, {
        runners: [create(RunnerSchema, { id: 'r1', pairedAt: 'x', online: true, lastSeen: 'x' })]
      })
    )
    render(<FactoryClient />)
    expect(screen.getByText('Online')).toBeInTheDocument()
    expect(screen.getByText('Online').className).toContain('text-success')
  })

  it('links to the agent accounts', () => {
    mockTasks({ data: board })
    render(<FactoryClient />)
    expect(screen.getByRole('link', { name: 'Agents' })).toHaveAttribute('href', '/factory/agents')
  })

  it('links to the repo settings', () => {
    mockTasks({ data: board })
    render(<FactoryClient />)
    expect(screen.getByRole('link', { name: 'Repos' })).toHaveAttribute('href', '/factory/repos')
  })

  it('filters the board to one product', () => {
    mockTasks({
      data: create(ListTasksResponseSchema, {
        tasks: [
          create(TaskSchema, {
            id: 'b1',
            title: 'Book task',
            status: TaskStatus.BACKLOG,
            repoId: 'r1',
            labels: ['books']
          }),
          create(TaskSchema, {
            id: 'p1',
            title: 'Platform task',
            status: TaskStatus.BACKLOG,
            repoId: 'r1',
            labels: ['platform']
          })
        ]
      })
    })
    render(<FactoryClient />)
    expect(screen.getByRole('button', { name: 'Book task' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Platform task' })).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Product'), { target: { value: 'books' } })
    expect(screen.getByRole('button', { name: 'Book task' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Platform task' })).not.toBeInTheDocument()
  })

  describe('auto pickup', () => {
    const pickupRepos = create(ListReposResponseSchema, {
      repos: [create(RepoSchema, { id: 'r1', owner: 'octo', name: 'cat', autoPickup: true })]
    })

    it('shows nothing about it while no repo has it on', () => {
      mockTasks({ data: board })
      mockRepos(oneRepo)
      render(<FactoryClient />)
      expect(screen.queryByText('Auto pickup')).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Pause' })).not.toBeInTheDocument()
    })

    it('shows it once any one of several repos has it on, and not while loading', () => {
      const mixed = create(ListReposResponseSchema, {
        repos: [
          create(RepoSchema, { id: 'r1', owner: 'octo', name: 'cat', autoPickup: false }),
          create(RepoSchema, { id: 'r2', owner: 'octo', name: 'dog', autoPickup: true })
        ]
      })
      mockRepos(mixed)
      mockTasks({ isLoading: true })
      const { unmount } = render(<FactoryClient />)
      expect(screen.queryByText(/Auto pickup/)).not.toBeInTheDocument()
      unmount()

      mockTasks({ data: board })
      render(<FactoryClient />)
      expect(screen.getByText(/Auto pickup/)).toHaveTextContent('Auto pickup On')
    })

    it('pauses it from the board, and resumes it', async () => {
      mockTasks({ data: board })
      mockRepos(pickupRepos)
      const { unmount } = render(<FactoryClient />)
      expect(screen.getByText('On').className).toContain('text-success')
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Pause' }))
      await waitFor(() => expect(mockSetPaused).toHaveBeenCalledWith(true))
      unmount()

      mockTasks({ data: create(ListTasksResponseSchema, { tasks: [], pickupPaused: true }) })
      render(<FactoryClient />)
      expect(screen.getByText('Paused').className).toContain('text-warn')
      fireEvent.click(screen.getByRole('button', { name: 'Resume' }))
      await waitFor(() => expect(mockSetPaused).toHaveBeenLastCalledWith(false))
    })

    it('says why it could not be changed', async () => {
      mockSetPaused.mockRejectedValueOnce(new Error('only a human can resume'))
      mockTasks({ data: board })
      mockRepos(pickupRepos)
      render(<FactoryClient />)
      fireEvent.click(screen.getByRole('button', { name: 'Pause' }))
      expect(await screen.findByRole('alert')).toHaveTextContent('only a human can resume')

      mockSetPaused.mockRejectedValueOnce('boom')
      fireEvent.click(screen.getByRole('button', { name: 'Pause' }))
      await waitFor(() =>
        expect(screen.getByRole('alert')).toHaveTextContent('Couldn\u2019t change auto pickup')
      )

      fireEvent.click(screen.getByRole('button', { name: 'Pause' }))
      await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
    })

    it('flags a Ready task of such a repo whose text is not approved', () => {
      const ready = (id: string, pickupApproved: boolean) =>
        create(TaskSchema, {
          id,
          title: id,
          status: TaskStatus.READY,
          repoId: 'r1',
          pickupApproved
        })
      mockTasks({
        data: create(ListTasksResponseSchema, { tasks: [ready('A', false), ready('B', true)] })
      })
      mockRepos(pickupRepos)
      render(<FactoryClient />)
      expect(screen.getAllByTestId('awaiting')).toHaveLength(1)
      expect(screen.getByRole('button', { name: /A/ })).toHaveTextContent('awaiting')
    })
  })

  it('offers no repo filter or repo names with a single repo', () => {
    mockTasks({ data: board })
    mockRepos(oneRepo)
    render(<FactoryClient />)
    expect(screen.queryByLabelText('Repo')).not.toBeInTheDocument()
    expect(screen.queryByTestId('repo')).not.toBeInTheDocument()
  })

  it('filters the board by repo and names each task repo when there are several', () => {
    mockTasks({ data: board })
    mockRepos(twoRepos)
    render(<FactoryClient />)
    expect(screen.getAllByTestId('repo').map((e) => e.textContent)).toEqual(['cat', 'dog'])
    expect(screen.getByRole('option', { name: 'All repos' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'octo/dog' })).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Repo'), { target: { value: 'r2' } })
    expect(screen.queryByRole('button', { name: /Alpha/ })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Beta/ })).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Repo'), { target: { value: '' } })
    expect(screen.getByRole('button', { name: /Alpha/ })).toBeInTheDocument()
  })

  it('starts a new task in the repo being filtered to', () => {
    mockTasks({ data: board })
    mockRepos(twoRepos)
    render(<FactoryClient />)
    fireEvent.change(screen.getByLabelText('Repo'), { target: { value: 'r2' } })
    fireEvent.click(screen.getByRole('button', { name: 'New task' }))
    expect(screen.getByTestId('create')).toHaveAttribute('data-default-repo', 'r2')
  })

  it('offers a label filter once a task has a label, and shows only matching tasks', () => {
    const labelled = (labels: string[]) =>
      create(TaskSchema, {
        id: labels[0] ?? 'x',
        title: labels.join('+') || 'Plain',
        status: TaskStatus.BACKLOG,
        labels
      })
    mockTasks({
      data: create(ListTasksResponseSchema, {
        tasks: [labelled(['bug']), labelled(['ui']), labelled([])]
      })
    })
    render(<FactoryClient />)

    const filter = screen.getByLabelText('Label')
    expect(screen.getByRole('option', { name: 'All labels' })).toBeInTheDocument()
    fireEvent.change(filter, { target: { value: 'ui' } })
    expect(screen.getByRole('button', { name: 'ui' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'bug' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Plain' })).not.toBeInTheDocument()

    fireEvent.change(filter, { target: { value: '' } })
    expect(screen.getByRole('button', { name: 'bug' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Plain' })).toBeInTheDocument()
  })

  it('moves a card dropped on another column', async () => {
    mockTasks({ data: board })
    render(<FactoryClient />)
    const dt = { getData: vi.fn(() => 'a'), setData: vi.fn() }
    fireEvent.drop(screen.getByRole('region', { name: 'Done' }), {
      dataTransfer: dt
    })
    await waitFor(() => expect(mockMove).toHaveBeenCalledWith('a', TaskStatus.DONE))
  })

  it('tracks a card drag through start, over and end', async () => {
    mockTasks({ data: board })
    render(<FactoryClient />)
    // Star a drag on the Backlog card.
    fireEvent.dragStart(screen.getByRole('button', { name: 'Alpha' }))
    // Dragging over a column highlights it.
    fireEvent.dragOver(screen.getByRole('region', { name: 'Done' }))
    expect(screen.getByRole('region', { name: 'Done' }).className).toContain('ring-accent')
    fireEvent.dragLeave(screen.getByRole('region', { name: 'Done' }))
    expect(screen.getByRole('region', { name: 'Done' }).className).not.toContain('ring-accent')
    fireEvent.dragEnd(screen.getByRole('button', { name: 'Alpha' }))
  })

  it('does not move a card dropped back onto its own column', async () => {
    mockTasks({ data: board })
    render(<FactoryClient />)
    const dt = { getData: vi.fn(() => 'a'), setData: vi.fn() }
    // Alpha sits in Backlog; dropping there should be a no-op.
    fireEvent.drop(screen.getByRole('region', { name: 'Backlog' }), {
      dataTransfer: dt
    })
    expect(mockMove).not.toHaveBeenCalled()
  })

  it('says so when a dropped move is refused', async () => {
    mockMove.mockRejectedValue(new ConnectError('no', Code.PermissionDenied))
    mockTasks({ data: board })
    render(<FactoryClient />)
    const dt = { getData: vi.fn(() => 'a'), setData: vi.fn() }
    fireEvent.drop(screen.getByRole('region', { name: 'Done' }), {
      dataTransfer: dt
    })
    expect(await screen.findByRole('alert')).toHaveTextContent(/Only you, in the browser/)
  })
})
