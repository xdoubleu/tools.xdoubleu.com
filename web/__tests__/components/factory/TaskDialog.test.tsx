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

  it('starts a new task empty, titled "New task", with no alert', () => {
    render(<TaskDialog onClose={vi.fn()} />)
    expect(screen.getByText('New task')).toBeInTheDocument()
    expect(screen.getByLabelText('Description')).toHaveValue('')
    expect(screen.getByLabelText('Labels')).toHaveValue('')
    expect(screen.getByLabelText('Priority')).toHaveValue(String(TaskPriority.P2))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('titles an existing task "Edit task"', () => {
    render(<TaskDialog task={create(TaskSchema, { id: 't', title: 'x' })} onClose={vi.fn()} />)
    expect(screen.getByText('Edit task')).toBeInTheDocument()
    expect(screen.getByLabelText('Description')).toHaveValue('')
    expect(screen.getByLabelText('Labels')).toHaveValue('')
  })

  it('keeps Save off for a blank title and shows progress while saving', async () => {
    let finish: (v: { id: string }) => void = () => undefined
    mockCreate.mockReturnValue(new Promise((resolve) => (finish = resolve)))
    render(<TaskDialog onClose={vi.fn()} />)
    const title = screen.getByLabelText('Title')

    fireEvent.change(title, { target: { value: '   ' } })
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()

    fireEvent.change(title, { target: { value: 'real' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(await screen.findByRole('button', { name: 'Saving…' })).toBeDisabled()
    finish({ id: 'x' })
  })

  it('creates without an onSaved callback, and reports an empty id when none comes back', async () => {
    const onClose = vi.fn()
    const { unmount } = render(<TaskDialog onClose={onClose} />)
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'a' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    unmount()

    mockCreate.mockResolvedValue(undefined)
    const onSaved = vi.fn()
    render(<TaskDialog onClose={vi.fn()} onSaved={onSaved} />)
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'a' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(''))
  })

  it('reports the edited task id', async () => {
    const onSaved = vi.fn()
    render(
      <TaskDialog
        task={create(TaskSchema, { id: 't-9', title: 'x' })}
        onClose={vi.fn()}
        onSaved={onSaved}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith('t-9'))
  })

  it('clears an earlier failure when saving again', async () => {
    mockCreate.mockRejectedValueOnce(new Error('nope'))
    render(<TaskDialog onClose={vi.fn()} />)
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'a' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await screen.findByRole('alert')

    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })

  it('closes on Escape', () => {
    const onClose = vi.fn()
    render(<TaskDialog onClose={onClose} />)
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })

  it('handles the form submit itself, without a page reload', () => {
    render(<TaskDialog onClose={vi.fn()} />)
    const form = screen.getByLabelText('Title').closest('form')
    if (!form) throw new Error('no form')
    // fireEvent returns false when the handler called preventDefault.
    expect(fireEvent.submit(form)).toBe(false)
  })
})
