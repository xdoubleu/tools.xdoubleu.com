import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

jest.mock('@/hooks/useBacklog', () => ({
  useBacklogLibrary: jest.fn()
}))

jest.mock('@/components/backlog/BookSearchBar', () => {
  return function MockBookSearchBar({ onAdded }: { onAdded: () => void }) {
    return (
      <button data-testid="book-search-bar" onClick={onAdded}>
        search
      </button>
    )
  }
})

jest.mock('@/components/backlog/BooksLibrary', () => {
  return function MockBooksLibrary({
    library,
    onSaved
  }: {
    library: { reading: unknown[] }
    onSaved: () => void
  }) {
    return (
      <div data-testid="books-library" data-reading-count={library.reading.length}>
        <button data-testid="trigger-saved" onClick={onSaved}>
          save
        </button>
      </div>
    )
  }
})

jest.mock('swr', () => ({ mutate: jest.fn() }))

import BooksSection from '@/components/backlog/BooksSection'
import { useBacklogLibrary } from '@/hooks/useBacklog'
import { mutate } from 'swr'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  LibraryResponseSchema,
  GetLibraryResponseSchema
} from '@/lib/gen/backlog/v1/books_pb'

const mockUseBacklogLibrary = jest.mocked(useBacklogLibrary)

function mockLibrary() {
  const readingBook = create(UserBookSchema, {
    id: '1',
    status: 'currently-reading',
    tags: [],
    formats: [],
    book: create(BookSchema, { title: 'Dune', authors: ['Frank Herbert'] })
  })
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseBacklogLibrary.mockReturnValue({
    data: create(GetLibraryResponseSchema, {
      library: create(LibraryResponseSchema, {
        reading: [readingBook],
        finished: [],
        wishlist: [],
        shelves: []
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

  it('renders the search bar', () => {
    mockLibrary()
    render(<BooksSection />)
    expect(screen.getByTestId('book-search-bar')).toBeInTheDocument()
  })

  it('passes library data to BooksLibrary', () => {
    mockLibrary()
    render(<BooksSection />)
    const lib = screen.getByTestId('books-library')
    expect(lib).toBeInTheDocument()
    expect(lib).toHaveAttribute('data-reading-count', '1')
  })

  it('shows a loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    render(<BooksSection />)
    expect(screen.getByText('Loading books...')).toBeInTheDocument()
    expect(screen.queryByTestId('books-library')).not.toBeInTheDocument()
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
    expect(screen.queryByTestId('books-library')).not.toBeInTheDocument()
  })

  it('calls mutate when search bar triggers onAdded', () => {
    mockLibrary()
    render(<BooksSection />)
    fireEvent.click(screen.getByTestId('book-search-bar'))
    expect(mutate).toHaveBeenCalledWith('/backlog/books')
  })

  it('calls mutate when BooksLibrary triggers onSaved', () => {
    mockLibrary()
    render(<BooksSection />)
    fireEvent.click(screen.getByTestId('trigger-saved'))
    expect(mutate).toHaveBeenCalledWith('/backlog/books')
  })
})
