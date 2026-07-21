import type { LibraryResponse, UserBook } from '@/lib/gen/reading/v1/library_pb'

// Flattens every book/RSS-item list on a library response into one array.
// RSS items live outside the reading-state shelves (library.rss), so callers
// that want "all books" — sidebar filters, detail pages, author pages — must
// go through this rather than re-listing the shelf fields themselves.
export function flattenLibrary(library: LibraryResponse | null | undefined): UserBook[] {
  if (!library) return []
  return [
    ...library.reading,
    ...library.wishlist,
    ...library.finished,
    ...library.shelves.flatMap((s) => s.books),
    ...library.rss
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

// The four fixed reading-state shelves. Custom shelves are any other status value.
export const BUILT_IN_STATUSES = new Set(['to-read', 'currently-reading', 'read', 'dropped'])

export const BOOK_STATUSES: { value: string; label: string }[] = [
  { value: 'to-read', label: 'Want to read' },
  { value: 'currently-reading', label: 'Currently reading' },
  { value: 'read', label: 'Read' },
  { value: 'dropped', label: 'Dropped' }
]

// Return the display label for a built-in status, or the raw value for custom.
export function statusLabel(status: string): string {
  return BOOK_STATUSES.find((s) => s.value === status)?.label ?? status
}

// Shelf ids that are fixed and not user-editable: the four reading-state
// statuses plus the "favourite" pseudo-shelf (backed by a tag, not a status).
export function isBuiltInShelfId(id: string): boolean {
  return BUILT_IN_STATUSES.has(id) || id === 'favourite'
}

// Returns display tags (non-special user tags).
export function displayTags(tags: string[]): string[] {
  return tags.filter((t) => !SPECIAL_TAGS.has(t))
}

// Display name for an external search-result provider. Falls back to the raw
// value for providers without a friendly label.
const PROVIDER_LABELS: Record<string, string> = {
  unicat: 'UniCat',
  hardcover: 'Hardcover'
}

export function providerLabel(provider: string): string {
  return PROVIDER_LABELS[provider] ?? provider
}
