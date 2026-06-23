import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  LibraryResponseSchema,
  GetLibraryResponseSchema,
  BookShelfSchema
} from '@/lib/gen/backlog/v1/books_pb'

jest.mock('@/hooks/useBacklog', () => ({
  useBacklogLibrary: jest.fn()
}))

jest.mock('next/navigation', () => ({
  useRouter: jest.fn(() => ({ push: jest.fn() }))
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

jest.mock('@/components/backlog/BookProgressBar', () => {
  return function MockProgressBar() {
    return <div role="progressbar" />
  }
})

jest.mock('@/components/backlog/BookEditModal', () => {
  return function MockBookEditModal({ onClose }: { onClose: () => void }) {
    return (
      <div role="dialog" aria-label="Edit Book">
        <button onClick={onClose}>Close</button>
      </div>
    )
  }
})

jest.mock('swr', () => ({
  mutate: jest.fn()
}))

import BookDetailClient from '@/app/backlog/books/[id]/BookDetailClient'
import { useBacklogLibrary } from '@/hooks/useBacklog'

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
  notes: 'Great read so far.',
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
  // Default: loaded with mockUserBook
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  jest.mocked(useBacklogLibrary).mockReturnValue({
    data: makeLibraryData(),
    isLoading: false,
    error: undefined
  })
})

describe('BookDetailClient', () => {
  it('shows loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogLibrary).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('Loading book...')).toBeInTheDocument()
  })

  it('shows error state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogLibrary).mockReturnValue({
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
    jest.mocked(useBacklogLibrary).mockReturnValue({
      data: makeLibraryData([noDescBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('No description available.')).toBeInTheDocument()
  })

  it('renders star rating', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByLabelText('4 out of 5 stars')).toBeInTheDocument()
  })

  it('renders page count', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('412 pages')).toBeInTheDocument()
  })

  it('renders ISBN', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('ISBN: 9780441013593')).toBeInTheDocument()
  })

  it('renders progress bar for currently-reading', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByRole('progressbar')).toBeInTheDocument()
  })

  it('does not render progress bar for non-reading status', () => {
    const wishlistBook = create(UserBookSchema, { ...mockUserBook, status: 'wishlist' })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogLibrary).mockReturnValue({
      data: makeLibraryData([], [wishlistBook]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-1" />)
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })

  it('renders notes', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('Great read so far.')).toBeInTheDocument()
  })

  it('renders user tags (excluding system tags)', () => {
    render(<BookDetailClient id="ub-1" />)
    expect(screen.getByText('sci-fi')).toBeInTheDocument()
    // favourite is shown as a badge, not in the tags list
    expect(screen.getByText('Favourite')).toBeInTheDocument()
  })

  it('renders breadcrumb link back to Books', () => {
    render(<BookDetailClient id="ub-1" />)
    const booksLink = screen.getByText('Books').closest('a')
    expect(booksLink).toHaveAttribute('href', '/backlog/books')
  })

  it('opens BookEditModal when Edit is clicked', () => {
    render(<BookDetailClient id="ub-1" />)
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
    expect(screen.getByRole('dialog', { name: 'Edit Book' })).toBeInTheDocument()
  })

  it('closes BookEditModal when closed', () => {
    render(<BookDetailClient id="ub-1" />)
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(screen.queryByRole('dialog', { name: 'Edit Book' })).not.toBeInTheDocument()
  })

  it('finds a book in shelves', () => {
    const shelfBook = create(UserBookSchema, {
      ...mockUserBook,
      id: 'ub-shelf',
      status: 'wishlist'
    })
    const shelf = create(BookShelfSchema, { name: 'To Buy', books: [shelfBook] })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogLibrary).mockReturnValue({
      data: makeLibraryData([], [], [], [shelf]),
      isLoading: false,
      error: undefined
    })
    render(<BookDetailClient id="ub-shelf" />)
    expect(screen.getByRole('heading', { name: 'Dune' })).toBeInTheDocument()
  })
})
