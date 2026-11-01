import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

vi.mock('@/hooks/useFactory', () => ({
  useFactoryTasks: vi.fn(),
  useFactoryRepos: vi.fn(),
  useFactoryRunners: vi.fn()
}))
vi.mock('@/components/factory/TaskCard', () => ({
  __esModule: true,
  default: ({
    task,
    repo,
    onOpen
  }: {
    task: { id: string; title: string }
    repo?: { name: string }
    onOpen: (id: string) => void
  }) => (
    <button onClick={() => onOpen(task.id)}>
      {task.title}
      {repo && <span data-testid="repo">{repo.name}</span>}
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
    expect(screen.getAllByText('Nothing here.')).toHaveLength(4)
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
    expect(screen.getAllByRole('region')).toHaveLength(6)
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
    expect(screen.getByRole('link', { name: 'Runner' })).toHaveAttribute('href', '/factory/runners')
    unmount()

    mockRunners(
      create(ListRunnersResponseSchema, {
        runners: [create(RunnerSchema, { id: 'r1', pairedAt: 'x', online: true, lastSeen: 'x' })]
      })
    )
    render(<FactoryClient />)
    expect(screen.getByText('Online')).toBeInTheDocument()
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
})
