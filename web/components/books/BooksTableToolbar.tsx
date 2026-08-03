'use client'

import { Popover } from '@/components/ui/popover'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import type { ColumnKey, BookColumn } from '@/components/books/booksTableColumns'

export interface LibraryFilters {
  /** Special ownership tags: 'own-physical' | 'own-digital'. */
  ownership: Set<string>
  /** Format strings from UserBook.formats: 'pdf' | 'epub'. */
  format: Set<string>
  /** Kobo sync tag: 'kobo-sync'. */
  kobo: Set<string>
}

interface BooksTableToolbarProps {
  columns: BookColumn[]
  visibleColumns: Set<ColumnKey>
  onToggleColumn: (key: ColumnKey) => void
  filters: LibraryFilters
  onToggleOwnership: (tag: string) => void
  onToggleFormat: (fmt: string) => void
  onToggleKobo: (tag: string) => void
  onClearFilters: () => void
}

export default function BooksTableToolbar({
  columns,
  visibleColumns,
  onToggleColumn,
  filters,
  onToggleOwnership,
  onToggleFormat,
  onToggleKobo,
  onClearFilters
}: BooksTableToolbarProps) {
  const activeFilterCount = filters.ownership.size + filters.format.size + filters.kobo.size
  const toggleableColumns = columns.filter((c) => !c.alwaysVisible)

  return (
    <div className="flex items-center gap-2">
      {/* Columns toggle */}
      <Popover
        align="left"
        trigger={({ open, onClick }) => (
          <Button variant="secondary" size="sm" onClick={onClick} aria-expanded={open}>
            Columns
          </Button>
        )}
      >
        <div className="space-y-1.5">
          <p className="text-xs font-semibold text-muted uppercase tracking-wide mb-2">
            Show columns
          </p>
          <div className="flex flex-col gap-1">
            {toggleableColumns.map((col) => (
              <Checkbox
                key={col.key}
                id={`col-${col.key}`}
                label={col.label}
                checked={visibleColumns.has(col.key)}
                onChange={() => onToggleColumn(col.key)}
              />
            ))}
          </div>
        </div>
      </Popover>

      {/* Filters */}
      <Popover
        align="left"
        trigger={({ open, onClick }) => (
          <Button variant="secondary" size="sm" onClick={onClick} aria-expanded={open}>
            Filters
            {activeFilterCount > 0 && (
              <Badge variant="default" className="ml-1.5 px-1.5 py-0 text-xs">
                {activeFilterCount}
              </Badge>
            )}
          </Button>
        )}
      >
        <div className="space-y-3 min-w-40">
          {/* Ownership group */}
          <div className="space-y-1.5">
            <p className="text-xs font-semibold text-muted uppercase tracking-wide">Ownership</p>
            <div className="flex flex-col gap-1">
              <Checkbox
                id="filter-own-physical"
                label="Physical"
                checked={filters.ownership.has('own-physical')}
                onChange={() => onToggleOwnership('own-physical')}
              />
              <Checkbox
                id="filter-own-digital"
                label="Digital"
                checked={filters.ownership.has('own-digital')}
                onChange={() => onToggleOwnership('own-digital')}
              />
            </div>
          </div>

          {/* Format group */}
          <div className="space-y-1.5">
            <p className="text-xs font-semibold text-muted uppercase tracking-wide">Format</p>
            <div className="flex flex-col gap-1">
              <Checkbox
                id="filter-format-pdf"
                label="PDF"
                checked={filters.format.has('pdf')}
                onChange={() => onToggleFormat('pdf')}
              />
              <Checkbox
                id="filter-format-epub"
                label="EPUB"
                checked={filters.format.has('epub')}
                onChange={() => onToggleFormat('epub')}
              />
            </div>
          </div>

          {/* Kobo group */}
          <div className="space-y-1.5">
            <p className="text-xs font-semibold text-muted uppercase tracking-wide">Kobo</p>
            <div className="flex flex-col gap-1">
              <Checkbox
                id="filter-kobo-sync"
                label="Synced to Kobo"
                checked={filters.kobo.has('kobo-sync')}
                onChange={() => onToggleKobo('kobo-sync')}
              />
            </div>
          </div>

          {activeFilterCount > 0 && (
            <Button variant="secondary" size="sm" onClick={onClearFilters} className="w-full">
              Clear filters
            </Button>
          )}
        </div>
      </Popover>
    </div>
  )
}
