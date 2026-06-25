import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

jest.mock('@/hooks/useBacklog', () => ({
  useBacklogLibrary: jest.fn()
}))

jest.mock('next/image', () => {
  return function MockImage({ src, alt, ...props }: { src: string; alt: string }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} {...props} />
  }
})

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/backlog/BookSearchBar', () => {
  return function MockBookSearchBar({ onAdded }: { onAdded: () => void }) {
    return (
      <button data-testid="book-search-bar" onClick={onAdded}>
        search
      </button>
    )
  }
})

// Stub out the interactive child controls so BooksSection tests focus on
// the library structure (shelves, search, loading/error states), not the
// individual edit controls.
jest.mock('@/components/backlog/BookProgressEditor', () => {
  return function MockProgressEditor() {
    return <div role="progressbar" />
  }
})

jest.mock('@/components/backlog/BookRatingStars', () => {
  return function MockRatingStars() {
    return <div data-testid="rating-stars" />
  }
})

jest.mock('@/components/backlog/BookFavouriteButton', () => {
  return function MockFavButton() {
    return <div data-testid="favourite-button" />
  }
})

jest.mock('@/components/backlog/BookOwnershipToggles', () => {
  return function MockOwnership() {
    return <div data-testid="ownership-toggles" />
  }
})

jest.mock('@/components/backlog/BookShelfPopover', () => {
  return function MockShelfPopover() {
    return <div data-testid="shelf-popover" />
  }
})

import BooksSection from '@/components/backlog/BooksSection'
import { useBacklogLibrary } from '@/hooks/useBacklog'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  BookShelfSchema,
  LibraryResponseSchema,
  GetLibraryResponseSchema
} from '@/lib/gen/backlog/v1/books_pb'

jest.mock('swr', () => ({ mutate: jest.fn() }))
import { mutate } from 'swr'

const mockUseBacklogLibrary = jest.mocked(useBacklogLibrary)

const readingBook = create(UserBookSchema, {
  id: '1',
  status: 'currently-reading',
  rating: 4,
  tags: ['favourite'],
  formats: [],
  progressMode: 'pages',
  currentPage: 50,
  book: create(BookSchema, {
    title: 'Dune',
    authors: ['Frank Herbert'],
    coverUrl: 'http://example.com/dune.png',
    pageCount: 200
  })
})

const finishedBook = create(UserBookSchema, {
  id: '2',
  status: 'read',
  formats: [],
  book: create(BookSchema, { title: 'Foundation', authors: ['Isaac Asimov'] })
})

const wishlistBook = create(UserBookSchema, {
  id: '3',
  status: 'to-read',
  formats: [],
  book: create(BookSchema, { title: 'Hyperion', authors: ['Dan Simmons'] })
})

const shelfBook = create(UserBookSchema, {
  id: '4',
  status: 'currently-reading',
  formats: [],
  book: create(BookSchema, { title: 'Neuromancer', authors: ['William Gibson'] })
})

function mockLibrary() {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseBacklogLibrary.mockReturnValue({
    data: create(GetLibraryResponseSchema, {
      library: create(LibraryResponseSchema, {
        reading: [readingBook],
        finished: [finishedBook],
        wishlist: [wishlistBook],
        shelves: [create(BookShelfSchema, { name: 'Sci-Fi', books: [shelfBook] })]
      })
    }),
    error: undefined,
    isLoading: false
  })
}

describe('BooksSection', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('renders the default shelf (Currently Reading) on load', () => {
    mockLibrary()
    render(<BooksSection />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.queryByText('Foundation')).not.toBeInTheDocument()
  })

  it('shows shelf nav buttons in the sidebar', () => {
    mockLibrary()
    render(<BooksSection />)
    expect(screen.getAllByText('Currently Reading').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Wishlist').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Finished').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Sci-Fi').length).toBeGreaterThan(0)
  })

  it('shows a loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    render(<BooksSection />)
    expect(screen.getByText('Loading books...')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue({
      data: undefined,
      error: new Error('boom'),
      isLoading: false
    })
    render(<BooksSection />)
    expect(screen.getByText('Failed to load books.')).toBeInTheDocument()
  })

  it('shows the search bar and refreshes library on add', () => {
    mockLibrary()
    render(<BooksSection />)
    fireEvent.click(screen.getByTestId('book-search-bar'))
    expect(mutate).toHaveBeenCalledWith('/backlog/books')
  })

  it('shows a progress bar for currently reading books', () => {
    mockLibrary()
    render(<BooksSection />)
    expect(screen.getAllByRole('progressbar').length).toBeGreaterThan(0)
  })

  it('renders rating, favourite, and shelf popover for finished (read) books', () => {
    mockLibrary()
    render(<BooksSection />)
    // Switch to the Finished shelf, which contains the 'read' book
    fireEvent.click(screen.getAllByText('Finished')[0])
    expect(screen.getByTestId('rating-stars')).toBeInTheDocument()
    expect(screen.getByTestId('favourite-button')).toBeInTheDocument()
    expect(screen.getByTestId('shelf-popover')).toBeInTheDocument()
  })

  it('hides rating and favourite for currently-reading books (shelf popover still shown)', () => {
    mockLibrary()
    render(<BooksSection />)
    // Default shelf is Currently Reading
    expect(screen.queryByTestId('rating-stars')).not.toBeInTheDocument()
    expect(screen.queryByTestId('favourite-button')).not.toBeInTheDocument()
    expect(screen.getByTestId('shelf-popover')).toBeInTheDocument()
  })
})
