import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  LibraryResponseSchema,
  UserBookSchema,
  BookSchema,
  BookShelfSchema
} from '@/lib/gen/reading/v1/library_pb'
import { GetSharedLibraryResponseSchema } from '@/lib/gen/reading/v1/public_pb'

const mockUseSharedLibrary = jest.fn()

jest.mock('@/hooks/useProfile', () => ({
  useSharedLibrary: () => mockUseSharedLibrary()
}))

import ProfileBooksLibrary from '@/components/profile/ProfileBooksLibrary'

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
      ],
      rss: [
        create(UserBookSchema, {
          id: 'ub-rss',
          status: 'read',
          tags: [],
          book: create(BookSchema, { title: 'RSS Post', authors: ['Author D'], category: 'rss' })
        })
      ]
    }),
    lastSyncedAt: '2026-07-01T10:00:00Z'
  })
}

describe('ProfileBooksLibrary', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('shows custom shelves and tags in the sidebar', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksLibrary token="tok-1" />)

    // Desktop sidebar + mobile chips each render the shelf/tag once.
    expect(screen.getAllByText('custom-shelf').length).toBeGreaterThan(0)
    expect(screen.getAllByText('sci-fi').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Favourites').length).toBeGreaterThan(0)
  })

  it('is read-only: no shelf/tag editing', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksLibrary token="tok-1" />)

    expect(screen.queryByText('Edit shelves & tags')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /add to favourites/i })).not.toBeInTheDocument()
  })

  it('shows ratings on book cards', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksLibrary token="tok-1" />)

    expect(screen.getByLabelText('Rated 4 of 5')).toBeInTheDocument()
  })

  it('renders a loading state before data arrives', () => {
    mockUseSharedLibrary.mockReturnValue({ data: undefined, isLoading: true })
    render(<ProfileBooksLibrary token="tok-1" />)

    expect(screen.getByText('Loading books…')).toBeInTheDocument()
  })

  it('shows an error state when the library fails to load', () => {
    mockUseSharedLibrary.mockReturnValue({ data: undefined, error: new Error('nope') })
    render(<ProfileBooksLibrary token="tok-1" />)

    expect(screen.getByText('Failed to load books.')).toBeInTheDocument()
  })

  it('filters books across the library with the search input', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    render(<ProfileBooksLibrary token="tok-1" />)

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
    render(<ProfileBooksLibrary token="tok-1" />)

    // Default "all books" view shows every book once in the grid.
    expect(screen.getAllByText('Reading Book')).toHaveLength(1)

    // Fixed favourite shelf narrows the grid to the tagged book.
    fireEvent.click(screen.getAllByText('Favourites')[0]!)
    expect(screen.getByText('Wishlist Book')).toBeInTheDocument()
    expect(screen.queryByText('Reading Book')).not.toBeInTheDocument()

    // Custom shelf shows its own books.
    fireEvent.click(screen.getAllByText('custom-shelf')[0]!)
    expect(screen.getByText('Shelved Book')).toBeInTheDocument()

    // Fixed status shelves work too.
    fireEvent.click(screen.getAllByRole('button', { name: /^Want to read \d/ })[0]!)
    expect(screen.getByText('Wishlist Book')).toBeInTheDocument()
    fireEvent.click(screen.getAllByRole('button', { name: /^Currently reading \d/ })[0]!)
    fireEvent.click(screen.getAllByRole('button', { name: /^Read \d/ })[0]!)
    fireEvent.click(screen.getAllByRole('button', { name: /^All books \d/ })[0]!)

    // Tag selection filters the grid, clicking again returns to all books.
    fireEvent.click(screen.getAllByText('sci-fi')[0]!)
    expect(screen.getByText('Wishlist Book')).toBeInTheDocument()
    expect(screen.queryByText('Reading Book')).not.toBeInTheDocument()
    fireEvent.click(screen.getAllByText('sci-fi')[0]!)
    expect(screen.getAllByText('Reading Book')).toHaveLength(1)
  })
})
