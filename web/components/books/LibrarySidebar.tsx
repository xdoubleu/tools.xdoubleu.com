'use client'

import { cn } from '@/lib/cn'
import type { LibraryResponse } from '@/lib/gen/books/v1/library_pb'
import { SPECIAL_TAGS } from '@/lib/books/bookShelves'

export type ShelfId =
  | 'all'
  | 'favourite'
  | 'currently-reading'
  | 'wishlist'
  | 'finished'
  | (string & Record<never, never>)

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
  const allBooks = [
    ...library.reading,
    ...library.wishlist,
    ...library.finished,
    ...library.shelves.flatMap((s) => s.books)
  ]
  const fixed: Shelf[] = [
    {
      id: 'all',
      label: 'All books',
      count: allBooks.length
    },
    { id: 'currently-reading', label: 'Currently reading', count: library.reading.length },
    { id: 'wishlist', label: 'Want to read', count: library.wishlist.length },
    { id: 'finished', label: 'Read', count: library.finished.length },
    {
      id: 'favourite',
      label: 'Favourites',
      count: allBooks.filter((b) => b.tags.includes('favourite')).length
    }
  ]
  const dynamic: Shelf[] = library.shelves.map((s) => ({
    id: s.name,
    label: s.name,
    count: s.books.length
  }))
  return [...fixed, ...dynamic]
}

export function buildTags(library: LibraryResponse): TagEntry[] {
  const all = [
    ...library.reading,
    ...library.wishlist,
    ...library.finished,
    ...library.shelves.flatMap((s) => s.books)
  ]
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

interface LibrarySidebarProps {
  shelves: Shelf[]
  allTags: TagEntry[]
  selectedShelfId: ShelfId | null
  selectedTag: string | null
  onSelectShelf: (id: ShelfId) => void
  onSelectTag: (tag: string) => void
  onManage: () => void
}

function NavItem({
  active,
  onClick,
  label,
  count
}: {
  active: boolean
  onClick: () => void
  label: string
  count?: number
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'flex items-center justify-between w-full text-left px-3 py-2 rounded-xl text-sm transition-colors',
        active
          ? 'bg-accent/10 text-accent font-medium'
          : 'text-subtle hover:bg-surface hover:text-foreground'
      )}
    >
      <span className="truncate">{label}</span>
      {count !== undefined && <span className="ml-2 text-xs text-muted shrink-0">{count}</span>}
    </button>
  )
}

export default function LibrarySidebar({
  shelves,
  allTags,
  selectedShelfId,
  selectedTag,
  onSelectShelf,
  onSelectTag,
  onManage
}: LibrarySidebarProps) {
  return (
    <>
      {/* Desktop: vertical sidebar */}
      <nav
        className="hidden md:flex flex-col gap-1 min-w-44 shrink-0"
        aria-label="Library navigation"
      >
        <p className="px-3 py-1 text-xs font-semibold text-muted uppercase tracking-wide">
          Shelves
        </p>
        {shelves.map((shelf) => (
          <NavItem
            key={shelf.id}
            active={selectedShelfId === shelf.id}
            onClick={() => onSelectShelf(shelf.id)}
            label={shelf.label}
            count={shelf.count}
          />
        ))}

        {allTags.length > 0 && (
          <>
            <div className="my-1 h-px bg-border" />
            <p className="px-3 py-1 text-xs font-semibold text-muted uppercase tracking-wide">
              Tags
            </p>
            {allTags.map((tag) => (
              <NavItem
                key={tag.name}
                active={selectedTag === tag.name}
                onClick={() => onSelectTag(tag.name)}
                label={tag.name}
                count={tag.count}
              />
            ))}
          </>
        )}

        <div className="my-1 h-px bg-border" />
        <button
          type="button"
          onClick={onManage}
          className="w-full text-left px-3 py-2 rounded-xl text-sm text-subtle hover:bg-surface hover:text-foreground transition-colors"
        >
          Edit shelves & tags
        </button>
      </nav>

      {/* Mobile: horizontal scrollable chip rows */}
      <div className="flex md:hidden flex-col gap-2">
        <div
          className="flex gap-2 overflow-x-auto pb-1 -mx-1 px-1"
          role="tablist"
          aria-label="Shelves"
        >
          {shelves.map((shelf) => (
            <button
              key={shelf.id}
              role="tab"
              aria-selected={selectedShelfId === shelf.id}
              onClick={() => onSelectShelf(shelf.id)}
              className={cn(
                'flex items-center gap-1 shrink-0 px-3 py-1.5 rounded-full text-sm whitespace-nowrap transition-colors',
                selectedShelfId === shelf.id
                  ? 'bg-accent/10 text-accent font-medium'
                  : 'bg-surface text-subtle hover:text-foreground'
              )}
            >
              {shelf.label}
              <span className="text-xs opacity-60">{shelf.count}</span>
            </button>
          ))}
        </div>
        {allTags.length > 0 && (
          <div className="flex gap-2 overflow-x-auto pb-1 -mx-1 px-1">
            {allTags.map((tag) => (
              <button
                key={tag.name}
                onClick={() => onSelectTag(tag.name)}
                className={cn(
                  'flex items-center gap-1 shrink-0 px-2 py-1 rounded-full text-xs whitespace-nowrap transition-colors border',
                  selectedTag === tag.name
                    ? 'bg-accent/10 text-accent border-accent/30 font-medium'
                    : 'bg-surface text-subtle border-border hover:text-foreground'
                )}
              >
                {tag.name}
                <span className="opacity-60">{tag.count}</span>
              </button>
            ))}
          </div>
        )}
      </div>
    </>
  )
}
