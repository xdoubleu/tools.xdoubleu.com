import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  BookSchema,
  GetLibraryResponseSchema,
  LibraryResponseSchema,
  UserBookSchema
} from '@/lib/gen/books/v1/library_pb'

const mockUseLibrary = jest.fn()
const mockUseGetBookFile = jest.fn()
const mockRouterPush = jest.fn()
let mockSearchParams = new URLSearchParams()

jest.mock('@/hooks/useBooks', () => ({
  useLibrary: () => mockUseLibrary(),
  useGetBookFile: (...args: unknown[]) => mockUseGetBookFile(...args)
}))

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockRouterPush }),
  useSearchParams: () => mockSearchParams
}))

jest.mock('@/components/books/reader/BookReader', () => ({
  __esModule: true,
  default: ({ url, title, onClose }: { url: string; title: string; onClose: () => void }) => (
    <div data-testid="reader" data-url={url} data-title={title}>
      <button type="button" onClick={onClose}>
        Close reader
      </button>
    </div>
  )
}))

import ReadBookClient from '@/app/books/[id]/read/ReadBookClient'

function setLibrary(formats: string[]) {
  const userBook = create(UserBookSchema, {
    id: 'ub-1',
    bookId: 'book-1',
    book: create(BookSchema, { id: 'book-1', title: 'Dune' }),
    status: 'currently-reading',
    formats
  })
  mockUseLibrary.mockReturnValue({
    data: create(GetLibraryResponseSchema, {
      library: create(LibraryResponseSchema, { reading: [userBook] })
    }),
    error: undefined,
    isLoading: false
  })
}

beforeEach(() => {
  jest.clearAllMocks()
  mockSearchParams = new URLSearchParams()
  setLibrary(['epub', 'pdf'])
  mockUseGetBookFile.mockReturnValue({ data: { url: 'https://r2/book' }, error: undefined })
})

describe('ReadBookClient', () => {
  it('opens the EPUB by default', async () => {
    render(<ReadBookClient id="ub-1" />)
    expect(mockUseGetBookFile).toHaveBeenCalledWith('book-1', 'epub')
    const reader = await screen.findByTestId('reader')
    expect(reader).toHaveAttribute('data-url', 'https://r2/book')
    expect(reader).toHaveAttribute('data-title', 'Dune')
  })

  it('opens the requested format', () => {
    mockSearchParams = new URLSearchParams('format=pdf')
    render(<ReadBookClient id="ub-1" />)
    expect(mockUseGetBookFile).toHaveBeenCalledWith('book-1', 'pdf')
  })

  it('opens the PDF of a PDF-only book', () => {
    setLibrary(['pdf', 'kepub'])
    render(<ReadBookClient id="ub-1" />)
    expect(mockUseGetBookFile).toHaveBeenCalledWith('book-1', 'pdf')
  })

  it('explains when the book has no readable file', () => {
    setLibrary([])
    render(<ReadBookClient id="ub-1" />)
    expect(mockUseGetBookFile).toHaveBeenCalledWith(null, null)
    expect(screen.getByText('This book has no EPUB or PDF file.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Back to book' })).toHaveAttribute(
      'href',
      '/books/ub-1'
    )
  })

  it('shows an error when the signed URL fails', () => {
    mockUseGetBookFile.mockReturnValue({ data: undefined, error: new Error('boom') })
    render(<ReadBookClient id="ub-1" />)
    expect(screen.getByText('Failed to load book file.')).toBeInTheDocument()
    expect(screen.queryByTestId('reader')).not.toBeInTheDocument()
  })

  it('shows loading while the URL is fetched', () => {
    mockUseGetBookFile.mockReturnValue({ data: undefined, error: undefined })
    render(<ReadBookClient id="ub-1" />)
    expect(screen.getByText('Loading book…')).toBeInTheDocument()
  })

  it('shows loading while the library loads', () => {
    mockUseLibrary.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    render(<ReadBookClient id="ub-1" />)
    expect(screen.getByText('Loading book…')).toBeInTheDocument()
  })

  it('shows an error when the library fails', () => {
    mockUseLibrary.mockReturnValue({ data: undefined, error: new Error('x'), isLoading: false })
    render(<ReadBookClient id="ub-1" />)
    expect(screen.getByText('Failed to load book.')).toBeInTheDocument()
  })

  it('reports an unknown book', () => {
    render(<ReadBookClient id="missing" />)
    expect(screen.getByText('Book not found.')).toBeInTheDocument()
  })

  it('waits for a fresh URL instead of opening a cached one', () => {
    mockUseGetBookFile.mockReturnValue({
      data: { url: 'https://r2/stale' },
      error: undefined,
      isValidating: true
    })
    render(<ReadBookClient id="ub-1" />)
    expect(screen.getByText('Loading book…')).toBeInTheDocument()
    expect(screen.queryByTestId('reader')).not.toBeInTheDocument()
  })

  it('keeps the first URL and the open reader across refetches and their errors', async () => {
    const { rerender } = render(<ReadBookClient id="ub-1" />)
    expect(await screen.findByTestId('reader')).toHaveAttribute('data-url', 'https://r2/book')
    mockUseGetBookFile.mockReturnValue({
      data: { url: 'https://r2/new' },
      error: new Error('offline'),
      isValidating: false
    })
    mockUseLibrary.mockReturnValue({
      ...mockUseLibrary(),
      error: new Error('offline')
    })
    rerender(<ReadBookClient id="ub-1" />)
    expect(screen.getByTestId('reader')).toHaveAttribute('data-url', 'https://r2/book')
  })

  it('returns to the book page on close', async () => {
    render(<ReadBookClient id="ub-1" />)
    fireEvent.click(await screen.findByRole('button', { name: 'Close reader' }))
    expect(mockRouterPush).toHaveBeenCalledWith('/books/ub-1')
  })
})
