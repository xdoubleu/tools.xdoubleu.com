import { renderHook, act } from '@testing-library/react'
import { usePaginatedList } from '@/hooks/usePaginatedList'

describe('usePaginatedList', () => {
  it('starts with the initial page', () => {
    // `initial` held stable across re-renders, as a real caller must (see
    // the hook's doc comment) — an inline literal would be a new reference
    // on every state-driven re-render and loop.
    const initial = { items: [1, 2], hasMore: true }
    const { result } = renderHook(() => usePaginatedList(initial, jest.fn()))
    expect(result.current.items).toEqual([1, 2])
    expect(result.current.hasMore).toBe(true)
  })

  it('appends the next page and updates hasMore on loadMore', async () => {
    const fetchPage = jest.fn().mockResolvedValue({ items: [3], hasMore: false })
    const initial = { items: [1, 2], hasMore: true }
    const { result } = renderHook(() => usePaginatedList(initial, fetchPage))

    await act(async () => {
      await result.current.loadMore()
    })

    expect(fetchPage).toHaveBeenCalledWith(2)
    expect(result.current.items).toEqual([1, 2, 3])
    expect(result.current.hasMore).toBe(false)
  })

  it('resets to a new initial page when it changes', () => {
    const { result, rerender } = renderHook(
      ({ initial }: { initial: { items: number[]; hasMore: boolean } }) =>
        usePaginatedList(initial, jest.fn()),
      { initialProps: { initial: { items: [1], hasMore: true } } }
    )
    expect(result.current.items).toEqual([1])

    rerender({ initial: { items: [9], hasMore: false } })
    expect(result.current.items).toEqual([9])
    expect(result.current.hasMore).toBe(false)
  })
})
