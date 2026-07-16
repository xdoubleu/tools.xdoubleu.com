import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  LibraryResponseSchema,
  UserBookSchema,
  BookSchema,
  BookShelfSchema
} from '@/lib/gen/books/v1/library_pb'
import { GetSharedLibraryResponseSchema } from '@/lib/gen/books/v1/public_pb'

const mockUseSharedLibrary = jest.fn()
const mockUseSharedBooksProgress = jest.fn()

jest.mock('@/hooks/useProfile', () => ({
  useSharedLibrary: () => mockUseSharedLibrary(),
  useSharedBooksProgress: () => mockUseSharedBooksProgress()
}))

jest.mock('@/components/books/BooksProgressChart', () => () => (
  <div data-testid="books-progress-chart" />
))

import ProfileBooksClient from '@/components/profile/ProfileBooksClient'

function makeLibrary() {
  return create(GetSharedLibraryResponseSchema, {
    library: create(LibraryResponseSchema, {
      reading: [
        create(UserBookSchema, {
          id: 'ub-1',
          status: 'currently-reading',
          tags: [],
          progressPercent: 40,
          book: create(BookSchema, { title: 'Reading Book', authors: ['Author A'] })
        })
      ],
      wishlist: [
        create(UserBookSchema, {
          id: 'ub-2',
          status: 'to-read',
          tags: ['favourite', 'sci-fi'],
          rating: 4,
          book: create(BookSchema, { title: 'Wishlist Book', authors: ['Author B'] })
        })
      ],
      finished: [],
      shelves: [
        create(BookShelfSchema, {
          name: 'custom-shelf',
          books: [
            create(UserBookSchema, {
              id: 'ub-3',
              status: 'custom-shelf',
              tags: [],
              book: create(BookSchema, { title: 'Shelved Book', authors: ['Author C'] })
            })
          ]
        })
      ]
    }),
    lastSyncedAt: '2026-07-01T10:00:00Z'
  })
}

describe('ProfileBooksClient', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUseSharedBooksProgress.mockReturnValue({ data: undefined })
  })

  it('renders stat cards and last synced state', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksClient token="tok-1" />)

    expect(screen.getByText('Total books')).toBeInTheDocument()
    expect(screen.getByText('Read this year')).toBeInTheDocument()
    expect(screen.getByText(/Last synced:/)).toBeInTheDocument()
  })

  it('shows custom shelves and tags in the sidebar', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksClient token="tok-1" />)

    // Desktop sidebar + mobile chips each render the shelf/tag once.
    expect(screen.getAllByText('custom-shelf').length).toBeGreaterThan(0)
    expect(screen.getAllByText('sci-fi').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Favourites').length).toBeGreaterThan(0)
  })

  it('is read-only: no refresh button and no shelf/tag editing', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksClient token="tok-1" />)

    expect(screen.queryByRole('button', { name: /refresh/i })).not.toBeInTheDocument()
    expect(screen.queryByText('Edit shelves & tags')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /favourites$/i })).toBeDefined()
    expect(screen.queryByRole('button', { name: /add to favourites/i })).not.toBeInTheDocument()
  })

  it('shows ratings on book cards', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksClient token="tok-1" />)

    expect(screen.getByLabelText('Rated 4 of 5')).toBeInTheDocument()
  })

  it('shows an error state when the library fails to load', () => {
    mockUseSharedLibrary.mockReturnValue({ data: undefined, error: new Error('nope') })
    render(<ProfileBooksClient token="tok-1" />)

    expect(screen.getByText('Failed to load books.')).toBeInTheDocument()
  })

  it('filters books across the library with the search input', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksClient token="tok-1" />)

    fireEvent.change(screen.getByPlaceholderText('Search books…'), {
      target: { value: 'shelved' }
    })

    expect(screen.getByText('Search results')).toBeInTheDocument()
    expect(screen.getByText('Shelved Book')).toBeInTheDocument()
    expect(screen.queryByText('Wishlist Book')).not.toBeInTheDocument()

    fireEvent.change(screen.getByPlaceholderText('Search books…'), {
      target: { value: 'zzz-no-match' }
    })
    expect(screen.getByText('No books.')).toBeInTheDocument()
  })

  it('selects shelves and tags from the sidebar', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksClient token="tok-1" />)

    // In the default all-books view the reading book renders twice: once in
    // the currently-reading strip and once in the shelf grid.
    expect(screen.getAllByText('Reading Book')).toHaveLength(2)

    // Fixed favourite shelf narrows the grid to the tagged book; the
    // currently-reading strip keeps its own copy.
    fireEvent.click(screen.getAllByText('Favourites')[0]!)
    expect(screen.getByText('Wishlist Book')).toBeInTheDocument()
    expect(screen.getAllByText('Reading Book')).toHaveLength(1)

    // Custom shelf shows its own books.
    fireEvent.click(screen.getAllByText('custom-shelf')[0]!)
    expect(screen.getByText('Shelved Book')).toBeInTheDocument()

    // Fixed status shelves work too ("Want to read" also appears as a stat
    // card label, so target the sidebar buttons by role).
    fireEvent.click(screen.getAllByRole('button', { name: /^Want to read \d/ })[0]!)
    expect(screen.getByText('Wishlist Book')).toBeInTheDocument()
    fireEvent.click(screen.getAllByRole('button', { name: /^Currently reading \d/ })[0]!)
    fireEvent.click(screen.getAllByRole('button', { name: /^Read \d/ })[0]!)
    fireEvent.click(screen.getAllByRole('button', { name: /^All books \d/ })[0]!)

    // Tag selection filters the grid, clicking again returns to all books.
    fireEvent.click(screen.getAllByText('sci-fi')[0]!)
    expect(screen.getByText('Wishlist Book')).toBeInTheDocument()
    expect(screen.getAllByText('Reading Book')).toHaveLength(1)
    fireEvent.click(screen.getAllByText('sci-fi')[0]!)
    expect(screen.getAllByText('Reading Book')).toHaveLength(2)
  })

  it('switches to the all-time chart and loads progress data', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    mockUseSharedBooksProgress.mockReturnValue({
      data: {
        progress: {
          labels: ['2026-01-01', '2026-01-02'],
          values: ['1', '2'],
          dateStart: '2026-01-01',
          dateEnd: '2026-01-02'
        }
      }
    })
    render(<ProfileBooksClient token="tok-1" />)

    fireEvent.click(screen.getByRole('tab', { name: 'All time' }))

    expect(screen.getByTestId('books-progress-chart')).toBeInTheDocument()
    expect(screen.getByLabelText('From')).toBeInTheDocument()
    expect(screen.getByLabelText('To')).toBeInTheDocument()
  })
})
