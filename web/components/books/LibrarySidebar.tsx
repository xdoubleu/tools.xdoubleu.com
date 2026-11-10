import type { ComponentProps } from 'react'
import { ShelfSidebar } from '@/components/ui/shelf-sidebar'
import type { LibraryResponse } from '@/lib/gen/books/v1/library_pb'
import { SPECIAL_TAGS, flattenLibrary, statusLabel } from '@/lib/books/bookShelves'
import { seriesHref, type SeriesSummary } from '@/lib/books/series'

export type ShelfId =
  'all' | 'favourite' | 'currently-reading' | 'to-read' | 'read' | (string & Record<never, never>)

export interface Shelf {
  id: ShelfId
  label: string
  count: number
}

export interface TagEntry {
  name: string
  count: number
}

export function buildShelves(library: LibraryResponse): Shelf[] {
  const allBooks = flattenLibrary(library)
  // Dropped books arrive as a generic "dropped" shelf; render it as a fixed shelf.
  const droppedShelf = library.shelves.find((s) => s.name === 'dropped')
  const fixed: Shelf[] = [
    {
      id: 'all',
      label: 'All books',
      count: allBooks.length
    },
    {
      id: 'currently-reading',
      label: statusLabel('currently-reading'),
      count: library.reading.length
    },
    { id: 'to-read', label: statusLabel('to-read'), count: library.wishlist.length },
    { id: 'read', label: statusLabel('read'), count: library.finished.length },
    {
      id: 'favourite',
      label: 'Favourites',
      count: allBooks.filter((b) => b.tags.includes('favourite')).length
    },
    ...(droppedShelf ? [{ id: 'dropped', label: 'Dropped', count: droppedShelf.books.length }] : [])
  ]
  const dynamic: Shelf[] = library.shelves
    .filter((s) => s.name !== 'dropped')
    .map((s) => ({
      id: s.name,
      label: s.name,
      count: s.books.length
    }))
  return [...fixed, ...dynamic]
}

export function buildTags(library: LibraryResponse): TagEntry[] {
  const all = flattenLibrary(library)
  const counts = new Map<string, number>()
  for (const ub of all) {
    for (const t of ub.tags) {
      if (!SPECIAL_TAGS.has(t)) {
        counts.set(t, (counts.get(t) ?? 0) + 1)
      }
    }
  }
  return Array.from(counts.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([name, count]) => ({ name, count }))
}

type LibrarySidebarProps = Omit<ComponentProps<typeof ShelfSidebar>, 'series'> & {
  series?: SeriesSummary[]
}

export default function LibrarySidebar({ series, ...props }: LibrarySidebarProps) {
  return (
    <ShelfSidebar
      {...props}
      series={series?.map((s) => ({
        name: s.name,
        read: s.read,
        total: s.total,
        href: seriesHref(s.name)
      }))}
    />
  )
}
