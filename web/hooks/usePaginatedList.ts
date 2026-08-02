import { useCallback, useEffect, useState } from 'react'

export const DEFAULT_PAGE_SIZE = 50

interface Page<T> {
  items: T[]
  hasMore: boolean
}

// Wraps an SWR-fetched first page with client-managed "load more" state.
// Deliberately not useSWRInfinite: the first page still flows through the
// existing SWR hook/SWRFallback wiring unchanged, and later pages are just
// appended locally — no new SWR cache-key scheme to keep in sync.
//
// `initial` must be a stable reference across renders that don't represent a
// genuinely new first page (wrap it in useMemo, keyed off the SWR data) — the
// sync effect below compares it by reference, so a fresh object literal every
// render would re-trigger the effect on every state change loadMore causes
// and loop.
export function usePaginatedList<T>(
  initial: Page<T>,
  fetchPage: (offset: number) => Promise<Page<T>>
) {
  const [items, setItems] = useState(initial.items)
  const [hasMore, setHasMore] = useState(initial.hasMore)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setItems(initial.items)
    setHasMore(initial.hasMore)
  }, [initial.items, initial.hasMore])

  const loadMore = useCallback(async () => {
    setLoading(true)
    try {
      const next = await fetchPage(items.length)
      setItems((prev) => [...prev, ...next.items])
      setHasMore(next.hasMore)
    } finally {
      setLoading(false)
    }
  }, [items.length, fetchPage])

  return { items, hasMore, loading, loadMore }
}
