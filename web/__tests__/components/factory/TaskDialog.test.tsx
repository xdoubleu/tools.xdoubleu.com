import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockCreate = vi.fn()
const mockUpdate = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryActions: vi.fn(() => ({ create: mockCreate, update: mockUpdate }))
}))

import TaskDialog from '@/components/factory/TaskDialog'
import { TaskPriority, TaskSchema } from '@/lib/gen/factory/v1/factory_pb'

beforeEach(() => {
  vi.clearAllMocks()
  mockCreate.mockResolvedValue({ id: 'new' })
  mockUpdate.mockResolvedValue(undefined)
})

describe('TaskDialog', () => {
  it('creates a task from the form and reports its id', async () => {
    const onClose = vi.fn()
    const onSaved = vi.fn()
    render(<TaskDialog onClose={onClose} onSaved={onSaved} />)

    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'New thing' } })
    fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'details' } })
    fireEvent.change(screen.getByLabelText('Priority'), {
      target: { value: String(TaskPriority.P0) }
    })
    fireEvent.change(screen.getByLabelText('Labels'), { target: { value: 'a, b' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith({
        title: 'New thing',
        body: 'details',
        priority: TaskPriority.P0,
        labels: ['a', 'b']
      })
    )
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    expect(onSaved).toHaveBeenCalledWith('new')
  })

  it('edits an existing task, starting from its values', async () => {
    const task = create(TaskSchema, {
      id: 't-1',
      title: 'Old',
      priority: TaskPriority.P1,
      labels: ['x', 'y']
    })
    const onClose = vi.fn()
    render(<TaskDialog task={task} body="old body" onClose={onClose} />)

    expect(screen.getByLabelText('Title')).toHaveValue('Old')
    expect(screen.getByLabelText('Description')).toHaveValue('old body')
    expect(screen.getByLabelText('Labels')).toHaveValue('x, y')
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Renamed' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith('t-1', {
        title: 'Renamed',
        body: 'old body',
        priority: TaskPriority.P1,
        labels: ['x', 'y']
      })
    )
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('stays open and says so when saving fails', async () => {
    mockCreate.mockRejectedValue(new Error('nope'))
    const onClose = vi.fn()
    render(<TaskDialog onClose={onClose} />)
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'x' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(/Couldn.t save/)
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
  })

  it('cancels without saving', () => {
    const onClose = vi.fn()
    render(<TaskDialog onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onClose).toHaveBeenCalled()
    expect(mockCreate).not.toHaveBeenCalled()
  })
})
