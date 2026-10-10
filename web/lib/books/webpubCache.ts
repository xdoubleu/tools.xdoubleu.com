// Offline copy of a book's WebPub (manifest + sections), kept in the Cache API
// beside the blob store. Reads are network-first and refresh the copy; offline
// they fall back to it. Sign-out clears every `tools-` cache via the service
// worker's 'clear' message (lib/offline/session.ts).
import { getApiUrl } from '@/lib/env'

const CACHE = 'tools-webpub'
const WARM_CONCURRENCY = 4

const cacheApi = (): CacheStorage | null => (typeof caches === 'undefined' ? null : caches)

export function webPubBaseUrl(bookId: string): string {
  return `${getApiUrl()}/books/api/book/${bookId}/webpub/`
}

const manifestUrl = (bookId: string) => `${webPubBaseUrl(bookId)}manifest.json`

/** fetch with the session cookie that keeps a copy of each good GET, and serves it when offline. */
export const webPubFetch: typeof fetch = async (input, init) => {
  const store = cacheApi()
  const isGet = !init?.method || init.method === 'GET'
  const request = new Request(input, { ...init, credentials: 'include' })
  try {
    const res = await fetch(request.clone())
    if (store && isGet && res.status === 200) {
      void store.open(CACHE).then((cache) => cache.put(request, res.clone()))
    }
    return res
  } catch (err) {
    const hit = store && isGet ? await (await store.open(CACHE)).match(request) : undefined
    if (hit) return hit
    throw err
  }
}

interface ManifestLinks {
  readingOrder?: { href: string }[]
  resources?: { href: string }[]
}

/** Fetches the manifest and every section and resource into the cache. */
export async function warmWebPub(bookId: string): Promise<void> {
  const res = await webPubFetch(manifestUrl(bookId))
  if (!res.ok) throw new Error(`manifest request failed: ${res.status}`)
  const manifest = (await res.json()) as ManifestLinks // eslint-disable-line @typescript-eslint/no-unsafe-type-assertion -- our own API's manifest
  const hrefs = [...(manifest.readingOrder ?? []), ...(manifest.resources ?? [])].map(
    (l) => new URL(l.href, manifestUrl(bookId)).href
  )
  let next = 0
  await Promise.all(
    Array.from({ length: WARM_CONCURRENCY }, async () => {
      while (next < hrefs.length) {
        const r = await webPubFetch(hrefs[next++]!)
        if (!r.ok) throw new Error(`resource request failed: ${r.status}`)
      }
    })
  )
}

/** Whether the book's manifest is cached. */
export async function isWebPubWarm(bookId: string): Promise<boolean> {
  const store = cacheApi()
  if (!store) return false
  return (await (await store.open(CACHE)).match(manifestUrl(bookId))) !== undefined
}

/** Drops the book's cached manifest and sections. */
export async function evictWebPub(bookId: string): Promise<void> {
  const store = cacheApi()
  if (!store) return
  const cache = await store.open(CACHE)
  const base = webPubBaseUrl(bookId)
  for (const request of await cache.keys()) {
    if (request.url.startsWith(base)) await cache.delete(request)
  }
}

/** Drops every cached WebPub. */
export async function evictAllWebPubs(): Promise<void> {
  await cacheApi()?.delete(CACHE)
}
