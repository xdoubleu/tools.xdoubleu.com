import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

vi.mock('@/hooks/useFactory', () => ({ useJobLog: vi.fn() }))

import JobLog from '@/components/factory/JobLog'
import { useJobLog } from '@/hooks/useFactory'
import { JobEventSchema } from '@/lib/gen/factory/v1/factory_pb'

const mockLog = vi.mocked(useJobLog)
const event = (seq: number, payload: unknown) =>
  create(JobEventSchema, { seq, kind: 'x', payload: JSON.stringify(payload) })
const show = (value: unknown) => {
  // @ts-expect-error -- partial SWRResponse for the test
  mockLog.mockReturnValue(value)
}

describe('JobLog', () => {
  it('reads the log of the job it is given, following it only while active', () => {
    show({ data: [], isLoading: false })
    const { rerender } = render(<JobLog jobId="j-1" active />)
    expect(mockLog).toHaveBeenLastCalledWith('j-1', true)
    rerender(<JobLog jobId="j-1" active={false} />)
    expect(mockLog).toHaveBeenLastCalledWith('j-1', false)
  })

  it('shows loading, an error, and an empty log in words', () => {
    show({ data: undefined, isLoading: true })
    const { rerender } = render(<JobLog jobId="j-1" active />)
    expect(screen.getByText(/Loading log/)).toBeInTheDocument()

    show({ data: undefined, isLoading: false, error: new Error('x') })
    rerender(<JobLog jobId="j-1" active />)
    expect(screen.getByRole('alert')).toHaveTextContent('Failed to load the log.')

    show({ data: [], isLoading: false })
    rerender(<JobLog jobId="j-1" active />)
    expect(screen.getByText('No output yet.')).toBeInTheDocument()
    rerender(<JobLog jobId="j-1" active={false} />)
    expect(screen.getByText('No log kept.')).toBeInTheDocument()
  })

  it('shows the agent text, closed tool calls, errors and the result', () => {
    show({
      data: [
        event(1, {
          type: 'assistant',
          message: {
            content: [
              { type: 'text', text: 'Looking around.' },
              { type: 'tool_use', name: 'Bash', input: { command: 'ls' } }
            ]
          }
        }),
        event(2, {
          type: 'user',
          message: { content: [{ type: 'tool_result', content: 'boom', is_error: true }] }
        }),
        event(3, { type: 'result', result: 'Fixed it.' })
      ],
      isLoading: false
    })
    render(<JobLog jobId="j-1" active={false} />)

    expect(screen.getByRole('log', { name: 'Job log' })).toBeInTheDocument()
    expect(screen.getByText('Looking around.')).toBeInTheDocument()
    expect(screen.getByText('Tool error')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('Tool error · boom')
    expect(screen.getByRole('status')).toHaveTextContent('Finished · Fixed it.')
    expect(screen.getByRole('status')).toHaveClass('text-success')
    expect(screen.getByRole('alert')).toHaveClass('text-danger')

    // A tool call is a closed disclosure until opened.
    expect(screen.queryByText('ls')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Bash/ }))
    expect(screen.getByText('ls')).toBeInTheDocument()
  })

  it('shows a result with nothing to say, and an empty tool call', () => {
    show({
      data: [
        event(1, { type: 'result', result: '' }),
        event(2, {
          type: 'assistant',
          message: { content: [{ type: 'tool_use', name: 'Idle', input: {} }] }
        })
      ],
      isLoading: false
    })
    render(<JobLog jobId="j-1" active />)
    expect(screen.getByRole('status')).toHaveTextContent(/^Finished$/)
    fireEvent.click(screen.getByRole('button', { name: /Idle/ }))
    expect(screen.getByText('(empty)')).toBeInTheDocument()
  })

  it('shows what it has while a refresh is loading', () => {
    show({ data: [event(1, { type: 'result', result: 'Done' })], isLoading: true })
    render(<JobLog jobId="j-1" active />)
    expect(screen.getByText(/Done/)).toBeInTheDocument()
    expect(screen.queryByText(/Loading log/)).not.toBeInTheDocument()
  })

  it('shows events as they arrive', () => {
    show({ data: [], isLoading: false })
    const { rerender } = render(<JobLog jobId="j-1" active />)
    show({ data: [event(1, { type: 'result', result: 'Done' })], isLoading: false })
    rerender(<JobLog jobId="j-1" active />)
    expect(screen.getByText(/Done/)).toBeInTheDocument()
  })

  describe('following a running job', () => {
    const logWith = (count: number) =>
      Array.from({ length: count }, (_, i) =>
        event(i + 1, {
          type: 'assistant',
          message: { content: [{ type: 'text', text: `step ${i}` }] }
        })
      )

    beforeEach(() => {
      vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(900)
    })
    afterEach(() => vi.restoreAllMocks())

    it('scrolls to the end as lines arrive', () => {
      show({ data: logWith(2), isLoading: false })
      const { rerender } = render(<JobLog jobId="j-1" active />)
      const box = screen.getByRole('log')
      expect(box.scrollTop).toBe(900)

      box.scrollTop = 10
      show({ data: logWith(3), isLoading: false })
      rerender(<JobLog jobId="j-1" active />)
      expect(box.scrollTop).toBe(900)
    })

    it('leaves a finished job where the reader put it', () => {
      show({ data: logWith(2), isLoading: false })
      render(<JobLog jobId="j-1" active={false} />)
      expect(screen.getByRole('log').scrollTop).toBe(0)
    })
  })
})
