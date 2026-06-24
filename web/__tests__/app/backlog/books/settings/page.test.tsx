import React from 'react'
import { render, screen, fireEvent, within } from '@testing-library/react'

const mockImportBooks = jest.fn()
const mockRefresh = jest.fn()

// Default resync state: idle, no last-refresh time.
let mockResyncState = {
  connected: true,
  isRefreshing: false,
  lastRefresh: null as Date | null,
  processed: null as number | null,
  total: null as number | null,
  refresh: mockRefresh
}

jest.mock('@/hooks/useBacklog', () => ({
  useImportBooks: () => mockImportBooks,
  useClearLibrary: () => jest.fn(),
  useResyncOpenLibrary: () => jest.fn(),
  useFindDuplicates: () => ({ data: undefined, isLoading: false, mutate: jest.fn() }),
  useMergeBooks: () => jest.fn()
}))

jest.mock('@/lib/backlog/resyncRefresh', () => ({
  useResyncRefresh: () => mockResyncState
}))

jest.mock('@/components/backlog/BulkBookUploader', () => ({
  __esModule: true,
  default: () => <div data-testid="bulk-uploader" />
}))

jest.mock('@/components/backlog/KoboSetup', () => ({
  __esModule: true,
  default: () => <div data-testid="kobo-setup" />
}))

jest.mock('@/components/backlog/KoboDevices', () => ({
  __esModule: true,
  default: () => <div data-testid="kobo-devices" />
}))

jest.mock('@/components/backlog/ClearLibraryDialog', () => ({
  __esModule: true,
  default: ({ open }: { open: boolean }) => (open ? <div data-testid="clear-dialog" /> : null)
}))

jest.mock('swr', () => ({ __esModule: true, mutate: jest.fn(), default: jest.fn() }))

import BacklogBooksSettingsPage from '@/app/backlog/books/settings/page'

describe('BacklogBooksSettingsPage', () => {
  beforeEach(() => {
    mockResyncState = {
      connected: true,
      isRefreshing: false,
      lastRefresh: null,
      processed: null,
      total: null,
      refresh: mockRefresh
    }
    mockRefresh.mockClear()
  })

  it('renders the Books Settings heading', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByRole('heading', { name: 'Books Settings' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /backlog/books', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByRole('link', { name: 'Books' })).toHaveAttribute('href', '/backlog/books')
  })

  it('renders the Import books section', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByText('Import books')).toBeInTheDocument()
    expect(screen.getByText('Import CSV')).toBeInTheDocument()
  })

  it('renders the Upload ebooks section with BulkBookUploader', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByText('Upload ebooks')).toBeInTheDocument()
    expect(screen.getByTestId('bulk-uploader')).toBeInTheDocument()
  })

  it('renders the Kobo section header', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByText('Kobo')).toBeInTheDocument()
  })

  it('renders the KoboSetup component', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByTestId('kobo-setup')).toBeInTheDocument()
  })

  it('renders the KoboDevices component', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByTestId('kobo-devices')).toBeInTheDocument()
  })

  it('renders the Connected devices heading', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByText('Connected devices')).toBeInTheDocument()
  })

  it('renders the Danger zone section with clear-library button', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByText('Danger zone')).toBeInTheDocument()
    expect(screen.getByTestId('clear-library-btn')).toBeInTheDocument()
  })

  it('renders the Resync with Open Library section and button', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.getAllByText('Resync with Open Library').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByTestId('resync-openlibrary-btn')).toBeInTheDocument()
  })

  it('calls refresh when the resync button is clicked', () => {
    render(<BacklogBooksSettingsPage />)
    fireEvent.click(screen.getByTestId('resync-openlibrary-btn'))
    expect(mockRefresh).toHaveBeenCalledTimes(1)
  })

  it('disables the button and shows Resyncing… while isRefreshing', () => {
    mockResyncState = { ...mockResyncState, isRefreshing: true }
    render(<BacklogBooksSettingsPage />)
    const btn = screen.getByTestId('resync-openlibrary-btn')
    expect(btn).toBeDisabled()
    expect(btn.textContent).toBe('Resyncing…')
  })

  it('shows indeterminate progress text before total arrives', () => {
    mockResyncState = { ...mockResyncState, isRefreshing: true, processed: null, total: null }
    render(<BacklogBooksSettingsPage />)
    const progress = screen.getByTestId('resync-openlibrary-progress')
    expect(progress).toBeInTheDocument()
    expect(within(progress).getByText('Resyncing…')).toBeInTheDocument()
  })

  it('shows X / N count and progress bar when total is known', () => {
    mockResyncState = { ...mockResyncState, isRefreshing: true, processed: 37, total: 210 }
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByTestId('resync-openlibrary-progress')).toBeInTheDocument()
    expect(screen.getByText('37 / 210')).toBeInTheDocument()
  })

  it('shows last-synced time after a completed run', () => {
    const ts = new Date('2026-06-24T12:00:00Z')
    mockResyncState = { ...mockResyncState, isRefreshing: false, lastRefresh: ts }
    render(<BacklogBooksSettingsPage />)
    expect(screen.getByTestId('resync-openlibrary-status')).toBeInTheDocument()
    expect(screen.getByTestId('resync-openlibrary-status').textContent).toContain('Last synced')
  })

  it('hides progress and last-synced when idle with no prior run', () => {
    render(<BacklogBooksSettingsPage />)
    expect(screen.queryByTestId('resync-openlibrary-progress')).not.toBeInTheDocument()
    expect(screen.queryByTestId('resync-openlibrary-status')).not.toBeInTheDocument()
  })
})
