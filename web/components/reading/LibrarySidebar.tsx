'use client'

import { cn } from '@/lib/cn'
import type { LibraryResponse } from '@/lib/gen/reading/v1/library_pb'
import { SPECIAL_TAGS, statusLabel } from '@/lib/reading/bookShelves'
import { categoryLabel, categoryOf, type Category } from '@/lib/reading/categories'

export type ShelfId =
  | 'all'
  | 'favourite'
  | 'currently-reading'
  | 'to-read'
  | 'read'
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
    ...library.shelves.flatMap((s) => s.books),
    ...library.rss
  ]
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

export interface CategoryEntry {
  id: Category
  label: string
  count: number
}

// buildCategories returns the categories present in the library. A pure book
// library yields a single 'book' entry, which the sidebar treats as "nothing
// to filter" and hides the section entirely.
export function buildCategories(library: LibraryResponse): CategoryEntry[] {
  const all = [
    ...library.reading,
    ...library.wishlist,
    ...library.finished,
    ...library.shelves.flatMap((s) => s.books),
    ...library.rss
  ]
  const counts = new Map<Category, number>()
  for (const ub of all) {
    const cat = categoryOf(ub.book?.category)
    counts.set(cat, (counts.get(cat) ?? 0) + 1)
  }
  return Array.from(counts.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([id, count]) => ({ id, label: categoryLabel(id), count }))
}

export function buildTags(library: LibraryResponse): TagEntry[] {
  const all = [
    ...library.reading,
    ...library.wishlist,
    ...library.finished,
    ...library.shelves.flatMap((s) => s.books),
    ...library.rss
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
  /**
   * Categories present in the library. Omit (or pass only 'book') to hide
   * the section — a pure book library needs no category filter.
   */
  categories?: CategoryEntry[]
  selectedShelfId: ShelfId | null
  selectedTag: string | null
  selectedCategory?: Category | null
  onSelectShelf: (id: ShelfId) => void
  onSelectTag: (tag: string) => void
  onSelectCategory?: (category: Category) => void
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
  categories,
  selectedShelfId,
  selectedTag,
  selectedCategory,
  onSelectShelf,
  onSelectTag,
  onSelectCategory,
  onManage
}: LibrarySidebarProps) {
  // Only offer the filter when there is something other than books to
  // filter by — zero UI change for pure book libraries.
  const showCategories =
    !!onSelectCategory &&
    (categories ?? []).some((c) => c.id !== 'book') &&
    (categories ?? []).length > 1
  const shownCategories = showCategories ? (categories ?? []) : []
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

        {shownCategories.length > 0 && (
          <>
            <div className="my-1 h-px bg-border" />
            <p className="px-3 py-1 text-xs font-semibold text-muted uppercase tracking-wide">
              Categories
            </p>
            {shownCategories.map((cat) => (
              <NavItem
                key={cat.id}
                active={selectedCategory === cat.id}
                onClick={() => onSelectCategory?.(cat.id)}
                label={cat.label}
                count={cat.count}
              />
            ))}
          </>
        )}

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
            <button
              type="button"
              onClick={onManage}
              className="w-full text-left px-3 py-2 rounded-xl text-sm text-subtle hover:bg-surface hover:text-foreground transition-colors"
            >
              Edit shelves & tags
            </button>
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
        {shownCategories.length > 0 && (
          <div className="flex gap-2 overflow-x-auto pb-1 -mx-1 px-1">
            {shownCategories.map((cat) => (
              <button
                key={cat.id}
                onClick={() => onSelectCategory?.(cat.id)}
                className={cn(
                  'flex items-center gap-1 shrink-0 px-2 py-1 rounded-full text-xs whitespace-nowrap transition-colors border',
                  selectedCategory === cat.id
                    ? 'bg-accent/10 text-accent border-accent/30 font-medium'
                    : 'bg-surface text-subtle border-border hover:text-foreground'
                )}
              >
                {cat.label}
                <span className="opacity-60">{cat.count}</span>
              </button>
            ))}
          </div>
        )}
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
