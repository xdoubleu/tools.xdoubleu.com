/** Grouping normalization mirroring api/apps/books/internal/services/book_matching.go. */

/** Normalize a raw string: NFD + strip diacritics, lowercase, alphanumeric only. */
function normalizeString(s: string): string {
  return s
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '')
}

/** Matches a "(...)" or "[...]" segment — series/edition annotations. */
const PARENTHETICAL_RE = /[([][^)\]]*[)\]]/g

const LEADING_ARTICLE_RE = /^(the|an?)\s+/i

/**
 * Matches a volume/edition/part keyword plus number. Requires the keyword so
 * shelf markers like "(Series, #1)" stay stripped.
 */
const VOLUME_NUMBER_RE = /\b(?:volume|vol|book|part|edition|ed)\.?\s*#?\s*(\d+)/gi

/** Returns the volume/edition/part numbers found in s, in order. */
function volumeNumbers(s: string): string[] {
  return [...s.matchAll(VOLUME_NUMBER_RE)].map((m) => m[1] ?? '')
}

/** Volume numbers in raw missing from main (multiset difference), space-joined. */
function lostVolumeNumbers(raw: string, main: string): string {
  const mainCounts = new Map<string, number>()
  for (const n of volumeNumbers(main)) mainCounts.set(n, (mainCounts.get(n) ?? 0) + 1)
  const missing: string[] = []
  for (const n of volumeNumbers(raw)) {
    const count = mainCounts.get(n) ?? 0
    if (count > 0) {
      mainCounts.set(n, count - 1)
      continue
    }
    missing.push(n)
  }
  return missing.join(' ')
}

/**
 * Normalizes a title like Go's normalizeTitle: strips subtitle, "(...)"/"[...]"
 * and a leading article, then re-appends any lost volume number so volumes
 * stay distinct.
 */
export function normalizeTitle(s: string): string {
  let stripped = s.split(':')[0] ?? s
  stripped = stripped.split(';')[0] ?? stripped
  stripped = stripped.split(' - ')[0] ?? stripped
  stripped = stripped.replace(PARENTHETICAL_RE, '').trim()

  const lost = lostVolumeNumbers(s, stripped)
  if (lost) stripped = `${stripped} ${lost}`.trim()

  stripped = stripped.replace(LEADING_ARTICLE_RE, '')
  return normalizeString(stripped)
}

/**
 * Last-name token, like Go's normalizeAuthor: before the first comma if
 * present, else the last word.
 */
export function normalizeAuthor(s: string): string {
  const t = s.trim()
  if (!t) return ''
  let lastName: string
  if (t.includes(',')) {
    lastName = t.split(',')[0] ?? ''
  } else {
    const parts = t.split(/\s+/)
    lastName = parts[parts.length - 1] ?? ''
  }
  return normalizeString(lastName)
}

/** Grouping key for an ISBN-less book, or null without title/authors. */
export function isbnLessGroupKey(title: string, authors: readonly string[]): string | null {
  const nt = normalizeTitle(title)
  if (!nt) return null

  // First author only, matching buildSearchQuery.
  const firstAuthor = authors[0]
  if (!firstAuthor) return null
  const na = normalizeAuthor(firstAuthor)
  if (!na) return null

  return `${nt}\x00${na}`
}
