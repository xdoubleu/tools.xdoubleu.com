import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

jest.mock('@/hooks/useBooks', () => ({
  useLibrary: jest.fn(),
  useBooksProgress: jest.fn()
}))

jest.mock('next/image', () => {
  return function MockImage({ src, alt, ...props }: { src: string; alt: string }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} {...props} />
  }
})

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

jest.mock('@/components/reading/BookSearchBar', () => {
  return function MockBookSearchBar() {
    return <div data-testid="book-search-bar" />
  }
})

jest.mock('@/components/reading/BooksProgressChart', () => {
  return function MockBooksProgressChart() {
    return <div data-testid="books-progress-chart" />
  }
})

jest.mock('@/components/profile/ProfileShareButton', () => {
  return function MockProfileShareButton() {
    return <div data-testid="profile-share-button" />
  }
})

jest.mock('@/components/reading/SubscribedFeedsCard', () => {
  return function MockSubscribedFeedsCard() {
    return <div data-testid="subscribed-feeds-card" />
  }
})

jest.mock('@/components/reading/AddToLibraryDialog', () => {
  return function MockAddToLibraryDialog() {
    return <div data-testid="add-to-library-dialog" />
  }
})

jest.mock('swr', () => ({ mutate: jest.fn() }))

import BooksDashboard from '@/components/reading/BooksDashboard'
import { useLibrary, useBooksProgress } from '@/hooks/useBooks'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  LibraryResponseSchema,
  GetLibraryResponseSchema
} from '@/lib/gen/reading/v1/library_pb'

const mockUseBacklogLibrary = jest.mocked(useLibrary)
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
    expect(screen.getAllByText('Currently reading').length).toBeGreaterThan(0)
    expect(screen.getByText('Read')).toBeInTheDocument()
    expect(screen.getByText('Read this year')).toBeInTheDocument()
    expect(screen.getByText('Want to read')).toBeInTheDocument()
  })

  it('renders currently reading cards with a progress bar', () => {
    mockLibrary()
    render(<BooksDashboard />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.getByText('100 / 400 pages')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '25')
  })

  it('links reading cards to the book detail page', () => {
    mockLibrary()
    render(<BooksDashboard />)
    const link = screen.getByText('Dune').closest('a')
    expect(link).toHaveAttribute('href', '/reading/1')
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
      '/reading/library'
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
    fireEvent.change(from, { target: { value: '01/01/2026' } })
    expect(from).toHaveValue('01/01/2026')
  })

  it('shows a loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBooksProgress.mockReturnValue({ data: undefined })
    render(<BooksDashboard />)
    expect(screen.getByText('Loading dashboard…')).toBeInTheDocument()
  })

  it('shows RSS stat cards when the library has rss items', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue({
      data: create(GetLibraryResponseSchema, {
        library: create(LibraryResponseSchema, {
          rss: [
            create(UserBookSchema, { id: 'r1', status: 'read' }),
            create(UserBookSchema, { id: 'r2', status: 'to-read' })
          ]
        })
      }),
      error: undefined,
      isLoading: false
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBooksProgress.mockReturnValue({ data: undefined })
    render(<BooksDashboard />)
    expect(screen.getByText('RSS items')).toBeInTheDocument()
    expect(screen.getByText('RSS read')).toBeInTheDocument()
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
