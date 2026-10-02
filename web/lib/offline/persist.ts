import { unstable_serialize, type Middleware } from 'swr'
import { Code, ConnectError } from '@connectrpc/connect'
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
  '/monitoring/',
  '/user-management/'
]

// SWR passes the key to the fetcher as one argument: a string or a tuple
// starting with the swrKeys path.
function isPersisted(key: unknown): key is string | readonly [string, ...unknown[]] {
  const path: unknown = Array.isArray(key) ? key[0] : key
  return typeof path === 'string' && !NOT_PERSISTED.some((p) => path.startsWith(p))
}

/** True when the request never reached the API (offline, DNS, proxy down). */
export function isNetworkError(err: unknown): boolean {
  if (typeof navigator !== 'undefined' && !navigator.onLine) return true
  if (err instanceof TypeError) return true
  if (err instanceof ConnectError) {
    return err.code === Code.Unavailable || err.cause instanceof TypeError
  }
  return false
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
    void saveEntry(storageKey, data)
    return data
  } catch (err) {
    if (!isNetworkError(err)) throw err
    const entry = await loadEntry(storageKey)
    if (!entry) throw err
    markServedFromCache(entry.savedAt)
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- saved from this same key's fetcher
    return entry.data as T
  }
}

/**
 * Saves every successful fetch to IndexedDB and, when the network is down,
 * resolves with the last saved copy instead of an error. SWR's
 * revalidate-on-reconnect then replaces it with live data.
 */
export const persistMiddleware: Middleware = (useSWRNext) => (key, fetcher, config) =>
  useSWRNext(key, fetcher && ((...args: unknown[]) => fetchWithFallback(args, fetcher)), config)
