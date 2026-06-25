'use client'

import { cn } from '@/lib/cn'
import type { LibraryResponse } from '@/lib/gen/backlog/v1/books_pb'
import { SPECIAL_TAGS } from '@/lib/backlog/bookShelves'

export type ShelfId =
  | 'all'
  | 'currently-reading'
  | 'wishlist'
  | 'finished'
  | (string & Record<never, never>)

export interface Shelf {
  id: ShelfId
  label: string
  count: number
}

export function buildShelves(library: LibraryResponse): Shelf[] {
  const fixed: Shelf[] = [
    {
      id: 'all',
      label: 'All books',
      count:
        library.reading.length +
        library.wishlist.length +
        library.finished.length +
        library.shelves.reduce((s, sh) => s + sh.books.length, 0)
    },
    { id: 'currently-reading', label: 'Currently reading', count: library.reading.length },
    { id: 'wishlist', label: 'Want to read', count: library.wishlist.length },
    { id: 'finished', label: 'Read', count: library.finished.length }
  ]
  const dynamic: Shelf[] = library.shelves.map((s) => ({
    id: s.name,
    label: s.name,
    count: s.books.length
  }))
  return [...fixed, ...dynamic]
}

export function buildTags(library: LibraryResponse): string[] {
  const all = [
    ...library.reading,
    ...library.wishlist,
    ...library.finished,
    ...library.shelves.flatMap((s) => s.books)
  ]
  const seen = new Set<string>()
  for (const ub of all) {
    for (const t of ub.tags) {
      if (!SPECIAL_TAGS.has(t)) seen.add(t)
    }
  }
  return Array.from(seen).sort()
}

interface LibrarySidebarProps {
  shelves: Shelf[]
  allTags: string[]
  selectedShelf: ShelfId
  selectedTags: Set<string>
  onSelectShelf: (id: ShelfId) => void
  onToggleTag: (tag: string) => void
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
  selectedShelf,
  selectedTags,
  onSelectShelf,
  onToggleTag,
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
            active={selectedShelf === shelf.id}
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
                key={tag}
                active={selectedTags.has(tag)}
                onClick={() => onToggleTag(tag)}
                label={tag}
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
              aria-selected={selectedShelf === shelf.id}
              onClick={() => onSelectShelf(shelf.id)}
              className={cn(
                'flex items-center gap-1 shrink-0 px-3 py-1.5 rounded-full text-sm whitespace-nowrap transition-colors',
                selectedShelf === shelf.id
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
                key={tag}
                onClick={() => onToggleTag(tag)}
                className={cn(
                  'shrink-0 px-2 py-1 rounded-full text-xs whitespace-nowrap transition-colors border',
                  selectedTags.has(tag)
                    ? 'bg-accent/10 text-accent border-accent/30 font-medium'
                    : 'bg-surface text-subtle border-border hover:text-foreground'
                )}
              >
                {tag}
              </button>
            ))}
          </div>
        )}
      </div>
    </>
  )
}
