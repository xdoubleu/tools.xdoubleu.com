import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockMove = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryActions: vi.fn(() => ({ move: mockMove }))
}))

import TaskCard from '@/components/factory/TaskCard'
import { TaskPriority, TaskSchema, TaskStatus } from '@/lib/gen/factory/v1/factory_pb'

const task = create(TaskSchema, {
  id: 't-1',
  title: 'Fix login',
  status: TaskStatus.BACKLOG,
  priority: TaskPriority.P0,
  labels: ['bug'],
  commentCount: 2
})

beforeEach(() => {
  vi.clearAllMocks()
  mockMove.mockResolvedValue(undefined)
})

describe('TaskCard', () => {
  it('shows title, priority, labels and comment count', () => {
    render(<TaskCard task={task} onOpen={vi.fn()} />)
    expect(screen.getByText('Fix login')).toBeInTheDocument()
    expect(screen.getByText('P0')).toBeInTheDocument()
    expect(screen.getByText('bug')).toBeInTheDocument()
    expect(screen.getByText('2 comments')).toBeInTheDocument()
  })

  it('omits the comment count when there are no comments', () => {
    render(<TaskCard task={create(TaskSchema, { ...task, commentCount: 0 })} onOpen={vi.fn()} />)
    expect(screen.queryByText(/comments/)).not.toBeInTheDocument()
  })

  it('opens the task from its title', () => {
    const onOpen = vi.fn()
    render(<TaskCard task={task} onOpen={onOpen} />)
    fireEvent.click(screen.getByRole('button', { name: 'Fix login' }))
    expect(onOpen).toHaveBeenCalledWith('t-1')
  })

  it('moves the task through the menu', async () => {
    render(<TaskCard task={task} onOpen={vi.fn()} />)
    fireEvent.change(screen.getByLabelText('Move Fix login'), {
      target: { value: String(TaskStatus.READY) }
    })
    await waitFor(() => expect(mockMove).toHaveBeenCalledWith('t-1', TaskStatus.READY))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('says so when a move is refused', async () => {
    mockMove.mockRejectedValue(new Error('denied'))
    render(<TaskCard task={task} onOpen={vi.fn()} />)
    fireEvent.change(screen.getByLabelText('Move Fix login'), {
      target: { value: String(TaskStatus.READY) }
    })
    expect(await screen.findByRole('alert')).toHaveTextContent(/Couldn.t move/)
  })
})
