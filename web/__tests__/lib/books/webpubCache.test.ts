/** @jest-environment node */
import {
  evictAllWebPubs,
  evictWebPub,
  isWebPubWarm,
  warmWebPub,
  webPubBaseUrl,
  webPubFetch
} from '@/lib/books/webpubCache'

jest.mock('@/lib/env', () => ({ getApiUrl: () => 'https://api.test' }))

const store = new Map<string, Response>()
const cache = {
  put: jest.fn(async (req: Request, res: Response) => void store.set(req.url, res)),
  match: jest.fn(async (req: Request | string) =>
    store.get(typeof req === 'string' ? req : req.url)?.clone()
  ),
  keys: jest.fn(async () => [...store.keys()].map((url) => new Request(url))),
  delete: jest.fn(async (req: Request) => store.delete(req.url))
}
const caches = {
  open: jest.fn(async () => cache),
  delete: jest.fn(async () => true)
}
const flush = () => new Promise((r) => setTimeout(r, 0))

const base = 'https://api.test/books/api/book/b1/webpub/'
const realFetch = global.fetch
const fetchMock = jest.fn()

beforeEach(() => {
  store.clear()
  jest.clearAllMocks()
  Object.defineProperty(globalThis, 'caches', { value: caches, configurable: true })
  global.fetch = fetchMock
})
afterEach(() => {
  global.fetch = realFetch
})

const ok = (body: string) => new Response(body, { status: 200 })

describe('webPubFetch', () => {
  it('sends the session cookie and keeps a copy of each good GET', async () => {
    fetchMock.mockResolvedValue(ok('x'))
    const res = await webPubFetch(`${base}res/a.xhtml`)
    expect(await res.text()).toBe('x')
    expect(fetchMock.mock.calls[0]![0].credentials).toBe('include')
    await flush()
    expect(store.has(`${base}res/a.xhtml`)).toBe(true)
  })

  it('keeps a copy even when the caller reads the body before the cache opens', async () => {
    caches.open.mockImplementationOnce(
      () => new Promise((resolve) => setTimeout(() => resolve(cache), 5))
    )
    fetchMock.mockResolvedValue(ok('x'))
    const res = await webPubFetch(`${base}res/a.xhtml`)
    expect(await res.text()).toBe('x')
    await new Promise((r) => setTimeout(r, 10))
    expect(await store.get(`${base}res/a.xhtml`)?.text()).toBe('x')
  })

  it('ignores a failed cache write', async () => {
    cache.put.mockRejectedValueOnce(new TypeError('quota'))
    fetchMock.mockResolvedValue(ok('x'))
    expect(await (await webPubFetch(`${base}res/a.xhtml`)).text()).toBe('x')
    await flush()
    expect(store.size).toBe(0)
  })

  it('does not keep failed responses or non-GET requests', async () => {
    fetchMock.mockResolvedValueOnce(new Response('no', { status: 404 }))
    await webPubFetch(`${base}res/missing`)
    fetchMock.mockResolvedValueOnce(ok('x'))
    await webPubFetch(`${base}res/head`, { method: 'HEAD' })
    await flush()
    expect(store.size).toBe(0)
  })

  it('serves the kept copy when the network is down, else rethrows', async () => {
    fetchMock.mockResolvedValueOnce(ok('kept'))
    await webPubFetch(`${base}res/a.xhtml`)
    await flush()
    fetchMock.mockRejectedValue(new TypeError('offline'))
    expect(await (await webPubFetch(`${base}res/a.xhtml`)).text()).toBe('kept')
    await expect(webPubFetch(`${base}res/other`)).rejects.toThrow('offline')
    await expect(webPubFetch(`${base}res/a.xhtml`, { method: 'HEAD' })).rejects.toThrow('offline')
  })

  it('works without the Cache API', async () => {
    Object.defineProperty(globalThis, 'caches', { value: undefined, configurable: true })
    fetchMock.mockResolvedValueOnce(ok('x'))
    expect((await webPubFetch(`${base}res/a`)).ok).toBe(true)
    fetchMock.mockRejectedValueOnce(new TypeError('offline'))
    await expect(webPubFetch(`${base}res/a`)).rejects.toThrow('offline')
    await expect(isWebPubWarm('b1')).resolves.toBe(false)
    await expect(evictWebPub('b1')).resolves.toBeUndefined()
    await expect(evictAllWebPubs()).resolves.toBeUndefined()
  })
})

describe('warmWebPub', () => {
  const manifest = JSON.stringify({
    links: [{ href: 'manifest.json' }, { href: 'positions.json' }],
    readingOrder: [{ href: 'res/a.xhtml' }, { href: 'res/b.xhtml' }],
    resources: [{ href: 'res/c.css' }]
  })

  it('caches the manifest, position list, sections and resources', async () => {
    fetchMock.mockImplementation(async (req: Request) =>
      req.url.endsWith('manifest.json') ? ok(manifest) : ok('body')
    )
    await warmWebPub('b1')
    await flush()
    expect([...store.keys()].sort()).toEqual(
      [
        `${base}manifest.json`,
        `${base}positions.json`,
        `${base}res/a.xhtml`,
        `${base}res/b.xhtml`,
        `${base}res/c.css`
      ].sort()
    )
    expect(fetchMock).toHaveBeenCalledTimes(5)
    await expect(isWebPubWarm('b1')).resolves.toBe(true)
    await expect(isWebPubWarm('other')).resolves.toBe(false)
  })

  it('copes with a manifest listing nothing', async () => {
    fetchMock.mockResolvedValue(ok('{}'))
    await expect(warmWebPub('b1')).resolves.toBeUndefined()
  })

  it('fails on a bad manifest or resource response', async () => {
    fetchMock.mockResolvedValueOnce(new Response('x', { status: 403 }))
    await expect(warmWebPub('b1')).rejects.toThrow('manifest')
    fetchMock.mockImplementation(async (req: Request) =>
      req.url.endsWith('manifest.json') ? ok(manifest) : new Response('x', { status: 500 })
    )
    await expect(warmWebPub('b1')).rejects.toThrow('resource')
  })
})

describe('eviction', () => {
  it("drops one book's entries, or the whole cache", async () => {
    store.set(`${base}manifest.json`, ok('m'))
    store.set('https://api.test/books/api/book/b2/webpub/manifest.json', ok('m'))
    await evictWebPub('b1')
    expect([...store.keys()]).toEqual(['https://api.test/books/api/book/b2/webpub/manifest.json'])
    await evictAllWebPubs()
    expect(caches.delete).toHaveBeenCalledWith('tools-webpub')
  })

  it('knows the base URL', () => {
    expect(webPubBaseUrl('b1')).toBe(base)
  })
})
