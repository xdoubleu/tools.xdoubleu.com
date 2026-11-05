import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockComment = vi.fn()
const mockRun = vi.fn()
const mockStop = vi.fn()
const mockReply = vi.fn()
const mockAgents = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryTask: vi.fn(),
  useFactoryAgents: () => mockAgents(),
  useFactoryActions: vi.fn(() => ({
    comment: mockComment,
    move: vi.fn(),
    run: mockRun,
    reply: mockReply,
    stop: mockStop
  }))
}))
vi.mock('@/components/factory/JobLog', () => ({
  __esModule: true,
  default: ({ jobId, active }: { jobId: string; active: boolean }) => (
    <div data-testid="job-log">{`${jobId} ${active ? 'active' : 'idle'}`}</div>
  )
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
import { AgentAccountSchema } from '@/lib/gen/factory/v1/agent_accounts_pb'
import {
  CommentSchema,
  GetTaskResponseSchema,
  Harness,
  JobSchema,
  JobStatus,
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
  mockAgents.mockReturnValue({ data: undefined })
  mockComment.mockResolvedValue(undefined)
  mockRun.mockResolvedValue(undefined)
  mockStop.mockResolvedValue(undefined)
  mockReply.mockResolvedValue(undefined)
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

  describe('replying to the agent', () => {
    const finished = (job: object = {}, status = TaskStatus.IN_REVIEW) =>
      create(GetTaskResponseSchema, {
        task: create(TaskSchema, {
          id: 't-1',
          title: 'Fix login',
          body: 'Steps to reproduce',
          status,
          priority: TaskPriority.P1,
          labels: ['bug'],
          createdBy: 'human:me'
        }),
        comments: [],
        jobs: [
          create(JobSchema, {
            id: 'j-1',
            status: JobStatus.SUCCEEDED,
            branch: 'agent/t-1-fix-login',
            ...job
          })
        ]
      })

    it('sends the reply and clears the box', async () => {
      mockTask({ data: finished() })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      const box = screen.getByLabelText('Reply to the agent')
      expect(screen.getByRole('button', { name: 'Reply' })).toBeDisabled()
      fireEvent.change(box, { target: { value: 'Also cover the empty case' } })
      fireEvent.click(screen.getByRole('button', { name: 'Reply' }))

      await waitFor(() =>
        expect(mockReply).toHaveBeenCalledWith('t-1', 'Also cover the empty case')
      )
      await waitFor(() => expect(box).toHaveValue(''))
    })

    it('is busy while sending, and a retry clears the earlier error', async () => {
      let finish: () => void = () => {}
      mockReply.mockRejectedValueOnce(new Error('nope'))
      mockReply.mockReturnValueOnce(new Promise<void>((resolve) => (finish = resolve)))
      mockTask({ data: finished({ prState: 'open', prCi: 'failing' }) })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      const box = screen.getByLabelText('Reply to the agent')
      fireEvent.change(box, { target: { value: '   ' } })
      expect(screen.getByRole('button', { name: 'Reply' })).toBeDisabled()

      fireEvent.change(box, { target: { value: 'Go' } })
      fireEvent.click(screen.getByRole('button', { name: 'Reply' }))
      expect(await screen.findByRole('alert')).toHaveTextContent('nope')

      fireEvent.click(screen.getByRole('button', { name: 'Reply' }))
      expect(await screen.findByRole('button', { name: 'Sending…' })).toBeDisabled()
      expect(screen.getByRole('button', { name: 'Reply: fix CI' })).toBeDisabled()
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      finish()
      expect(await screen.findByRole('button', { name: 'Reply' })).toBeDisabled()
      expect(box).toHaveValue('')
    })

    it('does not submit the page', async () => {
      mockTask({ data: finished() })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      fireEvent.change(screen.getByLabelText('Reply to the agent'), {
        target: { value: 'Go' }
      })
      const form = screen.getByRole('form', { name: 'Reply' })
      expect(fireEvent.submit(form)).toBe(false)
      await waitFor(() => expect(mockReply).toHaveBeenCalledWith('t-1', 'Go'))
    })

    it('keeps the text and shows why the reply was refused', async () => {
      mockReply.mockRejectedValueOnce(new Error('the last job left no session to resume'))
      mockTask({ data: finished() })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      const box = screen.getByLabelText('Reply to the agent')
      fireEvent.change(box, { target: { value: 'Try again' } })
      fireEvent.click(screen.getByRole('button', { name: 'Reply' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('no session to resume')
      expect(box).toHaveValue('Try again')
    })

    it('says so when the reply fails without a message', async () => {
      mockReply.mockRejectedValueOnce('boom')
      mockTask({ data: finished() })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      fireEvent.change(screen.getByLabelText('Reply to the agent'), {
        target: { value: 'Try again' }
      })
      fireEvent.click(screen.getByRole('button', { name: 'Reply' }))
      expect(await screen.findByRole('alert')).toHaveTextContent(
        'Couldn\u2019t send the reply. Try again.'
      )
    })

    it('offers "fix CI" only for an open pull request with failing checks', async () => {
      mockTask({ data: finished({ prState: 'open', prCi: 'passing' }) })
      const { unmount } = render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.queryByRole('button', { name: 'Reply: fix CI' })).not.toBeInTheDocument()
      unmount()

      mockTask({ data: finished({ prState: 'merged', prCi: 'failing' }) })
      const merged = render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.queryByRole('button', { name: 'Reply: fix CI' })).not.toBeInTheDocument()
      merged.unmount()

      mockTask({ data: finished({ prState: 'open', prCi: 'failing' }) })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      fireEvent.click(screen.getByRole('button', { name: 'Reply: fix CI' }))
      await waitFor(() =>
        expect(mockReply).toHaveBeenCalledWith('t-1', expect.stringContaining('CI is red'))
      )
    })

    it('is not offered while a job is active, before any job, or once the task is done', () => {
      mockTask({ data: finished({ status: JobStatus.RUNNING }) })
      const running = render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.queryByLabelText('Reply to the agent')).not.toBeInTheDocument()
      running.unmount()

      mockTask({ data: detail })
      const none = render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.queryByLabelText('Reply to the agent')).not.toBeInTheDocument()
      none.unmount()

      mockTask({ data: finished({}, TaskStatus.DONE) })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.queryByLabelText('Reply to the agent')).not.toBeInTheDocument()
    })
  })

  describe('running the task', () => {
    const withJob = (status: JobStatus) =>
      create(GetTaskResponseSchema, {
        ...detail,
        jobs: [create(JobSchema, { id: 'j-1', status, branch: 'agent/t-1-fix-login' })]
      })

    it('runs on a picked harness and model, and only then asks for a model', async () => {
      mockTask({ data: detail })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      const harness = screen.getByLabelText('Harness')
      expect(harness).toHaveValue(String(Harness.UNSPECIFIED))
      expect(screen.queryByLabelText('Model')).not.toBeInTheDocument()

      fireEvent.change(harness, { target: { value: String(Harness.OPENCODE) } })
      fireEvent.change(screen.getByLabelText('Model'), { target: { value: ' vendor/m ' } })
      fireEvent.click(screen.getByRole('button', { name: 'Run' }))

      await waitFor(() =>
        expect(mockRun).toHaveBeenCalledWith('t-1', '', {
          harness: Harness.OPENCODE,
          model: 'vendor/m'
        })
      )
    })

    it('runs on Claude Code without a model', async () => {
      mockTask({ data: detail })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      fireEvent.change(screen.getByLabelText('Harness'), {
        target: { value: String(Harness.CLAUDE_CODE) }
      })
      expect(screen.queryByLabelText('Model')).not.toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Run' }))
      await waitFor(() =>
        expect(mockRun).toHaveBeenCalledWith('t-1', '', {
          harness: Harness.CLAUDE_CODE,
          model: ''
        })
      )
    })

    it('queues a job with the optional instructions and clears the box', async () => {
      mockTask({ data: detail })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.queryByTestId('job-status')).not.toBeInTheDocument()
      const box = screen.getByLabelText(/Instructions for this run/)
      fireEvent.change(box, { target: { value: 'Mind the tests' } })
      fireEvent.click(screen.getByRole('button', { name: 'Run' }))

      await waitFor(() => expect(mockRun).toHaveBeenCalledWith('t-1', 'Mind the tests'))
      await waitFor(() => expect(box).toHaveValue(''))
    })

    it('handles the submit itself instead of reloading the page', () => {
      mockTask({ data: detail })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      const form = screen.getByLabelText(/Instructions for this run/).closest('form')
      if (!form) throw new Error('no form')
      // fireEvent returns false when a handler called preventDefault.
      expect(fireEvent.submit(form)).toBe(false)
    })

    it('can run with no instructions', async () => {
      mockTask({ data: detail })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      fireEvent.click(screen.getByRole('button', { name: 'Run' }))
      await waitFor(() => expect(mockRun).toHaveBeenCalledWith('t-1', ''))
    })

    it('shows the reason the server gives when the run is refused, and keeps the text', async () => {
      mockRun.mockRejectedValue(new Error('Not run: the body was edited outside a browser'))
      mockTask({ data: detail })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      const box = screen.getByLabelText(/Instructions for this run/)
      fireEvent.change(box, { target: { value: 'go' } })
      fireEvent.click(screen.getByRole('button', { name: 'Run' }))

      expect(await screen.findByText(/edited outside a browser/)).toBeInTheDocument()
      expect(box).toHaveValue('go')
    })

    it('clears an earlier refusal on the next try, and the button comes back', async () => {
      mockRun.mockRejectedValueOnce(new Error('Not run: nope'))
      mockTask({ data: detail })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      fireEvent.click(screen.getByRole('button', { name: 'Run' }))
      expect(await screen.findByText(/Not run: nope/)).toBeInTheDocument()

      fireEvent.click(screen.getByRole('button', { name: 'Run' }))
      await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
      expect(await screen.findByRole('button', { name: 'Run' })).toBeEnabled()
    })

    it('has its own words when the failure carries none', async () => {
      mockRun.mockRejectedValueOnce('plain string')
      mockTask({ data: detail })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      fireEvent.click(screen.getByRole('button', { name: 'Run' }))
      expect(await screen.findByRole('alert')).toHaveTextContent(
        'Couldn’t run the task. Try again.'
      )
    })

    it('shows the newest job and blocks another run while it is active', () => {
      mockTask({ data: withJob(JobStatus.RUNNING) })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.getByTestId('job-status')).toHaveTextContent(
        'Job running · agent/t-1-fix-login'
      )
      expect(screen.getByRole('button', { name: 'Running…' })).toBeDisabled()
    })

    it('links the job\u2019s pull request with its status', () => {
      const job = create(JobSchema, {
        id: 'j-1',
        status: JobStatus.SUCCEEDED,
        branch: 'agent/t-1-fix-login',
        prUrl: 'https://github.com/o/n/pull/7',
        prState: 'open',
        prCi: 'failing',
        prReview: 'changes_requested'
      })
      mockTask({ data: create(GetTaskResponseSchema, { ...detail, jobs: [job] }) })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      const line = screen.getByTestId('job-pr')
      expect(line).toHaveTextContent('Pull request · PR open · CI failing · changes requested')
      expect(screen.getByRole('link', { name: 'Pull request' })).toHaveAttribute(
        'href',
        'https://github.com/o/n/pull/7'
      )
    })

    it('shows no pull request line before the job has one', () => {
      mockTask({ data: withJob(JobStatus.RUNNING) })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.queryByTestId('job-pr')).not.toBeInTheDocument()
    })

    it('says which harness and model the job ran on', () => {
      const job = create(JobSchema, {
        id: 'j-1',
        status: JobStatus.RUNNING,
        branch: 'agent/t-1-fix-login',
        harness: Harness.OPENCODE,
        model: 'vendor/model'
      })
      mockTask({ data: create(GetTaskResponseSchema, { ...detail, jobs: [job] }) })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.getByTestId('job-status')).toHaveTextContent(
        'Job running · agent/t-1-fix-login · OpenCode · vendor/model'
      )
    })

    it('stops an active job', async () => {
      mockTask({ data: withJob(JobStatus.RUNNING) })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      fireEvent.click(screen.getByRole('button', { name: 'Stop' }))
      await waitFor(() => expect(mockStop).toHaveBeenCalledWith('t-1'))
      expect(await screen.findByRole('button', { name: 'Stop' })).toBeEnabled()
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    })

    it('shows Stopping… while the stop is in flight', async () => {
      mockStop.mockReturnValue(new Promise(() => {}))
      mockTask({ data: withJob(JobStatus.QUEUED) })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      fireEvent.click(screen.getByRole('button', { name: 'Stop' }))
      expect(await screen.findByRole('button', { name: 'Stopping…' })).toBeDisabled()
    })

    it('says why a stop failed, and has its own words when there are none', async () => {
      mockStop.mockRejectedValueOnce(new Error('this task has no queued or running job'))
      mockTask({ data: withJob(JobStatus.RUNNING) })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      fireEvent.click(screen.getByRole('button', { name: 'Stop' }))
      expect(await screen.findByRole('alert')).toHaveTextContent('no queued or running job')

      mockStop.mockRejectedValueOnce('plain string')
      fireEvent.click(screen.getByRole('button', { name: 'Stop' }))
      await waitFor(() =>
        expect(screen.getByRole('alert')).toHaveTextContent(
          'Couldn\u2019t stop the job. Try again.'
        )
      )
    })

    it('has no Stop button when nothing is running', () => {
      mockTask({ data: withJob(JobStatus.FAILED) })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.queryByRole('button', { name: 'Stop' })).not.toBeInTheDocument()
    })

    it('shows the latest jobs log, followed while it is active', () => {
      mockTask({ data: withJob(JobStatus.RUNNING) })
      const { rerender } = render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.getByTestId('job-log')).toHaveTextContent('j-1 active')

      mockTask({ data: withJob(JobStatus.FAILED) })
      rerender(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.getByTestId('job-log')).toHaveTextContent('j-1 idle')
    })

    it('has no log before the first run', () => {
      mockTask({ data: detail })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.queryByTestId('job-log')).not.toBeInTheDocument()
    })

    it('allows another run once the job has finished', () => {
      mockTask({ data: withJob(JobStatus.FAILED) })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      expect(screen.getByTestId('job-status')).toHaveTextContent('Job failed')
      expect(screen.getByRole('button', { name: 'Run' })).toBeEnabled()
    })

    it('says it is queuing while the call is in flight', async () => {
      let finish: () => void = () => undefined
      mockRun.mockReturnValue(new Promise<void>((resolve) => (finish = resolve)))
      mockTask({ data: detail })
      render(<TaskDetail id="t-1" onClose={vi.fn()} />)
      fireEvent.click(screen.getByRole('button', { name: 'Run' }))
      expect(await screen.findByRole('button', { name: 'Queuing…' })).toBeDisabled()
      finish()
    })
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

  it('handles the comment form submit itself, without a page reload', () => {
    mockTask({ data: detail })
    render(<TaskDetail id="t-1" onClose={vi.fn()} />)
    const form = screen.getByLabelText('Add a comment').closest('form')
    if (!form) throw new Error('no form')
    // fireEvent returns false when the handler called preventDefault.
    expect(fireEvent.submit(form)).toBe(false)
  })

  it('shows agent actors by account name', () => {
    mockAgents.mockReturnValue({
      data: { accounts: [create(AgentAccountSchema, { userId: 'u-7', name: 'Overnight agent' })] }
    })
    const byAgent = create(GetTaskResponseSchema, {
      task: create(TaskSchema, {
        id: 't-3',
        title: 'Agent work',
        status: TaskStatus.IN_REVIEW,
        createdBy: 'agent:u-7'
      }),
      comments: [create(CommentSchema, { id: 'c-9', author: 'agent:u-7', body: 'Done' })]
    })
    mockTask({ data: byAgent })
    render(<TaskDetail id="t-3" onClose={vi.fn()} />)
    expect(screen.getByText('In review · Overnight agent')).toBeInTheDocument()
    expect(screen.getByText('Overnight agent', { selector: 'p' })).toBeInTheDocument()
  })

  it('closes from the close button', () => {
    mockTask({ data: detail })
    const onClose = vi.fn()
    render(<TaskDetail id="t-1" onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(onClose).toHaveBeenCalled()
  })
})
