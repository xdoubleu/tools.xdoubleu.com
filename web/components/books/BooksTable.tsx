import { useState, useMemo } from 'react'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableCell,
  TableHead,
  SortableHeader,
  type SortDir
} from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import BookCard from '@/components/books/BookCard'
import BooksTableRow from '@/components/books/BooksTableRow'
import {
  ALL_COLUMNS,
  sortBooks,
  nextDir,
  type SortKey,
  type SortState
} from '@/components/books/booksTableColumns'
import { applyFilters, type LibraryTableState } from '@/components/books/useLibraryTableState'
import { EmptyState } from '@/components/ui/states'

const PAGE_SIZE = 20

interface BooksTableProps {
  books: UserBook[]
  knownShelves: string[]
  knownTags: string[]
  onSaved?: () => void
  /** Columns, filters and page, shared with the library toolbar. */
  state: LibraryTableState
}

export default function BooksTable({
  books,
  knownShelves,
  knownTags,
  onSaved,
  state
}: BooksTableProps) {
  const [sort, setSort] = useState<SortState>({ key: 'added', dir: null })
  const { visibleColumns, filters, page, setPage } = state

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

  const activeColumns = useMemo(
    () => ALL_COLUMNS.filter((col) => col.alwaysVisible || visibleColumns.has(col.key)),
    [visibleColumns]
  )

  const filtered = useMemo(() => applyFilters(books, filters), [books, filters])
  const sorted = useMemo(() => sortBooks(filtered, sort), [filtered, sort])
  const pageCount = Math.max(1, Math.ceil(sorted.length / PAGE_SIZE))
  const safePage = Math.min(page, pageCount)
  const pageBooks = sorted.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE)

  const ctx = { knownShelves, knownTags, onSaved }

  return (
    <div className="space-y-3">
      {/* Mobile-first card-per-row view; the rich table would force a ~1167px
          wide sideways scroll below `sm`. */}
      <ul className="grid grid-cols-1 gap-3 sm:hidden">
        {pageBooks.length === 0 ? (
          <li>
            <EmptyState>No books match the current filters.</EmptyState>
          </li>
        ) : (
          pageBooks.map((ub) => (
            <li key={ub.id}>
              <BookCard userBook={ub} onSaved={onSaved} query={undefined} showDescriptionToggle />
            </li>
          ))
        )}
      </ul>

      {/* A container, so an expanded description fits the visible width. */}
      <div className="hidden sm:block @container">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-8 pr-0">
                <span className="sr-only">Description</span>
              </TableHead>
              {activeColumns.map((col) =>
                col.sortKey ? (
                  <SortableHeader
                    key={col.key}
                    dir={sortDir(col.sortKey)}
                    onSort={() => handleSort(col.sortKey!)}
                    className={col.headClassName}
                  >
                    {col.label}
                  </SortableHeader>
                ) : (
                  <TableHead key={col.key} className={col.headClassName}>
                    {col.label}
                  </TableHead>
                )
              )}
            </TableRow>
          </TableHeader>
          <TableBody>
            {pageBooks.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={activeColumns.length + 1}
                  className="py-8 text-center text-muted text-sm"
                >
                  No books match the current filters.
                </TableCell>
              </TableRow>
            ) : (
              pageBooks.map((ub) => (
                <BooksTableRow key={ub.id} userBook={ub} columns={activeColumns} ctx={ctx} />
              ))
            )}
          </TableBody>
        </Table>
      </div>

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
