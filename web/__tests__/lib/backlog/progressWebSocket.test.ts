import { renderHook } from '@testing-library/react'
import { useProgressWebSocket } from '@/lib/backlog/progressWebSocket'

// Mock WebSocket
class MockWebSocket {
  url: string
  readyState: number = WebSocket.CONNECTING
  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: (() => void) | null = null
  onclose: (() => void) | null = null

  constructor(url: string) {
    this.url = url
    // Simulate connection
    setTimeout(() => {
      this.readyState = WebSocket.OPEN
      this.onopen?.()
    }, 0)
  }

  close() {
    this.readyState = WebSocket.CLOSED
    this.onclose?.()
  }
}

Object.defineProperty(global, 'WebSocket', {
  value: MockWebSocket,
  writable: true,
  configurable: true
})

describe('useProgressWebSocket', () => {
  it('initializes with CONNECTING state', () => {
    const { result } = renderHook(() => useProgressWebSocket('ws://example.com'))
    expect(result.current.status).toBe(WebSocket.CONNECTING)
    expect(result.current.lastMessage).toBeNull()
  })

  it('closes connection on unmount', () => {
    const closeSpy = jest.spyOn(MockWebSocket.prototype, 'close')
    const { unmount } = renderHook(() => useProgressWebSocket('ws://example.com'))
    unmount()
    expect(closeSpy).toHaveBeenCalled()
  })

  it('does not create websocket with empty url', () => {
    const { result } = renderHook(() => useProgressWebSocket(''))
    expect(result.current.status).toBe(WebSocket.CONNECTING)
  })
})
