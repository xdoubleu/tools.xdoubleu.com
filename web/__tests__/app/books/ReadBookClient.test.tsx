import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  BookSchema,
  GetLibraryResponseSchema,
  LibraryResponseSchema,
  UserBookSchema
} from '@/lib/gen/books/v1/library_pb'

const mockUseLibrary = jest.fn()
const mockUseOfflineBookFile = jest.fn()
const mockUseReadingState = jest.fn()
const mockSave = jest.fn()
const mockIsWebPubWarm = jest.fn()
jest.mock('@/lib/books/webpubCache', () => ({
  isWebPubWarm: (...args: unknown[]) => mockIsWebPubWarm(...args)
}))
const mockRouterPush = jest.fn()
const mockRouterBack = jest.fn()
let mockSearchParams = new URLSearchParams()

jest.mock('@/hooks/useBooks', () => ({
  useLibrary: () => mockUseLibrary()
}))

jest.mock('@/hooks/useOfflineBooks', () => ({
  useOfflineBookFile: (...args: unknown[]) => mockUseOfflineBookFile(...args)
}))

// The real hook unless a test stubs it (e.g. the hydration pass).
jest.mock('@/hooks/useReadingState', () => ({
  useReadingState: (...args: unknown[]) => mockUseReadingState(...args),
  useReadingProgressSaver: () => mockSave
}))

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockRouterPush, back: mockRouterBack }),
  useSearchParams: () => mockSearchParams
}))

interface MockReaderProps {
  file: File
  bookId: string
  title: string
  onClose: () => void
  onRelocate: (location: unknown) => void
  initialPosition: unknown
}

jest.mock('@/components/books/reader/BookReader', () => ({
  __esModule: true,
  default: ({ file, title, onClose, onRelocate, initialPosition }: MockReaderProps) => (
    <div
      data-testid="reader"
      data-file={file.name}
      data-title={title}
      data-initial={JSON.stringify(initialPosition)}
    >
      <button type="button" onClick={onClose}>
        Close reader
      </button>
      <button type="button" onClick={() => onRelocate({ fraction: 0.5, section: 2 })}>
        Turn page
      </button>
      <button
        type="button"
        onClick={() =>
          onRelocate({
            fraction: 0.1234,
            section: 3,
            position: { href: 'OEBPS/ch3.xhtml', offset: 42 }
          })
        }
      >
        Read on
      </button>
      <button
        type="button"
        onClick={() => onRelocate({ fraction: 0.1234, section: 2, position: { page: 3 } })}
      >
        Read PDF page
      </button>
    </div>
  )
}))

jest.mock('@/components/books/reader/PdfBookReader', () => ({
  __esModule: true,
  default: ({ file, title, onClose, onRelocate, initialPosition }: MockReaderProps) => (
    <div
      data-testid="reader"
      data-file={file.name}
      data-title={title}
      data-initial={JSON.stringify(initialPosition)}
    >
      <button type="button" onClick={onClose}>
        Close reader
      </button>
      <button type="button" onClick={() => onRelocate({ fraction: 0.5, section: 2 })}>
        Turn page
      </button>
      <button
        type="button"
        onClick={() =>
          onRelocate({
            fraction: 0.1234,
            section: 3,
            position: { href: 'OEBPS/ch3.xhtml', offset: 42 }
          })
        }
      >
        Read on
      </button>
      <button
        type="button"
        onClick={() => onRelocate({ fraction: 0.1234, section: 2, position: { page: 3 } })}
      >
        Read PDF page
      </button>
    </div>
  )
}))

jest.mock('@/components/books/reader/PdfBookReader', () => ({
  __esModule: true,
  default: ({ file, title, onClose, onRelocate, initialPosition }: MockReaderProps) => (
    <div
      data-testid="reader"
      data-file={file.name}
      data-title={title}
      data-initial={JSON.stringify(initialPosition)}
    >
      <button type="button" onClick={onClose}>
        Close reader
      </button>
      <button type="button" onClick={() => onRelocate({ fraction: 0.5, section: 2 })}>
        Turn page
      </button>
      <button
        type="button"
        onClick={() =>
          onRelocate({
            fraction: 0.1234,
            section: 3,
            position: { href: 'OEBPS/ch3.xhtml', offset: 42 }
          })
        }
      >
        Read on
      </button>
      <button
        type="button"
        onClick={() => onRelocate({ fraction: 0.1234, section: 2, position: { page: 3 } })}
      >
        Read PDF page
      </button>
    </div>
  )
}))

jest.mock('@/components/books/reader/ReadiumBookReader', () => ({
  __esModule: true,
  default: ({ bookId, title, onClose, onRelocate, initialPosition }: MockReaderProps) => (
    <div
      data-testid="reader"
      data-file={`readium:${bookId}`}
      data-title={title}
      data-initial={JSON.stringify(initialPosition)}
    >
      <button type="button" onClick={onClose}>
        Close reader
      </button>
      <button type="button" onClick={() => onRelocate({ fraction: 0.5, section: 2 })}>
        Turn page
      </button>
      <button
        type="button"
        onClick={() =>
          onRelocate({
            fraction: 0.1234,
            section: 3,
            position: { href: 'OEBPS/ch3.xhtml', offset: 42 }
          })
        }
      >
        Read on
      </button>
      <button
        type="button"
        onClick={() => onRelocate({ fraction: 0.1234, section: 2, position: { page: 3 } })}
      >
        Read PDF page
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
    formats,
    fileVersions: { epub: 'v-epub', kepub: 'v-kepub' }
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
  localStorage.clear()
  mockSearchParams = new URLSearchParams()
  setLibrary(['epub', 'pdf'])
  mockUseOfflineBookFile.mockImplementation((bookId: string, format: string) => ({
    file: new File(['x'], `${bookId}.${format}`)
  }))
  setFreshReadingState(undefined)
})

/** The hook's cached data is stale; only `mutate()` (a fresh read) counts. */
function setFreshReadingState(state: unknown, stale: unknown = { percent: 99 }) {
  const mutate = jest.fn(() => Promise.resolve(state === undefined ? undefined : { state }))
  mockUseReadingState.mockReturnValue({ data: { state: stale }, error: undefined, mutate })
  return mutate
}

describe('ReadBookClient', () => {
  describe('opened offline', () => {
    let online: jest.SpyInstance
    beforeEach(() => {
      online = jest.spyOn(window.navigator, 'onLine', 'get').mockReturnValue(false)
    })
    afterEach(() => online.mockRestore())

    it('reads the offline copy in Readium', async () => {
      mockIsWebPubWarm.mockResolvedValue(true)
      render(<ReadBookClient id="ub-1" />)
      expect(await screen.findByTestId('reader')).toHaveAttribute('data-file', 'readium:book-1')
    })

    it('falls back to the downloaded file in foliate-js without an offline copy', async () => {
      mockIsWebPubWarm.mockResolvedValue(false)
      render(<ReadBookClient id="ub-1" />)
      expect(await screen.findByTestId('reader')).toHaveAttribute('data-file', 'book-1.epub')
    })

    it('falls back when the offline copy cannot be checked', async () => {
      mockIsWebPubWarm.mockRejectedValue(new Error('x'))
      render(<ReadBookClient id="ub-1" />)
      expect(await screen.findByTestId('reader')).toHaveAttribute('data-file', 'book-1.epub')
    })

    it('waits for the check', () => {
      mockIsWebPubWarm.mockReturnValue(new Promise(() => {}))
      render(<ReadBookClient id="ub-1" />)
      expect(screen.queryByTestId('reader')).not.toBeInTheDocument()
    })
  })

  it('opens the EPUB by default', async () => {
    render(<ReadBookClient id="ub-1" />)
    expect(mockUseOfflineBookFile).toHaveBeenCalledWith('book-1', 'epub', 'v-epub')
    const reader = await screen.findByTestId('reader')
    expect(reader).toHaveAttribute('data-file', 'readium:book-1')
    expect(reader).toHaveAttribute('data-title', 'Dune')
  })

  it('opens a PDF in the PDF reader', async () => {
    setLibrary(['pdf'])
    render(<ReadBookClient id="ub-1" />)
    expect(await screen.findByTestId('reader')).toHaveAttribute('data-file', 'book-1.pdf')
  })

  it('opens the requested format', () => {
    mockSearchParams = new URLSearchParams('format=pdf')
    render(<ReadBookClient id="ub-1" />)
    expect(mockUseOfflineBookFile).toHaveBeenCalledWith('book-1', 'pdf', '')
  })

  it('opens the PDF of a PDF-only book', () => {
    setLibrary(['pdf', 'kepub'])
    render(<ReadBookClient id="ub-1" />)
    expect(mockUseOfflineBookFile).toHaveBeenCalledWith('book-1', 'pdf', '')
  })

  it('explains when the book has no readable file', () => {
    setLibrary([])
    render(<ReadBookClient id="ub-1" />)
    expect(mockUseOfflineBookFile).toHaveBeenCalledWith(null, null, '')
    expect(screen.getByText('This book has no EPUB or PDF file.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Back to book' })).toHaveAttribute(
      'href',
      '/books/ub-1'
    )
  })

  it('shows an error when the file can neither be loaded nor downloaded', () => {
    mockUseOfflineBookFile.mockReturnValue({ error: new Error('boom') })
    render(<ReadBookClient id="ub-1" />)
    expect(screen.getByText('Failed to load book file.')).toBeInTheDocument()
    expect(screen.queryByTestId('reader')).not.toBeInTheDocument()
  })

  it('shows loading while the file loads', () => {
    mockUseOfflineBookFile.mockReturnValue({})
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

  it('keeps the open reader when the library later fails', async () => {
    const { rerender } = render(<ReadBookClient id="ub-1" />)
    expect(await screen.findByTestId('reader')).toHaveAttribute('data-file', 'readium:book-1')
    mockUseLibrary.mockReturnValue({ ...mockUseLibrary(), error: new Error('offline') })
    rerender(<ReadBookClient id="ub-1" />)
    expect(screen.getByTestId('reader')).toHaveAttribute('data-file', 'readium:book-1')
  })

  it('returns to the book page on close when there is no prior history', async () => {
    Object.defineProperty(window.history, 'length', { configurable: true, value: 1 })
    render(<ReadBookClient id="ub-1" />)
    fireEvent.click(await screen.findByRole('button', { name: 'Close reader' }))
    expect(mockRouterPush).toHaveBeenCalledWith('/books/ub-1')
  })

  it('goes back to where the reader was opened from when there is prior history', async () => {
    Object.defineProperty(window.history, 'length', { configurable: true, value: 3 })
    render(<ReadBookClient id="ub-1" />)
    fireEvent.click(await screen.findByRole('button', { name: 'Close reader' }))
    expect(mockRouterBack).toHaveBeenCalled()
    expect(mockRouterPush).not.toHaveBeenCalled()
  })

  it('resumes at the freshly read position, not the cached one', async () => {
    const mutate = setFreshReadingState({
      percent: 40,
      position: { href: 'ch2.xhtml', offset: 15, page: 0 }
    })
    render(<ReadBookClient id="ub-1" />)
    expect(mockUseReadingState).toHaveBeenCalledWith('book-1')
    expect(await screen.findByTestId('reader')).toHaveAttribute(
      'data-initial',
      JSON.stringify({ position: { href: 'ch2.xhtml', offset: 15 }, percent: 40 })
    )
    expect(mutate).toHaveBeenCalledTimes(1)
  })

  it('waits for a fresh reading state before opening', () => {
    mockUseReadingState.mockReturnValue({
      data: { state: { percent: 10 } },
      error: undefined,
      mutate: () => new Promise(() => {})
    })
    render(<ReadBookClient id="ub-1" />)
    expect(screen.getByText('Loading book…')).toBeInTheDocument()
    expect(screen.queryByTestId('reader')).not.toBeInTheDocument()
  })

  it('opens at the start when there is no reading state', async () => {
    setFreshReadingState(undefined)
    render(<ReadBookClient id="ub-1" />)
    expect(await screen.findByTestId('reader')).toHaveAttribute(
      'data-initial',
      JSON.stringify({ percent: 0 })
    )
  })

  it('falls back to the cached reading state when the fresh read fails', async () => {
    mockUseReadingState.mockReturnValue({
      data: { state: { percent: 25 } },
      error: undefined,
      mutate: () => Promise.reject(new Error('internal'))
    })
    render(<ReadBookClient id="ub-1" />)
    expect(await screen.findByTestId('reader')).toHaveAttribute(
      'data-initial',
      JSON.stringify({ percent: 25 })
    )
  })

  it('ignores a fresh read that lands after unmounting', async () => {
    let resolve: (v: unknown) => void = () => {}
    mockUseReadingState.mockReturnValue({
      data: undefined,
      error: undefined,
      mutate: () => new Promise((r) => (resolve = r))
    })
    const { unmount } = render(<ReadBookClient id="ub-1" />)
    unmount()
    await act(async () => resolve({ state: { percent: 50 } }))
    expect(screen.queryByTestId('reader')).not.toBeInTheDocument()
  })

  it('keeps the opening position when the reading state changes', async () => {
    const { rerender } = render(<ReadBookClient id="ub-1" />)
    expect(await screen.findByTestId('reader')).toHaveAttribute(
      'data-initial',
      JSON.stringify({ percent: 0 })
    )
    setFreshReadingState({ percent: 80 })
    rerender(<ReadBookClient id="ub-1" />)
    expect(screen.getByTestId('reader')).toHaveAttribute(
      'data-initial',
      JSON.stringify({ percent: 0 })
    )
  })

  it('saves reader page changes', async () => {
    render(<ReadBookClient id="ub-1" />)
    fireEvent.click(await screen.findByRole('button', { name: 'Turn page' }))
    expect(mockSave).toHaveBeenCalledWith({ fraction: 0.5, section: 2 })
  })

  it('ignores ?format=kepub and opens the original', async () => {
    mockSearchParams = new URLSearchParams('format=kepub')
    render(<ReadBookClient id="ub-1" />)
    expect(await screen.findByTestId('reader')).toHaveAttribute('data-file', 'readium:book-1')
    expect(mockUseOfflineBookFile).toHaveBeenCalledWith('book-1', 'epub', 'v-epub')
  })
})
