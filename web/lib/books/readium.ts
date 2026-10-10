// Readium ts-toolkit glue: opens the API's WebPub manifest (api/apps/books
// webpub routes) and maps between Readium locators and the neutral
// `{href, offset}` positions of adr-0029. Browser-only.
import { HttpFetcher, Locator, LocatorLocations, Manifest, Publication } from '@readium/shared'
import type { Link } from '@readium/shared'
import { getApiUrl } from '@/lib/env'
import type { FoliateTocItem } from './foliate'
import type { ReaderPosition, ReaderResume } from './readerPosition'
import { readerBackground, readerTextColors, type ReaderTheme } from './readerSettings'

/** Resources are served under `res/<zip path>`, relative to the manifest. */
const RESOURCE_PREFIX = 'res/'

function webPubBaseUrl(bookId: string): string {
  return `${getApiUrl()}/books/api/book/${bookId}/webpub/`
}

const withCredentials: typeof fetch = (input, init) =>
  fetch(input, { ...init, credentials: 'include' })

/** Fetches the manifest and wraps it in a Publication that reads sections over HTTP. */
export async function loadPublication(bookId: string): Promise<Publication> {
  const base = webPubBaseUrl(bookId)
  const res = await withCredentials(`${base}manifest.json`)
  if (!res.ok) throw new Error(`manifest request failed: ${res.status}`)
  const manifest = Manifest.deserialize(await res.json())
  if (!manifest) throw new Error('invalid manifest')
  manifest.setSelfLink(`${base}manifest.json`)
  return new Publication({ manifest, fetcher: new HttpFetcher(withCredentials, base) })
}

/** Navigator preferences for the reader's theme and font-size percent. */
export function readiumPreferences(theme: ReaderTheme, fontSize: number) {
  const { fg, link } = readerTextColors(theme)
  return {
    backgroundColor: readerBackground(theme),
    textColor: fg,
    linkColor: link,
    visitedColor: link,
    fontSize: fontSize / 100,
    scroll: false
  }
}

/** The TOC in the shape the contents drawer renders. */
export function tocItems(links: readonly Link[] | undefined): FoliateTocItem[] {
  return (links ?? []).map((l) => ({
    label: l.title ?? '',
    href: l.href || undefined,
    subitems: l.children ? tocItems(l.children.items) : undefined
  }))
}

const stripFragment = (href: string) => href.split('#')[0]!

/** The zip-root path of a manifest href, as stored in neutral positions. */
function zipPath(href: string): string {
  return stripFragment(href).replace(RESOURCE_PREFIX, '')
}

/** Reading-order index of a locator's section, or -1. */
export function sectionIndex(pub: Publication, href: string): number {
  const target = stripFragment(href)
  return pub.readingOrder.items.findIndex((l) => stripFragment(l.href) === target)
}

const bodyLengths = new Map<string, Promise<number>>()

/** Length of a section body's textContent, which neutral offsets count in. */
/** Forgets cached body lengths (tests). */
export function clearBodyLengthCache() {
  bodyLengths.clear()
}

function bodyTextLength(pub: Publication, link: Link): Promise<number> {
  const key = `${pub.baseURL ?? ''}${link.href}`
  let cached = bodyLengths.get(key)
  if (!cached) {
    cached = pub
      .get(link)
      .readAsString()
      .then((text) => {
        const doc = new DOMParser().parseFromString(text ?? '', 'application/xhtml+xml')
        return doc.body?.textContent?.length ?? 0
      })
    bodyLengths.set(key, cached)
    cached.catch(() => bodyLengths.delete(key))
  }
  return cached
}

/** Overall progress in 0..1: Readium's own when it knows the positions, else by section. */
export function overallFraction(pub: Publication, locator: Locator): number {
  const total = locator.locations.totalProgression
  if (total !== undefined) return total
  const n = pub.readingOrder.items.length
  const index = sectionIndex(pub, locator.href)
  if (n === 0 || index < 0) return 0
  return (index + (locator.locations.progression ?? 0)) / n
}

/** The neutral position of a locator; progression is mapped through the body length. */
export async function locatorToPosition(
  pub: Publication,
  locator: Locator
): Promise<ReaderPosition | undefined> {
  const index = sectionIndex(pub, locator.href)
  const link = pub.readingOrder.items[index]
  if (!link) return undefined
  let offset = 0
  try {
    offset = Math.round((locator.locations.progression ?? 0) * (await bodyTextLength(pub, link)))
  } catch {
    // Fall back to the section start.
  }
  return { href: zipPath(link.href), offset }
}

function locatorAt(link: Link, progression: number): Locator {
  return new Locator({
    href: link.href,
    type: link.type ?? 'application/xhtml+xml',
    locations: new LocatorLocations({ progression })
  })
}

/** Where to reopen: the stored position's section and offset, else the stored percent. */
export async function resumeLocator(
  pub: Publication,
  resume: ReaderResume | undefined
): Promise<Locator | undefined> {
  const items = pub.readingOrder.items
  const position = resume?.position
  if (position && 'href' in position) {
    const link = items.find((l) => zipPath(l.href) === position.href)
    if (link) {
      let progression = 0
      try {
        const length = await bodyTextLength(pub, link)
        progression = length > 0 ? Math.min(1, position.offset / length) : 0
      } catch {
        // Open at the section start.
      }
      return locatorAt(link, progression)
    }
  }
  const percent = resume?.percent ?? 0
  if (percent <= 0 || items.length === 0) return undefined
  const scaled = Math.min(percent / 100, 0.9999) * items.length
  const link = items[Math.floor(scaled)]
  return link ? locatorAt(link, scaled - Math.floor(scaled)) : undefined
}
