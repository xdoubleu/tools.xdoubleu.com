'use client'

import { useState, useMemo } from 'react'
import Link from 'next/link'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableCell,
  SortableHeader,
  type SortDir
} from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import BookCover from '@/components/backlog/BookCover'
import BookRatingStars from '@/components/backlog/BookRatingStars'
import BookFavouriteButton from '@/components/backlog/BookFavouriteButton'
import BookShelfTagCell from '@/components/backlog/BookShelfTagCell'
import { statusLabel, displayTags } from '@/lib/backlog/bookShelves'

const PAGE_SIZE = 20

type SortKey = 'title' | 'author' | 'pages' | 'rating' | 'favourite' | 'shelf' | 'added' | 'read'

interface SortState {
  key: SortKey
  dir: SortDir
}

function nextDir(current: SortDir): SortDir {
  if (current === null) return 'asc'
  if (current === 'asc') return 'desc'
  return null
}

function formatShortDate(iso: string | undefined): string {
  if (!iso) return ''
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

function latestFinishedAt(ub: UserBook): string {
  if (!ub.finishedAt.length) return ''
  return [...ub.finishedAt].sort().at(-1) ?? ''
}

function sortBooks(books: UserBook[], sort: SortState): UserBook[] {
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

interface BooksTableProps {
  books: UserBook[]
  knownShelves: string[]
  knownTags: string[]
  onSaved?: () => void
}

export default function BooksTable({ books, knownShelves, knownTags, onSaved }: BooksTableProps) {
  const [sort, setSort] = useState<SortState>({ key: 'added', dir: null })
  const [page, setPage] = useState(1)

  const sorted = useMemo(() => sortBooks(books, sort), [books, sort])
  const pageCount = Math.max(1, Math.ceil(sorted.length / PAGE_SIZE))
  const safePage = Math.min(page, pageCount)
  const pageBooks = sorted.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE)

  function handleSort(key: SortKey) {
    setSort((prev) => ({
      key,
      dir: prev.key === key ? nextDir(prev.dir) : 'asc'
    }))
    setPage(1)
  }

  function sortDir(key: SortKey): SortDir {
    return sort.key === key ? sort.dir : null
  }

  return (
    <div className="space-y-3">
      <Table>
        <TableHeader>
          <TableRow>
            <SortableHeader dir={null} onSort={() => {}} className="w-12">
              Cover
            </SortableHeader>
            <SortableHeader dir={sortDir('title')} onSort={() => handleSort('title')}>
              Title
            </SortableHeader>
            <SortableHeader dir={sortDir('author')} onSort={() => handleSort('author')}>
              Author
            </SortableHeader>
            <SortableHeader dir={sortDir('pages')} onSort={() => handleSort('pages')}>
              Pages
            </SortableHeader>
            <SortableHeader dir={sortDir('rating')} onSort={() => handleSort('rating')}>
              Rating
            </SortableHeader>
            <SortableHeader dir={sortDir('favourite')} onSort={() => handleSort('favourite')}>
              Fav
            </SortableHeader>
            <SortableHeader dir={sortDir('shelf')} onSort={() => handleSort('shelf')}>
              Shelf & tags
            </SortableHeader>
            <SortableHeader dir={sortDir('added')} onSort={() => handleSort('added')}>
              Date added
            </SortableHeader>
            <SortableHeader dir={sortDir('read')} onSort={() => handleSort('read')}>
              Date read
            </SortableHeader>
          </TableRow>
        </TableHeader>
        <TableBody>
          {pageBooks.length === 0 ? (
            <TableRow>
              <TableCell colSpan={9} className="py-8 text-center text-muted text-sm">
                No books match the current filters.
              </TableCell>
            </TableRow>
          ) : (
            pageBooks.map((ub) => {
              const book = ub.book
              const coverUrl = book?.coverUrl ?? ''
              const title = book?.title ?? ''
              const authors = book?.authors ?? []

              return (
                <TableRow key={ub.id}>
                  {/* Cover */}
                  <TableCell className="w-12 pr-0">
                    <Link href={`/backlog/books/${ub.id}`} tabIndex={-1}>
                      <BookCover coverUrl={coverUrl} title={title} size="sm" />
                    </Link>
                  </TableCell>

                  {/* Title */}
                  <TableCell className="max-w-48">
                    <Link
                      href={`/backlog/books/${ub.id}`}
                      className="text-sm font-medium hover:text-accent transition-colors line-clamp-2"
                    >
                      {title}
                    </Link>
                  </TableCell>

                  {/* Author */}
                  <TableCell className="max-w-36">
                    <div className="flex flex-col gap-0.5">
                      {authors.map((author) => (
                        <Link
                          key={author}
                          href={`/backlog/books/author/${encodeURIComponent(author)}`}
                          className="text-sm text-subtle hover:text-accent transition-colors truncate"
                        >
                          {author}
                        </Link>
                      ))}
                    </div>
                  </TableCell>

                  {/* Page count */}
                  <TableCell className="text-sm text-muted w-16 text-right">
                    {book?.pageCount ?? ''}
                  </TableCell>

                  {/* Rating */}
                  <TableCell className="w-28">
                    <BookRatingStars userBook={ub} size="sm" onSaved={onSaved} />
                  </TableCell>

                  {/* Favourite */}
                  <TableCell className="w-10">
                    <BookFavouriteButton userBook={ub} onSaved={onSaved} />
                  </TableCell>

                  {/* Shelf & tags */}
                  <TableCell className="max-w-44">
                    <BookShelfTagCell
                      userBook={ub}
                      knownShelves={knownShelves}
                      knownTags={knownTags}
                      onSaved={onSaved}
                    />
                    {displayTags(ub.tags).length > 0 && (
                      <div className="mt-0.5 text-xs text-muted truncate">
                        {displayTags(ub.tags).join(', ')}
                      </div>
                    )}
                  </TableCell>

                  {/* Date added */}
                  <TableCell className="text-xs text-muted whitespace-nowrap">
                    {formatShortDate(ub.addedAt)}
                  </TableCell>

                  {/* Date read */}
                  <TableCell className="text-xs text-muted whitespace-nowrap">
                    {formatShortDate(latestFinishedAt(ub))}
                  </TableCell>
                </TableRow>
              )
            })
          )}
        </TableBody>
      </Table>

      {/* Pagination */}
      {pageCount > 1 && (
        <div className="flex items-center justify-center gap-3">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={safePage <= 1}
          >
            Prev
          </Button>
          <span className="text-sm text-muted">
            {safePage} / {pageCount}
          </span>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setPage((p) => Math.min(pageCount, p + 1))}
            disabled={safePage >= pageCount}
          >
            Next
          </Button>
        </div>
      )}
    </div>
  )
}

export { statusLabel }
