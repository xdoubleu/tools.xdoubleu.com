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

jest.mock('@/components/backlog/BookSearchBar', () => {
  return function MockBookSearchBar({ onAdded }: { onAdded: () => void }) {
    return (
      <button data-testid="book-search-bar" onClick={onAdded}>
        search
      </button>
    )
  }
})

jest.mock('@/components/backlog/BookEditModal', () => {
  return function MockBookEditModal() {
    return <div data-testid="book-edit-modal" />
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
  status: 'finished',
  formats: [],
  book: create(BookSchema, { title: 'Foundation', authors: ['Isaac Asimov'] })
})

const wishlistBook = create(UserBookSchema, {
  id: '3',
  status: 'wishlist',
  formats: [],
  book: create(BookSchema, { title: 'Hyperion', authors: ['Dan Simmons'] })
})

const shelfBook = create(UserBookSchema, {
  id: '4',
  status: 'reading',
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
    // The first non-empty shelf is "Currently Reading"; Dune should be visible
    expect(screen.getByText('Dune')).toBeInTheDocument()
    // Other shelves' books are not rendered until selected
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

  it('opens the edit modal when a book Edit button is clicked', () => {
    mockLibrary()
    render(<BooksSection />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Edit' })[0])
    expect(screen.getByTestId('book-edit-modal')).toBeInTheDocument()
  })

  it('gives book cards a white card background', () => {
    mockLibrary()
    render(<BooksSection />)
    expect(screen.getByText('Dune').closest('.bg-card')).toBeInTheDocument()
  })

  it('shows a progress bar for currently reading books', () => {
    mockLibrary()
    render(<BooksSection />)
    expect(screen.getAllByRole('progressbar').length).toBeGreaterThan(0)
  })
})
