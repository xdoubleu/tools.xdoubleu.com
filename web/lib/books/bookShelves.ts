import type { LibraryResponse, UserBook } from '@/lib/gen/books/v1/library_pb'

// Flattens the backlog (reading/wishlist/finished/shelves) into one array.
export function flattenLibrary(library: LibraryResponse | null | undefined): UserBook[] {
  if (!library) return []
  return [
    ...library.reading,
    ...library.wishlist,
    ...library.finished,
    ...library.shelves.flatMap((s) => s.books)
  ]
}

// Tags that have reserved UI treatment — not user-visible shelves/tags.
export const SPECIAL_TAGS = new Set([
  'favourite',
  'own-physical',
  'own-digital',
  'kobo-sync',
  'kobo-format-pdf'
])

// Custom shelves are any other status value.
export const BUILT_IN_STATUSES = new Set(['to-read', 'currently-reading', 'read', 'dropped'])

export const BOOK_STATUSES: { value: string; label: string }[] = [
  { value: 'to-read', label: 'Want to read' },
  { value: 'currently-reading', label: 'Currently reading' },
  { value: 'read', label: 'Read' },
  { value: 'dropped', label: 'Dropped' }
]

export function statusLabel(status: string): string {
  return BOOK_STATUSES.find((s) => s.value === status)?.label ?? status
}

// Fixed shelf ids: the built-in statuses plus the tag-backed "favourite".
export function isBuiltInShelfId(id: string): boolean {
  return BUILT_IN_STATUSES.has(id) || id === 'favourite'
}

export function displayTags(tags: string[]): string[] {
  return tags.filter((t) => !SPECIAL_TAGS.has(t))
}

const PROVIDER_LABELS: Record<string, string> = {
  unicat: 'UniCat',
  hardcover: 'Hardcover'
}

export function providerLabel(provider: string): string {
  return PROVIDER_LABELS[provider] ?? provider
}
