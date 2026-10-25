import { renderHook, act } from '@testing-library/react'
import { useAsyncAction } from '@/hooks/useAsyncAction'

describe('useAsyncAction', () => {
  it('runs the action, reports success and leaves no error', async () => {
    const action = vi.fn().mockResolvedValue('ok')
    const { result } = renderHook(() => useAsyncAction())

    let ok: boolean | undefined
    await act(async () => {
      ok = await result.current.run(action, "Couldn't save.")
    })

    expect(action).toHaveBeenCalledTimes(1)
    expect(ok).toBe(true)
    expect(result.current.error).toBe('')
    expect(result.current.pending).toBe(false)
  })

  it('keeps the failure message instead of rejecting', async () => {
    const { result } = renderHook(() => useAsyncAction())

    let ok: boolean | undefined
    await act(async () => {
      ok = await result.current.run(() => Promise.reject(new Error('boom')), "Couldn't save.")
    })

    expect(ok).toBe(false)
    expect(result.current.error).toBe("Couldn't save.")
    expect(result.current.pending).toBe(false)
  })

  it('is pending while the action runs', async () => {
    let resolve!: () => void
    const { result } = renderHook(() => useAsyncAction())

    let running!: Promise<boolean>
    act(() => {
      running = result.current.run(() => new Promise<void>((r) => (resolve = r)), 'x')
    })
    expect(result.current.pending).toBe(true)

    await act(async () => {
      resolve()
      await running
    })
    expect(result.current.pending).toBe(false)
  })

  it('clears the previous error when run again', async () => {
    const { result } = renderHook(() => useAsyncAction())
    await act(async () => {
      await result.current.run(() => Promise.reject(new Error('boom')), "Couldn't save.")
    })
    await act(async () => {
      await result.current.run(() => Promise.resolve(), "Couldn't save.")
    })
    expect(result.current.error).toBe('')
  })

  it('keeps run stable across renders', () => {
    const { result, rerender } = renderHook(() => useAsyncAction())
    const first = result.current.run
    rerender()
    expect(result.current.run).toBe(first)
  })
})
