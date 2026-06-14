import React from 'react'
import { render, screen } from '@testing-library/react'

const mockUseGetBookFile = jest.fn()
const mockUseKEPUBStatus = jest.fn()
const mockRequestConversion = jest.fn()
const mockUseRequestKEPUBConversion = jest.fn(() => mockRequestConversion)

jest.mock('next/dynamic', () => () => {
  // Return a stub component synchronously so tests don't need async import resolution.
  const Stub = (props: { url?: unknown; epubInitOptions?: { openAs?: unknown } }) => (
    <div
      data-testid="react-reader"
      data-url={String(props.url ?? '')}
      data-open-as={String(props.epubInitOptions?.openAs ?? '')}
    />
  )
  Stub.displayName = 'ReactReaderStub'
  return Stub
})

jest.mock('@/hooks/useBacklog', () => ({
  useGetBookFile: (...args: unknown[]) => mockUseGetBookFile(...args),
  useKEPUBStatus: (...args: unknown[]) => mockUseKEPUBStatus(...args),
  useRequestKEPUBConversion: () => mockUseRequestKEPUBConversion()
}))

jest.mock('@/components/ui/dialog', () => ({
  Dialog: ({ children, open }: { children: React.ReactNode; open: boolean }) =>
    open ? <div data-testid="dialog">{children}</div> : null,
  DialogContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="dialog-content">{children}</div>
  ),
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>,
  DialogClose: ({ children }: { children: React.ReactNode; 'aria-label'?: string }) => (
    <button>{children}</button>
  )
}))

import BookPreviewDialog from '@/components/backlog/BookPreviewDialog'

const BOOK_ID = 'book-uuid-1234'
const TITLE = 'The Great Gatsby'

function setupPdfEpubMocks({
  data = null,
  error = null
}: {
  data?: { url: string } | null
  error?: Error | null
} = {}) {
  mockUseGetBookFile.mockReturnValue({ data, error })
  mockUseKEPUBStatus.mockReturnValue({ data: null })
  mockRequestConversion.mockResolvedValue({ kepubStatus: 'converting' })
}

describe('BookPreviewDialog', () => {
  beforeEach(() => {
    mockUseGetBookFile.mockReset()
    mockUseKEPUBStatus.mockReset()
    mockRequestConversion.mockReset()
    mockUseRequestKEPUBConversion.mockClear()
    setupPdfEpubMocks()
  })

  it('does not render when open is false', () => {
    render(
      <BookPreviewDialog
        bookId={BOOK_ID}
        format="pdf"
        title={TITLE}
        open={false}
        onOpenChange={jest.fn()}
      />
    )
    expect(screen.queryByTestId('dialog')).not.toBeInTheDocument()
  })

  it('shows loading state while data is not yet available', () => {
    render(
      <BookPreviewDialog
        bookId={BOOK_ID}
        format="pdf"
        title={TITLE}
        open={true}
        onOpenChange={jest.fn()}
      />
    )
    expect(screen.getByText('Loading preview...')).toBeInTheDocument()
  })

  it('shows error state when fetch fails', () => {
    mockUseGetBookFile.mockReturnValue({ data: null, error: new Error('not found') })
    render(
      <BookPreviewDialog
        bookId={BOOK_ID}
        format="pdf"
        title={TITLE}
        open={true}
        onOpenChange={jest.fn()}
      />
    )
    expect(screen.getByText('Failed to load preview.')).toBeInTheDocument()
  })

  it('renders an iframe for PDF format', () => {
    mockUseGetBookFile.mockReturnValue({
      data: { url: 'https://r2.example.com/book.pdf' },
      error: null
    })
    render(
      <BookPreviewDialog
        bookId={BOOK_ID}
        format="pdf"
        title={TITLE}
        open={true}
        onOpenChange={jest.fn()}
      />
    )
    const iframe = screen.getByTitle(`Preview: ${TITLE}`)
    expect(iframe).toBeInTheDocument()
    expect(iframe).toHaveAttribute('src', 'https://r2.example.com/book.pdf')
  })

  it('renders the EPUB reader for epub format', () => {
    mockUseGetBookFile.mockReturnValue({
      data: { url: 'https://r2.example.com/book.epub' },
      error: null
    })
    render(
      <BookPreviewDialog
        bookId={BOOK_ID}
        format="epub"
        title={TITLE}
        open={true}
        onOpenChange={jest.fn()}
      />
    )
    const reader = screen.getByTestId('react-reader')
    expect(reader).toBeInTheDocument()
    expect(reader).toHaveAttribute('data-url', 'https://r2.example.com/book.epub')
    expect(reader).toHaveAttribute('data-open-as', 'epub')
  })

  it('passes null bookId to useGetBookFile when dialog is closed', () => {
    render(
      <BookPreviewDialog
        bookId={BOOK_ID}
        format="pdf"
        title={TITLE}
        open={false}
        onOpenChange={jest.fn()}
      />
    )
    // When closed, the hook should receive null so it doesn't fetch.
    expect(mockUseGetBookFile).toHaveBeenCalledWith(null, null)
  })

  it('passes bookId and format to useGetBookFile when dialog is open', () => {
    render(
      <BookPreviewDialog
        bookId={BOOK_ID}
        format="epub"
        title={TITLE}
        open={true}
        onOpenChange={jest.fn()}
      />
    )
    expect(mockUseGetBookFile).toHaveBeenCalledWith(BOOK_ID, 'epub')
  })

  it('shows the book title in the dialog header', () => {
    render(
      <BookPreviewDialog
        bookId={BOOK_ID}
        format="pdf"
        title={TITLE}
        open={true}
        onOpenChange={jest.fn()}
      />
    )
    expect(screen.getByText(TITLE)).toBeInTheDocument()
  })

  describe('KEPUB on-demand conversion (format="kepub")', () => {
    it('shows converting state while KEPUB is not ready', () => {
      mockUseKEPUBStatus.mockReturnValue({ data: { kepubStatus: 'converting' } })
      mockUseGetBookFile.mockReturnValue({ data: null, error: null })
      render(
        <BookPreviewDialog
          bookId={BOOK_ID}
          format="kepub"
          title={TITLE}
          open={true}
          onOpenChange={jest.fn()}
        />
      )
      expect(screen.getByText('Converting... this may take a moment.')).toBeInTheDocument()
    })

    it('shows failed state when conversion fails', () => {
      mockUseKEPUBStatus.mockReturnValue({ data: { kepubStatus: 'failed' } })
      mockUseGetBookFile.mockReturnValue({ data: null, error: null })
      render(
        <BookPreviewDialog
          bookId={BOOK_ID}
          format="kepub"
          title={TITLE}
          open={true}
          onOpenChange={jest.fn()}
        />
      )
      expect(screen.getByText('Conversion failed. Cannot preview EPUB.')).toBeInTheDocument()
    })

    it('renders EPUB reader once KEPUB is ready and URL is fetched', () => {
      mockUseKEPUBStatus.mockReturnValue({ data: { kepubStatus: 'ready' } })
      mockUseGetBookFile.mockReturnValue({
        data: { url: 'https://r2.example.com/book.kepub' },
        error: null
      })
      render(
        <BookPreviewDialog
          bookId={BOOK_ID}
          format="kepub"
          title={TITLE}
          open={true}
          onOpenChange={jest.fn()}
        />
      )
      const reader = screen.getByTestId('react-reader')
      expect(reader).toBeInTheDocument()
      expect(reader).toHaveAttribute('data-url', 'https://r2.example.com/book.kepub')
      expect(reader).toHaveAttribute('data-open-as', 'epub')
    })

    it('gates file fetch on KEPUB ready status — passes null format while converting', () => {
      mockUseKEPUBStatus.mockReturnValue({ data: { kepubStatus: 'converting' } })
      mockUseGetBookFile.mockReturnValue({ data: null, error: null })
      render(
        <BookPreviewDialog
          bookId={BOOK_ID}
          format="kepub"
          title={TITLE}
          open={true}
          onOpenChange={jest.fn()}
        />
      )
      // format is null until KEPUB is ready, so no file fetch fires
      expect(mockUseGetBookFile).toHaveBeenCalledWith(BOOK_ID, null)
    })

    it('passes bookId and kepub format to useGetBookFile when KEPUB is ready', () => {
      mockUseKEPUBStatus.mockReturnValue({ data: { kepubStatus: 'ready' } })
      mockUseGetBookFile.mockReturnValue({ data: null, error: null })
      render(
        <BookPreviewDialog
          bookId={BOOK_ID}
          format="kepub"
          title={TITLE}
          open={true}
          onOpenChange={jest.fn()}
        />
      )
      expect(mockUseGetBookFile).toHaveBeenCalledWith(BOOK_ID, 'kepub')
    })

    it('polls useKEPUBStatus with bookId when format is kepub and dialog is open', () => {
      mockUseKEPUBStatus.mockReturnValue({ data: { kepubStatus: 'converting' } })
      mockUseGetBookFile.mockReturnValue({ data: null, error: null })
      render(
        <BookPreviewDialog
          bookId={BOOK_ID}
          format="kepub"
          title={TITLE}
          open={true}
          onOpenChange={jest.fn()}
        />
      )
      expect(mockUseKEPUBStatus).toHaveBeenCalledWith(BOOK_ID)
    })

    it('passes null to useKEPUBStatus when dialog is closed', () => {
      mockUseKEPUBStatus.mockReturnValue({ data: null })
      mockUseGetBookFile.mockReturnValue({ data: null, error: null })
      render(
        <BookPreviewDialog
          bookId={BOOK_ID}
          format="kepub"
          title={TITLE}
          open={false}
          onOpenChange={jest.fn()}
        />
      )
      expect(mockUseKEPUBStatus).toHaveBeenCalledWith(null)
    })
  })
})
