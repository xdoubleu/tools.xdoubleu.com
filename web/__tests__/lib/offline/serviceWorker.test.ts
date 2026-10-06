/** @jest-environment node */
import {
  serviceWorker,
  serviceWorkerScript,
  type SwFetchEvent,
  type SwMessageEvent,
  type SwScope
} from '@/lib/offline/serviceWorker'

const ORIGIN = 'https://tools.test'

function keyOf(req: RequestInfo | URL): string {
  if (typeof req === 'string') return new URL(req, ORIGIN).href
  return req instanceof URL ? req.href : req.url
}

class FakeCache {
  entries = new Map<string, Response>()
  async match(req: RequestInfo | URL) {
    return this.entries.get(keyOf(req))?.clone()
  }
  async put(req: RequestInfo | URL, res: Response) {
    this.entries.delete(keyOf(req))
    this.entries.set(keyOf(req), res)
  }
  async keys() {
    return [...this.entries.keys()].map((k) => new Request(k))
  }
  async delete(req: RequestInfo | URL) {
    return this.entries.delete(keyOf(req))
  }
}

class FakeCacheStorage {
  stores = new Map<string, FakeCache>()
  async open(name: string) {
    let cache = this.stores.get(name)
    if (!cache) {
      cache = new FakeCache()
      this.stores.set(name, cache)
    }
    return cache
  }
  async keys() {
    return [...this.stores.keys()]
  }
  async delete(name: string) {
    return this.stores.delete(name)
  }
}

type Listener = (event: SwFetchEvent & SwMessageEvent) => void

function setup(enabled = true, readerVersion = 'pin1') {
  const listeners = new Map<string, Listener>()
  const caches = new FakeCacheStorage()
  const fetch = jest.fn<Promise<Response>, [RequestInfo | URL, RequestInit?]>()
  const scope: SwScope = {
    location: { origin: ORIGIN },
    caches,
    registration: { unregister: jest.fn(async () => true) },
    clients: { claim: jest.fn(async () => {}) },
    skipWaiting: jest.fn(async () => {}),
    fetch,
    addEventListener: (type: string, listener: Listener) => listeners.set(type, listener)
  }
  serviceWorker(scope, enabled, readerVersion)

  async function dispatch(type: string, extra: { request?: Request; data?: unknown } = {}) {
    const waits: Promise<unknown>[] = []
    let response: Promise<Response> | undefined
    listeners.get(type)?.({
      request: extra.request ?? new Request(ORIGIN),
      data: extra.data,
      waitUntil: (p) => waits.push(p),
      respondWith: (r) => {
        response = r
      }
    })
    const res = await response
    // waitUntil may be called while the response settles.
    while (waits.length) await waits.shift()
    return res
  }

  return { listeners, caches, fetch, scope, dispatch }
}

function navigate(path: string) {
  const req = new Request(new URL(path, ORIGIN))
  Object.defineProperty(req, 'mode', { value: 'navigate' })
  return { request: req }
}

function html(body = 'page', init: ResponseInit = {}) {
  return new Response(body, {
    headers: { 'Content-Type': 'text/html; charset=utf-8', Date: new Date().toUTCString() },
    ...init
  })
}

describe('serviceWorker', () => {
  it('activates immediately and drops stale caches', async () => {
    const { caches, scope, dispatch } = setup()
    await caches.open('tools-pages')
    await caches.open('tools-old')
    await caches.open('other-app')

    await dispatch('install')
    await dispatch('activate')

    expect(scope.skipWaiting).toHaveBeenCalled()
    expect(scope.clients.claim).toHaveBeenCalled()
    expect(await caches.keys()).toEqual(['tools-pages', 'other-app'])
  })

  it('saves navigations and serves them when the network fails', async () => {
    const { caches, fetch, dispatch } = setup()
    fetch.mockResolvedValueOnce(html('fresh'))

    const online = await dispatch('fetch', navigate('/shoppinglist#top'))
    expect(await online?.text()).toBe('fresh')
    expect(caches.stores.get('tools-pages')?.entries.has(`${ORIGIN}/shoppinglist`)).toBe(true)

    fetch.mockRejectedValueOnce(new TypeError('Failed to fetch'))
    const offline = await dispatch('fetch', navigate('/shoppinglist'))
    expect(await offline?.text()).toBe('fresh')
  })

  it('serves an offline page for a navigation never saved', async () => {
    const { fetch, dispatch } = setup()
    fetch.mockRejectedValue(new TypeError('Failed to fetch'))

    const res = await dispatch('fetch', navigate('/recipes/list'))

    expect(res?.status).toBe(503)
    expect(await res?.text()).toContain('You&#39;re offline')
  })

  it.each([
    ['a redirect', () => Object.defineProperty(html(), 'redirected', { value: true })],
    ['a server error', () => html('boom', { status: 500 })],
    [
      'a non-HTML response',
      () => new Response('{}', { headers: { 'Content-Type': 'application/json' } })
    ]
  ])('does not save %s', async (_, response) => {
    const { caches, fetch, dispatch } = setup()
    fetch.mockResolvedValueOnce(response())

    await dispatch('fetch', navigate('/feeds'))

    expect(caches.stores.get('tools-pages')?.entries.size ?? 0).toBe(0)
  })

  it('keeps only the newest 100 pages', async () => {
    const { caches, fetch, dispatch } = setup()
    fetch.mockImplementation(async () => html())

    for (let i = 0; i <= 100; i++) await dispatch('fetch', navigate(`/recipes/${i}`))

    const keys = [...(caches.stores.get('tools-pages')?.entries.keys() ?? [])]
    expect(keys).toHaveLength(100)
    expect(keys[0]).toBe(`${ORIGIN}/recipes/1`)
  })

  it.each([
    ['an online-only page', navigate('/auth/login')],
    ['a cross-origin request', { request: new Request('https://elsewhere.test/x') }],
    ['a POST', { request: new Request(`${ORIGIN}/api/x`, { method: 'POST' }) }],
    ['a same-origin subresource', { request: new Request(`${ORIGIN}/icon.svg`) }]
  ])('leaves %s to the network', async (_, event) => {
    const { fetch, dispatch } = setup()

    expect(await dispatch('fetch', event)).toBeUndefined()
    expect(fetch).not.toHaveBeenCalled()
  })

  it('serves static assets cache-first', async () => {
    const { fetch, dispatch } = setup()
    const asset = () => ({ request: new Request(`${ORIGIN}/_next/static/chunks/a.js`) })
    fetch.mockResolvedValueOnce(new Response('js'))

    expect(await (await dispatch('fetch', asset()))?.text()).toBe('js')
    expect(await (await dispatch('fetch', asset()))?.text()).toBe('js')
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('does not cache a failed static asset', async () => {
    const { caches, fetch, dispatch } = setup()
    fetch.mockResolvedValueOnce(new Response('', { status: 404 }))

    await dispatch('fetch', { request: new Request(`${ORIGIN}/_next/static/chunks/b.js`) })

    expect(caches.stores.get('tools-static')?.entries.size).toBe(0)
  })

  it('saves a page on request, at most once per interval', async () => {
    const { caches, fetch, dispatch } = setup()
    fetch.mockImplementation(async () => html('saved'))

    await dispatch('message', { data: { type: 'save-page', url: `${ORIGIN}/feeds?x=1#a` } })
    await dispatch('message', { data: { type: 'save-page', url: `${ORIGIN}/feeds?x=1` } })

    expect(fetch).toHaveBeenCalledTimes(1)
    expect(fetch).toHaveBeenCalledWith(`${ORIGIN}/feeds?x=1`, { credentials: 'same-origin' })
    expect(caches.stores.get('tools-pages')?.entries.has(`${ORIGIN}/feeds?x=1`)).toBe(true)
  })

  it('re-saves a page once its saved copy is older than the interval', async () => {
    const { fetch, dispatch } = setup()
    const stale = new Date(Date.now() - 11 * 60_000).toUTCString()
    fetch.mockResolvedValueOnce(
      new Response('old', { headers: { 'Content-Type': 'text/html', Date: stale } })
    )
    fetch.mockImplementation(async () => html('new'))

    await dispatch('message', { data: { type: 'save-page', url: '/feeds' } })
    await dispatch('message', { data: { type: 'save-page', url: '/feeds' } })
    await dispatch('message', { data: { type: 'save-page', url: '/feeds' } })

    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('ignores save requests for online-only pages and swallows fetch failures', async () => {
    const { caches, fetch, dispatch } = setup()
    fetch.mockRejectedValue(new TypeError('Failed to fetch'))

    await dispatch('message', { data: { type: 'save-page', url: '/monitoring' } })
    expect(fetch).not.toHaveBeenCalled()

    await dispatch('message', { data: { type: 'save-page', url: '/feeds' } })
    expect(caches.stores.get('tools-pages')?.entries.size).toBe(0)
  })

  it('ignores malformed messages', async () => {
    const { fetch, dispatch } = setup()

    await dispatch('message', { data: null })
    await dispatch('message', { data: 'clear' })
    await dispatch('message', { data: { type: 'save-page' } })

    expect(fetch).not.toHaveBeenCalled()
  })

  it('serves reader modules cache-first under the pinned version', async () => {
    const { caches, fetch, dispatch } = setup()
    const worker = () => ({
      request: new Request(`${ORIGIN}/foliate-js/vendor/pdfjs/pdf.worker.mjs`)
    })
    fetch.mockResolvedValueOnce(new Response('worker'))

    expect(await (await dispatch('fetch', worker()))?.text()).toBe('worker')
    expect(await (await dispatch('fetch', worker()))?.text()).toBe('worker')
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(caches.stores.get('tools-reader-pin1')?.entries.size).toBe(1)
  })

  it('does not cache a failed reader module', async () => {
    const { caches, fetch, dispatch } = setup()
    fetch.mockResolvedValueOnce(new Response('', { status: 404 }))

    const res = await dispatch('fetch', { request: new Request(`${ORIGIN}/foliate-js/view.js`) })

    expect(res?.status).toBe(404)
    expect(caches.stores.get('tools-reader-pin1')?.entries.size).toBe(0)
  })

  it('drops reader modules of another pin on activate', async () => {
    const { caches, dispatch } = setup(true, 'pin2')
    await caches.open('tools-reader-pin1')
    await caches.open('tools-reader-pin2')

    await dispatch('activate')

    expect(await caches.keys()).toEqual(['tools-reader-pin2'])
  })

  it('clears saved pages but keeps static assets and reader modules', async () => {
    const { caches, fetch, dispatch } = setup()
    fetch.mockImplementation(async () => html())
    await caches.open('tools-static')
    await caches.open('tools-reader-pin1')
    await dispatch('message', { data: { type: 'save-page', url: '/feeds' } })

    await dispatch('message', { data: { type: 'clear' } })
    expect(await caches.keys()).toEqual(['tools-static', 'tools-reader-pin1'])

    // Nothing saved any more, so the next request fetches again.
    await dispatch('message', { data: { type: 'save-page', url: '/feeds' } })
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('unregisters and deletes its caches when disabled', async () => {
    const { caches, scope, listeners, dispatch } = setup(false)
    await caches.open('tools-pages')
    await caches.open('tools-static')

    await dispatch('activate')

    expect(await caches.keys()).toEqual([])
    expect(scope.registration.unregister).toHaveBeenCalled()
    expect(listeners.has('fetch')).toBe(false)
  })
})

describe('serviceWorkerScript', () => {
  it('invokes the worker with the enabled flag and reader version', () => {
    expect(serviceWorkerScript(true, 'abc')).toMatch(/\)\(self, true, "abc"\);\n$/)
    expect(serviceWorkerScript(false, 'abc')).toMatch(/\)\(self, false, "abc"\);\n$/)
  })
})
