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
