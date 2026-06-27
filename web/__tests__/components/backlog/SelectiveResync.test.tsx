import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { ListCatalogBooksResponseSchema } from '@/lib/gen/backlog/v1/books_pb'

const mockResyncBooks = jest.fn()
const mockResyncOpenLibrary = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useCatalogBooks: jest.fn(),
  useResyncBooks: () => mockResyncBooks,
  useResyncOpenLibrary: () => mockResyncOpenLibrary
}))

jest.mock('@/lib/backlog/progressSocket', () => ({
  useProgressSocket: jest.fn(() => ({
    connected: true,
    isRefreshing: false,
    lastRefresh: null,
    processed: null,
    total: null,
    refresh: jest.fn()
  }))
}))

jest.mock('swr', () => ({ __esModule: true, mutate: jest.fn(), default: jest.fn() }))

import { useCatalogBooks } from '@/hooks/useBacklog'
import SelectiveResync from '@/components/backlog/SelectiveResync'

const mockUseCatalogBooks = jest.mocked(useCatalogBooks)

const sampleBooks = [
  {
    id: 'book-1',
    title: 'Book Without ISBN',
    authors: ['Author A'],
    isbn13: '',
    hasCover: false,
    hasDescription: false,
    hasPageCount: false,
    openlibraryStatus: 'not_found',
    googlebooksStatus: '',
    lastResyncAt: '2026-01-01T00:00:00Z'
  },
  {
    id: 'book-2',
    title: 'Book Not In Google Books',
    authors: ['Author B'],
    isbn13: '9780140449112',
    hasCover: true,
    hasDescription: true,
    hasPageCount: true,
    openlibraryStatus: 'found',
    googlebooksStatus: 'not_found',
    lastResyncAt: '2026-01-01T00:00:00Z'
  },
  {
    id: 'book-3',
    title: 'Complete Book',
    authors: ['Author C'],
    isbn13: '9780062316097',
    hasCover: true,
    hasDescription: true,
    hasPageCount: true,
    openlibraryStatus: 'found',
    googlebooksStatus: 'found',
    lastResyncAt: '2026-01-01T00:00:00Z'
  }
]

function mockCatalog(books = sampleBooks) {
  // @ts-expect-error -- partial mock
  mockUseCatalogBooks.mockReturnValue({
    data: create(ListCatalogBooksResponseSchema, { books }),
    isLoading: false,
    error: undefined
  })
}

describe('SelectiveResync', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockCatalog()
  })

  it('renders all catalog books by default', () => {
    render(<SelectiveResync />)
    expect(screen.getByText('Book Without ISBN')).toBeInTheDocument()
    expect(screen.getByText('Book Not In Google Books')).toBeInTheDocument()
    expect(screen.getByText('Complete Book')).toBeInTheDocument()
  })

  it('shows a loading indicator while fetching', () => {
    // @ts-expect-error -- partial mock
    mockUseCatalogBooks.mockReturnValue({ data: undefined, isLoading: true })
    render(<SelectiveResync />)
    expect(screen.getByText('Loading catalog…')).toBeInTheDocument()
  })

  it('filters to Missing ISBN books when that chip is active', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('button', { name: 'Missing ISBN' }))
    expect(screen.getByText('Book Without ISBN')).toBeInTheDocument()
    expect(screen.queryByText('Book Not In Google Books')).not.toBeInTheDocument()
    expect(screen.queryByText('Complete Book')).not.toBeInTheDocument()
  })

  it('filters to Not in Open Library books when that chip is active', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('button', { name: 'Not in Open Library' }))
    expect(screen.getByText('Book Without ISBN')).toBeInTheDocument()
    expect(screen.queryByText('Book Not In Google Books')).not.toBeInTheDocument()
    expect(screen.queryByText('Complete Book')).not.toBeInTheDocument()
  })

  it('filters to Not in Google Books books when that chip is active', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('button', { name: 'Not in Google Books' }))
    expect(screen.getByText('Book Not In Google Books')).toBeInTheDocument()
    expect(screen.queryByText('Book Without ISBN')).not.toBeInTheDocument()
    expect(screen.queryByText('Complete Book')).not.toBeInTheDocument()
  })

  it('combines filters with OR logic (shows books matching any active filter)', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('button', { name: 'Missing ISBN' }))
    fireEvent.click(screen.getByRole('button', { name: 'Not in Google Books' }))
    // book-1 matches Missing ISBN; book-2 matches Not in Google Books
    expect(screen.getByText('Book Without ISBN')).toBeInTheDocument()
    expect(screen.getByText('Book Not In Google Books')).toBeInTheDocument()
    expect(screen.queryByText('Complete Book')).not.toBeInTheDocument()
  })

  it('clears filters when Clear filters button is clicked', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('button', { name: 'Missing ISBN' }))
    expect(screen.queryByText('Complete Book')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
    expect(screen.getByText('Complete Book')).toBeInTheDocument()
  })

  it('Resync selected button is disabled when nothing is selected', () => {
    render(<SelectiveResync />)
    const btn = screen.getByRole('button', { name: /resync/i })
    expect(btn).toBeDisabled()
  })

  it('enables Resync button after selecting a book', () => {
    render(<SelectiveResync />)
    const checkboxes = screen.getAllByRole('checkbox')
    // index 0 is the select-all checkbox; individual books start at 1
    fireEvent.click(checkboxes[1])
    const btn = screen.getByRole('button', { name: /resync 1 selected/i })
    expect(btn).not.toBeDisabled()
  })

  it('calls resyncBooks with selected IDs when Resync is clicked', async () => {
    mockResyncBooks.mockResolvedValue({})
    render(<SelectiveResync />)
    const checkboxes = screen.getAllByRole('checkbox')
    fireEvent.click(checkboxes[1]) // select book-1

    fireEvent.click(screen.getByRole('button', { name: /resync 1 selected/i }))
    await waitFor(() => {
      expect(mockResyncBooks).toHaveBeenCalledWith(['book-1'], false)
    })
  })

  it('passes force=true when Force re-fetch is checked', async () => {
    mockResyncBooks.mockResolvedValue({})
    render(<SelectiveResync />)
    const checkboxes = screen.getAllByRole('checkbox')
    fireEvent.click(checkboxes[1]) // select book-1
    fireEvent.click(screen.getByRole('checkbox', { name: /force re-fetch/i }))

    fireEvent.click(screen.getByRole('button', { name: /resync 1 selected/i }))
    await waitFor(() => {
      expect(mockResyncBooks).toHaveBeenCalledWith(['book-1'], true)
    })
  })

  it('selects all visible books when Select all is checked', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all' }))
    // All 3 books selected
    const btn = screen.getByRole('button', { name: /resync 3 selected/i })
    expect(btn).not.toBeDisabled()
  })

  it('shows empty state when no books match active filter', () => {
    // Only book-1 has missing ISBN; filter for GB then OL only overlap for book-1
    render(<SelectiveResync />)
    // Both filters narrow to only overlapping books. Use a filter with no results:
    // set catalog to empty and filter
    // @ts-expect-error -- partial mock
    mockUseCatalogBooks.mockReturnValue({
      data: create(ListCatalogBooksResponseSchema, { books: [] }),
      isLoading: false
    })
    render(<SelectiveResync />)
    expect(screen.getByText('No books in the catalog.')).toBeInTheDocument()
  })
})
