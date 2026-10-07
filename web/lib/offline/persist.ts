import { unstable_serialize, type Key, type Middleware } from 'swr'
import { isNetworkError } from './network'
import { applyPending } from './outbox'
import { loadEntry, saveEntry } from './store'
import { markLive, markServedFromCache } from './status'

// Key prefixes (first key element) never saved: live data, admin views, and
// short-lived or bulky payloads (presigned URLs, whole-book HTML).
const NOT_PERSISTED = [
  '/release',
  '/trains/journey',
  '/books/kobo/',
  '/books/file',
  '/books/content',
  '/movies/search',
  '/monitoring/',
  '/user-management/'
]

// SWR passes the key to the fetcher as one argument: a string or a tuple
// starting with the swrKeys path.
function isPersisted(key: unknown): key is string | readonly [string, ...unknown[]] {
  const path: unknown = Array.isArray(key) ? key[0] : key
  return typeof path === 'string' && !NOT_PERSISTED.some((p) => path.startsWith(p))
}

// Serialized keys whose latest response was the saved copy.
const cachedKeys = new Set<string>()

/** Whether `key`'s latest response was the saved copy rather than live data. */
export function servedFromCache(key: Key): boolean {
  return cachedKeys.has(unstable_serialize(key))
}

async function fetchWithFallback<T>(
  args: unknown[],
  fetcher: (...args: unknown[]) => T | Promise<T>
): Promise<T> {
  if (!isPersisted(args[0])) return fetcher(...args)
  const storageKey = unstable_serialize(args[0])
  try {
    const data = await fetcher(...args)
    markLive()
    cachedKeys.delete(storageKey)
    void saveEntry(storageKey, data)
    return data
  } catch (err) {
    if (!isNetworkError(err)) throw err
    const entry = await loadEntry(storageKey)
    if (!entry) throw err
    markServedFromCache(entry.savedAt)
    cachedKeys.add(storageKey)
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- saved from this same key's fetcher
    return entry.data as T
  }
}

/**
 * Saves every successful fetch to IndexedDB and, when the network is down,
 * resolves with the last saved copy instead of an error. SWR's
 * revalidate-on-reconnect then replaces it with live data. Queued offline
 * writes are re-applied on top, so they survive refetches until sent.
 */
export const persistMiddleware: Middleware = (useSWRNext) => (key, fetcher, config) =>
  useSWRNext(
    key,
    fetcher &&
      (async (...args: unknown[]) => applyPending(args[0], await fetchWithFallback(args, fetcher))),
    config
  )
