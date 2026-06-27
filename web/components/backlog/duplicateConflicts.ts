/**
 * Utilities for detecting per-field catalog conflicts within a duplicate group
 * and building the resolved-metadata payload for MergeBooks.
 *
 * Mirrors the fields scored by metadataCompleteness in book_matching.go —
 * cover is detected by presence (proxy URLs differ by bookId so cannot be
 * compared directly; use resolvedCoverSourceBookId on the proto request).
 */

import { create } from '@bufbuild/protobuf'
import type { Book } from '@/lib/gen/backlog/v1/books_pb'
import { BookSchema } from '@/lib/gen/backlog/v1/books_pb'

// ---------------------------------------------------------------------------
// Duck-typed interfaces (avoids importing branded proto Message types so tests
// can pass plain fixture objects without unsafe assertions)
// ---------------------------------------------------------------------------

interface DupBook {
  id: string
  title: string
  authors: string[]
  isbn13: string
  isbn10: string
  coverUrl: string
  description: string
  pageCount: number
  externalRefs: Record<string, string>
}

interface DupEntry {
  bookId: string
  book?: DupBook | null
}

export interface DupGroup {
  entries: DupEntry[]
  reason: string
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type BookConflictField =
  | 'title'
  | 'authors'
  | 'isbn13'
  | 'isbn10'
  | 'description'
  | 'pageCount'
  | 'externalRefs'
  | 'cover'

export interface FieldChoice {
  /** bookId of the UserBook entry whose value wins for this field. */
  bookId: string
  /** Human-readable value for display in the picker. */
  displayValue: string
  /** Whether this entry actually has a value for the field. */
  hasValue: boolean
}

export interface FieldConflict {
  field: BookConflictField
  choices: FieldChoice[]
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function authorsKey(authors: string[]): string {
  return [...authors].sort().join('\x00')
}

function externalRefsKey(refs: Record<string, string>): string {
  return Object.entries(refs)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}=${v}`)
    .join('\x00')
}

function fieldValue(book: DupBook, field: BookConflictField): string {
  switch (field) {
    case 'title':
      return book.title
    case 'authors':
      return authorsKey(book.authors)
    case 'isbn13':
      return book.isbn13
    case 'isbn10':
      return book.isbn10
    case 'description':
      return book.description
    case 'pageCount':
      return book.pageCount > 0 ? String(book.pageCount) : ''
    case 'externalRefs':
      return externalRefsKey(book.externalRefs)
    case 'cover':
      // Compare by presence only — proxy URLs differ by bookId.
      return book.coverUrl ? 'present' : ''
  }
}

function displayValue(book: DupBook, field: BookConflictField): string {
  switch (field) {
    case 'title':
      return book.title || '(empty)'
    case 'authors':
      return book.authors.length > 0 ? book.authors.join(', ') : '(none)'
    case 'isbn13':
      return book.isbn13 ? `ISBN ${book.isbn13}` : '(none)'
    case 'isbn10':
      return book.isbn10 ? `ISBN ${book.isbn10}` : '(none)'
    case 'description':
      return book.description
        ? book.description.slice(0, 80) + (book.description.length > 80 ? '…' : '')
        : '(none)'
    case 'pageCount':
      return book.pageCount > 0 ? `${book.pageCount}p` : '(unknown)'
    case 'externalRefs': {
      const keys = Object.keys(book.externalRefs)
      return keys.length > 0 ? keys.join(', ') : '(none)'
    }
    case 'cover':
      return book.coverUrl ? 'Has cover' : 'No cover'
  }
}

export const ALL_CONFLICT_FIELDS: BookConflictField[] = [
  'title',
  'authors',
  'isbn13',
  'isbn10',
  'cover',
  'description',
  'pageCount',
  'externalRefs'
]

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * Returns fields whose values differ across entries in the group.
 * Fields where all entries agree are excluded.
 */
export function detectConflicts(group: DupGroup): FieldConflict[] {
  const conflicts: FieldConflict[] = []

  for (const field of ALL_CONFLICT_FIELDS) {
    const choices: FieldChoice[] = []
    const seen = new Set<string>()

    for (const ub of group.entries) {
      const book = ub.book
      if (!book) continue
      const key = fieldValue(book, field)
      seen.add(key)
      choices.push({
        bookId: ub.bookId,
        displayValue: displayValue(book, field),
        hasValue: key !== ''
      })
    }

    if (seen.size > 1) {
      conflicts.push({ field, choices })
    }
  }

  return conflicts
}

/**
 * Builds the resolved Book metadata object from the per-field choices map.
 * coverUrl is intentionally excluded — pass resolvedCoverSourceBookId separately.
 */
export function buildResolvedMetadata(
  group: DupGroup,
  fieldChoices: Partial<Record<BookConflictField, string>>
): Book {
  const bookById = new Map(group.entries.filter((e) => e.book).map((e) => [e.bookId, e.book!]))

  // Start from the winner entry's book as the base.
  const winner = group.entries[0]?.book
  if (!winner) return create(BookSchema)

  // coverUrl is intentionally excluded — cover is controlled via
  // resolvedCoverSourceBookId on the MergeBooksRequest.
  const resolved = create(BookSchema, {
    title: winner.title,
    authors: winner.authors,
    isbn13: winner.isbn13,
    isbn10: winner.isbn10,
    description: winner.description,
    pageCount: winner.pageCount,
    externalRefs: winner.externalRefs
  })

  const fields: Array<Exclude<BookConflictField, 'cover'>> = [
    'title',
    'authors',
    'isbn13',
    'isbn10',
    'description',
    'pageCount',
    'externalRefs'
  ]

  for (const field of fields) {
    const chosenBookId = fieldChoices[field]
    if (!chosenBookId) continue
    const src = bookById.get(chosenBookId)
    if (!src) continue

    switch (field) {
      case 'title':
        resolved.title = src.title
        break
      case 'authors':
        resolved.authors = src.authors
        break
      case 'isbn13':
        resolved.isbn13 = src.isbn13
        break
      case 'isbn10':
        resolved.isbn10 = src.isbn10
        break
      case 'description':
        resolved.description = src.description
        break
      case 'pageCount':
        resolved.pageCount = src.pageCount
        break
      case 'externalRefs':
        resolved.externalRefs = src.externalRefs
        break
    }
  }

  return resolved
}
