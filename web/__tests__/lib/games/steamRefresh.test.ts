import { renderHook, act } from '@testing-library/react'

const mockTrigger = jest.fn()

jest.mock('@/hooks/useGames', () => ({
  useRefreshSteam: () => mockTrigger
}))

jest.mock('@/lib/env', () => ({
  getApiUrl: jest.fn(() => 'https://api.test')
}))

import { useSteamRefresh } from '@/lib/games/steamRefresh'
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

describe('useSteamRefresh', () => {
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

  it('connects to the progress endpoint and subscribes to steam on open', () => {
    const { result } = renderHook(() => useSteamRefresh())
    expect(latest().url).toBe('wss://api.test/games/api/progress')
    expect(result.current.connected).toBe(false)

    act(() => latest().emitOpen())
    expect(result.current.connected).toBe(true)
    expect(latest().sent).toEqual([JSON.stringify({ subject: 'steam' })])
  })

  it('converts an http api url to a ws url', () => {
    mockGetApiUrl.mockReturnValue('http://localhost:4000/')
    renderHook(() => useSteamRefresh())
    expect(latest().url).toBe('ws://localhost:4000/games/api/progress')
  })

  it('does not open a socket when the api url is empty', () => {
    mockGetApiUrl.mockReturnValue('')
    renderHook(() => useSteamRefresh())
    expect(MockWebSocket.instances).toHaveLength(0)
  })

  it('tracks the refreshing state and last refresh time', () => {
    const { result } = renderHook(() => useSteamRefresh())
    act(() => latest().emitOpen())

    act(() => latest().emit({ isRefreshing: true, lastRefresh: null }))
    expect(result.current.isRefreshing).toBe(true)
    expect(result.current.lastRefresh).toBeNull()

    act(() => latest().emit({ isRefreshing: false, lastRefresh: '2026-01-02T03:04:05Z' }))
    expect(result.current.isRefreshing).toBe(false)
    expect(result.current.lastRefresh).toEqual(new Date('2026-01-02T03:04:05Z'))
  })

  it('invokes onSynced once a refresh transitions from running to finished', () => {
    const onSynced = jest.fn()
    renderHook(() => useSteamRefresh(onSynced))
    act(() => latest().emitOpen())

    act(() => latest().emit({ isRefreshing: true, lastRefresh: null }))
    expect(onSynced).not.toHaveBeenCalled()

    act(() => latest().emit({ isRefreshing: false, lastRefresh: null }))
    expect(onSynced).toHaveBeenCalledTimes(1)
  })

  it('does not invoke onSynced when no refresh was in progress', () => {
    const onSynced = jest.fn()
    renderHook(() => useSteamRefresh(onSynced))
    act(() => latest().emitOpen())
    act(() => latest().emit({ isRefreshing: false, lastRefresh: null }))
    expect(onSynced).not.toHaveBeenCalled()
  })

  it('ignores messages that are not valid state payloads', () => {
    const { result } = renderHook(() => useSteamRefresh())
    act(() => latest().emitOpen())
    act(() => latest().emit({ unexpected: 'shape' }))
    expect(result.current.isRefreshing).toBe(false)
  })

  it('triggers a server refresh when refresh is called', () => {
    const { result } = renderHook(() => useSteamRefresh())
    act(() => result.current.refresh())
    expect(mockTrigger).toHaveBeenCalledTimes(1)
  })

  it('marks the connection as closed when the socket errors', () => {
    const { result } = renderHook(() => useSteamRefresh())
    act(() => latest().emitOpen())
    expect(result.current.connected).toBe(true)

    act(() => latest().onerror?.())
    expect(result.current.connected).toBe(false)
  })

  it('reconnects and re-subscribes after an unexpected close', () => {
    const { result } = renderHook(() => useSteamRefresh())
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
    expect(latest().sent).toEqual([JSON.stringify({ subject: 'steam' })])
  })

  it('delivers a completion that arrives on the reconnected socket', () => {
    const onSynced = jest.fn()
    renderHook(() => useSteamRefresh(onSynced))
    act(() => latest().emitOpen())
    act(() => latest().emit({ isRefreshing: true, lastRefresh: null }))

    // Socket drops mid-sync; the job finishes while disconnected.
    act(() => latest().close())
    act(() => {
      jest.advanceTimersByTime(1500)
    })
    act(() => latest().emitOpen())
    act(() => latest().emit({ isRefreshing: false, lastRefresh: '2026-01-02T03:04:05Z' }))

    expect(onSynced).toHaveBeenCalledTimes(1)
  })

  it('closes the socket and does not reconnect on unmount', () => {
    const { unmount } = renderHook(() => useSteamRefresh())
    act(() => latest().emitOpen())
    const closeSpy = jest.spyOn(latest(), 'close')

    unmount()
    expect(closeSpy).toHaveBeenCalled()

    act(() => {
      jest.advanceTimersByTime(1500)
    })
    expect(MockWebSocket.instances).toHaveLength(1)
  })
})
