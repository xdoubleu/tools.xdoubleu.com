import {
  clearBodyLengthCache,
  loadPublication,
  locatorToPosition,
  overallFraction,
  readiumPreferences,
  resumeLocator,
  sectionIndex,
  tocItems
} from '@/lib/books/readium'

jest.mock(
  '@readium/shared',
  () => {
    class LocatorLocations {
      progression?: number
      totalProgression?: number
      constructor(v: { progression?: number; totalProgression?: number }) {
        Object.assign(this, v)
      }
    }
    class Locator {
      href: string
      type: string
      locations: LocatorLocations
      constructor(v: { href: string; type: string; locations?: LocatorLocations }) {
        this.href = v.href
        this.type = v.type
        this.locations = v.locations ?? new LocatorLocations({})
      }
    }
    return {
      Locator,
      LocatorLocations,
      HttpFetcher: class {},
      Manifest: {
        deserialize: (json: unknown) => (json ? { setSelfLink: jest.fn(), json } : undefined)
      },
      Publication: class {
        manifest: unknown
        constructor(v: { manifest: unknown }) {
          this.manifest = v.manifest
        }
      }
    }
  },
  { virtual: true }
)

const mockWebPubFetch = jest.fn()
jest.mock('@/lib/books/webpubCache', () => ({
  webPubBaseUrl: (id: string) => `https://api.test/books/api/book/${id}/webpub/`,
  webPubFetch: (...args: unknown[]) => mockWebPubFetch(...args)
}))

interface FakeLink {
  href: string
  type?: string
  title?: string
  children?: { items: FakeLink[] }
}

const ch1: FakeLink = { href: 'res/OEBPS/ch1.xhtml', type: 'application/xhtml+xml' }
const ch2: FakeLink = { href: 'res/OEBPS/ch2.xhtml' }
let bodies: Record<string, string> = {}

function fakePub(): never {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- a minimal Publication
  return {
    baseURL: 'https://api.test/books/api/book/b/webpub/',
    readingOrder: { items: [ch1, ch2] },
    get: (link: FakeLink) => ({
      readAsString: () => {
        const body = bodies[link.href]
        return body === undefined ? Promise.reject(new Error('gone')) : Promise.resolve(body)
      }
    })
  } as never
}

const xhtml = (text: string) =>
  `<html xmlns="http://www.w3.org/1999/xhtml"><body>${text}</body></html>`

beforeEach(() => {
  bodies = {}
  clearBodyLengthCache()
})

describe('readiumPreferences', () => {
  it('maps theme colours and the font-size percent', () => {
    const prefs = readiumPreferences('dark', 120)
    expect(prefs.fontSize).toBeCloseTo(1.2)
    expect(prefs.backgroundColor).toBe('#161616')
    expect(prefs.scroll).toBe(false)
  })
})

describe('tocItems', () => {
  it('maps links, nesting and missing hrefs', () => {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- minimal links
    const links = [
      {
        href: 'res/a.xhtml',
        title: 'A',
        children: { items: [{ href: 'res/b.xhtml#x', title: 'B' }] }
      },
      { href: '', title: 'Group' }
    ] as never
    const items = tocItems(links)
    expect(items[0]).toMatchObject({ label: 'A', href: 'res/a.xhtml' })
    expect(items[0]!.subitems![0]).toMatchObject({ label: 'B', href: 'res/b.xhtml#x' })
    expect(items[1]).toMatchObject({ label: 'Group', href: undefined })
    expect(tocItems(undefined)).toEqual([])
  })
})

// eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- a minimal Response
const response = (init: object) => init as Response

describe('loadPublication', () => {
  it('fetches the manifest through the offline-aware fetch', async () => {
    mockWebPubFetch.mockResolvedValue(response({ ok: true, json: () => Promise.resolve({ a: 1 }) }))
    await loadPublication('b')
    expect(mockWebPubFetch).toHaveBeenCalledWith(
      'https://api.test/books/api/book/b/webpub/manifest.json'
    )
  })

  it('rejects on a failed request or an invalid manifest', async () => {
    mockWebPubFetch.mockResolvedValueOnce(response({ ok: false, status: 404 }))
    await expect(loadPublication('b')).rejects.toThrow('404')
    mockWebPubFetch.mockResolvedValueOnce(response({ ok: true, json: () => Promise.resolve(null) }))
    await expect(loadPublication('b')).rejects.toThrow('invalid manifest')
  })
})

describe('locators', () => {
  const locator = (href: string, progression?: number, totalProgression?: number) =>
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- test locator
    ({ href, type: 'x', locations: { progression, totalProgression } }) as never

  it('finds the section index ignoring fragments', () => {
    expect(sectionIndex(fakePub(), 'res/OEBPS/ch2.xhtml#frag')).toBe(1)
    expect(sectionIndex(fakePub(), 'res/other.xhtml')).toBe(-1)
  })

  it('prefers Readium total progression, else derives it from the section', () => {
    expect(overallFraction(fakePub(), locator('res/OEBPS/ch1.xhtml', 0.5, 0.3))).toBe(0.3)
    expect(overallFraction(fakePub(), locator('res/OEBPS/ch2.xhtml', 0.5))).toBe(0.75)
    expect(overallFraction(fakePub(), locator('res/unknown.xhtml', 0.5))).toBe(0)
  })

  it('maps progression to a text offset in the section body', async () => {
    bodies['res/OEBPS/ch1.xhtml'] = xhtml('0123456789')
    await expect(
      locatorToPosition(fakePub(), locator('res/OEBPS/ch1.xhtml', 0.5))
    ).resolves.toEqual({
      href: 'OEBPS/ch1.xhtml',
      offset: 5
    })
    await expect(
      locatorToPosition(fakePub(), locator('res/nope.xhtml', 0.5))
    ).resolves.toBeUndefined()
  })

  it('falls back to the section start when the body cannot be read', async () => {
    await expect(
      locatorToPosition(fakePub(), locator('res/OEBPS/ch2.xhtml', 0.5))
    ).resolves.toEqual({
      href: 'OEBPS/ch2.xhtml',
      offset: 0
    })
  })
})

describe('resumeLocator', () => {
  it('opens at the stored text offset', async () => {
    bodies['res/OEBPS/ch2.xhtml'] = xhtml('abcdefghij')
    const loc = await resumeLocator(fakePub(), {
      position: { href: 'OEBPS/ch2.xhtml', offset: 4 },
      percent: 10
    })
    expect(loc?.href).toBe('res/OEBPS/ch2.xhtml')
    expect(loc?.locations.progression).toBeCloseTo(0.4)
  })

  it('clamps the offset and tolerates an unreadable body', async () => {
    bodies['res/OEBPS/ch1.xhtml'] = xhtml('ab')
    const clamped = await resumeLocator(fakePub(), {
      position: { href: 'OEBPS/ch1.xhtml', offset: 99 },
      percent: 0
    })
    expect(clamped?.locations.progression).toBe(1)
    const unreadable = await resumeLocator(fakePub(), {
      position: { href: 'OEBPS/ch2.xhtml', offset: 3 },
      percent: 0
    })
    expect(unreadable?.locations.progression).toBe(0)
  })

  it('falls back to the percent, spread over the sections', async () => {
    const loc = await resumeLocator(fakePub(), { position: { page: 3 }, percent: 75 })
    expect(loc?.href).toBe('res/OEBPS/ch2.xhtml')
    expect(loc?.locations.progression).toBeCloseTo(0.5)
    const end = await resumeLocator(fakePub(), { percent: 100 })
    expect(end?.href).toBe('res/OEBPS/ch2.xhtml')
  })

  it('starts at the beginning without a position or percent', async () => {
    await expect(resumeLocator(fakePub(), undefined)).resolves.toBeUndefined()
    await expect(resumeLocator(fakePub(), { percent: 0 })).resolves.toBeUndefined()
  })
})
