import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'

const mockMove = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryActions: vi.fn(() => ({ move: mockMove }))
}))

import { MoveSelect } from '@/components/factory/MoveSelect'
import { TaskSchema, TaskStatus } from '@/lib/gen/factory/v1/factory_pb'

const task = create(TaskSchema, { id: 't-1', title: 'Fix login', status: TaskStatus.BACKLOG })

beforeEach(() => {
  vi.clearAllMocks()
  mockMove.mockResolvedValue(undefined)
})

describe('MoveSelect', () => {
  it('moves the task through the menu', async () => {
    render(<MoveSelect task={task} />)
    fireEvent.change(screen.getByLabelText('Move Fix login'), {
      target: { value: String(TaskStatus.READY) }
    })
    await waitFor(() => expect(mockMove).toHaveBeenCalledWith('t-1', TaskStatus.READY))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('says so when a move is refused', async () => {
    mockMove.mockRejectedValue(new Error('denied'))
    render(<MoveSelect task={task} />)
    fireEvent.change(screen.getByLabelText('Move Fix login'), {
      target: { value: String(TaskStatus.READY) }
    })
    expect(await screen.findByRole('alert')).toHaveTextContent(/Couldn.t move/)
  })

  it('explains a permission refusal', async () => {
    mockMove.mockRejectedValue(new ConnectError('no', Code.PermissionDenied))
    render(<MoveSelect task={task} />)
    fireEvent.change(screen.getByLabelText('Move Fix login'), {
      target: { value: String(TaskStatus.READY) }
    })
    expect(await screen.findByRole('alert')).toHaveTextContent(/Only you, in the browser/)
  })

  it('shows no message before any move', () => {
    render(<MoveSelect task={task} />)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('clears an earlier refusal when the next move starts', async () => {
    mockMove.mockRejectedValueOnce(new Error('denied'))
    render(<MoveSelect task={task} />)
    const select = screen.getByLabelText('Move Fix login')
    fireEvent.change(select, { target: { value: String(TaskStatus.READY) } })
    await screen.findByRole('alert')
    fireEvent.change(select, { target: { value: String(TaskStatus.DONE) } })
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })
})
