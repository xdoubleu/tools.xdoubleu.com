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

  it('merges a revalidated first page instead of dropping loaded pages, when the leading items still match', async () => {
    // Simulates a background SWR revalidation: same logical items, but a
    // fresh object per fetch (as a real ConnectRPC response would be) —
    // reference equality alone would treat this as a genuinely new page.
    const fetchPage = jest.fn().mockResolvedValue({ items: [{ id: '3' }], hasMore: false })
    const { result, rerender } = renderHook(
      ({ initial }: { initial: { items: { id: string }[]; hasMore: boolean } }) =>
        usePaginatedList(initial, fetchPage, (a, b) => a.id === b.id),
      { initialProps: { initial: { items: [{ id: '1' }, { id: '2' }], hasMore: true } } }
    )

    await act(async () => {
      await result.current.loadMore()
    })
    expect(result.current.items).toEqual([{ id: '1' }, { id: '2' }, { id: '3' }])

    // Revalidation returns the same leading items (new object references,
    // possibly with refreshed fields) — the loaded third page must survive.
    rerender({ initial: { items: [{ id: '1' }, { id: '2' }], hasMore: true } })
    expect(result.current.items).toEqual([{ id: '1' }, { id: '2' }, { id: '3' }])
  })

  it('still fully resets when the revalidated leading items genuinely differ', async () => {
    const fetchPage = jest.fn().mockResolvedValue({ items: [{ id: '3' }], hasMore: false })
    const { result, rerender } = renderHook(
      ({ initial }: { initial: { items: { id: string }[]; hasMore: boolean } }) =>
        usePaginatedList(initial, fetchPage, (a, b) => a.id === b.id),
      { initialProps: { initial: { items: [{ id: '1' }, { id: '2' }], hasMore: true } } }
    )

    await act(async () => {
      await result.current.loadMore()
    })
    expect(result.current.items).toEqual([{ id: '1' }, { id: '2' }, { id: '3' }])

    // e.g. item "1" was marked read and dropped out of an unread-only list.
    rerender({ initial: { items: [{ id: '2' }], hasMore: false } })
    expect(result.current.items).toEqual([{ id: '2' }])
    expect(result.current.hasMore).toBe(false)
  })

  it('resets to empty when the revalidated first page has no items (#913)', () => {
    const fetchPage = jest.fn()
    const { result, rerender } = renderHook(
      ({ initial }: { initial: { items: { id: string }[]; hasMore: boolean } }) =>
        usePaginatedList(initial, fetchPage, (a, b) => a.id === b.id),
      { initialProps: { initial: { items: [{ id: '1' }], hasMore: false } } }
    )

    rerender({ initial: { items: [], hasMore: false } })
    expect(result.current.items).toEqual([])
  })
})
