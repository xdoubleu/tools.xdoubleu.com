/**
 * Client-side normalization helpers for grouping catalog books in the resync
 * UI. The logic mirrors the Go implementation in
 * api/apps/books/internal/services/book_matching.go so that grouping is
 * consistent with the backend duplicate-detection heuristics.
 */

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
 * Normalize a book title for grouping. Strips subtitle/series/edition noise
 * (everything after the first ':'/';'/' - ', plus any "(...)"/"[...]"
 * segment) and a leading article, matching the Go normalizeTitle logic.
 */
export function normalizeTitle(s: string): string {
  let stripped = s.split(':')[0] ?? s
  stripped = stripped.split(';')[0] ?? stripped
  stripped = stripped.split(' - ')[0] ?? stripped
  stripped = stripped.replace(PARENTHETICAL_RE, '').trim()
  stripped = stripped.replace(LEADING_ARTICLE_RE, '')
  return normalizeString(stripped)
}

/**
 * Normalize an author name to its last-name token for grouping, matching the
 * Go normalizeAuthor logic:
 *  - "Last, First…" (comma present) → everything before the first comma
 *  - "First… Last"  (no comma)      → the last whitespace-delimited token
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

/**
 * Compute a grouping key for an ISBN-less book entry.
 * Returns null when a key cannot be derived (no title or no authors).
 */
export function isbnLessGroupKey(title: string, authors: readonly string[]): string | null {
  const nt = normalizeTitle(title)
  if (!nt) return null

  // Use only the first author's last name, matching buildSearchQuery behaviour.
  const firstAuthor = authors[0]
  if (!firstAuthor) return null
  const na = normalizeAuthor(firstAuthor)
  if (!na) return null

  return `${nt}\x00${na}`
}
