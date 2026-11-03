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
const said = (seq: number, text: string) =>
  event(seq, { type: 'assistant', message: { content: [{ type: 'text', text }] } })
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
        event(3, { type: 'result', result: 'Fixed it.' }),
        event(4, { type: 'result', is_error: true, result: 'Out of turns.' })
      ],
      isLoading: false
    })
    render(<JobLog jobId="j-1" active={false} />)

    expect(screen.getByRole('log', { name: 'Job log' })).toBeInTheDocument()
    expect(screen.getByText('Looking around.')).toBeInTheDocument()
    // A failing tool is routine: closed like any tool output, not a red alert.
    expect(screen.getByRole('button', { name: /Tool error/ })).toBeInTheDocument()
    expect(screen.getAllByRole('alert')).toHaveLength(1) // only the failed run
    expect(screen.getByRole('status')).toHaveTextContent(/^Finished$/)
    expect(screen.getByRole('status')).toHaveClass('text-success')

    expect(screen.getByRole('alert')).toHaveTextContent('Failed · Out of turns.')
    expect(screen.getByRole('alert')).toHaveClass('text-danger')

    // A tool call is a closed disclosure, titled by what it ran, until opened.
    expect(screen.getByRole('button', { name: 'Bash · ls' })).toHaveAttribute(
      'aria-expanded',
      'false'
    )
    fireEvent.click(screen.getByRole('button', { name: 'Bash · ls' }))
    expect(screen.getByRole('button', { name: 'Bash · ls' })).toHaveAttribute(
      'aria-expanded',
      'true'
    )
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
    // Nothing ran with it, so the title is just the tool's name.
    fireEvent.click(screen.getByRole('button', { name: 'Idle' }))
    expect(screen.getByText('(empty)')).toBeInTheDocument()
  })

  it('shortens what a tool call ran to its first line in the title', () => {
    const long = 'a'.repeat(100)
    show({
      data: [
        event(1, {
          type: 'assistant',
          message: {
            content: [
              { type: 'tool_use', name: 'Bash', input: { command: `${long}\nsecond line` } },
              { type: 'tool_use', name: 'Read', input: { file_path: 'x'.repeat(60) } }
            ]
          }
        })
      ],
      isLoading: false
    })
    render(<JobLog jobId="j-1" active />)
    expect(screen.getByRole('button', { name: `Bash · ${'a'.repeat(60)}…` })).toBeInTheDocument()
    // Exactly the limit is kept whole.
    expect(screen.getByRole('button', { name: `Read · ${'x'.repeat(60)}` })).toBeInTheDocument()
  })

  it('shows what it has while a refresh is loading', () => {
    show({ data: [said(1, 'Done')], isLoading: true })
    render(<JobLog jobId="j-1" active />)
    expect(screen.getByText(/Done/)).toBeInTheDocument()
    expect(screen.queryByText(/Loading log/)).not.toBeInTheDocument()
  })

  it('shows events as they arrive', () => {
    show({ data: [], isLoading: false })
    const { rerender } = render(<JobLog jobId="j-1" active />)
    show({ data: [said(1, 'Done')], isLoading: false })
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

    it('leaves a reader who scrolled up alone, and follows again at the end', () => {
      show({ data: logWith(2), isLoading: false })
      const { rerender } = render(<JobLog jobId="j-1" active />)
      const box = screen.getByRole('log')
      vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(320)

      // 900 tall, 320 shown: scrollTop 100 is 480 from the end.
      box.scrollTop = 100
      fireEvent.scroll(box)
      show({ data: logWith(3), isLoading: false })
      rerender(<JobLog jobId="j-1" active />)
      expect(box.scrollTop).toBe(100)

      // Back within 40px of the end: following resumes.
      box.scrollTop = 541
      fireEvent.scroll(box)
      show({ data: logWith(4), isLoading: false })
      rerender(<JobLog jobId="j-1" active />)
      expect(box.scrollTop).toBe(900)

      // Exactly 40px away is no longer the end.
      box.scrollTop = 540
      fireEvent.scroll(box)
      show({ data: logWith(5), isLoading: false })
      rerender(<JobLog jobId="j-1" active />)
      expect(box.scrollTop).toBe(540)
    })

    it('leaves a finished job where the reader put it', () => {
      show({ data: logWith(2), isLoading: false })
      render(<JobLog jobId="j-1" active={false} />)
      expect(screen.getByRole('log').scrollTop).toBe(0)
    })
  })
})
