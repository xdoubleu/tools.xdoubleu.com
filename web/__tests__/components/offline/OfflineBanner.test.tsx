import { act, fireEvent, render, screen } from '@testing-library/react'
import OfflineBanner from '@/components/offline/OfflineBanner'
import { markLive, markServedFromCache } from '@/lib/offline/status'
import { dismissFailed, getOutboxSnapshot, type OutboxState } from '@/lib/offline/outbox'

const listeners = new Set<() => void>()
let mockOutbox: OutboxState = { pending: 0, failed: [], authBlocked: false }

vi.mock('@/lib/offline/outbox', () => ({
  subscribeOutbox: (l: () => void) => {
    listeners.add(l)
    return () => listeners.delete(l)
  },
  getOutboxSnapshot: vi.fn(() => mockOutbox),
  getOutboxServerSnapshot: () => ({ pending: 0, failed: [], authBlocked: false }),
  dismissFailed: vi.fn(async () => {})
}))

function setOutbox(next: OutboxState) {
  mockOutbox = next
  vi.mocked(getOutboxSnapshot).mockImplementation(() => mockOutbox)
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
    vi.useRealTimers()
  })

  it('renders nothing while online with live data and nothing queued', () => {
    const { container } = render(<OfflineBanner />)
    expect(container).toBeEmptyDOMElement()
  })

  it('shows offline state, queued writes and the age of the saved data', () => {
    vi.useFakeTimers({ now: 10 * 60_000 })
    render(<OfflineBanner />)

    act(() => setOnline(false))
    expect(screen.getByRole('status')).toHaveTextContent('Offline')

    act(() => {
      markServedFromCache(5 * 60_000)
      setOutbox({ pending: 2, failed: [], authBlocked: false })
    })
    act(() => vi.advanceTimersByTime(2_000))
    expect(screen.getByRole('status')).toHaveTextContent(
      'Offline · 2 changes waiting to sync · showing data saved 5 min ago'
    )

    act(() => vi.advanceTimersByTime(60_000))
    expect(screen.getByRole('status')).toHaveTextContent('6 min ago')
  })

  it('explains an unreachable server while the browser is online', () => {
    render(<OfflineBanner />)

    act(() => markServedFromCache(Date.now()))

    expect(screen.getByRole('status')).toHaveTextContent(
      'Can’t reach the server · showing data saved just now'
    )
  })

  it('stays quiet while a write is sent right away', () => {
    vi.useFakeTimers()
    const { container } = render(<OfflineBanner />)

    act(() => setOutbox({ pending: 1, failed: [], authBlocked: false }))
    act(() => vi.advanceTimersByTime(1_000))
    act(() => setOutbox({ pending: 0, failed: [], authBlocked: false }))
    act(() => vi.advanceTimersByTime(5_000))

    expect(container).toBeEmptyDOMElement()
  })

  it('shows writes still waiting to sync while online', () => {
    vi.useFakeTimers()
    render(<OfflineBanner />)

    act(() => setOutbox({ pending: 1, failed: [], authBlocked: false }))
    act(() => vi.advanceTimersByTime(2_000))

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
