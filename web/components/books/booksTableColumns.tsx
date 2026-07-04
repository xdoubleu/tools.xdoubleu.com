import type { ReactNode } from 'react'
import Link from 'next/link'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import type { SortDir } from '@/components/ui/table'
import BookCover from '@/components/books/BookCover'
import BookRatingStars from '@/components/books/BookRatingStars'
import BookFavouriteButton from '@/components/books/BookFavouriteButton'
import BookOwnershipToggles from '@/components/books/BookOwnershipToggles'
import BookShelfTagCell from '@/components/books/BookShelfTagCell'
import { displayTags } from '@/lib/books/bookShelves'

export type ColumnKey =
  | 'cover'
  | 'title'
  | 'author'
  | 'pages'
  | 'isbn'
  | 'rating'
  | 'favourite'
  | 'owned'
  | 'shelf'
  | 'added'
  | 'read'

export type SortKey =
  | 'title'
  | 'author'
  | 'pages'
  | 'rating'
  | 'favourite'
  | 'shelf'
  | 'added'
  | 'read'

export interface SortState {
  key: SortKey
  dir: SortDir
}

export interface CellContext {
  knownShelves: string[]
  knownTags: string[]
  onSaved?: () => void
}

export interface BookColumn {
  key: ColumnKey
  label: string
  sortKey?: SortKey
  /** Extra classes on the <SortableHeader> cell. */
  headClassName?: string
  /** Extra classes on the <TableCell>. */
  cellClassName?: string
  /** Column cannot be hidden via the Columns toggle. */
  alwaysVisible?: boolean
  renderCell: (ub: UserBook, ctx: CellContext) => ReactNode
}

export function nextDir(current: SortDir): SortDir {
  if (current === null) return 'asc'
  if (current === 'asc') return 'desc'
  return null
}

export function formatShortDate(iso: string | undefined): string {
  if (!iso) return ''
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

export function latestFinishedAt(ub: UserBook): string {
  if (!ub.finishedAt.length) return ''
  return [...ub.finishedAt].sort().at(-1) ?? ''
}

export function sortBooks(books: UserBook[], sort: SortState): UserBook[] {
  if (!sort.dir) return books
  const factor = sort.dir === 'asc' ? 1 : -1
  return [...books].sort((a, b) => {
    let cmp = 0
    switch (sort.key) {
      case 'title':
        cmp = (a.book?.title ?? '').localeCompare(b.book?.title ?? '')
        break
      case 'author':
        cmp = (a.book?.authors[0] ?? '').localeCompare(b.book?.authors[0] ?? '')
        break
      case 'pages':
        cmp = (a.book?.pageCount ?? 0) - (b.book?.pageCount ?? 0)
        break
      case 'rating':
        cmp = (a.rating ?? 0) - (b.rating ?? 0)
        break
      case 'favourite':
        cmp = (a.tags.includes('favourite') ? 1 : 0) - (b.tags.includes('favourite') ? 1 : 0)
        break
      case 'shelf':
        cmp = a.status.localeCompare(b.status)
        break
      case 'added':
        cmp = (a.addedAt ?? '').localeCompare(b.addedAt ?? '')
        break
      case 'read':
        cmp = latestFinishedAt(a).localeCompare(latestFinishedAt(b))
        break
    }
    return cmp * factor
  })
}

/** All 11 columns. Order here is the display order in the table. */
export const ALL_COLUMNS: BookColumn[] = [
  {
    key: 'cover',
    label: 'Cover',
    alwaysVisible: true,
    headClassName: 'w-12',
    cellClassName: 'w-12 pr-0',
    renderCell: (ub) => (
      <Link href={`/books/${ub.id}`} tabIndex={-1}>
        <BookCover coverUrl={ub.book?.coverUrl ?? ''} title={ub.book?.title ?? ''} size="sm" />
      </Link>
    )
  },
  {
    key: 'title',
    label: 'Title',
    sortKey: 'title',
    alwaysVisible: true,
    cellClassName: 'max-w-48',
    renderCell: (ub) => (
      <Link
        href={`/books/${ub.id}`}
        className="text-sm font-medium hover:text-accent transition-colors line-clamp-2"
      >
        {ub.book?.title ?? ''}
      </Link>
    )
  },
  {
    key: 'author',
    label: 'Author',
    sortKey: 'author',
    cellClassName: 'max-w-36',
    renderCell: (ub) => (
      <div className="flex flex-col gap-0.5">
        {(ub.book?.authors ?? []).map((author) => (
          <Link
            key={author}
            href={`/books/author/${encodeURIComponent(author)}`}
            className="text-sm text-subtle hover:text-accent transition-colors truncate"
          >
            {author}
          </Link>
        ))}
      </div>
    )
  },
  {
    key: 'pages',
    label: 'Pages',
    sortKey: 'pages',
    cellClassName: 'w-16 text-right',
    renderCell: (ub) => <span className="text-sm text-muted">{ub.book?.pageCount ?? ''}</span>
  },
  {
    key: 'isbn',
    label: 'ISBN',
    renderCell: (ub) => (
      <span className="text-xs text-muted whitespace-nowrap">
        {ub.book?.isbn13 ? `ISBN ${ub.book.isbn13}` : ''}
      </span>
    )
  },
  {
    key: 'rating',
    label: 'Rating',
    sortKey: 'rating',
    cellClassName: 'w-28',
    renderCell: (ub, ctx) => <BookRatingStars userBook={ub} size="sm" onSaved={ctx.onSaved} />
  },
  {
    key: 'favourite',
    label: 'Fav',
    sortKey: 'favourite',
    cellClassName: 'w-10',
    renderCell: (ub, ctx) => <BookFavouriteButton userBook={ub} onSaved={ctx.onSaved} />
  },
  {
    key: 'owned',
    label: 'Owned',
    renderCell: (ub, ctx) => <BookOwnershipToggles userBook={ub} onSaved={ctx.onSaved} />
  },
  {
    key: 'shelf',
    label: 'Shelf & tags',
    sortKey: 'shelf',
    cellClassName: 'max-w-44',
    renderCell: (ub, ctx) => (
      <>
        <BookShelfTagCell
          userBook={ub}
          knownShelves={ctx.knownShelves}
          knownTags={ctx.knownTags}
          onSaved={ctx.onSaved}
        />
        {displayTags(ub.tags).length > 0 && (
          <div className="mt-0.5 text-xs text-muted truncate">
            {displayTags(ub.tags).join(', ')}
          </div>
        )}
      </>
    )
  },
  {
    key: 'added',
    label: 'Date added',
    sortKey: 'added',
    renderCell: (ub) => (
      <span className="text-xs text-muted whitespace-nowrap">{formatShortDate(ub.addedAt)}</span>
    )
  },
  {
    key: 'read',
    label: 'Date read',
    sortKey: 'read',
    renderCell: (ub) => (
      <span className="text-xs text-muted whitespace-nowrap">
        {formatShortDate(latestFinishedAt(ub))}
      </span>
    )
  }
]

/** Default visible set: all columns. Users narrow via the Columns toolbar toggle. */
export const DEFAULT_VISIBLE_COLUMNS: ColumnKey[] = [
  'cover',
  'title',
  'author',
  'pages',
  'isbn',
  'rating',
  'favourite',
  'owned',
  'shelf',
  'added',
  'read'
]
