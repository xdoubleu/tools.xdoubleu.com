import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

jest.mock('@/hooks/useBacklog', () => ({
  useBacklogLibrary: jest.fn(),
  useBooksProgress: jest.fn()
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

jest.mock('@/components/backlog/BooksProgressChart', () => {
  return function MockBooksProgressChart() {
    return <div data-testid="books-progress-chart" />
  }
})

jest.mock('@/components/backlog/BookEditModal', () => {
  return function MockBookEditModal() {
    return <div data-testid="book-edit-modal" />
  }
})

import BooksSection from '@/components/backlog/BooksSection'
import { useBacklogLibrary, useBooksProgress } from '@/hooks/useBacklog'
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
const mockUseBooksProgress = jest.mocked(useBooksProgress)

const readingBook = create(UserBookSchema, {
  id: '1',
  status: 'reading',
  rating: 4,
  tags: ['favourite'],
  book: create(BookSchema, {
    title: 'Dune',
    authors: ['Frank Herbert'],
    coverUrl: 'http://example.com/dune.png'
  })
})

const finishedBook = create(UserBookSchema, {
  id: '2',
  status: 'finished',
  book: create(BookSchema, { title: 'Foundation', authors: ['Isaac Asimov'] })
})

const wishlistBook = create(UserBookSchema, {
  id: '3',
  status: 'wishlist',
  book: create(BookSchema, { title: 'Hyperion', authors: ['Dan Simmons'] })
})

const shelfBook = create(UserBookSchema, {
  id: '4',
  status: 'reading',
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
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseBooksProgress.mockReturnValue({ data: undefined })
}

describe('BooksSection', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('renders the library by default', () => {
    mockLibrary()
    render(<BooksSection />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.getByText('Foundation')).toBeInTheDocument()
    expect(screen.getByText('Hyperion')).toBeInTheDocument()
    expect(screen.getByText('Neuromancer')).toBeInTheDocument()
    expect(screen.getByText('Currently Reading (1)')).toBeInTheDocument()
    expect(screen.getByText('Finished (1)')).toBeInTheDocument()
    expect(screen.getByText('Wishlist (1)')).toBeInTheDocument()
    expect(screen.getByText('Sci-Fi (1)')).toBeInTheDocument()
  })

  it('shows a loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBooksProgress.mockReturnValue({ data: undefined })
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
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBooksProgress.mockReturnValue({ data: undefined })
    render(<BooksSection />)
    expect(screen.getByText('Failed to load books.')).toBeInTheDocument()
  })

  it('shows the search bar always and refreshes the library on add', () => {
    mockLibrary()
    render(<BooksSection />)
    expect(screen.queryByRole('button', { name: 'Search' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByTestId('book-search-bar'))
    expect(mutate).toHaveBeenCalledWith('/backlog/books')
  })

  it('switches to the progress tab and updates the date range', () => {
    mockLibrary()
    render(<BooksSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Progress' }))
    expect(screen.getByTestId('books-progress-chart')).toBeInTheDocument()

    const from = screen.getByLabelText('From')
    const to = screen.getByLabelText('To')
    fireEvent.change(from, { target: { value: '2026-01-01' } })
    fireEvent.change(to, { target: { value: '2026-02-01' } })
    expect(from).toHaveValue('2026-01-01')
    expect(to).toHaveValue('2026-02-01')
  })

  it('opens the edit modal when a book Edit button is clicked', () => {
    mockLibrary()
    render(<BooksSection />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Edit' })[0])
    expect(screen.getByTestId('book-edit-modal')).toBeInTheDocument()
  })
})
