import { renderHook, act } from '@testing-library/react'
import { useWakeLock } from '@/hooks/useWakeLock'

class MockSentinel {
  released = false
  private listeners: Record<string, (() => void)[]> = {}

  addEventListener(type: string, cb: () => void) {
    this.listeners[type] = [...(this.listeners[type] ?? []), cb]
  }

  removeEventListener() {}

  release = jest.fn(async () => {
    if (this.released) return
    this.released = true
    this.listeners['release']?.forEach((cb) => cb())
  })
}

afterEach(() => {
  Reflect.deleteProperty(navigator, 'wakeLock')
})

describe('useWakeLock', () => {
  it('reports unsupported when the Wake Lock API is unavailable', () => {
    const { result } = renderHook(() => useWakeLock())
    expect(result.current.isSupported).toBe(false)

    act(() => {
      result.current.toggle()
    })
    expect(result.current.isActive).toBe(false)
  })

  it('acquires and releases the wake lock on toggle', async () => {
    const sentinel = new MockSentinel()
    const request = jest.fn().mockResolvedValue(sentinel)
    Object.defineProperty(navigator, 'wakeLock', { value: { request }, configurable: true })

    const { result } = renderHook(() => useWakeLock())
    expect(result.current.isSupported).toBe(true)
    expect(result.current.isActive).toBe(false)

    await act(async () => {
      result.current.toggle()
    })
    expect(request).toHaveBeenCalledWith('screen')
    expect(result.current.isActive).toBe(true)

    await act(async () => {
      result.current.toggle()
    })
    expect(sentinel.release).toHaveBeenCalled()
    expect(result.current.isActive).toBe(false)
  })

  it('stays inactive if the wake lock request rejects', async () => {
    const request = jest.fn().mockRejectedValue(new Error('denied'))
    Object.defineProperty(navigator, 'wakeLock', { value: { request }, configurable: true })

    const { result } = renderHook(() => useWakeLock())
    await act(async () => {
      result.current.toggle()
    })
    expect(result.current.isActive).toBe(false)
  })

  it('re-requests the wake lock when the tab becomes visible again while still wanted', async () => {
    const sentinel1 = new MockSentinel()
    const sentinel2 = new MockSentinel()
    const request = jest.fn().mockResolvedValueOnce(sentinel1).mockResolvedValueOnce(sentinel2)
    Object.defineProperty(navigator, 'wakeLock', { value: { request }, configurable: true })

    const { result } = renderHook(() => useWakeLock())
    await act(async () => {
      result.current.toggle()
    })
    expect(result.current.isActive).toBe(true)

    await act(async () => {
      await sentinel1.release()
    })
    expect(result.current.isActive).toBe(false)

    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
    await act(async () => {
      document.dispatchEvent(new Event('visibilitychange'))
    })
    expect(request).toHaveBeenCalledTimes(2)
    expect(result.current.isActive).toBe(true)
  })

  it('does not re-request the wake lock on visibility change once turned off', async () => {
    const sentinel = new MockSentinel()
    const request = jest.fn().mockResolvedValue(sentinel)
    Object.defineProperty(navigator, 'wakeLock', { value: { request }, configurable: true })

    const { result } = renderHook(() => useWakeLock())
    await act(async () => {
      result.current.toggle()
    })
    await act(async () => {
      result.current.toggle()
    })
    expect(result.current.isActive).toBe(false)

    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
    await act(async () => {
      document.dispatchEvent(new Event('visibilitychange'))
    })
    expect(request).toHaveBeenCalledTimes(1)
  })

  it('releases the wake lock on unmount', async () => {
    const sentinel = new MockSentinel()
    const request = jest.fn().mockResolvedValue(sentinel)
    Object.defineProperty(navigator, 'wakeLock', { value: { request }, configurable: true })

    const { result, unmount } = renderHook(() => useWakeLock())
    await act(async () => {
      result.current.toggle()
    })
    unmount()
    expect(sentinel.release).toHaveBeenCalled()
  })
})
