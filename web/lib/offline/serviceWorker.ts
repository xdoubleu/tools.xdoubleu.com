// The service worker, served by app/sw.js/route.ts as
// `(${serviceWorker})(self, enabled)`. It must stay self-contained: no
// imports or outer references survive the toString().

interface SwEvent {
  waitUntil(promise: Promise<unknown>): void
}

export interface SwFetchEvent extends SwEvent {
  request: Request
  respondWith(response: Promise<Response>): void
}

export interface SwMessageEvent extends SwEvent {
  data: unknown
}

interface SwCache {
  match(request: RequestInfo | URL): Promise<Response | undefined>
  put(request: RequestInfo | URL, response: Response): Promise<void>
  keys(): Promise<readonly Request[]>
  delete(request: RequestInfo | URL): Promise<boolean>
}

export interface SwScope {
  location: { origin: string }
  caches: {
    open(name: string): Promise<SwCache>
    keys(): Promise<string[]>
    delete(name: string): Promise<boolean>
  }
  registration: { unregister(): Promise<boolean> }
  clients: { claim(): Promise<void> }
  skipWaiting(): Promise<void>
  fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response>
  addEventListener(type: 'install' | 'activate', listener: (event: SwEvent) => void): void
  addEventListener(type: 'fetch', listener: (event: SwFetchEvent) => void): void
  addEventListener(type: 'message', listener: (event: SwMessageEvent) => void): void
}

/** Cache holding saved pages; lib/offline/session.ts clears it on sign-out. */
export const PAGE_CACHE = 'tools-pages'

/**
 * Navigations go network-first and fall back to the last saved copy of the
 * page; `/_next/static` (content-hashed) is cache-first. With `enabled` false
 * it deletes its caches and unregisters itself (the kill switch).
 */
export function serviceWorker(sw: SwScope, enabled: boolean): void {
  const PREFIX = 'tools-'
  const PAGES = 'tools-pages'
  const STATIC = 'tools-static'
  const MAX_PAGES = 100
  const MAX_STATIC = 500
  // A page saved this recently isn't re-fetched on a 'save-page' message.
  const SAVE_INTERVAL_MS = 10 * 60 * 1000
  const ONLINE_ONLY = [
    '/api/',
    '/auth',
    '/oauth',
    '/monitoring',
    '/user-management',
    '/logs',
    '/metrics',
    '/release',
    '/health',
    '/sentry-tunnel',
    '/downloads/',
    '/sw.js'
  ]

  const deleteCaches = async (keep: string[]) => {
    const names = await sw.caches.keys()
    await Promise.all(
      names.filter((n) => n.startsWith(PREFIX) && !keep.includes(n)).map((n) => sw.caches.delete(n))
    )
  }

  sw.addEventListener('install', (event) => event.waitUntil(sw.skipWaiting()))

  if (!enabled) {
    sw.addEventListener('activate', (event) =>
      event.waitUntil(deleteCaches([]).then(() => sw.registration.unregister()))
    )
    return
  }

  sw.addEventListener('activate', (event) =>
    event.waitUntil(deleteCaches([PAGES, STATIC]).then(() => sw.clients.claim()))
  )

  // cache.keys() lists oldest writes first.
  const trim = async (cache: SwCache, max: number) => {
    const keys = await cache.keys()
    await Promise.all(keys.slice(0, Math.max(0, keys.length - max)).map((k) => cache.delete(k)))
  }

  const isPage = (url: URL) =>
    url.origin === sw.location.origin && !ONLINE_ONLY.some((p) => url.pathname.startsWith(p))

  const isSavable = (res: Response) =>
    res.status === 200 &&
    !res.redirected &&
    (res.headers.get('Content-Type') ?? '').startsWith('text/html')

  const savePage = async (key: string, res: Response) => {
    const cache = await sw.caches.open(PAGES)
    await cache.put(key, res)
    await trim(cache, MAX_PAGES)
  }

  const offlinePage = () =>
    new Response(
      '<!doctype html><html lang="en"><head><meta charset="utf-8">' +
        '<meta name="viewport" content="width=device-width,initial-scale=1">' +
        '<title>Offline</title><style>:root{color-scheme:light dark}' +
        'body{font-family:system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem}' +
        'a{display:inline-block;min-height:44px;line-height:44px;padding:0 .5rem}</style>' +
        '</head><body><h1>You&#39;re offline</h1>' +
        '<p>This page hasn&#39;t been saved for offline use yet. Open it once while online.</p>' +
        '<p><a href="/">Home</a> · <a href="">Try again</a></p></body></html>',
      { status: 503, headers: { 'Content-Type': 'text/html; charset=utf-8' } }
    )

  const networkFirst = async (event: SwFetchEvent, key: string) => {
    try {
      const res = await sw.fetch(event.request)
      if (isSavable(res)) event.waitUntil(savePage(key, res.clone()))
      return res
    } catch {
      const cached = await (await sw.caches.open(PAGES)).match(key)
      return cached ?? offlinePage()
    }
  }

  const cacheFirst = async (request: Request, name: string, max: number) => {
    const cache = await sw.caches.open(name)
    const hit = await cache.match(request)
    if (hit) return hit
    const res = await sw.fetch(request)
    if (res.status === 200) {
      await cache.put(request, res.clone())
      await trim(cache, max)
    }
    return res
  }

  // Client-side navigations never reach the fetch handler as a document
  // request, so the page asks for its URL to be saved.
  const refreshPage = async (href: string) => {
    const url = new URL(href, sw.location.origin)
    url.hash = ''
    if (!isPage(url)) return
    const key = url.href
    // The worker is stopped when idle, so freshness comes from the saved
    // response's Date header rather than memory.
    const saved = await (await sw.caches.open(PAGES)).match(key)
    if (Date.now() - Date.parse(saved?.headers.get('Date') ?? '') < SAVE_INTERVAL_MS) return
    const res = await sw.fetch(key, { credentials: 'same-origin' })
    if (!isSavable(res)) return
    const html = await res.clone().text()
    await savePage(key, res)
    await cacheAssets(html)
  }

  // A page saved before it was ever opened (a prefetched book's reader) can
  // only hydrate offline with its route's chunks cached too.
  const cacheAssets = async (html: string) => {
    const paths = new Set(html.match(/\/_next\/static\/[^"'\s<>\\)]+/g))
    for (const path of paths) {
      await cacheFirst(new Request(new URL(path, sw.location.origin)), STATIC, MAX_STATIC).catch(
        () => undefined
      )
    }
  }

  sw.addEventListener('fetch', (event) => {
    const { request } = event
    if (request.method !== 'GET') return
    const url = new URL(request.url)
    if (url.origin !== sw.location.origin) return
    if (url.pathname.startsWith('/_next/static/')) {
      event.respondWith(cacheFirst(request, STATIC, MAX_STATIC))
    } else if (request.mode === 'navigate' && isPage(url)) {
      url.hash = ''
      event.respondWith(networkFirst(event, url.href))
    }
  })

  sw.addEventListener('message', (event) => {
    const data: unknown = event.data
    if (typeof data !== 'object' || data === null || !('type' in data)) return
    if (data.type === 'save-page' && 'url' in data && typeof data.url === 'string') {
      event.waitUntil(refreshPage(data.url).catch(() => undefined))
    } else if (data.type === 'clear') {
      event.waitUntil(deleteCaches([STATIC]))
    }
  })
}

export function serviceWorkerScript(enabled: boolean): string {
  return `(${serviceWorker.toString()})(self, ${String(enabled)});\n`
}
