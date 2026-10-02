import { act, render, screen } from '@testing-library/react'
import OfflineBanner from '@/components/offline/OfflineBanner'
import { markLive, markServedFromCache } from '@/lib/offline/status'

function setOnline(value: boolean) {
  Object.defineProperty(navigator, 'onLine', { value, configurable: true })
  window.dispatchEvent(new Event(value ? 'online' : 'offline'))
}

describe('OfflineBanner', () => {
  afterEach(() => {
    act(() => {
      markLive()
      setOnline(true)
    })
    jest.useRealTimers()
  })

  it('renders nothing while online with live data', () => {
    const { container } = render(<OfflineBanner />)
    expect(container).toBeEmptyDOMElement()
  })

  it('shows offline state and the age of the saved data', () => {
    jest.useFakeTimers({ now: 10 * 60_000 })
    render(<OfflineBanner />)

    act(() => setOnline(false))
    expect(screen.getByRole('status')).toHaveTextContent('Offline')

    act(() => markServedFromCache(5 * 60_000))
    expect(screen.getByRole('status')).toHaveTextContent('Offline · showing data saved 5 min ago')

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
})
