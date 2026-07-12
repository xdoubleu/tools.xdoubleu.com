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
  openlibrary: 'OpenLibrary'
}

export function providerLabel(provider: string): string {
  return PROVIDER_LABELS[provider] ?? provider
}
