import {
  getServerSnapshot,
  getSnapshot,
  markLive,
  markServedFromCache,
  subscribe
} from '@/lib/offline/status'

function setOnline(value: boolean) {
  Object.defineProperty(navigator, 'onLine', { value, configurable: true })
  window.dispatchEvent(new Event(value ? 'online' : 'offline'))
}

describe('offline status', () => {
  afterEach(() => {
    markLive()
    setOnline(true)
  })

  it('tracks connectivity while subscribed', () => {
    const listener = jest.fn()
    const unsubscribe = subscribe(listener)

    setOnline(false)
    expect(getSnapshot().offline).toBe(true)
    expect(listener).toHaveBeenCalledTimes(1)

    setOnline(true)
    expect(getSnapshot().offline).toBe(false)

    unsubscribe()
    setOnline(false)
    expect(getSnapshot().offline).toBe(false)
  })

  it('keeps the oldest served-from-cache time until a live fetch', () => {
    const listener = jest.fn()
    const unsubscribe = subscribe(listener)

    markServedFromCache(200)
    markServedFromCache(100)
    markServedFromCache(300)
    expect(getSnapshot().servedFromCacheAt).toBe(100)

    markLive()
    expect(getSnapshot().servedFromCacheAt).toBeNull()
    markLive()
    expect(listener).toHaveBeenCalledTimes(3)
    unsubscribe()
  })

  it('reports online during server rendering', () => {
    expect(getServerSnapshot()).toEqual({ offline: false, servedFromCacheAt: null })
  })
})
