import { renderHook, act } from '@testing-library/react'

vi.mock('@/lib/env', () => ({ getApiUrl: vi.fn(() => 'https://api.test') }))

import { useChatSocket } from '@/hooks/useChatSocket'
import { getApiUrl } from '@/lib/env'

class MockWebSocket {
  static instances: MockWebSocket[] = []
  url: string
  onopen: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  onclose: (() => void) | null = null
  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }
  close() {
    this.onclose?.()
  }
  emit(data: unknown) {
    this.onmessage?.({ data: typeof data === 'string' ? data : JSON.stringify(data) })
  }
}
Object.defineProperty(global, 'WebSocket', { value: MockWebSocket, configurable: true })

const latest = () => MockWebSocket.instances[MockWebSocket.instances.length - 1]!
const text = (seq: number, t: string) => ({
  seq,
  kind: 'update',
  payload: JSON.stringify({ sessionUpdate: 'agent_message_chunk', content: { text: t } })
})

beforeEach(() => {
  MockWebSocket.instances = []
  vi.useFakeTimers()
})
afterEach(() => vi.useRealTimers())

describe('useChatSocket', () => {
  it('streams frames and tracks the state', () => {
    const { result } = renderHook(() => useChatSocket('c1'))
    expect(latest().url).toBe('wss://api.test/factory/chats/c1/ws?after=0')
    act(() => latest().onopen?.())
    act(() => {
      latest().emit(text(1, 'Hi'))
      latest().emit('not json')
      latest().emit({ nope: true })
      latest().emit({ seq: 0, kind: 'state', state: 'working' })
    })
    expect(result.current.connected).toBe(true)
    expect(result.current.state).toBe('working')
    expect(result.current.transcript.items).toHaveLength(1)
  })

  it('redials from the cursor after a drop', () => {
    renderHook(() => useChatSocket('c1'))
    act(() => latest().emit(text(7, 'x')))
    act(() => latest().onclose?.())
    act(() => void vi.advanceTimersByTime(2000))
    expect(MockWebSocket.instances).toHaveLength(2)
    expect(latest().url).toContain('after=7')
  })

  it('does not redial once the chat is closed, or after unmount', () => {
    const { unmount } = renderHook(() => useChatSocket('c1'))
    act(() => latest().emit({ seq: 0, kind: 'state', state: 'stopped' }))
    act(() => latest().onclose?.())
    act(() => void vi.advanceTimersByTime(5000))
    expect(MockWebSocket.instances).toHaveLength(1)
    unmount()
  })

  it('drops its pending redial on unmount', () => {
    const { unmount } = renderHook(() => useChatSocket('c1'))
    act(() => latest().onerror?.())
    unmount()
    act(() => void vi.advanceTimersByTime(5000))
    expect(MockWebSocket.instances).toHaveLength(1)
  })

  it('does nothing without an id or api url', () => {
    renderHook(() => useChatSocket(null))
    vi.mocked(getApiUrl).mockReturnValueOnce('')
    renderHook(() => useChatSocket('c1'))
    expect(MockWebSocket.instances).toHaveLength(0)
  })
})
