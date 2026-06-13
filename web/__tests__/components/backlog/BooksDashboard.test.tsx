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

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/backlog/BookSearchBar', () => {
  return function MockBookSearchBar() {
    return <div data-testid="book-search-bar" />
  }
})

jest.mock('@/components/backlog/BooksProgressChart', () => {
  return function MockBooksProgressChart() {
    return <div data-testid="books-progress-chart" />
  }
})

jest.mock('@/components/backlog/BookEditModal', () => {
  return function MockBookEditModal({ onClose }: { onClose: () => void }) {
    return (
      <button data-testid="book-edit-modal" onClick={onClose}>
        modal
      </button>
    )
  }
})

jest.mock('swr', () => ({ mutate: jest.fn() }))

import BooksDashboard from '@/components/backlog/BooksDashboard'
import { useBacklogLibrary, useBooksProgress } from '@/hooks/useBacklog'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  LibraryResponseSchema,
  GetLibraryResponseSchema
} from '@/lib/gen/backlog/v1/books_pb'

const mockUseBacklogLibrary = jest.mocked(useBacklogLibrary)
const mockUseBooksProgress = jest.mocked(useBooksProgress)

const readingBook = create(UserBookSchema, {
  id: '1',
  status: 'currently-reading',
  progressMode: 'pages',
  currentPage: 100,
  book: create(BookSchema, {
    title: 'Dune',
    authors: ['Frank Herbert'],
    coverUrl: 'http://example.com/dune.png',
    pageCount: 400
  })
})

function mockLibrary(reading = [readingBook]) {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseBacklogLibrary.mockReturnValue({
    data: create(GetLibraryResponseSchema, {
      library: create(LibraryResponseSchema, {
        reading,
        finished: [create(UserBookSchema, { id: '2', status: 'read' })],
        wishlist: [create(UserBookSchema, { id: '3', status: 'to-read' })]
      })
    }),
    error: undefined,
    isLoading: false
  })
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseBooksProgress.mockReturnValue({ data: undefined })
}

describe('BooksDashboard', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('renders the stat cards derived from the library', () => {
    mockLibrary()
    render(<BooksDashboard />)
    expect(screen.getByText('Total books')).toBeInTheDocument()
    expect(screen.getByText('In progress')).toBeInTheDocument()
    expect(screen.getByText('Finished')).toBeInTheDocument()
    expect(screen.getByText('Read this year')).toBeInTheDocument()
    expect(screen.getByText('Wishlist')).toBeInTheDocument()
  })

  it('renders currently reading cards with a progress bar', () => {
    mockLibrary()
    render(<BooksDashboard />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.getByText('100 / 400 pages')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '25')
  })

  it('opens the edit modal when a reading card is clicked', () => {
    mockLibrary()
    render(<BooksDashboard />)
    fireEvent.click(screen.getByText('Dune'))
    expect(screen.getByTestId('book-edit-modal')).toBeInTheDocument()
  })

  it('shows an empty message when nothing is in progress', () => {
    mockLibrary([])
    render(<BooksDashboard />)
    expect(screen.getByText('No books in progress.')).toBeInTheDocument()
  })

  it('links to the full library', () => {
    mockLibrary()
    render(<BooksDashboard />)
    expect(screen.getByText('Browse full library').closest('a')).toHaveAttribute(
      'href',
      '/backlog/books/library'
    )
  })

  it('shows YTD chart by default and reveals date inputs when All time is selected', () => {
    mockLibrary()
    render(<BooksDashboard />)
    // From/To inputs are hidden in YTD view
    expect(screen.queryByLabelText('From')).not.toBeInTheDocument()
    // switch to All time
    fireEvent.click(screen.getByRole('tab', { name: 'All time' }))
    const from = screen.getByLabelText('From')
    fireEvent.change(from, { target: { value: '2026-01-01' } })
    expect(from).toHaveValue('2026-01-01')
  })

  it('shows a loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBooksProgress.mockReturnValue({ data: undefined })
    render(<BooksDashboard />)
    expect(screen.getByText('Loading dashboard...')).toBeInTheDocument()
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
    render(<BooksDashboard />)
    expect(screen.getByText('Failed to load books.')).toBeInTheDocument()
  })
})
