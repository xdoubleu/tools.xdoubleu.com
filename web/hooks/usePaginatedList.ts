import { useCallback, useEffect, useRef, useState } from 'react'

interface Page<T> {
  items: T[]
  hasMore: boolean
}

// Wraps an SWR-fetched first page with local "load more" state (not
// useSWRInfinite, so existing SWR wiring is unchanged). `initial` must be
// referentially stable (useMemo) or the sync effect loops. On revalidation,
// `isSameItem` identifies an unchanged leading item so the refreshed first
// page merges in place instead of truncating loaded pages.
export function usePaginatedList<T>(
  initial: Page<T>,
  fetchPage: (offset: number) => Promise<Page<T>>,
  isSameItem: (a: T, b: T) => boolean = (a, b) => a === b
) {
  const [items, setItems] = useState(initial.items)
  const [hasMore, setHasMore] = useState(initial.hasMore)
  const [loading, setLoading] = useState(false)

  // Ref, so callers can pass an inline comparator.
  const isSameItemRef = useRef(isSameItem)
  isSameItemRef.current = isSameItem

  useEffect(() => {
    setItems((prev) => {
      const lead = prev.slice(0, initial.items.length)
      // An empty first page always resets, or stale items would linger.
      const sameLeadingPage =
        initial.items.length > 0 &&
        lead.length === initial.items.length &&
        lead.every((item, i) => isSameItemRef.current(item, initial.items[i]))
      return sameLeadingPage
        ? [...initial.items, ...prev.slice(initial.items.length)]
        : initial.items
    })
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
