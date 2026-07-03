import { renderHook, act } from '@testing-library/react'

const mockTrigger = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useResyncOpenLibrary: () => mockTrigger
}))

jest.mock('@/lib/env', () => ({
  getApiUrl: jest.fn(() => 'https://api.test')
}))

import { useResyncRefresh } from '@/lib/books/resyncRefresh'

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

describe('useResyncRefresh', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    MockWebSocket.instances = []
  })

  it('subscribes to the resync-openlibrary topic on connect', () => {
    renderHook(() => useResyncRefresh())
    act(() => latest().emitOpen())
    expect(latest().sent).toEqual([JSON.stringify({ subject: 'resync-openlibrary' })])
  })

  it('exposes processed and total counts from the server message', () => {
    const { result } = renderHook(() => useResyncRefresh())
    act(() => latest().emitOpen())
    act(() => latest().emit({ isRefreshing: true, lastRefresh: null, processed: 5, total: 20 }))
    expect(result.current.isRefreshing).toBe(true)
    expect(result.current.processed).toBe(5)
    expect(result.current.total).toBe(20)
  })

  it('clears counts and calls onSynced when run completes', () => {
    const onSynced = jest.fn()
    const { result } = renderHook(() => useResyncRefresh(onSynced))
    act(() => latest().emitOpen())
    act(() => latest().emit({ isRefreshing: true, lastRefresh: null, processed: 10, total: 10 }))
    act(() => latest().emit({ isRefreshing: false, lastRefresh: '2026-06-24T12:00:00Z' }))
    expect(result.current.isRefreshing).toBe(false)
    expect(result.current.processed).toBeNull()
    expect(result.current.total).toBeNull()
    expect(onSynced).toHaveBeenCalledTimes(1)
  })

  it('calls the trigger when refresh is invoked', () => {
    const { result } = renderHook(() => useResyncRefresh())
    act(() => result.current.refresh())
    expect(mockTrigger).toHaveBeenCalledTimes(1)
  })
})
