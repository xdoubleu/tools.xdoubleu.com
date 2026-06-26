'use client'

import { useState, useMemo } from 'react'
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
import BooksTableToolbar, { type LibraryFilters } from '@/components/backlog/BooksTableToolbar'
import {
  ALL_COLUMNS,
  DEFAULT_VISIBLE_COLUMNS,
  sortBooks,
  nextDir,
  type ColumnKey,
  type SortKey,
  type SortState
} from '@/components/backlog/booksTableColumns'
import { useLocalStorage } from '@/hooks/useLocalStorage'

const PAGE_SIZE = 20

function applyFilters(books: UserBook[], filters: LibraryFilters): UserBook[] {
  return books.filter((ub) => {
    // Ownership: book must have at least one of the selected ownership tags.
    if (
      filters.ownership.size > 0 &&
      ![...filters.ownership].some((tag) => ub.tags.includes(tag))
    ) {
      return false
    }
    // Format: book must have at least one of the selected formats.
    if (filters.format.size > 0 && ![...filters.format].some((fmt) => ub.formats.includes(fmt))) {
      return false
    }
    return true
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

  // Persisted column visibility — stored as an array for JSON serialisation.
  const [visibleColumnKeys, setVisibleColumnKeys] = useLocalStorage<ColumnKey[]>(
    'backlog:library:columns',
    DEFAULT_VISIBLE_COLUMNS
  )
  const visibleColumns = useMemo(() => new Set(visibleColumnKeys), [visibleColumnKeys])

  // Persisted filter selections.
  const [ownershipFilterKeys, setOwnershipFilterKeys] = useLocalStorage<string[]>(
    'backlog:library:filter:ownership',
    []
  )
  const [formatFilterKeys, setFormatFilterKeys] = useLocalStorage<string[]>(
    'backlog:library:filter:format',
    []
  )
  const filters: LibraryFilters = useMemo(
    () => ({
      ownership: new Set(ownershipFilterKeys),
      format: new Set(formatFilterKeys)
    }),
    [ownershipFilterKeys, formatFilterKeys]
  )

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

  function handleToggleColumn(key: ColumnKey) {
    const next = new Set(visibleColumns)
    if (next.has(key)) {
      next.delete(key)
    } else {
      next.add(key)
    }
    setVisibleColumnKeys(Array.from(next))
  }

  function handleToggleOwnership(tag: string) {
    const next = new Set(filters.ownership)
    if (next.has(tag)) {
      next.delete(tag)
    } else {
      next.add(tag)
    }
    setOwnershipFilterKeys(Array.from(next))
    setPage(1)
  }

  function handleToggleFormat(fmt: string) {
    const next = new Set(filters.format)
    if (next.has(fmt)) {
      next.delete(fmt)
    } else {
      next.add(fmt)
    }
    setFormatFilterKeys(Array.from(next))
    setPage(1)
  }

  function handleClearFilters() {
    setOwnershipFilterKeys([])
    setFormatFilterKeys([])
    setPage(1)
  }

  // Only render columns that are explicitly visible (alwaysVisible bypasses the set).
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
      <BooksTableToolbar
        columns={ALL_COLUMNS}
        visibleColumns={visibleColumns}
        onToggleColumn={handleToggleColumn}
        filters={filters}
        onToggleOwnership={handleToggleOwnership}
        onToggleFormat={handleToggleFormat}
        onClearFilters={handleClearFilters}
      />

      <Table>
        <TableHeader>
          <TableRow>
            {activeColumns.map((col) => (
              <SortableHeader
                key={col.key}
                dir={col.sortKey ? sortDir(col.sortKey) : null}
                onSort={col.sortKey ? () => handleSort(col.sortKey!) : () => {}}
                className={col.headClassName}
              >
                {col.label}
              </SortableHeader>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {pageBooks.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={activeColumns.length}
                className="py-8 text-center text-muted text-sm"
              >
                No books match the current filters.
              </TableCell>
            </TableRow>
          ) : (
            pageBooks.map((ub) => (
              <TableRow key={ub.id}>
                {activeColumns.map((col) => (
                  <TableCell key={col.key} className={col.cellClassName}>
                    {col.renderCell(ub, ctx)}
                  </TableCell>
                ))}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>

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
