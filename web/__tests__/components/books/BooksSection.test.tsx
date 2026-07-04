import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

jest.mock('@/hooks/useBooks', () => ({
  useLibrary: jest.fn()
}))

jest.mock('@/components/books/BookSearchBar', () => {
  return function MockBookSearchBar({
    query,
    onChange,
    onAdded
  }: {
    query: string
    onChange: (v: string) => void
    onAdded: () => void
    hasLibraryResults: boolean
  }) {
    return (
      <div>
        <input
          data-testid="book-search-bar"
          value={query}
          onChange={(e) => onChange(e.target.value)}
          placeholder="Search books..."
        />
        <button data-testid="trigger-added" onClick={onAdded}>
          added
        </button>
      </div>
    )
  }
})

jest.mock('@/components/books/BooksLibrary', () => {
  return function MockBooksLibrary({
    library,
    searchQuery,
    onSearchResultsChange,
    onSaved
  }: {
    library: { reading: unknown[] }
    searchQuery: string
    onSearchResultsChange: (v: boolean) => void
    onSaved: () => void
  }) {
    // Simulate: no results when query is "notfound"
    React.useEffect(() => {
      onSearchResultsChange(searchQuery !== 'notfound')
    }, [searchQuery, onSearchResultsChange])

    return (
      <div
        data-testid="books-library"
        data-reading-count={library.reading.length}
        data-search-query={searchQuery}
      >
        <button data-testid="trigger-saved" onClick={onSaved}>
          save
        </button>
      </div>
    )
  }
})

jest.mock('swr', () => ({ mutate: jest.fn() }))

import BooksSection from '@/components/books/BooksSection'
import { useLibrary } from '@/hooks/useBooks'
import { mutate } from 'swr'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  LibraryResponseSchema,
  GetLibraryResponseSchema
} from '@/lib/gen/books/v1/library_pb'

const mockUseBacklogLibrary = jest.mocked(useLibrary)

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
    fireEvent.click(screen.getByTestId('trigger-added'))
    expect(mutate).toHaveBeenCalledWith('/books')
  })

  it('calls mutate when BooksLibrary triggers onSaved', () => {
    mockLibrary()
    render(<BooksSection />)
    fireEvent.click(screen.getByTestId('trigger-saved'))
    expect(mutate).toHaveBeenCalledWith('/books')
  })

  it('passes searchQuery down to BooksLibrary as user types', () => {
    mockLibrary()
    render(<BooksSection />)
    fireEvent.change(screen.getByPlaceholderText('Search books...'), {
      target: { value: 'dune' }
    })
    expect(screen.getByTestId('books-library')).toHaveAttribute('data-search-query', 'dune')
  })
})
