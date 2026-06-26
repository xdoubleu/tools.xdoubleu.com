import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import BooksTableToolbar, { type LibraryFilters } from '@/components/backlog/BooksTableToolbar'
import { ALL_COLUMNS } from '@/components/backlog/booksTableColumns'
import type { ColumnKey } from '@/components/backlog/booksTableColumns'

const ALL_VISIBLE = new Set<ColumnKey>(ALL_COLUMNS.map((c) => c.key))

const EMPTY_FILTERS: LibraryFilters = {
  ownership: new Set(),
  format: new Set(),
  kobo: new Set()
}

function renderToolbar(
  overrides: Partial<{
    visibleColumns: Set<ColumnKey>
    filters: LibraryFilters
    onToggleColumn: (k: ColumnKey) => void
    onToggleOwnership: (t: string) => void
    onToggleFormat: (f: string) => void
    onToggleKobo: (t: string) => void
    onClearFilters: () => void
  }> = {}
) {
  return render(
    <BooksTableToolbar
      columns={ALL_COLUMNS}
      visibleColumns={ALL_VISIBLE}
      onToggleColumn={jest.fn()}
      filters={EMPTY_FILTERS}
      onToggleOwnership={jest.fn()}
      onToggleFormat={jest.fn()}
      onToggleKobo={jest.fn()}
      onClearFilters={jest.fn()}
      {...overrides}
    />
  )
}

describe('BooksTableToolbar', () => {
  it('renders Columns and Filters buttons', () => {
    renderToolbar()
    expect(screen.getByRole('button', { name: 'Columns' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Filters/ })).toBeInTheDocument()
  })

  describe('Columns popover', () => {
    it('opens when Columns button is clicked', () => {
      renderToolbar()
      fireEvent.click(screen.getByRole('button', { name: 'Columns' }))
      expect(screen.getByText('Show columns')).toBeInTheDocument()
    })

    it('shows checkboxes for all non-alwaysVisible columns', () => {
      renderToolbar()
      fireEvent.click(screen.getByRole('button', { name: 'Columns' }))
      const toggleable = ALL_COLUMNS.filter((c) => !c.alwaysVisible)
      for (const col of toggleable) {
        expect(screen.getByRole('checkbox', { name: col.label })).toBeInTheDocument()
      }
    })

    it('does not show Cover or Title in the columns list (alwaysVisible)', () => {
      renderToolbar()
      fireEvent.click(screen.getByRole('button', { name: 'Columns' }))
      expect(screen.queryByRole('checkbox', { name: 'Cover' })).not.toBeInTheDocument()
      expect(screen.queryByRole('checkbox', { name: 'Title' })).not.toBeInTheDocument()
    })

    it('calls onToggleColumn when a checkbox is changed', () => {
      const onToggleColumn = jest.fn()
      renderToolbar({ onToggleColumn })
      fireEvent.click(screen.getByRole('button', { name: 'Columns' }))
      fireEvent.click(screen.getByRole('checkbox', { name: 'ISBN' }))
      expect(onToggleColumn).toHaveBeenCalledWith('isbn')
    })

    it('shows checked state for visible columns and unchecked for hidden ones', () => {
      // Only cover and title are visible (both alwaysVisible, so not in toggle list).
      // All other toggleable columns should be unchecked.
      const onlyAlways = new Set<ColumnKey>(['cover', 'title'])
      renderToolbar({ visibleColumns: onlyAlways })
      fireEvent.click(screen.getByRole('button', { name: 'Columns' }))
      expect(screen.getByRole('checkbox', { name: 'Author' })).not.toBeChecked()
      expect(screen.getByRole('checkbox', { name: 'ISBN' })).not.toBeChecked()
      expect(screen.getByRole('checkbox', { name: 'Pages' })).not.toBeChecked()
    })
  })

  describe('Filters popover', () => {
    it('opens when Filters button is clicked', () => {
      renderToolbar()
      fireEvent.click(screen.getByRole('button', { name: /Filters/ }))
      expect(screen.getByText('Ownership')).toBeInTheDocument()
      expect(screen.getByText('Format')).toBeInTheDocument()
      expect(screen.getByText('Kobo')).toBeInTheDocument()
    })

    it('shows Physical, Digital, PDF, EPUB, Synced to Kobo checkboxes', () => {
      renderToolbar()
      fireEvent.click(screen.getByRole('button', { name: /Filters/ }))
      expect(screen.getByRole('checkbox', { name: 'Physical' })).toBeInTheDocument()
      expect(screen.getByRole('checkbox', { name: 'Digital' })).toBeInTheDocument()
      expect(screen.getByRole('checkbox', { name: 'PDF' })).toBeInTheDocument()
      expect(screen.getByRole('checkbox', { name: 'EPUB' })).toBeInTheDocument()
      expect(screen.getByRole('checkbox', { name: 'Synced to Kobo' })).toBeInTheDocument()
    })

    it('calls onToggleOwnership with own-physical when Physical is clicked', () => {
      const onToggleOwnership = jest.fn()
      renderToolbar({ onToggleOwnership })
      fireEvent.click(screen.getByRole('button', { name: /Filters/ }))
      fireEvent.click(screen.getByRole('checkbox', { name: 'Physical' }))
      expect(onToggleOwnership).toHaveBeenCalledWith('own-physical')
    })

    it('calls onToggleFormat with epub when EPUB is clicked', () => {
      const onToggleFormat = jest.fn()
      renderToolbar({ onToggleFormat })
      fireEvent.click(screen.getByRole('button', { name: /Filters/ }))
      fireEvent.click(screen.getByRole('checkbox', { name: 'EPUB' }))
      expect(onToggleFormat).toHaveBeenCalledWith('epub')
    })

    it('does not show Clear filters when no filters are active', () => {
      renderToolbar()
      fireEvent.click(screen.getByRole('button', { name: /Filters/ }))
      expect(screen.queryByRole('button', { name: 'Clear filters' })).not.toBeInTheDocument()
    })

    it('shows Clear filters button when a filter is active', () => {
      const activeFilters: LibraryFilters = {
        ownership: new Set(['own-physical']),
        format: new Set(),
        kobo: new Set()
      }
      renderToolbar({ filters: activeFilters })
      fireEvent.click(screen.getByRole('button', { name: /Filters/ }))
      expect(screen.getByRole('button', { name: 'Clear filters' })).toBeInTheDocument()
    })

    it('calls onClearFilters when Clear filters is clicked', () => {
      const onClearFilters = jest.fn()
      const activeFilters: LibraryFilters = {
        ownership: new Set(['own-physical']),
        format: new Set(),
        kobo: new Set()
      }
      renderToolbar({ filters: activeFilters, onClearFilters })
      fireEvent.click(screen.getByRole('button', { name: /Filters/ }))
      fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
      expect(onClearFilters).toHaveBeenCalled()
    })

    it('shows badge with active filter count on the Filters button', () => {
      const activeFilters: LibraryFilters = {
        ownership: new Set(['own-physical', 'own-digital']),
        format: new Set(['pdf']),
        kobo: new Set()
      }
      renderToolbar({ filters: activeFilters })
      expect(screen.getByText('3')).toBeInTheDocument()
    })

    it('calls onToggleKobo with kobo-sync when Synced to Kobo is clicked', () => {
      const onToggleKobo = jest.fn()
      renderToolbar({ onToggleKobo })
      fireEvent.click(screen.getByRole('button', { name: /Filters/ }))
      fireEvent.click(screen.getByRole('checkbox', { name: 'Synced to Kobo' }))
      expect(onToggleKobo).toHaveBeenCalledWith('kobo-sync')
    })

    it('includes kobo filter in badge count', () => {
      const activeFilters: LibraryFilters = {
        ownership: new Set(),
        format: new Set(),
        kobo: new Set(['kobo-sync'])
      }
      renderToolbar({ filters: activeFilters })
      expect(screen.getByText('1')).toBeInTheDocument()
    })

    it('shows Clear filters button when kobo filter is active', () => {
      const activeFilters: LibraryFilters = {
        ownership: new Set(),
        format: new Set(),
        kobo: new Set(['kobo-sync'])
      }
      renderToolbar({ filters: activeFilters })
      fireEvent.click(screen.getByRole('button', { name: /Filters/ }))
      expect(screen.getByRole('button', { name: 'Clear filters' })).toBeInTheDocument()
    })
  })
})
