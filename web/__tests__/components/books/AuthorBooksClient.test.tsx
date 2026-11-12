import React from 'react'
import { render, screen, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  BookShelfSchema,
  LibraryResponseSchema,
  type UserBook,
  type BookShelf
} from '@/lib/gen/books/v1/library_pb'

vi.mock('@/components/books/BookRatingStars', () => {
  return {
    default: function MockRatingStars() {
      return <div data-testid="rating-stars" />
    }
  }
})

vi.mock('@/components/books/BookFavouriteButton', () => {
  return {
    default: function MockFavButton() {
      return <div data-testid="favourite-button" />
    }
  }
})

vi.mock('@/hooks/useBooks', () => ({
  useLibrary: vi.fn(),
  useSetBookTag: () => vi.fn(),
  useRemoveBook: () => vi.fn(),
  useUpdateBookStatus: () => vi.fn()
}))

vi.mock('swr', async () => ({
  ...(await vi.importActual('swr')),
  mutate: vi.fn()
}))

import AuthorBooksClient from '@/components/books/AuthorBooksClient'
import { useLibrary } from '@/hooks/useBooks'

const mockUseBacklogLibrary = vi.mocked(useLibrary)

function makeLibraryWith(books: UserBook[], shelves: BookShelf[]) {
  return {
    data: {
      library: create(LibraryResponseSchema, {
        reading: books.filter((b) => b.status === 'currently-reading'),
        wishlist: books.filter((b) => b.status === 'to-read'),
        finished: books.filter((b) => b.status === 'read'),
        shelves
      })
    },
    error: null,
    isLoading: false
  }
}

const herbertBook = create(UserBookSchema, {
  id: 'ub-1',
  status: 'currently-reading',
  tags: [],
  formats: [],
  book: create(BookSchema, {
    title: 'Dune',
    authors: ['Frank Herbert'],
    pageCount: 412
  })
})

const leGuinBook = create(UserBookSchema, {
  id: 'ub-2',
  status: 'to-read',
  tags: [],
  formats: [],
  book: create(BookSchema, {
    title: 'The Left Hand of Darkness',
    authors: ['Ursula K. Le Guin'],
    pageCount: 286
  })
})

describe('AuthorBooksClient', () => {
  beforeEach(() => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue(makeLibraryWith([herbertBook, leGuinBook], []))
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows only books by the specified author', () => {
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getAllByText('Dune').length).toBeGreaterThan(0)
    expect(screen.queryByText('The Left Hand of Darkness')).not.toBeInTheDocument()
  })

  it('shows the author name as a heading', () => {
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getByRole('heading', { name: 'Frank Herbert' })).toBeInTheDocument()
  })

  it('shows a book count', () => {
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getByText('1 book in your library')).toBeInTheDocument()
  })

  it('shows plural when multiple books exist', () => {
    render(<AuthorBooksClient name="Ursula K. Le Guin" />)
    expect(screen.getByText('1 book in your library')).toBeInTheDocument()
  })

  it('shows no books message for unknown author', () => {
    render(<AuthorBooksClient name="Unknown Author" />)
    expect(screen.getByText('0 books in your library')).toBeInTheDocument()
    expect(screen.getAllByText('No books match the current filters.').length).toBeGreaterThan(0)
  })

  it('includes books from custom shelves (covers flattenLibrary shelf branch)', () => {
    const shelfBook = create(UserBookSchema, {
      id: 'ub-shelf',
      status: 'sci-fi',
      tags: ['favourite', 'space-opera'],
      formats: [],
      book: create(BookSchema, {
        title: 'Chapterhouse: Dune',
        authors: ['Frank Herbert'],
        pageCount: 480
      })
    })
    const shelf = create(BookShelfSchema, { name: 'sci-fi', books: [shelfBook] })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue(makeLibraryWith([], [shelf]))
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getAllByText('Chapterhouse: Dune').length).toBeGreaterThan(0)
  })

  it('collects non-special tags from books (covers knownTags tag filter)', () => {
    const taggedBook = create(UserBookSchema, {
      id: 'ub-tagged',
      status: 'currently-reading',
      tags: ['favourite', 'space-opera'],
      formats: [],
      book: create(BookSchema, {
        title: 'Dune Messiah',
        authors: ['Frank Herbert'],
        pageCount: 220
      })
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue(makeLibraryWith([taggedBook], []))
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getAllByText('Dune Messiah').length).toBeGreaterThan(0)
  })

  it('renders a breadcrumb back to library', () => {
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getByRole('link', { name: 'Library' })).toHaveAttribute('href', '/books/library')
  })

  it('links the Books crumb to the library and names the author last', () => {
    render(<AuthorBooksClient name="Frank Herbert" />)
    const crumbs = within(screen.getByRole('navigation', { name: 'Breadcrumb' }))
    expect(crumbs.getByRole('link', { name: 'Books' })).toHaveAttribute('href', '/books/library')
  })

  it('does not render a dead /backlog breadcrumb link', () => {
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.queryByRole('link', { name: 'Backlog' })).not.toBeInTheDocument()
  })

  it('shows a loading state, and the filter toolbar once loaded', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    const { rerender } = render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading…')
    expect(screen.queryByRole('toolbar')).not.toBeInTheDocument()

    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue(makeLibraryWith([herbertBook], []))
    rerender(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getByRole('toolbar', { name: 'Filter books' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Filters' })).toBeInTheDocument()
  })

  it('shows an error state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue({
      data: undefined,
      error: new Error('boom'),
      isLoading: false
    })
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getByText('Failed to load library.')).toBeInTheDocument()
  })
})
