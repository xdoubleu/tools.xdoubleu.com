import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  LibraryResponseSchema,
  GetLibraryResponseSchema,
  BookShelfSchema,
  ReadSuggestionSchema
} from '@/lib/gen/books/v1/library_pb'

vi.mock('@/hooks/useBooks', () => ({
  useLibrary: vi.fn(),
  useUpdateBookStatus: () => vi.fn().mockResolvedValue({}),
  useToggleTag: () => vi.fn().mockResolvedValue({}),
  useUpdateFinishedAt: () => vi.fn().mockResolvedValue({}),
  useGetBookContent: vi.fn(() => ({ data: undefined, error: undefined })),
  useUpdateBook: () => vi.fn().mockResolvedValue({})
}))

const mockRouterPush = vi.fn()
const mockUseSearchParams = vi.fn(() => new URLSearchParams())

vi.mock('@/hooks/useAuth', () => ({
  useCurrentUser: vi.fn()
}))

vi.mock('@/components/books/BookSourceSync', () => {
  return {
    default: function MockBookSourceSync() {
      return <div data-testid="book-source-sync" />
    }
  }
})

vi.mock('@/components/books/BookEditDialog', () => {
  return {
    default: function MockBookEditDialog({ open }: { open: boolean }) {
      return open ? <div data-testid="book-edit-dialog" /> : null
    }
  }
})

vi.mock('@/hooks/navigation', () => ({
  useRouter: vi.fn(() => ({ push: mockRouterPush })),
  useSearchParams: () => mockUseSearchParams()
}))

// Stub inline controls so detail-page tests focus on data display
vi.mock('@/components/books/BookProgressEditor', () => {
  return {
    default: function MockProgressEditor() {
      return <div role="progressbar" />
    }
  }
})

vi.mock('@/components/books/BookRatingStars', () => {
  return {
    default: function MockRatingStars({ userBook }: { userBook: { rating: number } }) {
      return <div aria-label={`${userBook.rating} out of 5 stars`} data-testid="rating-stars" />
    }
  }
})

vi.mock('@/components/books/BookFavouriteButton', () => {
  return {
    default: function MockFavButton({ userBook }: { userBook: { tags: string[] } }) {
      return (
        <div data-testid="favourite-button" aria-pressed={userBook.tags.includes('favourite')} />
      )
    }
  }
})

vi.mock('@/components/books/BookOwnershipToggles', () => {
  return {
    default: function MockOwnership() {
      return <div data-testid="ownership-toggles" />
    }
  }
})

vi.mock('@/components/books/BookShelfTagFields', () => {
  return {
    default: function MockShelfTagFields() {
      return <div data-testid="shelf-tag-fields" />
    }
  }
})

vi.mock('@/components/books/OfflineBookBadge', () => {
  return {
    default: function MockOfflineBadge({ bookId }: { bookId: string }) {
      return <span>offline:{bookId}</span>
    }
  }
})

vi.mock('@/components/books/KoboSyncToggle', () => {
  return {
    default: function MockKoboSyncToggle() {
      return <div data-testid="kobo-sync-toggle" />
    }
  }
})

vi.mock('@/components/books/RemoveBookDialog', () => {
  return {
    default: function MockRemoveBookDialog({
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
  }
})

vi.mock('swr', () => ({
  mutate: vi.fn()
}))

import BookDetailClient from '@/app/books/[id]/BookDetailClient'
import { useLibrary, useGetBookContent } from '@/hooks/useBooks'
import { useCurrentUser } from '@/hooks/useAuth'

const mockUseGetBookContent = vi.mocked(useGetBookContent)

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
  vi.clearAllMocks()
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  vi.mocked(useLibrary).mockReturnValue({
    data: makeLibraryData(),
    isLoading: false,
    error: undefined
  })
  // @ts-expect-error -- partial mock
  vi.mocked(useCurrentUser).mockReturnValue({ data: { role: 'user' }, isLoading: false })
})

describe('BookDetailClient', () => {
  it('links the series with its position', () => {
    const inSeries = create(UserBookSchema, {
      ...mockUserBook,
      book: create(BookSchema, { ...mockBook, seriesName: 'Dune Chronicles', seriesPosition: 1 })
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([inSeries]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByRole('link', { name: 'Dune Chronicles #1' })).toHaveAttribute(
      'href',
      '/books/series/Dune%20Chronicles'
    )
  })

  it('shows loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('Loading book…')).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toBeInTheDocument()
  })

  it('shows error state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
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

  it('renders the title without an author line when the book has no authors', () => {
    const noAuthorBook = create(UserBookSchema, {
      ...mockUserBook,
      book: create(BookSchema, { ...mockBook, authors: [] })
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([noAuthorBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByRole('heading', { name: 'Dune' })).toBeInTheDocument()
    expect(screen.queryByText('Frank Herbert')).not.toBeInTheDocument()
  })

  it('joins several authors with commas', () => {
    const two = create(UserBookSchema, {
      ...mockUserBook,
      book: create(BookSchema, { ...mockBook, authors: ['Ann', 'Bob'] })
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([two]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('Ann, Bob')).toBeInTheDocument()
  })

  it('leaves out the page count when it is unknown', () => {
    const noPages = create(UserBookSchema, {
      ...mockUserBook,
      book: create(BookSchema, { ...mockBook, pageCount: 0 })
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([noPages]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByText(/pages$/)).not.toBeInTheDocument()
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
    vi.mocked(useLibrary).mockReturnValue({
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
    vi.mocked(useLibrary).mockReturnValue({
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
    vi.mocked(useLibrary).mockReturnValue({
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
    expect(booksLink).toHaveAttribute('href', '/dashboard/reading')
    const libraryLink = screen.getByText('Library').closest('a')
    expect(libraryLink).toHaveAttribute('href', '/books/library')
  })

  it('carries the search query into the Library breadcrumb link', () => {
    mockUseSearchParams.mockReturnValueOnce(new URLSearchParams('q=dune'))
    render(<BookDetailClient id="ub-1" />)
    const libraryLink = screen.getByText('Library').closest('a')
    expect(libraryLink).toHaveAttribute('href', '/books/library?q=dune')
  })

  it('renders shelf/tag fields', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByTestId('shelf-tag-fields')).toBeInTheDocument()
  })

  it('renders favourite button for a read book', () => {
    const readBook = create(UserBookSchema, { ...mockUserBook, status: 'read' })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([], [], [readBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByTestId('favourite-button')).toBeInTheDocument()
  })

  it('shows the device suggestion once the book is analysed', () => {
    const analysed = create(UserBookSchema, {
      ...mockUserBook,
      readSuggestion: create(ReadSuggestionSchema, { format: 'epub', figureCount: 2, pages: 200 })
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([analysed]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('Best on phone/Kobo (EPUB)')).toBeInTheDocument()
    expect(screen.getByText('· 2 figures in ~200 pages', { exact: false })).toBeInTheDocument()
  })

  it('hides the device suggestion before the book is analysed', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByText(/Best on/)).not.toBeInTheDocument()
  })

  it('hides Kobo sync toggle when book has no epub/pdf format', () => {
    const noFormatBook = create(UserBookSchema, { ...mockUserBook, formats: [] })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([noFormatBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByTestId('kobo-sync-toggle')).not.toBeInTheDocument()
  })

  it('shows Kobo sync toggle when book has an epub format, even without the own-digital tag', () => {
    // Kobo sync depends only on formats, not the own-digital tag.
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByTestId('kobo-sync-toggle')).toBeInTheDocument()
  })

  it('shows Kobo sync toggle when book has a pdf format', () => {
    const pdfBook = create(UserBookSchema, { ...mockUserBook, formats: ['pdf'] })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([pdfBook]),
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
    vi.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([], [], [], [shelf]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-shelf" />)
    expect(screen.getByRole('heading', { name: 'Dune' })).toBeInTheDocument()
  })

  function setFormats(formats: string[]) {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([create(UserBookSchema, { ...mockUserBook, formats })]),
      isLoading: false,
      error: undefined
    })
  }

  it('links an EPUB book to the reader instead of a preview', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByRole('link', { name: 'Open reader' })).toHaveAttribute(
      'href',
      '/books/ub-1/read'
    )
    expect(screen.queryByRole('link', { name: 'Open KEPUB' })).not.toBeInTheDocument()
  })

  it('shows whether the book is available offline next to the reader links', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText(`offline:${mockUserBook.bookId}`)).toBeInTheDocument()
  })

  it('links a book with both files to each format', () => {
    setFormats(['epub', 'pdf'])
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByRole('link', { name: 'Open EPUB' })).toHaveAttribute(
      'href',
      '/books/ub-1/read?format=epub'
    )
    expect(screen.getByRole('link', { name: 'Open PDF' })).toHaveAttribute(
      'href',
      '/books/ub-1/read?format=pdf'
    )
    expect(screen.queryByRole('button', { name: 'Preview PDF' })).not.toBeInTheDocument()
  })

  it('opens the converted KEPUB of a PDF-only book in the reader', () => {
    setFormats(['pdf'])
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByRole('link', { name: 'Open reader' })).toHaveAttribute(
      'href',
      '/books/ub-1/read'
    )
    expect(screen.getByRole('link', { name: 'Open KEPUB' })).toHaveAttribute(
      'href',
      '/books/ub-1/read?format=kepub'
    )
    expect(screen.queryByRole('button', { name: 'Preview EPUB' })).not.toBeInTheDocument()
  })

  it('does not show preview buttons when no formats', () => {
    const noFormatBook = create(UserBookSchema, { ...mockUserBook, formats: [] })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([noFormatBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByRole('button', { name: 'Preview EPUB' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Preview PDF' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Open reader' })).not.toBeInTheDocument()
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
    vi.mocked(useLibrary).mockReturnValue({
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
    vi.mocked(useCurrentUser).mockReturnValue({ data: { role: 'admin' }, isLoading: false })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByTestId('book-source-sync')).toBeInTheDocument()
  })

  it('opens the edit dialog when Edit book is clicked for an admin', () => {
    // @ts-expect-error -- partial mock
    vi.mocked(useCurrentUser).mockReturnValue({ data: { role: 'admin' }, isLoading: false })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByTestId('book-edit-dialog')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Edit book' }))
    expect(screen.getByTestId('book-edit-dialog')).toBeInTheDocument()
  })

  it('shows one breadcrumb, the authors and a destructive remove entry', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getAllByRole('navigation', { name: /breadcrumb/i })).toHaveLength(1)
    expect(screen.getByText('Frank Herbert')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'More actions' }))
    expect(screen.getByRole('menuitem', { name: 'Remove from library' })).toHaveClass('text-danger')
  })

  it('opens the remove dialog when Remove from library is clicked', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByTestId('remove-book-dialog')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'More actions' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Remove from library' }))
    expect(screen.getByTestId('remove-book-dialog')).toBeInTheDocument()
  })

  it('redirects to the library after a book is removed', () => {
    mockRouterPush.mockClear()
    render(<BookDetailClient id="ub-1" />)
    fireEvent.click(screen.getByRole('button', { name: 'More actions' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Remove from library' }))
    fireEvent.click(screen.getByText('Confirm remove'))
    expect(mockRouterPush).toHaveBeenCalledWith('/books/library')
  })

  it('opens the in-app article reader when Read in app is clicked', () => {
    const rssBook = create(UserBookSchema, {
      ...mockUserBook,
      book: create(BookSchema, { ...mockBook, sourceUrl: 'https://example.com/article' })
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData([rssBook]),
      isLoading: false,
      error: undefined
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseGetBookContent.mockReturnValue({ data: undefined, error: undefined })
    render(<BookDetailClient id="ub-1" />)

    expect(mockUseGetBookContent).toHaveBeenCalledWith(null)
    fireEvent.click(screen.getByRole('button', { name: 'Read in app' }))
    expect(mockUseGetBookContent).toHaveBeenCalledWith('book-1')
  })
})
