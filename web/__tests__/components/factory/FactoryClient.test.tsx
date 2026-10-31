import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

vi.mock('@/hooks/useFactory', () => ({ useFactoryTasks: vi.fn() }))
vi.mock('@/components/factory/TaskCard', () => ({
  __esModule: true,
  default: ({
    task,
    onOpen
  }: {
    task: { id: string; title: string }
    onOpen: (id: string) => void
  }) => <button onClick={() => onOpen(task.id)}>{task.title}</button>
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
  default: ({ onSaved, onClose }: { onSaved: (id: string) => void; onClose: () => void }) => (
    <div data-testid="create">
      <button onClick={() => onSaved('new-id')}>save</button>
      <button onClick={onClose}>close</button>
    </div>
  )
}))

import FactoryClient from '@/components/factory/FactoryClient'
import { useFactoryTasks } from '@/hooks/useFactory'
import { ListTasksResponseSchema, TaskSchema, TaskStatus } from '@/lib/gen/factory/v1/factory_pb'

function mockTasks(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useFactoryTasks).mockReturnValue({ isLoading: false, ...value })
}

const board = create(ListTasksResponseSchema, {
  tasks: [
    create(TaskSchema, { id: 'a', title: 'Alpha', status: TaskStatus.BACKLOG }),
    create(TaskSchema, { id: 'b', title: 'Beta', status: TaskStatus.DONE })
  ]
})

beforeEach(() => vi.clearAllMocks())

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
})
