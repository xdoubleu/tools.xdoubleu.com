import { renderHook, act } from '@testing-library/react'

jest.mock('@/lib/env', () => ({
  getApiUrl: jest.fn(() => 'https://api.test')
}))

import { useJourneyLive } from '@/lib/trains/journeySocket'
import { getApiUrl } from '@/lib/env'

const mockGetApiUrl = jest.mocked(getApiUrl)

class MockWebSocket {
  static instances: MockWebSocket[] = []
  url: string
  sent: string[] = []
  readyState = 0
  onopen: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  onclose: (() => void) | null = null

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  send(data: string) {
    this.sent.push(data)
  }

  close() {
    this.readyState = 3
    this.onclose?.()
  }

  emitOpen() {
    this.readyState = 1
    this.onopen?.()
  }

  emit(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) })
  }
}

Object.defineProperty(global, 'WebSocket', {
  value: MockWebSocket,
  writable: true,
  configurable: true
})

function latest() {
  return MockWebSocket.instances[MockWebSocket.instances.length - 1]!
}

const journeyDetailPayload = {
  legs: [{ tripShortName: 'IC900' }],
  departureTime: '2026-01-01T08:00:00.000Z',
  arrivalTime: '2026-01-01T09:00:00.000Z'
}

describe('useJourneyLive', () => {
  beforeEach(() => {
    jest.useFakeTimers()
    jest.clearAllMocks()
    MockWebSocket.instances = []
    mockGetApiUrl.mockReturnValue('https://api.test')
  })

  afterEach(() => {
    jest.clearAllTimers()
    jest.useRealTimers()
  })

  it('connects to the journeys/live endpoint and subscribes to the journey id', () => {
    const refetch = jest.fn().mockResolvedValue(undefined)
    const { result } = renderHook(() => useJourneyLive('journey-1', refetch))

    expect(latest().url).toBe('wss://api.test/trains/api/journeys/live')
    expect(result.current.connected).toBe(false)

    act(() => latest().emitOpen())
    expect(result.current.connected).toBe(true)
    expect(latest().sent).toEqual([JSON.stringify({ subject: 'journey-1' })])
  })

  it('does not open a socket without an api url or journey id', () => {
    mockGetApiUrl.mockReturnValue('')
    renderHook(() => useJourneyLive('journey-1', jest.fn()))
    expect(MockWebSocket.instances).toHaveLength(0)
  })

  it('stores a pushed journey detail message', () => {
    const { result } = renderHook(() => useJourneyLive('journey-1', jest.fn()))
    act(() => latest().emitOpen())
    act(() => latest().emit(journeyDetailPayload))

    expect(result.current.pushedDetail?.legs).toHaveLength(1)
  })

  it('ignores a message that is not a journey detail shape', () => {
    const { result } = renderHook(() => useJourneyLive('journey-1', jest.fn()))
    act(() => latest().emitOpen())
    act(() => latest().emit({ unexpected: 'shape' }))
    expect(result.current.pushedDetail).toBeNull()
  })

  it('ignores unparseable message data', () => {
    const { result } = renderHook(() => useJourneyLive('journey-1', jest.fn()))
    act(() => latest().emitOpen())
    act(() => latest().onmessage?.({ data: 'not-json' }))
    expect(result.current.pushedDetail).toBeNull()
  })

  it('reconnects and re-subscribes after an unexpected close', () => {
    const { result } = renderHook(() => useJourneyLive('journey-1', jest.fn()))
    act(() => latest().emitOpen())

    act(() => latest().close())
    expect(result.current.connected).toBe(false)
    expect(MockWebSocket.instances).toHaveLength(1)

    act(() => {
      jest.advanceTimersByTime(1500)
    })
    expect(MockWebSocket.instances).toHaveLength(2)

    act(() => latest().emitOpen())
    expect(result.current.connected).toBe(true)
  })

  it('marks the connection as closed when the socket errors', () => {
    const { result } = renderHook(() => useJourneyLive('journey-1', jest.fn()))
    act(() => latest().emitOpen())
    act(() => latest().onerror?.())
    expect(result.current.connected).toBe(false)
  })

  // The core reconnect-after-sleep requirement (issue #1394): the page
  // becoming visible again must force a fresh socket AND refetch, not just
  // wait for the next push.
  it('forces a reconnect and refetch when the page becomes visible again', () => {
    const refetch = jest.fn().mockResolvedValue(undefined)
    renderHook(() => useJourneyLive('journey-1', refetch))
    act(() => latest().emitOpen())
    const staleSocket = latest()

    Object.defineProperty(document, 'visibilityState', {
      value: 'visible',
      configurable: true
    })
    act(() => {
      document.dispatchEvent(new Event('visibilitychange'))
    })

    expect(staleSocket.readyState).toBe(3)
    expect(MockWebSocket.instances).toHaveLength(2)
    expect(refetch).toHaveBeenCalledTimes(1)
  })

  it('does not reconnect on a visibilitychange while still hidden', () => {
    const refetch = jest.fn().mockResolvedValue(undefined)
    renderHook(() => useJourneyLive('journey-1', refetch))
    act(() => latest().emitOpen())

    Object.defineProperty(document, 'visibilityState', {
      value: 'hidden',
      configurable: true
    })
    act(() => {
      document.dispatchEvent(new Event('visibilitychange'))
    })

    expect(MockWebSocket.instances).toHaveLength(1)
    expect(refetch).not.toHaveBeenCalled()
  })

  it('forces a reconnect and refetch on pageshow (e.g. bfcache restore)', () => {
    const refetch = jest.fn().mockResolvedValue(undefined)
    renderHook(() => useJourneyLive('journey-1', refetch))
    act(() => latest().emitOpen())

    act(() => {
      window.dispatchEvent(new Event('pageshow'))
    })

    expect(MockWebSocket.instances).toHaveLength(2)
    expect(refetch).toHaveBeenCalledTimes(1)
  })

  it('forces a reconnect and refetch when the network comes back online', () => {
    const refetch = jest.fn().mockResolvedValue(undefined)
    renderHook(() => useJourneyLive('journey-1', refetch))
    act(() => latest().emitOpen())

    act(() => {
      window.dispatchEvent(new Event('online'))
    })

    expect(MockWebSocket.instances).toHaveLength(2)
    expect(refetch).toHaveBeenCalledTimes(1)
  })

  it('closes the socket and removes listeners on unmount', () => {
    const { unmount } = renderHook(() => useJourneyLive('journey-1', jest.fn()))
    act(() => latest().emitOpen())
    const closeSpy = jest.spyOn(latest(), 'close')

    unmount()
    expect(closeSpy).toHaveBeenCalled()

    act(() => {
      window.dispatchEvent(new Event('pageshow'))
    })
    expect(MockWebSocket.instances).toHaveLength(1)
  })
})
