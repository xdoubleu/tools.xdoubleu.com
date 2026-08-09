import { useCallback, useEffect, useRef, useState } from 'react'

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
//
// The effect still fires on every genuine revalidation of that SWR data
// though — a background refetch (focus/reconnect) or any mutation
// elsewhere that invalidates the same key — even when the page's content
// hasn't actually changed, since the underlying client hands back a fresh
// response object each fetch. Naively replacing `items` with just that
// first page on every such refire would truncate away any pages already
// appended via loadMore, visibly yanking a scrolled-down list back to page
// one. `isSameItem` lets a caller identify "this is really the same
// leading item, just refreshed" (e.g. by id) so that case merges the
// refreshed first page in place instead of discarding later pages — a
// genuinely different leading page (filter switch, item dropped out of a
// filtered list, ...) still fully resets.
export function usePaginatedList<T>(
  initial: Page<T>,
  fetchPage: (offset: number) => Promise<Page<T>>,
  isSameItem: (a: T, b: T) => boolean = (a, b) => a === b
) {
  const [items, setItems] = useState(initial.items)
  const [hasMore, setHasMore] = useState(initial.hasMore)
  const [loading, setLoading] = useState(false)

  // Read via ref rather than a dependency so callers can pass an inline
  // comparator without memoizing it — an unstable dependency here would
  // re-trigger the effect (and rebuild `items`) on every render.
  const isSameItemRef = useRef(isSameItem)
  isSameItemRef.current = isSameItem

  useEffect(() => {
    setItems((prev) => {
      const lead = prev.slice(0, initial.items.length)
      // An empty first page is never "the same leading page" — every item is
      // gone (last unread item read, filter matched nothing), so keeping the
      // stale ones would leave them on screen forever (issue #913).
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
