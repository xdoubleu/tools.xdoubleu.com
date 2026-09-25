/**
 * Per-field conflict detection for a duplicate group and the resolved-metadata
 * payload for MergeBooks. Mirrors metadataCompleteness in book_matching.go;
 * cover is compared by presence (proxy URLs differ by bookId).
 */

import { create } from '@bufbuild/protobuf'
import type { Book } from '@/lib/gen/books/v1/library_pb'
import { BookSchema } from '@/lib/gen/books/v1/library_pb'

// Duck-typed so tests can pass plain fixtures instead of proto Messages.

interface DupBook {
  id: string
  title: string
  authors: string[]
  isbn13: string
  coverUrl: string
  description: string
  pageCount: number
}

interface DupEntry {
  bookId: string
  status: string
  book?: DupBook | null
}

export interface DupGroup {
  entries: DupEntry[]
  reason: string
}

export type BookConflictField =
  'title' | 'authors' | 'isbn13' | 'description' | 'pageCount' | 'cover' | 'status'

export interface FieldChoice {
  bookId: string
  displayValue: string
  hasValue: boolean
}

export interface FieldConflict {
  field: BookConflictField
  choices: FieldChoice[]
}

function authorsKey(authors: string[]): string {
  return [...authors].sort().join('\x00')
}

function fieldValue(entry: DupEntry, field: BookConflictField): string {
  const book = entry.book
  switch (field) {
    case 'title':
      return book?.title ?? ''
    case 'authors':
      return book ? authorsKey(book.authors) : ''
    case 'isbn13':
      return book?.isbn13 ?? ''
    case 'description':
      return book?.description ?? ''
    case 'pageCount':
      return book && book.pageCount > 0 ? String(book.pageCount) : ''
    case 'cover':
      return book?.coverUrl ? 'present' : ''
    case 'status':
      return entry.status
  }
}

function displayValue(entry: DupEntry, field: BookConflictField): string {
  const book = entry.book
  switch (field) {
    case 'title':
      return book?.title || '(empty)'
    case 'authors':
      return book && book.authors.length > 0 ? book.authors.join(', ') : '(none)'
    case 'isbn13':
      return book?.isbn13 ? `ISBN ${book.isbn13}` : '(none)'
    case 'description':
      return book?.description
        ? book.description.slice(0, 80) + (book.description.length > 80 ? '…' : '')
        : '(none)'
    case 'pageCount':
      return book && book.pageCount > 0 ? `${book.pageCount}p` : '(unknown)'
    case 'cover':
      return book?.coverUrl ? 'Has cover' : 'No cover'
    case 'status':
      return entry.status || '(none)'
  }
}

export const ALL_CONFLICT_FIELDS: BookConflictField[] = [
  'status',
  'title',
  'authors',
  'isbn13',
  'cover',
  'description',
  'pageCount'
]

// Mirrors statusRank in book_matching.go; lower is worse.

const BUILT_IN_STATUSES = new Set(['to-read', 'currently-reading', 'read', 'dropped'])

const BUILTIN_RANK: Record<string, number> = {
  read: 3,
  'currently-reading': 2,
  'to-read': 1,
  dropped: 0
}

function statusRank(status: string): number {
  if (!status) return -1
  if (!BUILT_IN_STATUSES.has(status)) return 4 // custom shelf wins
  return BUILTIN_RANK[status] ?? 0
}

/**
 * Returns the bookId whose status wins under the backend's consolidation rule
 * (custom shelf > read > currently-reading > to-read > dropped); ties pick
 * entries[0].
 */
export function pickAutoStatusBookId(group: DupGroup): string {
  let best = group.entries[0]
  for (const e of group.entries) {
    if (statusRank(e.status) > statusRank(best?.status ?? '')) {
      best = e
    }
  }
  return best?.bookId ?? ''
}

/** Resolved status to send, or undefined to let auto-consolidation decide. */
export function resolveStatusChoice(
  group: DupGroup,
  fieldChoices: Partial<Record<BookConflictField, string>>
): string | undefined {
  const chosenBookId = fieldChoices['status']
  if (!chosenBookId) return undefined
  const entry = group.entries.find((e) => e.bookId === chosenBookId)
  return entry?.status
}

/** Returns fields whose values differ across entries in the group. */
export function detectConflicts(group: DupGroup): FieldConflict[] {
  const conflicts: FieldConflict[] = []

  for (const field of ALL_CONFLICT_FIELDS) {
    const choices: FieldChoice[] = []
    const seen = new Set<string>()

    for (const ub of group.entries) {
      const key = fieldValue(ub, field)
      seen.add(key)
      choices.push({
        bookId: ub.bookId,
        displayValue: displayValue(ub, field),
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
 * Builds resolved Book metadata from the choices map, excluding coverUrl and
 * status (sent as resolvedCoverSourceBookId / resolvedStatus).
 */
export function buildResolvedMetadata(
  group: DupGroup,
  fieldChoices: Partial<Record<BookConflictField, string>>
): Book {
  const bookById = new Map(group.entries.filter((e) => e.book).map((e) => [e.bookId, e.book!]))

  const winner = group.entries[0]?.book
  if (!winner) return create(BookSchema)

  const resolved = create(BookSchema, {
    title: winner.title,
    authors: winner.authors,
    isbn13: winner.isbn13,
    description: winner.description,
    pageCount: winner.pageCount
  })

  const fields: Array<Exclude<BookConflictField, 'cover' | 'status'>> = [
    'title',
    'authors',
    'isbn13',
    'description',
    'pageCount'
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
      case 'description':
        resolved.description = src.description
        break
      case 'pageCount':
        resolved.pageCount = src.pageCount
        break
    }
  }

  return resolved
}
