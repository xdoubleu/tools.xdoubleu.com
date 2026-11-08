import { Popover } from '@/components/ui/popover'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { SectionHeading } from '@/components/ui/section-heading'
import { ALL_COLUMNS, type ColumnKey, type BookColumn } from '@/components/books/booksTableColumns'
import type { LibraryTableState } from '@/components/books/useLibraryTableState'

export interface LibraryFilters {
  /** Special ownership tags: 'own-physical' | 'own-digital' | 'own-bol'. */
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
    <>
      <Popover
        align="left"
        trigger={({ open, onClick }) => (
          <Button variant="secondary" onClick={onClick} aria-expanded={open}>
            Columns
          </Button>
        )}
      >
        <div className="space-y-1.5">
          <SectionHeading level="eyebrow" className="mb-2">
            Show columns
          </SectionHeading>
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

      <Popover
        align="left"
        trigger={({ open, onClick }) => (
          <Button variant="secondary" onClick={onClick} aria-expanded={open}>
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
          <div className="space-y-1.5">
            <SectionHeading level="eyebrow">Ownership</SectionHeading>
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
              <Checkbox
                id="filter-own-bol"
                label="Kobo store"
                checked={filters.ownership.has('own-bol')}
                onChange={() => onToggleOwnership('own-bol')}
              />
            </div>
          </div>

          <div className="space-y-1.5">
            <SectionHeading level="eyebrow">Format</SectionHeading>
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

          <div className="space-y-1.5">
            <SectionHeading level="eyebrow">Kobo</SectionHeading>
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
            <Button variant="secondary" onClick={onClearFilters} className="w-full">
              Clear filters
            </Button>
          )}
        </div>
      </Popover>
    </>
  )
}

/** The Columns and Filters popovers bound to the library table's state, for a `Toolbar`. */
export function LibraryTableControls({ state }: { state: LibraryTableState }) {
  return (
    <BooksTableToolbar
      columns={ALL_COLUMNS}
      visibleColumns={state.visibleColumns}
      onToggleColumn={state.toggleColumn}
      filters={state.filters}
      onToggleOwnership={state.toggleOwnership}
      onToggleFormat={state.toggleFormat}
      onToggleKobo={state.toggleKobo}
      onClearFilters={state.clearFilters}
    />
  )
}
