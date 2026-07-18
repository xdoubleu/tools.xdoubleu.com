import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

jest.mock('@/hooks/useBooks', () => ({
  useLibrary: jest.fn(),
  useAddBookByURL: () => jest.fn()
}))

// The query lives in ?q= (see BooksSection), not component state, so the
// mock tracks it as a module-level URLSearchParams that router.replace
// mutates — a re-render then picks up the new value, mirroring how Next's
// real useSearchParams reflects a navigation.
let currentSearchParams = new URLSearchParams()
const mockReplace = jest.fn((url: string) => {
  const qIndex = url.indexOf('?')
  currentSearchParams = new URLSearchParams(qIndex >= 0 ? url.slice(qIndex + 1) : '')
})

jest.mock('next/navigation', () => ({
  useRouter: () => ({ replace: mockReplace }),
  useSearchParams: () => currentSearchParams
}))

jest.mock('@/components/reading/BookSearchBar', () => {
  return function MockBookSearchBar({
    query,
    onChange,
    onAdded
  }: {
    query: string
    onChange: (v: string) => void
    onAdded: () => void
  }) {
    return (
      <div>
        <input
          data-testid="book-search-bar"
          value={query}
          onChange={(e) => onChange(e.target.value)}
          placeholder="Search books…"
        />
        <button data-testid="trigger-added" onClick={onAdded}>
          added
        </button>
      </div>
    )
  }
})

jest.mock('@/components/reading/BooksLibrary', () => {
  return function MockBooksLibrary({
    library,
    searchQuery,
    onSaved
  }: {
    library: { reading: unknown[] }
    searchQuery: string
    onSaved: () => void
  }) {
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

import BooksSection from '@/components/reading/BooksSection'
import { useLibrary } from '@/hooks/useBooks'
import { mutate } from 'swr'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  LibraryResponseSchema,
  GetLibraryResponseSchema
} from '@/lib/gen/reading/v1/library_pb'

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
    currentSearchParams = new URLSearchParams()
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
    expect(screen.getByText('Loading books…')).toBeInTheDocument()
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
    expect(mutate).toHaveBeenCalledWith('/reading')
  })

  it('calls mutate when BooksLibrary triggers onSaved', () => {
    mockLibrary()
    render(<BooksSection />)
    fireEvent.click(screen.getByTestId('trigger-saved'))
    expect(mutate).toHaveBeenCalledWith('/reading')
  })

  it('passes searchQuery down to BooksLibrary as user types', () => {
    mockLibrary()
    const { rerender } = render(<BooksSection />)
    fireEvent.change(screen.getByPlaceholderText('Search books…'), {
      target: { value: 'dune' }
    })
    // The query lives in the URL, not component state — router.replace updated
    // it, so a re-render is needed to observe it (mirrors real navigation).
    rerender(<BooksSection />)
    expect(screen.getByTestId('books-library')).toHaveAttribute('data-search-query', 'dune')
  })

  it('writes the query into the URL via router.replace', () => {
    mockLibrary()
    render(<BooksSection />)
    fireEvent.change(screen.getByPlaceholderText('Search books…'), {
      target: { value: 'dune' }
    })
    expect(mockReplace).toHaveBeenCalledWith('/reading/library?q=dune', { scroll: false })
  })

  it('restores the query from the URL on mount (back-navigation)', () => {
    currentSearchParams = new URLSearchParams('q=dune')
    mockLibrary()
    render(<BooksSection />)
    expect(screen.getByTestId('books-library')).toHaveAttribute('data-search-query', 'dune')
    expect(screen.getByDisplayValue('dune')).toBeInTheDocument()
  })
})
