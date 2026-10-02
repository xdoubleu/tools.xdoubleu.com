import { act, render } from '@testing-library/react'
import OutboxSync from '@/components/offline/OutboxSync'
import { flushOutbox, getOutboxSnapshot } from '@/lib/offline/outbox'

jest.mock('@/lib/offline/outbox', () => ({
  flushOutbox: jest.fn(async () => {}),
  getOutboxSnapshot: jest.fn(() => ({ pending: 0, failed: [], authBlocked: false }))
}))

describe('OutboxSync', () => {
  beforeEach(() => {
    jest.useFakeTimers()
    jest.clearAllMocks()
  })

  afterEach(() => {
    jest.useRealTimers()
  })

  it('flushes on mount and when the browser comes back online', () => {
    const { unmount } = render(<OutboxSync />)
    expect(flushOutbox).toHaveBeenCalledTimes(1)

    act(() => {
      window.dispatchEvent(new Event('online'))
    })
    expect(flushOutbox).toHaveBeenCalledTimes(2)

    unmount()
    window.dispatchEvent(new Event('online'))
    expect(flushOutbox).toHaveBeenCalledTimes(2)
  })

  it('retries periodically only while writes are pending', () => {
    render(<OutboxSync />)

    act(() => jest.advanceTimersByTime(30_000))
    expect(flushOutbox).toHaveBeenCalledTimes(1)

    jest.mocked(getOutboxSnapshot).mockReturnValue({ pending: 2, failed: [], authBlocked: false })
    act(() => jest.advanceTimersByTime(30_000))
    expect(flushOutbox).toHaveBeenCalledTimes(2)
  })
})
