import { act, renderHook } from '@testing-library/react'
import { useCopyFeedback } from '@/hooks/useCopyFeedback'

describe('useCopyFeedback', () => {
  const writeText = vi.fn()

  beforeEach(() => {
    vi.useFakeTimers()
    writeText.mockReset().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('copies and reports copied until the reset delay passes', async () => {
    const { result } = renderHook(() => useCopyFeedback(1000))
    let ok = false
    await act(async () => {
      ok = await result.current.copy('hello')
    })
    expect(ok).toBe(true)
    expect(writeText).toHaveBeenCalledWith('hello')
    expect(result.current.copied).toBe(true)
    act(() => vi.advanceTimersByTime(1000))
    expect(result.current.copied).toBe(false)
  })

  it('names the key that was copied', async () => {
    const { result } = renderHook(() => useCopyFeedback())
    await act(async () => {
      await result.current.copy('cmd', 'apps')
    })
    expect(result.current.copiedKey).toBe('apps')
  })

  it('uses an empty key by default', async () => {
    const { result } = renderHook(() => useCopyFeedback())
    await act(async () => {
      await result.current.copy('cmd')
    })
    expect(result.current.copiedKey).toBe('')
  })

  it('restarts the reset delay on a second copy', async () => {
    const { result } = renderHook(() => useCopyFeedback(1000))
    await act(async () => {
      await result.current.copy('a', 'one')
    })
    act(() => vi.advanceTimersByTime(600))
    await act(async () => {
      await result.current.copy('b', 'two')
    })
    act(() => vi.advanceTimersByTime(600))
    expect(result.current.copiedKey).toBe('two')
    act(() => vi.advanceTimersByTime(400))
    expect(result.current.copied).toBe(false)
  })

  it('follows a changed reset delay', async () => {
    const { result, rerender } = renderHook(({ ms }) => useCopyFeedback(ms), {
      initialProps: { ms: 1000 }
    })
    rerender({ ms: 200 })
    await act(async () => {
      await result.current.copy('a')
    })
    act(() => vi.advanceTimersByTime(200))
    expect(result.current.copied).toBe(false)
  })

  it('clears the earlier copied state when a later copy fails', async () => {
    const { result } = renderHook(() => useCopyFeedback())
    await act(async () => {
      await result.current.copy('a', 'one')
    })
    writeText.mockRejectedValueOnce(new Error('denied'))
    await act(async () => {
      await result.current.copy('b', 'two')
    })
    expect(result.current.copiedKey).toBeNull()
  })

  it('cancels the pending reset on unmount', async () => {
    const clear = vi.spyOn(globalThis, 'clearTimeout')
    const { result, unmount } = renderHook(() => useCopyFeedback())
    await act(async () => {
      await result.current.copy('a')
    })
    clear.mockClear()
    unmount()
    expect(clear).toHaveBeenCalled()
    clear.mockRestore()
  })

  it('reports a failed copy', async () => {
    writeText.mockRejectedValueOnce(new Error('denied'))
    const { result } = renderHook(() => useCopyFeedback())
    let ok = true
    await act(async () => {
      ok = await result.current.copy('secret')
    })
    expect(ok).toBe(false)
    expect(result.current.failed).toBe(true)
    expect(result.current.copied).toBe(false)
  })
})
