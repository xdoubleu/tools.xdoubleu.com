import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  LibraryResponseSchema,
  GetLibraryResponseSchema,
  BookShelfSchema
} from '@/lib/gen/books/v1/library_pb'

jest.mock('@/hooks/useBooks', () => ({
  useLibrary: jest.fn(),
  useUpdateBookStatus: () => jest.fn().mockResolvedValue({})
}))

const mockRouterPush = jest.fn()

jest.mock('@/hooks/useAuth', () => ({
  useCurrentUser: jest.fn()
}))

jest.mock('@/components/books/BookSourceSync', () => {
  return function MockBookSourceSync() {
    return <div data-testid="book-source-sync" />
  }
})

jest.mock('next/navigation', () => ({
  useRouter: jest.fn(() => ({ push: mockRouterPush }))
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('next/image', () => {
  return function MockImage({ src, alt }: { src: string; alt: string }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} />
  }
})

// Stub inline controls so detail-page tests focus on data display
jest.mock('@/components/books/BookProgressEditor', () => {
  return function MockProgressEditor() {
    return <div role="progressbar" />
  }
})

jest.mock('@/components/books/BookRatingStars', () => {
  return function MockRatingStars({ userBook }: { userBook: { rating: number } }) {
    return <div aria-label={`${userBook.rating} out of 5 stars`} data-testid="rating-stars" />
  }
})

jest.mock('@/components/books/BookFavouriteButton', () => {
  return function MockFavButton({ userBook }: { userBook: { tags: string[] } }) {
    return <div data-testid="favourite-button" aria-pressed={userBook.tags.includes('favourite')} />
  }
})

jest.mock('@/components/books/BookOwnershipToggles', () => {
  return function MockOwnership() {
    return <div data-testid="ownership-toggles" />
  }
})

jest.mock('@/components/books/BookShelfPopover', () => {
  return function MockShelfPopover() {
    return <div data-testid="shelf-popover" />
  }
})

jest.mock('@/components/books/KoboSyncToggle', () => {
  return function MockKoboSyncToggle() {
    return <div data-testid="kobo-sync-toggle" />
  }
})

jest.mock('@/components/books/BookPreviewDialog', () => {
  return function MockBookPreviewDialog() {
    return <div data-testid="book-preview-dialog" />
  }
})

jest.mock('@/components/books/RemoveBookDialog', () => {
  return function MockRemoveBookDialog({
    open,
    onRemoved
  }: {
    open: boolean
    onRemoved: () => void
  }) {
    if (!open) return null
    return (
      <div data-testid="remove-book-dialog">
        <button type="button" onClick={onRemoved}>
          Confirm remove
        </button>
      </div>
    )
  }
})

jest.mock('swr', () => ({
  mutate: jest.fn()
}))

import BookDetailClient from '@/app/books/[id]/BookDetailClient'
import { useLibrary } from '@/hooks/useBooks'
import { useCurrentUser } from '@/hooks/useAuth'

const mockBook = create(BookSchema, {
  id: 'book-1',
  title: 'Dune',
  authors: ['Frank Herbert'],
  description: 'A science fiction epic set in the far future.',
  pageCount: 412,
  isbn13: '9780441013593',
  coverUrl: 'https://example.com/dune.jpg'
})

const mockUserBook = create(UserBookSchema, {
  id: 'ub-1',
  bookId: 'book-1',
  book: mockBook,
  status: 'currently-reading',
  rating: 4,
  tags: ['favourite', 'sci-fi'],
  formats: ['epub'],
  finishedAt: [],
  addedAt: '2026-01-15T00:00:00Z',
  updatedAt: '2026-06-01T00:00:00Z',
  progressMode: 'pages',
  currentPage: 200,
  progressPercent: 48
})

function makeLibraryData(
  reading: ReturnType<typeof create<typeof UserBookSchema>>[] = [mockUserBook],
  wishlist: ReturnType<typeof create<typeof UserBookSchema>>[] = [],
  finished: ReturnType<typeof create<typeof UserBookSchema>>[] = [],
  shelves: ReturnType<typeof create<typeof BookShelfSchema>>[] = []
) {
  return create(GetLibraryResponseSchema, {
    library: create(LibraryResponseSchema, { reading, wishlist, finished, shelves })
  })
}

beforeEach(() => {
  jest.clearAllMocks()
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  jest.mocked(useLibrary).mockReturnValue({
    data: makeLibraryData(),
    isLoading: false,
    error: undefined
  })
  // @ts-expect-error -- partial mock
  jest.mocked(useCurrentUser).mockReturnValue({ data: { role: 'user' }, isLoading: false })
})

describe('BookDetailClient', () => {
  it('shows loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('Loading book…')).toBeInTheDocument()
  })

  it('shows error state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('fail')
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('Failed to load book.')).toBeInTheDocument()
  })

  it('shows not found when id has no match', () => {
    render(<BookDetailClient id="ub-unknown" />)
    expect(screen.getByText('Book not found.')).toBeInTheDocument()
  })

  it('renders title and author', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByRole('heading', { name: 'Dune' })).toBeInTheDocument()
    expect(screen.getByText('Frank Herbert')).toBeInTheDocument()
  })

  it('renders description', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('A science fiction epic set in the far future.')).toBeInTheDocument()
  })

  it('shows no description fallback when description is empty', () => {
    const noDescBook = create(UserBookSchema, {
      ...mockUserBook,
      book: create(BookSchema, { ...mockBook, description: '' })
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([noDescBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('No description available.')).toBeInTheDocument()
  })

  it('renders star rating control for a read book', () => {
    const readBook = create(UserBookSchema, { ...mockUserBook, status: 'read' })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([], [], [readBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByTestId('rating-stars')).toBeInTheDocument()
    expect(screen.getByLabelText('4 out of 5 stars')).toBeInTheDocument()
  })

  it('hides star rating and favourite for a non-read book', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByTestId('rating-stars')).not.toBeInTheDocument()
    expect(screen.queryByTestId('favourite-button')).not.toBeInTheDocument()
  })

  it('renders page count', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('412 pages')).toBeInTheDocument()
  })

  it('renders ISBN', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('ISBN: 9780441013593')).toBeInTheDocument()
  })

  it('renders progress editor for currently-reading', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByRole('progressbar')).toBeInTheDocument()
  })

  it('does not render progress editor for non-reading status', () => {
    const wishlistBook = create(UserBookSchema, { ...mockUserBook, status: 'to-read' })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([], [wishlistBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })

  it('renders breadcrumb with Books and Library links', () => {
    render(<BookDetailClient id="ub-1" />)
    const booksLink = screen.getByText('Books').closest('a')
    expect(booksLink).toHaveAttribute('href', '/books')
    const libraryLink = screen.getByText('Library').closest('a')
    expect(libraryLink).toHaveAttribute('href', '/books/library')
  })

  it('renders shelf popover', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByTestId('shelf-popover')).toBeInTheDocument()
  })

  it('renders favourite button for a read book', () => {
    const readBook = create(UserBookSchema, { ...mockUserBook, status: 'read' })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([], [], [readBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByTestId('favourite-button')).toBeInTheDocument()
  })

  it('hides Kobo sync toggle when book is not owned digitally', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByTestId('kobo-sync-toggle')).not.toBeInTheDocument()
  })

  it('shows Kobo sync toggle when book is owned digitally', () => {
    const digitalBook = create(UserBookSchema, {
      ...mockUserBook,
      tags: ['favourite', 'sci-fi', 'own-digital']
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([digitalBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByTestId('kobo-sync-toggle')).toBeInTheDocument()
  })

  it('finds a book in shelves', () => {
    const shelfBook = create(UserBookSchema, {
      ...mockUserBook,
      id: 'ub-shelf',
      status: 'to-read'
    })
    const shelf = create(BookShelfSchema, { name: 'To Buy', books: [shelfBook] })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([], [], [], [shelf]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-shelf" />)
    expect(screen.getByRole('heading', { name: 'Dune' })).toBeInTheDocument()
  })

  it('shows preview buttons when book has epub format', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByRole('button', { name: 'Preview EPUB' })).toBeInTheDocument()
  })

  it('shows preview dialog when preview button is clicked', () => {
    render(<BookDetailClient id="ub-1" />)
    fireEvent.click(screen.getByRole('button', { name: 'Preview EPUB' }))
    expect(screen.getByTestId('book-preview-dialog')).toBeInTheDocument()
  })

  it('does not show preview buttons when no formats', () => {
    const noFormatBook = create(UserBookSchema, { ...mockUserBook, formats: [] })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([noFormatBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByRole('button', { name: 'Preview EPUB' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Preview PDF' })).not.toBeInTheDocument()
  })

  it('shows added date', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText(/Added/)).toBeInTheDocument()
  })

  it('shows finished dates when present', () => {
    const finishedBook = create(UserBookSchema, {
      ...mockUserBook,
      finishedAt: ['2026-03-10T00:00:00Z']
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([finishedBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('Finished')).toBeInTheDocument()
  })

  it('hides the metadata source sync control for a non-admin', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByTestId('book-source-sync')).not.toBeInTheDocument()
  })

  it('shows the metadata source sync control for an admin', () => {
    // @ts-expect-error -- partial mock
    jest.mocked(useCurrentUser).mockReturnValue({ data: { role: 'admin' }, isLoading: false })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByTestId('book-source-sync')).toBeInTheDocument()
  })

  it('opens the remove dialog when Remove from library is clicked', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByTestId('remove-book-dialog')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Remove from library' }))
    expect(screen.getByTestId('remove-book-dialog')).toBeInTheDocument()
  })

  it('redirects to the library after a book is removed', () => {
    mockRouterPush.mockClear()
    render(<BookDetailClient id="ub-1" />)
    fireEvent.click(screen.getByRole('button', { name: 'Remove from library' }))
    fireEvent.click(screen.getByText('Confirm remove'))
    expect(mockRouterPush).toHaveBeenCalledWith('/books/library')
  })
})
