import { act, fireEvent, render, screen } from '@testing-library/react'
import OfflineBanner from '@/components/offline/OfflineBanner'
import { markLive, markServedFromCache } from '@/lib/offline/status'
import { dismissFailed, getOutboxSnapshot, type OutboxState } from '@/lib/offline/outbox'

const listeners = new Set<() => void>()
let mockOutbox: OutboxState = { pending: 0, failed: [], authBlocked: false }

jest.mock('@/lib/offline/outbox', () => ({
  subscribeOutbox: (l: () => void) => {
    listeners.add(l)
    return () => listeners.delete(l)
  },
  getOutboxSnapshot: jest.fn(() => mockOutbox),
  getOutboxServerSnapshot: () => ({ pending: 0, failed: [], authBlocked: false }),
  dismissFailed: jest.fn(async () => {})
}))

function setOutbox(next: OutboxState) {
  mockOutbox = next
  jest.mocked(getOutboxSnapshot).mockImplementation(() => mockOutbox)
  listeners.forEach((l) => l())
}

function setOnline(value: boolean) {
  Object.defineProperty(navigator, 'onLine', { value, configurable: true })
  window.dispatchEvent(new Event(value ? 'online' : 'offline'))
}

describe('OfflineBanner', () => {
  afterEach(() => {
    act(() => {
      markLive()
      setOnline(true)
      setOutbox({ pending: 0, failed: [], authBlocked: false })
    })
    jest.useRealTimers()
  })

  it('renders nothing while online with live data and nothing queued', () => {
    const { container } = render(<OfflineBanner />)
    expect(container).toBeEmptyDOMElement()
  })

  it('shows offline state, queued writes and the age of the saved data', () => {
    jest.useFakeTimers({ now: 10 * 60_000 })
    render(<OfflineBanner />)

    act(() => setOnline(false))
    expect(screen.getByRole('status')).toHaveTextContent('Offline')

    act(() => {
      markServedFromCache(5 * 60_000)
      setOutbox({ pending: 2, failed: [], authBlocked: false })
    })
    expect(screen.getByRole('status')).toHaveTextContent(
      'Offline · 2 changes waiting to sync · showing data saved 5 min ago'
    )

    act(() => jest.advanceTimersByTime(60_000))
    expect(screen.getByRole('status')).toHaveTextContent('6 min ago')
  })

  it('explains an unreachable server while the browser is online', () => {
    render(<OfflineBanner />)

    act(() => markServedFromCache(Date.now()))

    expect(screen.getByRole('status')).toHaveTextContent(
      'Can’t reach the server · showing data saved just now'
    )
  })

  it('shows writes waiting to sync while online', () => {
    render(<OfflineBanner />)

    act(() => setOutbox({ pending: 1, failed: [], authBlocked: false }))

    expect(screen.getByRole('status')).toHaveTextContent('1 change waiting to sync')
  })

  it('asks to sign in again when the session expired', () => {
    render(<OfflineBanner />)

    act(() => setOutbox({ pending: 3, failed: [], authBlocked: true }))

    expect(screen.getByRole('status')).toHaveTextContent(
      'Your session expired. Sign in again to sync 3 changes.'
    )
    expect(screen.getByRole('link', { name: 'Sign in again' })).toHaveAttribute(
      'href',
      '/auth/sign-in'
    )
  })

  it('lists writes that couldn’t sync and dismisses them', () => {
    render(<OfflineBanner />)

    act(() =>
      setOutbox({
        pending: 0,
        failed: [{ description: 'Add “milk”', reason: 'not found' }],
        authBlocked: false
      })
    )

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Couldn’t sync 1 change:Add “milk” (not found)'
    )
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))
    expect(dismissFailed).toHaveBeenCalled()
  })
})
