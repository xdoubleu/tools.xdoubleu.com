import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockComment = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryTask: vi.fn(),
  useFactoryActions: vi.fn(() => ({ comment: mockComment, move: vi.fn() }))
}))
vi.mock('@/components/factory/TaskDialog', () => ({
  __esModule: true,
  default: ({ onClose }: { onClose: () => void }) => (
    <button data-testid="edit-dialog" onClick={onClose}>
      close edit
    </button>
  )
}))

import TaskDetail from '@/components/factory/TaskDetail'
import { useFactoryTask } from '@/hooks/useFactory'
import {
  CommentSchema,
  GetTaskResponseSchema,
  TaskPriority,
  TaskSchema,
  TaskStatus
} from '@/lib/gen/factory/v1/factory_pb'

function mockTask(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useFactoryTask).mockReturnValue({ isLoading: false, ...value })
}

const detail = create(GetTaskResponseSchema, {
  task: create(TaskSchema, {
    id: 't-1',
    title: 'Fix login',
    body: 'Steps to reproduce',
    status: TaskStatus.IN_REVIEW,
    priority: TaskPriority.P1,
    labels: ['bug'],
    createdBy: 'human:me'
  }),
  comments: [create(CommentSchema, { id: 'c-1', author: 'human:me', body: 'On it' })]
})

beforeEach(() => {
  vi.clearAllMocks()
  mockComment.mockResolvedValue(undefined)
})

describe('TaskDetail', () => {
  it('shows the task, its body and comments', () => {
    mockTask({ data: detail })
    render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    expect(screen.getByText('Fix login')).toBeInTheDocument()
    expect(screen.getByText('In review · human:me')).toBeInTheDocument()
    expect(screen.getByText('Steps to reproduce')).toBeInTheDocument()
    expect(screen.getByText('bug')).toBeInTheDocument()
    expect(screen.getByText('On it')).toBeInTheDocument()
  })

  it('says when there is no description or comment', () => {
    const bare = create(GetTaskResponseSchema, {
      task: create(TaskSchema, { id: 't-2', title: 'Bare' })
    })
    mockTask({ data: bare })
    render(<TaskDetail id="t-2" onClose={vi.fn()} />)
    expect(screen.getByText('No description.')).toBeInTheDocument()
    expect(screen.getByText('No comments yet.')).toBeInTheDocument()
  })

  it('shows loading and error states', () => {
    mockTask({ isLoading: true })
    const { unmount } = render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    expect(screen.getByText(/Loading task/)).toBeInTheDocument()
    unmount()

    mockTask({ error: new Error('boom') })
    render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    expect(screen.getByText(/Failed to load the task/)).toBeInTheDocument()
  })

  it('posts a comment and clears the box', async () => {
    mockTask({ data: detail })
    render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    const box = screen.getByLabelText('Add a comment')
    expect(screen.getByRole('button', { name: 'Comment' })).toBeDisabled()
    fireEvent.change(box, { target: { value: 'Looks good' } })
    fireEvent.click(screen.getByRole('button', { name: 'Comment' }))

    await waitFor(() => expect(mockComment).toHaveBeenCalledWith('t-1', 'Looks good'))
    await waitFor(() => expect(box).toHaveValue(''))
  })

  it('keeps the draft and says so when the comment fails', async () => {
    mockComment.mockRejectedValue(new Error('nope'))
    mockTask({ data: detail })
    render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    const box = screen.getByLabelText('Add a comment')
    fireEvent.change(box, { target: { value: 'Looks good' } })
    fireEvent.click(screen.getByRole('button', { name: 'Comment' }))

    expect(await screen.findByText(/Couldn.t post the comment/)).toBeInTheDocument()
    expect(box).toHaveValue('Looks good')
  })

  it('opens the edit sheet', () => {
    mockTask({ data: detail })
    render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
    expect(screen.getByTestId('edit-dialog')).toBeInTheDocument()
  })

  it('closes the sheet on Escape', () => {
    mockTask({ data: detail })
    const onClose = vi.fn()
    render(<TaskDetail id="t-1" onClose={onClose} />)
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })

  it('starts with no edit sheet, no alert and no spinner', () => {
    mockTask({ data: detail, error: new Error('refetch'), isLoading: true })
    render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    expect(screen.queryByTestId('edit-dialog')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByText(/Failed to load/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Loading task/)).not.toBeInTheDocument()
  })

  it('names the sheet generically until the task loads', () => {
    mockTask({ isLoading: true })
    render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    expect(screen.getByText('Task')).toBeInTheDocument()
    expect(screen.getByText('Task details')).toBeInTheDocument()
  })

  it('shows nothing of a response that has no task', () => {
    mockTask({ data: create(GetTaskResponseSchema, {}) })
    render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Add a comment')).not.toBeInTheDocument()
  })

  it('closes the edit sheet again', () => {
    mockTask({ data: detail })
    render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
    fireEvent.click(screen.getByTestId('edit-dialog'))
    expect(screen.queryByTestId('edit-dialog')).not.toBeInTheDocument()
  })

  it('does not post a blank comment, and shows progress while posting', async () => {
    let finish: () => void = () => undefined
    mockComment.mockReturnValue(new Promise<void>((resolve) => (finish = resolve)))
    mockTask({ data: detail })
    render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    const box = screen.getByLabelText('Add a comment')

    fireEvent.change(box, { target: { value: '   ' } })
    expect(screen.getByRole('button', { name: 'Comment' })).toBeDisabled()

    fireEvent.change(box, { target: { value: 'real' } })
    fireEvent.click(screen.getByRole('button', { name: 'Comment' }))
    expect(await screen.findByRole('button', { name: 'Posting…' })).toBeDisabled()
    finish()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Comment' })).toBeDisabled())
  })

  it('clears an earlier failure when the next comment is sent', async () => {
    mockComment.mockRejectedValueOnce(new Error('nope'))
    mockTask({ data: detail })
    render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    const box = screen.getByLabelText('Add a comment')
    fireEvent.change(box, { target: { value: 'x' } })
    fireEvent.click(screen.getByRole('button', { name: 'Comment' }))
    await screen.findByText(/Couldn.t post the comment/)

    fireEvent.click(screen.getByRole('button', { name: 'Comment' }))
    await waitFor(() => expect(screen.queryByText(/Couldn.t post/)).not.toBeInTheDocument())
  })
})
