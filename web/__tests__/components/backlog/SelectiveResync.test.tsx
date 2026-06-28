import React from 'react'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
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
import { useProgressSocket } from '@/lib/backlog/progressSocket'
import SelectiveResync from '@/components/backlog/SelectiveResync'

const mockUseCatalogBooks = jest.mocked(useCatalogBooks)
const mockUseProgressSocket = jest.mocked(useProgressSocket)

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
    // Found in OL but NOT in GB — should NOT appear under the "not_in_gb" filter
    // because OL already sourced the metadata.
    id: 'book-2',
    title: 'Book Found Only In Open Library',
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
  },
  {
    // Not found in either provider — should appear under "not_in_gb".
    id: 'book-4',
    title: 'Book Not Found Anywhere',
    authors: ['Author D'],
    isbn13: '9780000000000',
    hasCover: false,
    hasDescription: false,
    hasPageCount: false,
    openlibraryStatus: 'not_found',
    googlebooksStatus: 'not_found',
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
    // Reset progress socket to idle state
    mockUseProgressSocket.mockReturnValue({
      connected: true,
      isRefreshing: false,
      lastRefresh: null,
      processed: null,
      total: null,
      refresh: jest.fn()
    })
  })

  // ---------------------------------------------------------------------------
  // Rendering
  // ---------------------------------------------------------------------------

  it('renders all catalog books by default', () => {
    render(<SelectiveResync />)
    expect(screen.getByText('Book Without ISBN')).toBeInTheDocument()
    expect(screen.getByText('Book Found Only In Open Library')).toBeInTheDocument()
    expect(screen.getByText('Complete Book')).toBeInTheDocument()
    expect(screen.getByText('Book Not Found Anywhere')).toBeInTheDocument()
  })

  it('shows a loading indicator while fetching', () => {
    // @ts-expect-error -- partial mock
    mockUseCatalogBooks.mockReturnValue({ data: undefined, isLoading: true })
    render(<SelectiveResync />)
    expect(screen.getByText('Loading catalog…')).toBeInTheDocument()
  })

  it('renders one Resync button per visible book', () => {
    render(<SelectiveResync />)
    const resyncBtns = screen.getAllByRole('button', { name: 'Resync' })
    expect(resyncBtns).toHaveLength(sampleBooks.length)
  })

  it('shows empty state when no books match the active filter', () => {
    // @ts-expect-error -- partial mock
    mockUseCatalogBooks.mockReturnValue({
      data: create(ListCatalogBooksResponseSchema, { books: [] }),
      isLoading: false
    })
    render(<SelectiveResync />)
    expect(screen.getByText('No books in the catalog.')).toBeInTheDocument()
  })

  it('shows filter empty state when active filter matches nothing', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('button', { name: 'Missing ISBN' }))
    // Deactivate filter chips so we can test the "no filter match" message
    // by providing a catalog where no book is ISBN-less.
    // @ts-expect-error -- partial mock
    mockUseCatalogBooks.mockReturnValue({
      data: create(ListCatalogBooksResponseSchema, {
        books: [
          {
            id: 'x',
            title: 'A Book',
            authors: ['Auth'],
            isbn13: '9780000000001',
            hasCover: true,
            hasDescription: true,
            hasPageCount: true,
            openlibraryStatus: 'found',
            googlebooksStatus: 'found',
            lastResyncAt: '2026-01-01T00:00:00Z'
          }
        ]
      }),
      isLoading: false
    })
    render(<SelectiveResync />)
    // The second render uses the new catalog; click Missing ISBN
    const chips = screen.getAllByRole('button', { name: 'Missing ISBN' })
    fireEvent.click(chips[chips.length - 1])
    expect(screen.getByText('No books match the active filters.')).toBeInTheDocument()
  })

  // ---------------------------------------------------------------------------
  // GB badge visibility (Point 2)
  // ---------------------------------------------------------------------------

  it('does not show GB badge when OL found the book and GB did not', () => {
    render(<SelectiveResync />)
    // book-2: openlibraryStatus=found, googlebooksStatus=not_found
    // The OL badge should be shown; GB badge should not appear.
    const item = screen.getByText('Book Found Only In Open Library').closest('li')!
    expect(within(item).getByText(/OL:/)).toBeInTheDocument()
    expect(within(item).queryByText(/GB:/)).not.toBeInTheDocument()
  })

  it('shows GB badge when both OL and GB failed to find the book', () => {
    render(<SelectiveResync />)
    // book-4: openlibraryStatus=not_found, googlebooksStatus=not_found
    const item = screen.getByText('Book Not Found Anywhere').closest('li')!
    expect(within(item).getByText(/OL:/)).toBeInTheDocument()
    expect(within(item).getByText(/GB:/)).toBeInTheDocument()
  })

  it('shows GB badge when GB found the book (even if OL also found it)', () => {
    render(<SelectiveResync />)
    // book-3: both found
    const item = screen.getByText('Complete Book').closest('li')!
    expect(within(item).getByText(/OL:/)).toBeInTheDocument()
    expect(within(item).getByText(/GB:/)).toBeInTheDocument()
  })

  // ---------------------------------------------------------------------------
  // Per-row Resync buttons (Point 3)
  // ---------------------------------------------------------------------------

  it('calls resyncBooks with the book ID when its Resync button is clicked', async () => {
    mockResyncBooks.mockResolvedValue({})
    render(<SelectiveResync />)
    // Use the specific row for "Book Without ISBN" to avoid index-order assumptions.
    const row = screen.getByText('Book Without ISBN').closest('li')!
    fireEvent.click(within(row).getByRole('button', { name: 'Resync' }))
    await waitFor(() => {
      expect(mockResyncBooks).toHaveBeenCalledWith(['book-1'], false)
    })
  })

  it('passes force=true when Force re-fetch is checked', async () => {
    mockResyncBooks.mockResolvedValue({})
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('checkbox', { name: 'Force re-fetch' }))
    const row = screen.getByText('Book Without ISBN').closest('li')!
    fireEvent.click(within(row).getByRole('button', { name: 'Resync' }))
    await waitFor(() => {
      expect(mockResyncBooks).toHaveBeenCalledWith(['book-1'], true)
    })
  })

  it('disables all Resync buttons while the job is running', () => {
    mockUseProgressSocket.mockReturnValue({
      connected: true,
      isRefreshing: true,
      lastRefresh: null,
      processed: 1,
      total: 4,
      refresh: jest.fn()
    })
    render(<SelectiveResync />)
    const resyncBtns = screen.getAllByRole('button', { name: 'Resync' })
    resyncBtns.forEach((btn) => expect(btn).toBeDisabled())
  })

  // ---------------------------------------------------------------------------
  // ISBN-less duplicate grouping (Point 1)
  // ---------------------------------------------------------------------------

  it('collapses ISBN-less books with the same title+author into one row', () => {
    const duplicateBooks = [
      {
        id: 'dup-1',
        title: 'My Great Novel',
        authors: ['Jane Doe'],
        isbn13: '',
        hasCover: false,
        hasDescription: false,
        hasPageCount: false,
        openlibraryStatus: '',
        googlebooksStatus: '',
        lastResyncAt: ''
      },
      {
        id: 'dup-2',
        title: 'My Great Novel',
        authors: ['Jane Doe'],
        isbn13: '',
        hasCover: true,
        hasDescription: false,
        hasPageCount: false,
        openlibraryStatus: 'found',
        googlebooksStatus: '',
        lastResyncAt: '2026-01-01T00:00:00Z'
      }
    ]
    // @ts-expect-error -- partial mock
    mockUseCatalogBooks.mockReturnValue({
      data: create(ListCatalogBooksResponseSchema, { books: duplicateBooks }),
      isLoading: false
    })
    render(<SelectiveResync />)

    // Only one row for "My Great Novel"
    expect(screen.getAllByText('My Great Novel')).toHaveLength(1)
    // Count badge shows x2
    expect(screen.getByText('x2')).toBeInTheDocument()
    // Only one Resync button for the whole group
    expect(screen.getAllByRole('button', { name: 'Resync' })).toHaveLength(1)
  })

  it('resyncs all collapsed IDs when the group Resync button is clicked', async () => {
    mockResyncBooks.mockResolvedValue({})
    const duplicateBooks = [
      {
        id: 'dup-1',
        title: 'My Great Novel',
        authors: ['Jane Doe'],
        isbn13: '',
        hasCover: false,
        hasDescription: false,
        hasPageCount: false,
        openlibraryStatus: '',
        googlebooksStatus: '',
        lastResyncAt: ''
      },
      {
        id: 'dup-2',
        title: 'My Great Novel',
        authors: ['Jane Doe'],
        isbn13: '',
        hasCover: true,
        hasDescription: false,
        hasPageCount: false,
        openlibraryStatus: 'found',
        googlebooksStatus: '',
        lastResyncAt: '2026-01-01T00:00:00Z'
      }
    ]
    // @ts-expect-error -- partial mock
    mockUseCatalogBooks.mockReturnValue({
      data: create(ListCatalogBooksResponseSchema, { books: duplicateBooks }),
      isLoading: false
    })
    render(<SelectiveResync />)

    fireEvent.click(screen.getByRole('button', { name: 'Resync' }))
    await waitFor(() => {
      expect(mockResyncBooks).toHaveBeenCalledWith(
        expect.arrayContaining(['dup-1', 'dup-2']),
        false
      )
    })
  })

  it('does not collapse books that have different ISBNs', () => {
    const books = [
      {
        id: 'a',
        title: 'Same Title',
        authors: ['Same Author'],
        isbn13: '9780000000001',
        hasCover: false,
        hasDescription: false,
        hasPageCount: false,
        openlibraryStatus: '',
        googlebooksStatus: '',
        lastResyncAt: ''
      },
      {
        id: 'b',
        title: 'Same Title',
        authors: ['Same Author'],
        isbn13: '9780000000002',
        hasCover: false,
        hasDescription: false,
        hasPageCount: false,
        openlibraryStatus: '',
        googlebooksStatus: '',
        lastResyncAt: ''
      }
    ]
    // @ts-expect-error -- partial mock
    mockUseCatalogBooks.mockReturnValue({
      data: create(ListCatalogBooksResponseSchema, { books }),
      isLoading: false
    })
    render(<SelectiveResync />)

    // Two separate rows (same title shown twice), two Resync buttons.
    expect(screen.getAllByText('Same Title')).toHaveLength(2)
    expect(screen.getAllByRole('button', { name: 'Resync' })).toHaveLength(2)
  })

  // ---------------------------------------------------------------------------
  // Filters (Points from existing suite, now operating on groups)
  // ---------------------------------------------------------------------------

  it('filters to Missing ISBN books when that chip is active', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('button', { name: 'Missing ISBN' }))
    expect(screen.getByText('Book Without ISBN')).toBeInTheDocument()
    expect(screen.queryByText('Complete Book')).not.toBeInTheDocument()
    expect(screen.queryByText('Book Found Only In Open Library')).not.toBeInTheDocument()
  })

  it('filters to Not in Open Library books when that chip is active', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('button', { name: 'Not in Open Library' }))
    expect(screen.getByText('Book Without ISBN')).toBeInTheDocument()
    expect(screen.getByText('Book Not Found Anywhere')).toBeInTheDocument()
    expect(screen.queryByText('Complete Book')).not.toBeInTheDocument()
  })

  it('filters to Not in Google Books books when that chip is active', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('button', { name: 'Not in Google Books' }))
    // Only book-4 (not found in OL or GB) should appear.
    // book-2 (found in OL, not in GB) must NOT appear — OL already sourced it.
    expect(screen.getByText('Book Not Found Anywhere')).toBeInTheDocument()
    expect(screen.queryByText('Book Found Only In Open Library')).not.toBeInTheDocument()
    expect(screen.queryByText('Book Without ISBN')).not.toBeInTheDocument()
    expect(screen.queryByText('Complete Book')).not.toBeInTheDocument()
  })

  it('excludes OL-found books from the Not in Google Books filter', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('button', { name: 'Not in Google Books' }))
    expect(screen.queryByText('Book Found Only In Open Library')).not.toBeInTheDocument()
  })

  it('combines filters with OR logic', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('button', { name: 'Missing ISBN' }))
    fireEvent.click(screen.getByRole('button', { name: 'Not in Google Books' }))
    expect(screen.getByText('Book Without ISBN')).toBeInTheDocument()
    expect(screen.getByText('Book Not Found Anywhere')).toBeInTheDocument()
    expect(screen.queryByText('Book Found Only In Open Library')).not.toBeInTheDocument()
    expect(screen.queryByText('Complete Book')).not.toBeInTheDocument()
  })

  it('clears filters when Clear filters button is clicked', () => {
    render(<SelectiveResync />)
    fireEvent.click(screen.getByRole('button', { name: 'Missing ISBN' }))
    expect(screen.queryByText('Complete Book')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
    expect(screen.getByText('Complete Book')).toBeInTheDocument()
  })
})
