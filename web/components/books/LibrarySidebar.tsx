'use client'

import { cn } from '@/lib/cn'
import { MenuItem } from '@/components/ui/menu-item'
import { TogglePill } from '@/components/ui/toggle-pill'
import type { LibraryResponse } from '@/lib/gen/books/v1/library_pb'
import { SPECIAL_TAGS, flattenLibrary, statusLabel } from '@/lib/books/bookShelves'

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
  // The backend has no dedicated LibraryResponse field for dropped books —
  // they arrive as a generic shelf named "dropped". Pull it out and render
  // it as a fixed shelf with a proper label instead of the raw status value.
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

interface LibrarySidebarProps {
  shelves: Shelf[]
  allTags: TagEntry[]
  selectedShelfId: ShelfId | null
  selectedTag: string | null
  onSelectShelf: (id: ShelfId) => void
  onSelectTag: (tag: string) => void
  /** Omit on read-only views (public profile) to hide shelf/tag editing. */
  onManage?: () => void
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
    <MenuItem
      onClick={onClick}
      aria-current={active ? 'true' : undefined}
      className={cn(
        'justify-between rounded-xl px-3 py-2',
        active ? 'bg-accent/10 text-accent font-medium' : 'text-subtle hover:text-fg'
      )}
    >
      <span className="truncate">{label}</span>
      {count !== undefined && <span className="ml-2 text-xs text-muted shrink-0">{count}</span>}
    </MenuItem>
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

        {onManage && (
          <>
            <div className="my-1 h-px bg-border" />
            <MenuItem onClick={onManage} className="rounded-xl px-3 py-2 text-subtle hover:text-fg">
              Edit shelves &amp; tags
            </MenuItem>
          </>
        )}
      </nav>

      {/* Mobile: horizontal scrollable chip rows */}
      <div className="flex md:hidden flex-col gap-2">
        <div
          className="flex gap-2 overflow-x-auto pb-1 -mx-1 px-1"
          role="tablist"
          aria-label="Shelves"
        >
          {shelves.map((shelf) => (
            <TogglePill
              key={shelf.id}
              role="tab"
              aria-selected={selectedShelfId === shelf.id}
              aria-pressed={undefined}
              active={selectedShelfId === shelf.id}
              onClick={() => onSelectShelf(shelf.id)}
              label={
                <>
                  {shelf.label}
                  <span className="ml-1 text-xs opacity-60">{shelf.count}</span>
                </>
              }
              className="shrink-0 px-3 py-1.5 text-sm whitespace-nowrap"
            />
          ))}
        </div>
        {allTags.length > 0 && (
          <div className="flex gap-2 overflow-x-auto pb-1 -mx-1 px-1">
            {allTags.map((tag) => (
              <TogglePill
                key={tag.name}
                active={selectedTag === tag.name}
                onClick={() => onSelectTag(tag.name)}
                label={
                  <>
                    {tag.name}
                    <span className="ml-1 opacity-60">{tag.count}</span>
                  </>
                }
                className="shrink-0 px-2 py-1 whitespace-nowrap"
              />
            ))}
          </div>
        )}
      </div>
    </>
  )
}
