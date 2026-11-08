import { useMemo, useState } from 'react'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import type { LibraryFilters } from '@/components/books/BooksTableToolbar'
import { DEFAULT_VISIBLE_COLUMNS, type ColumnKey } from '@/components/books/booksTableColumns'
import { useLocalStorage } from '@/hooks/useLocalStorage'

export function applyFilters(books: UserBook[], filters: LibraryFilters): UserBook[] {
  return books.filter((ub) => {
    // own-bol (Kobo store) is a kind of digital ownership, so it also matches the Digital filter.
    const owned = new Set(ub.tags)
    if (owned.has('own-bol')) owned.add('own-digital')
    if (filters.ownership.size > 0 && ![...filters.ownership].some((tag) => owned.has(tag))) {
      return false
    }
    if (filters.format.size > 0 && ![...filters.format].some((fmt) => ub.formats.includes(fmt))) {
      return false
    }
    if (filters.kobo.size > 0 && !ub.tags.includes('kobo-sync')) {
      return false
    }
    return true
  })
}

function toggled<T>(values: Set<T>, value: T): T[] {
  const next = new Set(values)
  if (next.has(value)) {
    next.delete(value)
  } else {
    next.add(value)
  }
  return Array.from(next)
}

export interface LibraryTableState {
  visibleColumns: Set<ColumnKey>
  toggleColumn: (key: ColumnKey) => void
  filters: LibraryFilters
  toggleOwnership: (tag: string) => void
  toggleFormat: (fmt: string) => void
  toggleKobo: (tag: string) => void
  clearFilters: () => void
  page: number
  setPage: (page: number | ((prev: number) => number)) => void
}

/**
 * The library table's columns, filters and page, held above the table so the
 * library toolbar can render its controls. Columns and filters persist.
 */
export function useLibraryTableState(): LibraryTableState {
  const [page, setPage] = useState(1)
  // Stored as arrays for JSON.
  const [visibleColumnKeys, setVisibleColumnKeys] = useLocalStorage<ColumnKey[]>(
    'backlog:library:columns',
    DEFAULT_VISIBLE_COLUMNS
  )
  const [ownership, setOwnership] = useLocalStorage<string[]>(
    'backlog:library:filter:ownership',
    []
  )
  const [format, setFormat] = useLocalStorage<string[]>('backlog:library:filter:format', [])
  const [kobo, setKobo] = useLocalStorage<string[]>('backlog:library:filter:kobo', [])

  const visibleColumns = useMemo(() => new Set(visibleColumnKeys), [visibleColumnKeys])
  const filters: LibraryFilters = useMemo(
    () => ({ ownership: new Set(ownership), format: new Set(format), kobo: new Set(kobo) }),
    [ownership, format, kobo]
  )

  return {
    visibleColumns,
    toggleColumn: (key) => setVisibleColumnKeys(toggled(visibleColumns, key)),
    filters,
    toggleOwnership: (tag) => {
      setOwnership(toggled(filters.ownership, tag))
      setPage(1)
    },
    toggleFormat: (fmt) => {
      setFormat(toggled(filters.format, fmt))
      setPage(1)
    },
    toggleKobo: (tag) => {
      setKobo(toggled(filters.kobo, tag))
      setPage(1)
    },
    clearFilters: () => {
      setOwnership([])
      setFormat([])
      setKobo([])
      setPage(1)
    },
    page,
    setPage
  }
}
